package karmadactl

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	dockerterm "github.com/moby/term"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/podcmd"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/interrupt"
	"k8s.io/kubectl/pkg/util/term"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
)

const (
	defaultPodExecTimeout = 60 * time.Second
)

// NewCmdExec new exec command.
func NewCmdExec(karmadaConfig KarmadaConfig, parentCommand string) *cobra.Command {
	ioStreams := genericclioptions.IOStreams{In: getIn, Out: getOut, ErrOut: getErr}
	o := &ExecOptions{
		streamOptions: streamOptions{
			IOStreams: ioStreams,
		},

		Executor: &DefaultRemoteExecutor{},
	}
	cmd := &cobra.Command{
		Use:                   "exec (POD | TYPE/NAME) [-c CONTAINER] [flags] (-C CLUSTER) -- COMMAND [args...]",
		DisableFlagsInUseLine: true,
		Short:                 "Execute a command in a container in a cluster",
		Long:                  "Execute a command in a container in a cluster",
		Example:               execExample(parentCommand),
		RunE: func(cmd *cobra.Command, args []string) error {
			argsLenAtDash := cmd.ArgsLenAtDash()
			if err := o.Complete(karmadaConfig, cmd, args, argsLenAtDash); err != nil {
				return err
			}
			if err := o.Validate(cmd); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	o.GlobalCommandOptions.AddFlags(cmd.Flags())

	cmdutil.AddPodRunningTimeoutFlag(cmd, defaultPodExecTimeout)
	cmdutil.AddJsonFilenameFlag(cmd.Flags(), &o.FilenameOptions.Filenames, "to use to exec into the resource")
	cmd.Flags().StringVarP(&o.Cluster, "cluster", "C", "", "Specify a member cluster")
	cmdutil.AddContainerVarFlags(cmd, &o.ContainerName, o.ContainerName)
	cmd.Flags().BoolVarP(&o.Stdin, "stdin", "i", o.Stdin, "Pass stdin to the container")
	cmd.Flags().BoolVarP(&o.TTY, "tty", "t", o.TTY, "Stdin is a TTY")
	cmd.Flags().BoolVarP(&o.Quiet, "quiet", "q", o.Quiet, "Only print output from the remote session")
	return cmd
}

func execExample(parentCommand string) string {
	example := `
# Get output from running the 'date' command from pod mypod, using the first container by default in cluster(member1)` + "\n" +
		fmt.Sprintf("%s exec mypod -C=member1 -- date", parentCommand) + `

# Get output from running the 'date' command in ruby-container from pod mypod in cluster(member1)` + "\n" +
		fmt.Sprintf("%s exec mypod -c ruby-container -C=member1 -- date", parentCommand) + `

# Switch to raw terminal mode; sends stdin to 'bash' in ruby-container from pod mypod in cluster(member1)
# and sends stdout/stderr from 'bash' back to the client` + "\n" +
		fmt.Sprintf("%s exec mypod -c ruby-container -C=member1 -i -t -- bash -il", parentCommand) + `

# Get output from running 'date' command from the first pod of the deployment mydeployment, using the first container by default in cluster(member1)` + "\n" +
		fmt.Sprintf("%s exec deploy/mydeployment -C=member1 -- date", parentCommand) + `

# Get output from running 'date' command from the first pod of the service myservice, using the first container by default in cluster(member1)` + "\n" +
		fmt.Sprintf("%s exec svc/myservice -C=member1 -- date", parentCommand)
	return example
}

// RemoteExecutor defines the interface accepted by the Exec command - provided for test stubbing
type RemoteExecutor interface {
	Execute(method string, url *url.URL, config *restclient.Config, stdin io.Reader, stdout, stderr io.Writer, tty bool, terminalSizeQueue remotecommand.TerminalSizeQueue) error
}

// DefaultRemoteExecutor is the standard implementation of remote command execution
type DefaultRemoteExecutor struct{}

// Execute implement RemoteExecutor interface
func (*DefaultRemoteExecutor) Execute(method string, url *url.URL, config *restclient.Config, stdin io.Reader, stdout, stderr io.Writer, tty bool, terminalSizeQueue remotecommand.TerminalSizeQueue) error {
	exec, err := remotecommand.NewSPDYExecutor(config, method, url)
	if err != nil {
		return err
	}
	return exec.Stream(remotecommand.StreamOptions{
		Stdin:             stdin,
		Stdout:            stdout,
		Stderr:            stderr,
		Tty:               tty,
		TerminalSizeQueue: terminalSizeQueue,
	})
}

type streamOptions struct {
	Cluster       string
	Namespace     string
	PodName       string
	ContainerName string
	Stdin         bool
	TTY           bool
	// minimize unnecessary output
	Quiet bool
	// InterruptParent, if set, is used to handle interrupts while attached
	InterruptParent *interrupt.Handler

	genericclioptions.IOStreams

	// for testing
	overrideStreams func() (io.ReadCloser, io.Writer, io.Writer)
	isTerminalIn    func(t term.TTY) bool
}

// ExecOptions declare the arguments accepted by the Exec command
type ExecOptions struct {
	// global flags
	options.GlobalCommandOptions

	streamOptions
	resource.FilenameOptions

	ResourceName     string
	Command          []string
	EnforceNamespace bool

	Builder          func() *resource.Builder
	ExecutablePodFn  polymorphichelpers.AttachablePodForObjectFunc
	restClientGetter genericclioptions.RESTClientGetter

	Pod           *corev1.Pod
	Executor      RemoteExecutor
	PodClient     coreclient.PodsGetter
	GetPodTimeout time.Duration
	Config        *restclient.Config
}

// Complete verifies command line arguments and loads data from the command environment
func (p *ExecOptions) Complete(karmadaConfig KarmadaConfig, cmd *cobra.Command, argsIn []string, argsLenAtDash int) error {
	if len(argsIn) > 0 && argsLenAtDash != 0 {
		p.ResourceName = argsIn[0]
	}
	if argsLenAtDash > -1 {
		p.Command = argsIn[argsLenAtDash:]
	} else if len(argsIn) > 1 {
		if !p.Quiet {
			fmt.Fprint(p.ErrOut, "kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.\n")
		}
		p.Command = argsIn[1:]
	} else if len(argsIn) > 0 && len(p.FilenameOptions.Filenames) != 0 {
		if !p.Quiet {
			fmt.Fprint(p.ErrOut, "kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.\n")
		}
		p.Command = argsIn[0:]
		p.ResourceName = ""
	}

	var err error

	if len(p.Cluster) == 0 {
		return fmt.Errorf("must specify a cluster")
	}

	karmadaRestConfig, err := karmadaConfig.GetRestConfig(p.KarmadaContext, p.KubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			p.KarmadaContext, p.KubeConfig, err)
	}

	clusterInfo, err := getClusterInfo(karmadaRestConfig, p.Cluster, p.KubeConfig, p.KarmadaContext)
	if err != nil {
		return err
	}

	f := getFactory(p.Cluster, clusterInfo)

	p.Namespace, p.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	p.ExecutablePodFn = polymorphichelpers.AttachablePodForObjectFn

	p.Builder = f.NewBuilder
	p.restClientGetter = f

	p.Config, err = f.ToRESTConfig()
	if err != nil {
		return err
	}

	clientset, err := f.KubernetesClientSet()
	if err != nil {
		return err
	}
	p.PodClient = clientset.CoreV1()

	return nil
}

// Validate checks that the provided exec options are specified.
func (p *ExecOptions) Validate(cmd *cobra.Command) error {
	if len(p.PodName) == 0 && len(p.ResourceName) == 0 && len(p.FilenameOptions.Filenames) == 0 {
		return fmt.Errorf("pod, type/name or --filename must be specified")
	}
	if len(p.Command) == 0 {
		return fmt.Errorf("you must specify at least one command for the container")
	}
	if p.Out == nil || p.ErrOut == nil {
		return fmt.Errorf("both output and error output must be provided")
	}

	var err error
	p.GetPodTimeout, err = cmdutil.GetPodRunningTimeoutFlag(cmd)
	if err != nil {
		return cmdutil.UsageErrorf(cmd, err.Error())
	}
	return nil
}

func (o *streamOptions) setupTTY() term.TTY {
	t := term.TTY{
		Parent: o.InterruptParent,
		Out:    o.Out,
	}

	if !o.Stdin {
		// need to nil out o.In to make sure we don't create a stream for stdin
		o.In = nil
		o.TTY = false
		return t
	}

	t.In = o.In
	if !o.TTY {
		return t
	}

	if o.isTerminalIn == nil {
		o.isTerminalIn = func(tty term.TTY) bool {
			return tty.IsTerminalIn()
		}
	}
	if !o.isTerminalIn(t) {
		o.TTY = false

		if !o.Quiet && o.ErrOut != nil {
			fmt.Fprintln(o.ErrOut, "Unable to use a TTY - input is not a terminal or the right kind of file")
		}

		return t
	}

	// if we get to here, the user wants to attach stdin, wants a TTY, and o.In is a terminal, so we
	// can safely set t.Raw to true
	t.Raw = true

	if o.overrideStreams == nil {
		// use dockerterm.StdStreams() to get the right I/O handles on Windows
		o.overrideStreams = dockerterm.StdStreams
	}
	stdin, stdout, _ := o.overrideStreams()
	o.In = stdin
	t.In = stdin
	if o.Out != nil {
		o.Out = stdout
		t.Out = stdout
	}

	return t
}

// Run executes a validated remote execution against a pod.
func (p *ExecOptions) Run() error {
	var err error
	// we still need legacy pod getter when PodName in ExecOptions struct is provided,
	// since there are any other command run this function by providing Podname with PodsGetter
	// and without resource builder, eg: `kubectl cp`.
	if len(p.PodName) != 0 {
		p.Pod, err = p.PodClient.Pods(p.Namespace).Get(context.TODO(), p.PodName, metav1.GetOptions{})
		if err != nil {
			return err
		}
	} else {
		builder := p.Builder().
			WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
			FilenameParam(p.EnforceNamespace, &p.FilenameOptions).
			NamespaceParam(p.Namespace).DefaultNamespace()
		if len(p.ResourceName) > 0 {
			builder = builder.ResourceNames("pods", p.ResourceName)
		}

		obj, err := builder.Do().Object()
		if err != nil {
			return err
		}

		p.Pod, err = p.ExecutablePodFn(p.restClientGetter, obj, p.GetPodTimeout)
		if err != nil {
			return err
		}
	}

	pod := p.Pod

	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return fmt.Errorf("cannot exec into a container in a completed pod; current phase is %s", pod.Status.Phase)
	}

	containerName := p.ContainerName
	if len(containerName) == 0 {
		container, err := podcmd.FindOrDefaultContainerByName(pod, containerName, p.Quiet, p.ErrOut)
		if err != nil {
			return err
		}
		containerName = container.Name
	}

	// ensure we can recover the terminal while attached
	t := p.setupTTY()

	var sizeQueue remotecommand.TerminalSizeQueue
	if t.Raw {
		// this call spawns a goroutine to monitor/update the terminal size
		sizeQueue = t.MonitorSize(t.GetSize())

		// unset p.Err if it was previously set because both stdout and stderr go over p.Out when tty is
		// true
		p.ErrOut = nil
	}

	fn := func() error {
		restClient, err := restclient.RESTClientFor(p.Config)
		if err != nil {
			return err
		}

		// TODO: consider abstracting into a client invocation or client helper
		req := restClient.Post().
			Resource("pods").
			Name(pod.Name).
			Namespace(pod.Namespace).
			SubResource("exec")
		req.VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   p.Command,
			Stdin:     p.Stdin,
			Stdout:    p.Out != nil,
			Stderr:    p.ErrOut != nil,
			TTY:       t.Raw,
		}, scheme.ParameterCodec)

		return p.Executor.Execute("POST", req.URL(), p.Config, p.In, p.Out, p.ErrOut, t.Raw, sizeQueue)
	}

	if err := t.Safe(fn); err != nil {
		return err
	}

	return nil
}
