package karmadactl

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	kubectllogs "k8s.io/kubectl/pkg/cmd/logs"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
)

const (
	logsUsageStr = "logs [-f] [-p] (POD | TYPE/NAME) [-c CONTAINER] (-C CLUSTER)"
)

var (
	logsUsageErrStr = fmt.Sprintf("expected '%s'.\nPOD or TYPE/NAME is a required argument for the logs command", logsUsageStr)
	logsExample     = templates.Examples(`
		# Return snapshot logs from pod nginx with only one container in cluster(member1)
		%[1]s logs nginx -C=member1
	
		# Return snapshot logs from pod nginx with multi containers in cluster(member1)
		%[1]s logs nginx --all-containers=true -C=member1
	
		# Return snapshot logs from all containers in pods defined by label app=nginx in cluster(member1)
		%[1]s logs -l app=nginx --all-containers=true -C=member1
	
		# Return snapshot of previous terminated ruby container logs from pod web-1 in cluster(member1)
		%[1]s logs -p -c ruby web-1 -C=member1
	
		# Begin streaming the logs of the ruby container in pod web-1 in cluster(member1)
		%[1]s logs -f -c ruby web-1 -C=member1
	
		# Begin streaming the logs from all containers in pods defined by label app=nginx in cluster(member1)
		%[1]s logs -f -l app=nginx --all-containers=true -C=member1
	
		# Display only the most recent 20 lines of output in pod nginx in cluster(member1)
		%[1]s logs --tail=20 nginx -C=member1
	
		# Show all logs from pod nginx written in the last hour in cluster(member1)
		%[1]s logs --since=1h nginx -C=member1`)
)

// NewCmdLogs new logs command.
func NewCmdLogs(karmadaConfig KarmadaConfig, parentCommand string) *cobra.Command {
	streams := genericclioptions.IOStreams{In: getIn, Out: getOut, ErrOut: getErr}
	o := &LogsOptions{
		KubectlLogsOptions: kubectllogs.NewLogsOptions(streams, false),
	}

	cmd := &cobra.Command{
		Use:          logsUsageStr,
		Short:        "Print the logs for a container in a pod in a cluster",
		SilenceUsage: true,
		Example:      fmt.Sprintf(logsExample, parentCommand),
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
	o.KubectlLogsOptions.AddFlags(cmd)
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "If present, the namespace scope for this CLI request")
	cmd.Flags().StringVarP(&o.Cluster, "cluster", "C", "", "Specify a member cluster")
	return cmd
}

// LogsOptions contains the input to the logs command.
type LogsOptions struct {
	// global flags
	options.GlobalCommandOptions
	// flags specific to logs
	KubectlLogsOptions *kubectllogs.LogsOptions
	Namespace          string
	Cluster            string
}

// Complete ensures that options are valid and marshals them if necessary
func (o *LogsOptions) Complete(karmadaConfig KarmadaConfig, cmd *cobra.Command, args []string) error {
	if o.Cluster == "" {
		return fmt.Errorf("must specify a cluster")
	}

	// print correct usage message when the given arguments are invalid
	switch len(args) {
	case 0:
		if len(o.KubectlLogsOptions.Selector) == 0 {
			return cmdutil.UsageErrorf(cmd, "%s", logsUsageErrStr)
		}
	case 1:
		if len(o.KubectlLogsOptions.Selector) != 0 {
			return cmdutil.UsageErrorf(cmd, "only a selector (-l) or a POD name is allowed")
		}
	case 2:
	default:
		return cmdutil.UsageErrorf(cmd, "%s", logsUsageErrStr)
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
	return o.KubectlLogsOptions.Complete(f, cmd, args)
}

// Validate checks to the LogsOptions to see if there is sufficient information run the command
func (o *LogsOptions) Validate() error {
	return o.KubectlLogsOptions.Validate()
}

// Run retrieves a pod log
func (o *LogsOptions) Run() error {
	return o.KubectlLogsOptions.RunLogs()
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
