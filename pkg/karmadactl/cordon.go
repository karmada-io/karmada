package karmadactl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/kubectl/pkg/util/templates"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

var (
	cordonShort   = `Mark cluster as unschedulable`
	cordonLong    = `Mark cluster as unschedulable.`
	cordonExample = templates.Examples(`
		# Mark cluster "foo" as unschedulable.
		%[1]s cordon foo`)
)

// NewCmdCordon defines the `cordon` command that mark cluster as unschedulable.
func NewCmdCordon(f util.Factory, parentCommand string) *cobra.Command {
	opts := CommandCordonOption{}
	cmd := &cobra.Command{
		Use:                   "cordon CLUSTER",
		Short:                 cordonShort,
		Long:                  cordonLong,
		Example:               fmt.Sprintf(cordonExample, parentCommand),
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

// CommandCordonOption holds all command options for cordon
type CommandCordonOption struct {
	// ClusterName is the cluster's name that we are going to join with.
	ClusterName string

	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool
}

// Complete ensures that options are valid and marshals them if necessary.
func (o *CommandCordonOption) Complete(args []string) error {
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

// Run exec marks the cluster schedulable.
func (o *CommandCordonOption) Run(f util.Factory) error {
	karmadaClient, err := f.KarmadaClientSet()
	if err != nil {
		return err
	}

	cluster, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), o.ClusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	cordonHelper := NewCordonHelper(cluster, true)
	if cordonHelper.hasUnschedulerTaint() {
		fmt.Printf("%s cluster already cordoned\n", cluster.Name)
		return nil
	}

	if !o.DryRun {
		err := cordonHelper.PatchOrReplace(karmadaClient)
		if err != nil {
			return err
		}
	}

	fmt.Printf("%s cluster cordoned\n", cluster.Name)
	return nil
}

// CordonHelper wraps functionality to cordon/uncordon cluster
type CordonHelper struct {
	cordon  bool
	cluster *clusterv1alpha1.Cluster
}

// NewCordonHelper returns a new CordonHelper that help execute
// the cordon and uncordon commands
func NewCordonHelper(cluster *clusterv1alpha1.Cluster, cordon bool) *CordonHelper {
	return &CordonHelper{
		cluster: cluster,
		cordon:  cordon,
	}
}

func (c *CordonHelper) hasUnschedulerTaint() bool {
	unschedulerTaint := corev1.Taint{
		Key:    clusterv1alpha1.TaintClusterUnscheduler,
		Effect: corev1.TaintEffectNoSchedule,
	}

	for _, taint := range c.cluster.Spec.Taints {
		if taint.MatchTaint(&unschedulerTaint) {
			return true
		}
	}

	return false
}

// PatchOrReplace uses given karmada clientset to update the cluster unschedulable scheduler, either by patching or
// updating the given cluster object; it may return error if the object cannot be encoded as
// JSON, or if either patch or update calls fail; it will also return error whenever creating a patch has failed
func (c *CordonHelper) PatchOrReplace(karmadaClient karmadaclientset.Interface) error {
	client := karmadaClient.ClusterV1alpha1().Clusters()
	oldData, err := json.Marshal(c.cluster)
	if err != nil {
		return err
	}

	unschedulerTaint := corev1.Taint{
		Key:    clusterv1alpha1.TaintClusterUnscheduler,
		Effect: corev1.TaintEffectNoSchedule,
	}

	if c.cordon {
		c.cluster.Spec.Taints = append(c.cluster.Spec.Taints, unschedulerTaint)
	} else {
		for i, n := 0, len(c.cluster.Spec.Taints); i < n; i++ {
			if c.cluster.Spec.Taints[i].MatchTaint(&unschedulerTaint) {
				c.cluster.Spec.Taints[i] = c.cluster.Spec.Taints[n-1]
				c.cluster.Spec.Taints = c.cluster.Spec.Taints[:n-1]
				break
			}
		}
	}

	newData, err := json.Marshal(c.cluster)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, c.cluster)
	if err == nil {
		_, err = client.Patch(context.TODO(), c.cluster.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	} else {
		_, err = client.Update(context.TODO(), c.cluster, metav1.UpdateOptions{})
	}
	return err
}
