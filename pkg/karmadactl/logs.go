package karmadactl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/polymorphichelpers"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

const (
	defaultPodLogsTimeout = 20 * time.Second
	logsUsageStr          = "logs [-f] [-p] (POD | TYPE/NAME) [-c CONTAINER] (-C CLUSTER)"
)

var (
	selectorTail    int64 = 10
	logsUsageErrStr       = fmt.Sprintf("expected '%s'.\nPOD or TYPE/NAME is a required argument for the logs command", logsUsageStr)
)

// NewCmdLogs new logs command.
func NewCmdLogs(out io.Writer, karmadaConfig KarmadaConfig, parentCommand string) *cobra.Command {
	ioStreams := genericclioptions.IOStreams{In: getIn, Out: getOut, ErrOut: getErr}
	o := NewCommandLogsOptions(ioStreams, false)
	cmd := &cobra.Command{
		Use:          logsUsageStr,
		Short:        "Print the logs for a container in a pod in a cluster",
		SilenceUsage: true,
		Example:      logsExample(parentCommand),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(karmadaConfig, cmd, args); err != nil {
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
	o.AddFlags(cmd)

	return cmd
}

func logsExample(parentCommand string) string {
	example := `
# Return snapshot logs from pod nginx with only one container in cluster(member1)` + "\n" +
		fmt.Sprintf("%s logs nginx -C=member1", parentCommand) + `

# Return snapshot logs from pod nginx with multi containers in cluster(member1)` + "\n" +
		fmt.Sprintf("%s logs nginx --all-containers=true -C=member1", parentCommand) + `

# Return snapshot logs from all containers in pods defined by label app=nginx in cluster(member1)` + "\n" +
		fmt.Sprintf("%s logs -l app=nginx --all-containers=true -C=member1", parentCommand) + `

# Return snapshot of previous terminated ruby container logs from pod web-1 in cluster(member1)` + "\n" +
		fmt.Sprintf("%s logs -p -c ruby web-1 -C=member1", parentCommand) + `

# Begin streaming the logs of the ruby container in pod web-1 in cluster(member1)` + "\n" +
		fmt.Sprintf("%s logs -f -c ruby web-1 -C=member1", parentCommand) + `

# Begin streaming the logs from all containers in pods defined by label app=nginx in cluster(member1) ` + "\n" +
		fmt.Sprintf("%s logs -f -l app=nginx --all-containers=true -C=member1", parentCommand) + `

# Display only the most recent 20 lines of output in pod nginx in cluster(member1) ` + "\n" +
		fmt.Sprintf("%s logs --tail=20 nginx -C=member1", parentCommand) + `

# Show all logs from pod nginx written in the last hour in cluster(member1) ` + "\n" +
		fmt.Sprintf("%s logs --since=1h nginx -C=member1", parentCommand)
	return example
}

// CommandLogsOptions contains the input to the logs command.
type CommandLogsOptions struct {
	// global flags
	options.GlobalCommandOptions

	Cluster       string
	Namespace     string
	ResourceArg   string
	AllContainers bool
	Options       runtime.Object
	Resources     []string

	ConsumeRequestFn func(rest.ResponseWrapper, io.Writer) error

	// PodLogOptions
	SinceTime                    string
	Since                        time.Duration
	Follow                       bool
	Previous                     bool
	Timestamps                   bool
	IgnoreLogErrors              bool
	LimitBytes                   int64
	Tail                         int64
	Container                    string
	InsecureSkipTLSVerifyBackend bool

	// whether or not a container name was given via --container
	ContainerNameSpecified bool
	Selector               string
	MaxFollowConcurrency   int
	Prefix                 bool

	Object           runtime.Object
	GetPodTimeout    time.Duration
	RESTClientGetter genericclioptions.RESTClientGetter
	LogsForObject    polymorphichelpers.LogsForObjectFunc

	genericclioptions.IOStreams

	TailSpecified bool

	containerNameFromRefSpecRegexp *regexp.Regexp

	f cmdutil.Factory
}

// NewCommandLogsOptions returns a LogsOptions.
func NewCommandLogsOptions(streams genericclioptions.IOStreams, allContainers bool) *CommandLogsOptions {
	return &CommandLogsOptions{
		IOStreams:            streams,
		AllContainers:        allContainers,
		Tail:                 -1,
		MaxFollowConcurrency: 5,

		containerNameFromRefSpecRegexp: regexp.MustCompile(`spec\.(?:initContainers|containers|ephemeralContainers){(.+)}`),
	}
}

// AddFlags adds flags to the specified FlagSet.
func (o *CommandLogsOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&o.AllContainers, "all-containers", o.AllContainers, "Get all containers' logs in the pod(s).")
	cmd.Flags().BoolVarP(&o.Follow, "follow", "f", o.Follow, "Specify if the logs should be streamed.")
	cmd.Flags().BoolVar(&o.Timestamps, "timestamps", o.Timestamps, "Include timestamps on each line in the log output")
	cmd.Flags().Int64Var(&o.LimitBytes, "limit-bytes", o.LimitBytes, "Maximum bytes of logs to return. Defaults to no limit.")
	cmd.Flags().BoolVarP(&o.Previous, "previous", "p", o.Previous, "If true, print the logs for the previous instance of the container in a pod if it exists.")
	cmd.Flags().Int64Var(&o.Tail, "tail", o.Tail, "Lines of recent log file to display. Defaults to -1 with no selector, showing all log lines otherwise 10, if a selector is provided.")
	cmd.Flags().BoolVar(&o.IgnoreLogErrors, "ignore-errors", o.IgnoreLogErrors, "If watching / following pod logs, allow for any errors that occur to be non-fatal")
	cmd.Flags().StringVar(&o.SinceTime, "since-time", o.SinceTime, "Only return logs after a specific date (RFC3339). Defaults to all logs. Only one of since-time / since may be used.")
	cmd.Flags().DurationVar(&o.Since, "since", o.Since, "Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs. Only one of since-time / since may be used.")
	cmd.Flags().StringVarP(&o.Container, "container", "c", o.Container, "Print the logs of this container")
	cmd.Flags().BoolVar(&o.InsecureSkipTLSVerifyBackend, "insecure-skip-tls-verify-backend", o.InsecureSkipTLSVerifyBackend,
		"Skip verifying the identity of the kubelet that logs are requested from.  In theory, an attacker could provide invalid log content back. You might want to use this if your kubelet serving certificates have expired.")
	cmdutil.AddPodRunningTimeoutFlag(cmd, defaultPodLogsTimeout)
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on.")
	cmd.Flags().IntVar(&o.MaxFollowConcurrency, "max-log-requests", o.MaxFollowConcurrency, "Specify maximum number of concurrent logs to follow when using by a selector. Defaults to 5.")
	cmd.Flags().BoolVar(&o.Prefix, "prefix", o.Prefix, "Prefix each log line with the log source (pod name and container name)")

	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "default", "-n=namespace or -n namespace")
	cmd.Flags().StringVarP(&o.Cluster, "cluster", "C", "", "Specify a member cluster")
}

