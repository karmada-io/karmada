package karmadactl

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	kubectldescribe "k8s.io/kubectl/pkg/cmd/describe"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
)

var (
	describeExample = templates.Examples(`
		# Describe a pod in cluster(member1)
		%[1]s describe pods/nginx -C=member1
	
		# Describe all pods in cluster(member1)
		%[1]s describe pods -C=member1
	
		# Describe a pod identified by type and name in "pod.json" in cluster(member1)
		%[1]s describe -f pod.json -C=member1
	
		# Describe pods by label name=myLabel in cluster(member1)
		%[1]s describe po -l name=myLabel -C=member1
	
		# Describe all pods managed by the 'frontend' replication controller in cluster(member1)
		# (rc-created pods get the name of the rc as a prefix in the pod name)
		%[1]s describe pods frontend -C=member1`)
)

// NewCmdDescribe new describe command.
func NewCmdDescribe(karmadaConfig KarmadaConfig, parentCommand string, streams genericclioptions.IOStreams) *cobra.Command {
	o := &DescribeOptions{
		KubectlDescribeOptions: &kubectldescribe.DescribeOptions{
			FilenameOptions: &resource.FilenameOptions{},
			DescriberSettings: &describe.DescriberSettings{
				ShowEvents: true,
				ChunkSize:  cmdutil.DefaultChunkSize,
			},
			CmdParent: parentCommand,
			IOStreams: streams,
		},
	}

	cmd := &cobra.Command{
		Use:                   "describe (-f FILENAME | TYPE [NAME_PREFIX | -l label] | TYPE/NAME) (-C CLUSTER)",
		Short:                 "Show details of a specific resource or group of resources in a cluster",
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		Example:               fmt.Sprintf(describeExample, parentCommand),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(karmadaConfig, cmd, args); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	o.GlobalCommandOptions.AddFlags(cmd.Flags())

	usage := "containing the resource to describe"
	cmdutil.AddFilenameOptionFlags(cmd, o.KubectlDescribeOptions.FilenameOptions, usage)
	cmd.Flags().StringVarP(&o.Cluster, "cluster", "C", "", "Specify a member cluster")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "If present, the namespace scope for this CLI request")
	cmd.Flags().StringVarP(&o.KubectlDescribeOptions.Selector, "selector", "l", o.KubectlDescribeOptions.Selector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().BoolVarP(&o.KubectlDescribeOptions.AllNamespaces, "all-namespaces", "A", o.KubectlDescribeOptions.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().BoolVar(&o.KubectlDescribeOptions.DescriberSettings.ShowEvents, "show-events", o.KubectlDescribeOptions.DescriberSettings.ShowEvents, "If true, display events related to the described object.")
	cmdutil.AddChunkSizeFlag(cmd, &o.KubectlDescribeOptions.DescriberSettings.ChunkSize)

	return cmd
}

// DescribeOptions contains the input to the describe command.
type DescribeOptions struct {
	// global flags
	options.GlobalCommandOptions
	// flags specific to describe
	KubectlDescribeOptions *kubectldescribe.DescribeOptions
	Namespace              string
	Cluster                string
}

// Complete ensures that options are valid and marshals them if necessary
func (o *DescribeOptions) Complete(karmadaConfig KarmadaConfig, cmd *cobra.Command, args []string) error {
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
	return o.KubectlDescribeOptions.Complete(f, cmd, args)
}

// Run describe information of resources
func (o *DescribeOptions) Run() error {
	return o.KubectlDescribeOptions.Run()
}
