package karmadactl

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

var (
	uncordonShort   = `Mark cluster as schedulable`
	uncordonLong    = `Mark cluster as schedulable.`
	uncordonExample = templates.Examples(`
		# Mark cluster "foo" as schedulable.
		%[1]s uncordon foo`)
)

// NewCmdUncordon defines the `uncordon` command that mark cluster as schedulable.
func NewCmdUncordon(f util.Factory, parentCommand string) *cobra.Command {
	opts := CommandUncordonOption{}
	cmd := &cobra.Command{
		Use:                   "uncordon CLUSTER",
		Short:                 uncordonShort,
		Long:                  uncordonLong,
		Example:               fmt.Sprintf(uncordonExample, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(args); err != nil {
				return err
			}
			if err := opts.Run(f); err != nil {
				return err
			}
			return nil
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterManagement,
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
	flags.StringVar(defaultConfigFlags.KubeConfig, "kubeconfig", *defaultConfigFlags.KubeConfig, "Path to the kubeconfig file to use for CLI requests.")
	flags.StringVar(defaultConfigFlags.Context, "karmada-context", *defaultConfigFlags.Context, "The name of the kubeconfig context to use")
	flags.StringVarP(defaultConfigFlags.Namespace, "namespace", "n", *defaultConfigFlags.Namespace, "If present, the namespace scope for this CLI request")

	return cmd
}

// CommandUncordonOption holds all command options for uncordon
type CommandUncordonOption struct {
	// ClusterName is the cluster's name that we are going to join with.
	ClusterName string

	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool
}

// Complete ensures that options are valid and marshals them if necessary.
func (o *CommandUncordonOption) Complete(args []string) error {
	// Get cluster name from the command args.
	if len(args) == 0 {
		return errors.New("cluster name is required")
	}
	if len(args) > 1 {
		return errors.New("more than one cluster name is not supported")
	}
	o.ClusterName = args[0]
	return nil
}

// Run exec marks the cluster unschedulable.
func (o *CommandUncordonOption) Run(f util.Factory) error {
	karmadaClient, err := f.KarmadaClientSet()
	if err != nil {
		return err
	}

	cluster, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), o.ClusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	uncordonHelper := NewCordonHelper(cluster, false)
	if !uncordonHelper.hasUnschedulerTaint() {
		fmt.Printf("%s cluster already uncordoned\n", cluster.Name)
		return nil
	}

	if !o.DryRun {
		err := uncordonHelper.PatchOrReplace(karmadaClient)
		if err != nil {
			return err
		}
	}

	fmt.Printf("%s cluster uncordoned\n", cluster.Name)
	return nil
}