// Complete ensures that options are valid and marshals them if necessary
func (o *CommandLogsOptions) Complete(karmadaConfig KarmadaConfig, cmd *cobra.Command, args []string) error {
	o.ContainerNameSpecified = cmd.Flag("container").Changed
	o.TailSpecified = cmd.Flag("tail").Changed
	o.Resources = args

	switch len(args) {
	case 0:
		if len(o.Selector) == 0 {
			return cmdutil.UsageErrorf(cmd, "%s", logsUsageErrStr)
		}
	case 1:
		o.ResourceArg = args[0]
		if len(o.Selector) != 0 {
			return cmdutil.UsageErrorf(cmd, "only a selector (-l) or a POD name is allowed")
		}
	case 2:
		o.ResourceArg = args[0]
		o.Container = args[1]
	default:
		return cmdutil.UsageErrorf(cmd, "%s", logsUsageErrStr)
	}
	var err error

	o.ConsumeRequestFn = lifted.DefaultConsumeRequest

	o.GetPodTimeout, err = cmdutil.GetPodRunningTimeoutFlag(cmd)
	if err != nil {
		return err
	}

	o.Options, err = o.toLogOptions()
	if err != nil {
		return err
	}

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

	o.f = getFactory(o.Cluster, clusterInfo)

	o.RESTClientGetter = o.f
	o.LogsForObject = polymorphichelpers.LogsForObjectFn

	if err := o.completeObj(); err != nil {
		return err
	}

	return nil
}

