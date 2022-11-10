package cordon

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
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

var (
	cordonShort = `Mark cluster as unschedulable`
	cordonLong  = `Mark cluster as unschedulable.`

	uncordonShort = `Mark cluster as schedulable`
	uncordonLong  = `Mark cluster as schedulable.`

	cordonExample = templates.Examples(`
		# Mark cluster "foo" as unschedulable.
		%[1]s cordon foo`)
	uncordonExample = templates.Examples(`
		# Mark cluster "foo" as schedulable.
		%[1]s uncordon foo`)
)

const (
	// DesiredCordon a flag indicate karmadactl.RunCordonOrUncordon cordon a cluster,
	// cordon prevent new resource scheduler to cordoned cluster.
	DesiredCordon = iota
	// DesiredUnCordon a flag indicate karmadactl.RunCordonOrUncordon uncordon a cluster.
	DesiredUnCordon
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
			if err := RunCordonOrUncordon(DesiredCordon, f, opts); err != nil {
				return err
			}
			return nil
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterManagement,
		},
	}

	flags := cmd.Flags()
	options.AddKubeConfigFlags(flags)
	flags.BoolVar(&opts.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")

	return cmd
}

// NewCmdUncordon defines the `uncordon` command that mark cluster as schedulable.
func NewCmdUncordon(f util.Factory, parentCommand string) *cobra.Command {
	opts := CommandCordonOption{}
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
			if err := RunCordonOrUncordon(DesiredUnCordon, f, opts); err != nil {
				return err
			}
			return nil
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterManagement,
		},
	}

	flags := cmd.Flags()
	options.AddKubeConfigFlags(flags)
	flags.BoolVar(&opts.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")

	return cmd
}

// CommandCordonOption holds all command options for cordon and uncordon
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

// RunCordonOrUncordon exec marks the cluster unschedulable or schedulable according to desired.
// if true cordon cluster otherwise uncordon cluster.
func RunCordonOrUncordon(desired int, f util.Factory, opts CommandCordonOption) error {
	cordonOrUncordon := "cordon"
	if desired == DesiredUnCordon {
		cordonOrUncordon = "un" + cordonOrUncordon
	}

	karmadaClient, err := f.KarmadaClientSet()
	if err != nil {
		return err
	}

	cluster, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), opts.ClusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	cordonHelper := newCordonHelper(cluster)
	if !cordonHelper.updateIfRequired(desired) {
		fmt.Printf("%s cluster %s\n", cluster.Name, alreadyStr(desired))
		return nil
	}

	if !opts.DryRun {
		err := cordonHelper.patchOrReplace(karmadaClient)
		if err != nil {
			return err
		}
	}

	fmt.Printf("%s cluster %sed\n", cluster.Name, cordonOrUncordon)
	return nil
}

// cordonHelper wraps functionality to cordon/uncordon cluster
type cordonHelper struct {
	cluster *clusterv1alpha1.Cluster
	desired int
}

// newCordonHelper returns a new CordonHelper that help execute
// the cordon and uncordon commands
func newCordonHelper(cluster *clusterv1alpha1.Cluster) *cordonHelper {
	return &cordonHelper{
		cluster: cluster,
	}
}

// updateIfRequired returns true if unscheduler taint isn't already set,
// or false when no change is needed
func (c *cordonHelper) updateIfRequired(desired int) bool {
	c.desired = desired

	if desired == DesiredCordon && !c.hasUnschedulerTaint() {
		return true
	}

	if desired == DesiredUnCordon && c.hasUnschedulerTaint() {
		return true
	}

	return false
}

func (c *cordonHelper) hasUnschedulerTaint() bool {
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

// patchOrReplace uses given karmada clientset to update the cluster unschedulable scheduler, either by patching or
// updating the given cluster object; it may return error if the object cannot be encoded as
// JSON, or if either patch or update calls fail; it will also return error whenever creating a patch has failed
func (c *cordonHelper) patchOrReplace(karmadaClient karmadaclientset.Interface) error {
	client := karmadaClient.ClusterV1alpha1().Clusters()
	oldData, err := json.Marshal(c.cluster)
	if err != nil {
		return err
	}

	unschedulerTaint := corev1.Taint{
		Key:    clusterv1alpha1.TaintClusterUnscheduler,
		Effect: corev1.TaintEffectNoSchedule,
	}

	if c.desired == DesiredCordon {
		c.cluster.Spec.Taints = append(c.cluster.Spec.Taints, unschedulerTaint)
	}

	if c.desired == DesiredUnCordon {
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

func alreadyStr(desired int) string {
	if desired == DesiredCordon {
		return "already cordoned"
	}
	return "already uncordoned"
}
