package karmadactl

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubectllogs "k8s.io/kubectl/pkg/cmd/logs"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/util"
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
func NewCmdLogs(f util.Factory, parentCommand string, streams genericclioptions.IOStreams) *cobra.Command {
	o := &LogsOptions{
		KubectlLogsOptions: kubectllogs.NewLogsOptions(streams, false),
	}

	cmd := &cobra.Command{
		Use:                   logsUsageStr,
		Short:                 "Print the logs for a container in a pod in a cluster",
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		Example:               fmt.Sprintf(logsExample, parentCommand),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(cmd, args, f); err != nil {
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
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterTroubleshootingAndDebugging,
		},
	}

	flags := cmd.Flags()
	flags.StringVar(defaultConfigFlags.KubeConfig, "kubeconfig", *defaultConfigFlags.KubeConfig, "Path to the kubeconfig file to use for CLI requests.")
	flags.StringVar(defaultConfigFlags.Context, "karmada-context", *defaultConfigFlags.Context, "The name of the kubeconfig context to use")
	flags.StringVarP(defaultConfigFlags.Namespace, "namespace", "n", *defaultConfigFlags.Namespace, "If present, the namespace scope for this CLI request")
	flags.StringVarP(&o.Cluster, "cluster", "C", "", "Specify a member cluster")
	o.KubectlLogsOptions.AddFlags(cmd)

	return cmd
}

// LogsOptions contains the input to the logs command.
type LogsOptions struct {
	// flags specific to logs
	KubectlLogsOptions *kubectllogs.LogsOptions
	Cluster            string
}

// Complete ensures that options are valid and marshals them if necessary
func (o *LogsOptions) Complete(cmd *cobra.Command, args []string, f util.Factory) error {
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

	memberFactory, err := f.FactoryForMemberCluster(o.Cluster)
	if err != nil {
		return err
	}
	return o.KubectlLogsOptions.Complete(memberFactory, cmd, args)
}

// Validate checks to the LogsOptions to see if there is sufficient information run the command
func (o *LogsOptions) Validate() error {
	return o.KubectlLogsOptions.Validate()
}

// Run retrieves a pod log
func (o *LogsOptions) Run() error {
	return o.KubectlLogsOptions.RunLogs()
}