// Validate checks to the LogsOptions to see if there is sufficient information run the command
func (o *CommandLogsOptions) Validate() error {
	if len(o.SinceTime) > 0 && o.Since != 0 {
		return fmt.Errorf("at most one of `sinceTime` or `sinceSeconds` may be specified")
	}

	logsOptions, ok := o.Options.(*corev1.PodLogOptions)
	if !ok {
		return errors.New("unexpected logs options object")
	}
	if o.AllContainers && len(logsOptions.Container) > 0 {
		return fmt.Errorf("--all-containers=true should not be specified with container name %s", logsOptions.Container)
	}

	if o.ContainerNameSpecified && len(o.Resources) == 2 {
		return fmt.Errorf("only one of -c or an inline [CONTAINER] arg is allowed")
	}

	if o.LimitBytes < 0 {
		return fmt.Errorf("--limit-bytes must be greater than 0")
	}

	if logsOptions.SinceSeconds != nil && *logsOptions.SinceSeconds < int64(0) {
		return fmt.Errorf("--since must be greater than 0")
	}

	if logsOptions.TailLines != nil && *logsOptions.TailLines < -1 {
		return fmt.Errorf("--tail must be greater than or equal to -1")
	}

	return nil
}

// Run retrieves a pod log
func (o *CommandLogsOptions) Run() error {
	requests, err := o.LogsForObject(o.RESTClientGetter, o.Object, o.Options, o.GetPodTimeout, o.AllContainers)
	if err != nil {
		return err
	}

	if o.Follow && len(requests) > 1 {
		if len(requests) > o.MaxFollowConcurrency {
			return fmt.Errorf(
				"you are attempting to follow %d log streams, but maximum allowed concurrency is %d, use --max-log-requests to increase the limit",
				len(requests), o.MaxFollowConcurrency,
			)
		}

		return o.parallelConsumeRequest(requests)
	}

	return o.sequentialConsumeRequest(requests)
}

func (o *CommandLogsOptions) parallelConsumeRequest(requests map[corev1.ObjectReference]rest.ResponseWrapper) error {
	reader, writer := io.Pipe()
	wg := &sync.WaitGroup{}
	wg.Add(len(requests))
	for objRef, request := range requests {
		go func(objRef corev1.ObjectReference, request rest.ResponseWrapper) {
			defer wg.Done()
			out := o.addPrefixIfNeeded(objRef, writer)
			if err := o.ConsumeRequestFn(request, out); err != nil {
				if !o.IgnoreLogErrors {
					_ = writer.CloseWithError(err)

					// It's important to return here to propagate the error via the pipe
					return
				}

				fmt.Fprintf(writer, "error: %v\n", err)
			}
		}(objRef, request)
	}

	go func() {
		wg.Wait()
		writer.Close()
	}()

	_, err := io.Copy(o.Out, reader)
	return err
}

