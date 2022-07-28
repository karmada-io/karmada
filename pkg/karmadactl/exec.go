package karmadactl

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubectlexec "k8s.io/kubectl/pkg/cmd/exec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
)

const (
	defaultPodExecTimeout = 60 * time.Second
)

var (
	execExample = templates.Examples(`
		# Get output from running the 'date' command from pod mypod, using the first container by default in cluster(member1)
		%[1]s exec mypod -C=member1 -- date
	
		# Get output from running the 'date' command in ruby-container from pod mypod in cluster(member1)
		%[1]s exec mypod -c ruby-container -C=member1 -- date
	
		# Get output from running the 'date' command in ruby-container from pod mypod in cluster(member1)
		%[1]sexec mypod -c ruby-container -C=member1 -- date
		
		# Switch to raw terminal mode; sends stdin to 'bash' in ruby-container from pod mypod in cluster(member1)
		# and sends stdout/stderr from 'bash' back to the client
		%[1]s exec mypod -c ruby-container -C=member1 -i -t -- bash -il
	
		# Get output from running 'date' command from the first pod of the deployment mydeployment, using the first container by default in cluster(member1)
		%[1]s exec deploy/mydeployment -C=member1 -- date
	
		# Get output from running 'date' command from the first pod of the service myservice, using the first container by default in cluster(member1)
		%[1]s exec svc/myservice -C=member1 -- date`)
)

// NewCmdExec new exec command.
func NewCmdExec(karmadaConfig KarmadaConfig, parentCommand string, streams genericclioptions.IOStreams) *cobra.Command {
	o := &ExecOptions{
		KubectlExecOptions: &kubectlexec.ExecOptions{
			StreamOptions: kubectlexec.StreamOptions{
				IOStreams: streams,
			},
			Executor: &kubectlexec.DefaultRemoteExecutor{},
		},
	}

	cmd := &cobra.Command{
		Use:     "exec (POD | TYPE/NAME) [-c CONTAINER] [flags] (-C CLUSTER) -- COMMAND [args...]",
		Short:   "Execute a command in a container in a cluster",
		Long:    "Execute a command in a container in a cluster",
		Example: fmt.Sprintf(execExample, parentCommand),
		RunE: func(cmd *cobra.Command, args []string) error {
			argsLenAtDash := cmd.ArgsLenAtDash()
			if err := o.Complete(karmadaConfig, cmd, args, argsLenAtDash); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
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
	cmdutil.AddJsonFilenameFlag(cmd.Flags(), &o.KubectlExecOptions.FilenameOptions.Filenames, "to use to exec into the resource")
	cmdutil.AddContainerVarFlags(cmd, &o.KubectlExecOptions.ContainerName, o.KubectlExecOptions.ContainerName)

	cmd.Flags().BoolVarP(&o.KubectlExecOptions.Stdin, "stdin", "i", o.KubectlExecOptions.Stdin, "Pass stdin to the container")
	cmd.Flags().BoolVarP(&o.KubectlExecOptions.TTY, "tty", "t", o.KubectlExecOptions.TTY, "Stdin is a TTY")
	cmd.Flags().BoolVarP(&o.KubectlExecOptions.Quiet, "quiet", "q", o.KubectlExecOptions.Quiet, "Only print output from the remote session")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "If present, the namespace scope for this CLI request")
	cmd.Flags().StringVarP(&o.Cluster, "cluster", "C", "", "Specify a member cluster")
	return cmd
}

// ExecOptions declare the arguments accepted by the Exec command
type ExecOptions struct {
	// global flags
	options.GlobalCommandOptions
	// flags specific to exec
	KubectlExecOptions *kubectlexec.ExecOptions
	Namespace          string
	Cluster            string
}

// Complete verifies command line arguments and loads data from the command environment
func (o *ExecOptions) Complete(karmadaConfig KarmadaConfig, cmd *cobra.Command, argsIn []string, argsLenAtDash int) error {
	if len(o.Cluster) == 0 {
		return fmt.Errorf("must specify a cluster")
	}

	karmadaRestConfig, err := karmadaConfig.GetRestConfig(o.KarmadaContext, o.KubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			o.KarmadaContext, o.KubeConfig, err)
	}

	clusterInfo, err := getClusterInfo(karmadaRestConfig, o.Cluster, o.KubeConfig, o.KarmadaContext)
	if err != nil {
		return err
	}

	f := getFactory(o.Cluster, clusterInfo, o.Namespace)
	return o.KubectlExecOptions.Complete(f, cmd, argsIn, argsLenAtDash)
}

// Validate checks that the provided exec options are specified.
func (o *ExecOptions) Validate() error {
	return o.KubectlExecOptions.Validate()
}

// Run executes a validated remote execution against a pod.
func (o *ExecOptions) Run() error {
	return o.KubectlExecOptions.Run()
}
