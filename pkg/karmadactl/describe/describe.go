package describe

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubectldescribe "k8s.io/kubectl/pkg/cmd/describe"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

var (
	describeLong = templates.LongDesc(`
		Show details of a specific resource or group of resources in a member cluster.

		Print a detailed description of the selected resources, including related
		resources such as events or controllers. You may select a single object by name,
		all objects of that type, provide a name prefix, or label selector. For example:

		$ %[1]s describe TYPE NAME_PREFIX

		will first check for an exact match on TYPE and NAME_PREFIX. If no such
		resource exists, it will output details for every resource that has a name
		prefixed with NAME_PREFIX.`)

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
func NewCmdDescribe(f util.Factory, parentCommand string, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CommandDescribeOptions{}
	kubedescribeFlags := kubectldescribe.NewDescribeFlags(f, streams)

	cmd := &cobra.Command{
		Use:                   "describe (-f FILENAME | TYPE [NAME_PREFIX | -l label] | TYPE/NAME) (-C CLUSTER)",
		Short:                 "Show details of a specific resource or group of resources in a cluster",
		Long:                  fmt.Sprintf(describeLong, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		Example:               fmt.Sprintf(describeExample, parentCommand),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, args, kubedescribeFlags, parentCommand); err != nil {
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
	kubedescribeFlags.AddFlags(cmd)

	options.AddKubeConfigFlags(flags)
	flags.StringVarP(options.DefaultConfigFlags.Namespace, "namespace", "n", *options.DefaultConfigFlags.Namespace, "If present, the namespace scope for this CLI request")
	flags.StringVarP(&o.Cluster, "cluster", "C", "", "Specify a member cluster")

	return cmd
}

// CommandDescribeOptions contains the input to the describe command.
type CommandDescribeOptions struct {
	// flags specific to describe
	KubectlDescribeOptions *kubectldescribe.DescribeOptions
	Cluster                string
}

// Complete ensures that options are valid and marshals them if necessary
func (o *CommandDescribeOptions) Complete(f util.Factory, args []string, describeFlag *kubectldescribe.DescribeFlags, parentCommand string) error {
	if len(o.Cluster) == 0 {
		return fmt.Errorf("must specify a cluster")
	}

	memberFactory, err := f.FactoryForMemberCluster(o.Cluster)
	if err != nil {
		return err
	}
	describeFlag.Factory = memberFactory
	o.KubectlDescribeOptions, err = describeFlag.ToOptions(parentCommand, args)
	if err != nil {
		return err
	}
	return nil
}

// Run describe information of resources
func (o *CommandDescribeOptions) Run() error {
	return o.KubectlDescribeOptions.Run()
}