func (o *CommandLogsOptions) completeObj() error {
	if o.Object == nil {
		builder := o.f.NewBuilder().
			WithScheme(gclient.NewSchema(), gclient.NewSchema().PrioritizedVersionsAllGroups()...).
			NamespaceParam(o.Namespace).DefaultNamespace().
			SingleResourceType()
		if o.ResourceArg != "" {
			builder.ResourceNames("pods", o.ResourceArg)
		}
		if o.Selector != "" {
			builder.ResourceTypes("pods").LabelSelectorParam(o.Selector)
		}
		infos, err := builder.Do().Infos()
		if err != nil {
			return err
		}
		if o.Selector == "" && len(infos) != 1 {
			return errors.New("expected a resource")
		}
		o.Object = infos[0].Object
		if o.Selector != "" && len(o.Object.(*corev1.PodList).Items) == 0 {
			fmt.Fprintf(o.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
		}
	}

	return nil
}

func (o *CommandLogsOptions) toLogOptions() (*corev1.PodLogOptions, error) {
	logOptions := &corev1.PodLogOptions{
		Container:                    o.Container,
		Follow:                       o.Follow,
		Previous:                     o.Previous,
		Timestamps:                   o.Timestamps,
		InsecureSkipTLSVerifyBackend: o.InsecureSkipTLSVerifyBackend,
	}

	if len(o.SinceTime) > 0 {
		t, err := lifted.ParseRFC3339(o.SinceTime, metav1.Now)
		if err != nil {
			return nil, err
		}

		logOptions.SinceTime = &t
	}

	if o.LimitBytes != 0 {
		logOptions.LimitBytes = &o.LimitBytes
	}

	if o.Since != 0 {
		// round up to the nearest second
		sec := int64(o.Since.Round(time.Second))
		logOptions.SinceSeconds = &sec
	}

	if len(o.Selector) > 0 && o.Tail == -1 && !o.TailSpecified {
		logOptions.TailLines = &selectorTail
	} else if o.Tail != -1 {
		logOptions.TailLines = &o.Tail
	}

	return logOptions, nil
}

func (o *CommandLogsOptions) sequentialConsumeRequest(requests map[corev1.ObjectReference]rest.ResponseWrapper) error {
	for objRef, request := range requests {
		out := o.addPrefixIfNeeded(objRef, o.Out)
		if err := o.ConsumeRequestFn(request, out); err != nil {
			if !o.IgnoreLogErrors {
				return err
			}

			fmt.Fprintf(o.Out, "error: %v\n", err)
		}
	}

	return nil
}

func (o *CommandLogsOptions) addPrefixIfNeeded(ref corev1.ObjectReference, writer io.Writer) io.Writer {
	if !o.Prefix || ref.FieldPath == "" || ref.Name == "" {
		return writer
	}

	// We rely on ref.FieldPath to contain a reference to a container
	// including a container name (not an index) so we can get a container name
	// without making an extra API request.
	var containerName string
	containerNameMatches := o.containerNameFromRefSpecRegexp.FindStringSubmatch(ref.FieldPath)
	if len(containerNameMatches) == 2 {
		containerName = containerNameMatches[1]
	}

	prefix := fmt.Sprintf("[pod/%s/%s] ", ref.Name, containerName)
	return &prefixingWriter{
		prefix: []byte(prefix),
		writer: writer,
	}
}

// getClusterInfo get information of cluster
func getClusterInfo(karmadaRestConfig *rest.Config, clusterName, kubeConfig, karmadaContext string) (map[string]*ClusterInfo, error) {
	clusterClient := karmadaclientset.NewForConfigOrDie(karmadaRestConfig).ClusterV1alpha1().Clusters()

	// check if the cluster exist in karmada control plane
	_, err := clusterClient.Get(context.TODO(), clusterName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	clusterInfos := make(map[string]*ClusterInfo)
	clusterInfos[clusterName] = &ClusterInfo{}

	clusterInfos[clusterName].APIEndpoint = karmadaRestConfig.Host + fmt.Sprintf(proxyURL, clusterName)
	clusterInfos[clusterName].KubeConfig = kubeConfig
	clusterInfos[clusterName].Context = karmadaContext
	if clusterInfos[clusterName].KubeConfig == "" {
		env := os.Getenv("KUBECONFIG")
		if env != "" {
			clusterInfos[clusterName].KubeConfig = env
		} else {
			clusterInfos[clusterName].KubeConfig = defaultKubeConfig
		}
	}

	return clusterInfos, nil
}

type prefixingWriter struct {
	prefix []byte
	writer io.Writer
}

func (pw *prefixingWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	// Perform an "atomic" write of a prefix and p to make sure that it doesn't interleave
	// sub-line when used concurrently with io.PipeWrite.
	n, err := pw.writer.Write(append(pw.prefix, p...))
	if n > len(p) {
		// To comply with the io.Writer interface requirements we must
		// return a number of bytes written from p (0 <= n <= len(p)),
		// so we are ignoring the length of the prefix here.
		return len(p), err
	}
	return n, err
}
