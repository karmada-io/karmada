package karmadactl

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/generated/clientset/versioned/scheme"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

// Exported taint constant strings to mark the type of the current operation
const (
	MODIFIED  = "modified"
	TAINTED   = "tainted"
	UNTAINTED = "untainted"
)

var (
	taintShort = `Update the taints on one or more clusters.`
	taintLong  = `Update the taints on one or more clusters.`
)

// NewCmdTaint defines the `taint` command that mark cluster with taints
func NewCmdTaint(karmadaConfig KarmadaConfig, parentCommand string) *cobra.Command {
	opts := CommandTaintOption{}

	cmd := &cobra.Command{
		Use:          "taint CLUSTER NAME KEY_1=VAL_1:TAINT_EFFECT_1 ... KEY_N=VAL_N:TAINT_EFFECT_N",
		Short:        taintShort,
		Long:         taintLong,
		Example:      taintExample(parentCommand),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(args); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := RunTaint(karmadaConfig, opts); err != nil {
				return err
			}
			return nil
		},
	}

	flag := cmd.Flags()
	opts.AddFlags(flag)

	return cmd
}

func taintExample(parentCommand string) string {
	example := `
# Update cluster 'foo' with a taint with key 'dedicated' and value 'special-user' and effect 'NoSchedule'
# If a taint with that key and effect already exists, its value is replaced as specified` + "\n" +
		fmt.Sprintf("%s taint clusters foo dedicated=special-user:NoSchedule", parentCommand) + `

# Remove from cluster 'foo' the taint with key 'dedicated' and effect 'NoSchedule' if one exists` + "\n" +
		fmt.Sprintf("%s taint clusters foo dedicated:NoSchedule-", parentCommand) + `

# Remove from cluster 'foo' all the taints with key 'dedicated'` + "\n" +
		fmt.Sprintf("%s taint clusters foo dedicated-", parentCommand) + `

# Add to cluster 'foo' a taint with key 'bar' and no value` + "\n" +
		fmt.Sprintf("%s taint clusters foo bar:NoSchedule", parentCommand)
	return example
}

// CommandTaintOption holds all command options for taint
type CommandTaintOption struct {
	options.GlobalCommandOptions

	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool

	resources      []string
	taintsToAdd    []corev1.Taint
	taintsToRemove []corev1.Taint
	overwrite      bool
	builder        *resource.Builder
}

// Complete ensures that options are valid and marshals them if necessary
func (o *CommandTaintOption) Complete(args []string) error {
	taintArgs, err := o.parseTaintArgs(args)
	if err != nil {
		return err
	}

	if len(o.resources) < 1 {
		return fmt.Errorf("one or more resources must be specified as <resource> <name>")
	}
	if len(taintArgs) < 1 {
		return fmt.Errorf("at least one taint update is required")
	}

	if o.taintsToAdd, o.taintsToRemove, err = lifted.ParseTaints(taintArgs); err != nil {
		return err
	}

	kubeConfigFlags := genericclioptions.NewConfigFlags(false).WithDeprecatedPasswordFlag()

	if o.KubeConfig != "" {
		kubeConfigFlags.KubeConfig = &o.KubeConfig
		kubeConfigFlags.Context = &o.KarmadaContext
	}

	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

	namespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.builder = f.NewBuilder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		ContinueOnError().
		NamespaceParam(namespace).DefaultNamespace()

	o.builder = o.builder.ResourceNames("cluster", o.resources[1:]...)

	return nil
}

// Validate checks to the TaintOptions to see if there is sufficient information run the command
func (o *CommandTaintOption) Validate() error {
	resourceType := strings.ToLower(o.resources[0])

	if resourceType != "cluster" && resourceType != "clusters" {
		return fmt.Errorf("invalid resource type %s, only [\"cluster\" \"clusters\"] are supported", resourceType)
	}

	// check the format of taint args and checks removed taints aren't in the new taints list
	var conflictTaints []string
	for _, taintAdd := range o.taintsToAdd {
		for _, taintRemove := range o.taintsToRemove {
			if taintAdd.Key != taintRemove.Key {
				continue
			}
			if len(taintRemove.Effect) == 0 || taintAdd.Effect == taintRemove.Effect {
				conflictTaint := fmt.Sprintf("{\"%s\":\"%s\"}", taintRemove.Key, taintRemove.Effect)
				conflictTaints = append(conflictTaints, conflictTaint)
			}
		}
	}
	if len(conflictTaints) > 0 {
		return fmt.Errorf("can not both modify and remove the following taint(s) in the same command: %s", strings.Join(conflictTaints, ", "))
	}

	if len(o.resources) < 2 {
		return fmt.Errorf("at least one resource name must be specified")
	}

	return nil
}

// AddFlags adds flags to the specified FlagSet.
func (o *CommandTaintOption) AddFlags(flags *pflag.FlagSet) {
	o.GlobalCommandOptions.AddFlags(flags)

	flags.BoolVar(&o.overwrite, "overwrite", o.overwrite, "If true, allow taints to be overwritten, otherwise reject taint updates that overwrite existing taints.")
	flags.BoolVar(&o.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
}

// RunTaint set taints for the clusters
func RunTaint(karmadaConfig KarmadaConfig, opts CommandTaintOption) error {
	// Get control plane kube-apiserver client
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(opts.KarmadaContext, opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			opts.KarmadaContext, opts.KubeConfig, err)
	}

	client := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig).ClusterV1alpha1().Clusters()

	r := opts.builder.Do()
	if err := r.Err(); err != nil {
		return err
	}

	return r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		obj := info.Object
		name, _ := info.Name, info.Namespace
		oldData, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		operation, err := opts.updateTaints(obj)
		if err != nil {
			return err
		}
		newData, err := json.Marshal(obj)
		if err != nil {
			return err
		}

		if !opts.DryRun {
			patchBytes, patchErr := strategicpatch.CreateTwoWayMergePatch(oldData, newData, obj)

			cluster, err := client.Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if patchErr == nil {
				_, err = client.Patch(context.TODO(), cluster.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
			} else {
				_, err = client.Update(context.TODO(), cluster, metav1.UpdateOptions{})
			}

			if err != nil {
				return err
			}
		}

		fmt.Println("cluster/" + name + " " + operation)

		return nil
	})
}

func (o *CommandTaintOption) updateTaints(obj runtime.Object) (string, error) {
	cluster, ok := obj.(*clusterv1alpha1.Cluster)
	if !ok {
		return "", fmt.Errorf("unexpected type %T, expected Cluster", obj)
	}
	if !o.overwrite {
		if exists := checkIfTaintsAlreadyExists(cluster.Spec.Taints, o.taintsToAdd); len(exists) != 0 {
			return "", fmt.Errorf("cluster %s already has %v taint(s) with same effect(s) and --overwrite is false", cluster.Name, exists)
		}
	}
	operation, newTaints, err := reorganizeTaints(cluster, o.overwrite, o.taintsToAdd, o.taintsToRemove)
	if err != nil {
		return "", err
	}
	cluster.Spec.Taints = newTaints
	return operation, nil
}

// parseTaintArgs retrieves resource and taint args from args
func (o *CommandTaintOption) parseTaintArgs(args []string) ([]string, error) {
	var taintArgs []string
	metTaintArg := false
	for _, s := range args {
		isTaint := strings.Contains(s, "=") || strings.Contains(s, ":") || strings.HasSuffix(s, "-")
		switch {
		case !metTaintArg && isTaint:
			metTaintArg = true
			fallthrough
		case metTaintArg && isTaint:
			taintArgs = append(taintArgs, s)
		case !metTaintArg && !isTaint:
			o.resources = append(o.resources, s)
		case metTaintArg && !isTaint:
			return nil, fmt.Errorf("all resources must be specified before taint changes: %s", s)
		}
	}
	return taintArgs, nil
}

// reorganizeTaints returns the updated set of taints, taking into account old taints that were not updated,
// old taints that were updated, old taints that were deleted, and new taints.
func reorganizeTaints(cluster *clusterv1alpha1.Cluster, overwrite bool, taintsToAdd []corev1.Taint, taintsToRemove []corev1.Taint) (string, []corev1.Taint, error) {
	newTaints := append([]corev1.Taint{}, taintsToAdd...)
	oldTaints := cluster.Spec.Taints
	// add taints that already existing but not updated to newTaints
	added := addTaints(oldTaints, &newTaints)
	allErrs, deleted := deleteTaints(taintsToRemove, &newTaints)
	if (added && deleted) || overwrite {
		return MODIFIED, newTaints, utilerrors.NewAggregate(allErrs)
	} else if added {
		return TAINTED, newTaints, utilerrors.NewAggregate(allErrs)
	}
	return UNTAINTED, newTaints, utilerrors.NewAggregate(allErrs)
}

// deleteTaints deletes the given taints from the cluster's taintlist.
func deleteTaints(taintsToRemove []corev1.Taint, newTaints *[]corev1.Taint) ([]error, bool) {
	allErrs := []error{}
	var removed bool
	for i, taintToRemove := range taintsToRemove {
		if len(taintToRemove.Effect) > 0 {
			*newTaints, removed = deleteTaint(*newTaints, &taintsToRemove[i])
		} else {
			*newTaints, removed = deleteTaintsByKey(*newTaints, taintToRemove.Key)
		}
		if !removed {
			allErrs = append(allErrs, fmt.Errorf("taint %q not found", taintToRemove.ToString()))
		}
	}
	return allErrs, removed
}

// addTaints adds existing ones to newTaints list and updates the newTaints List.
func addTaints(oldTaints []corev1.Taint, newTaints *[]corev1.Taint) bool {
	for i, oldTaint := range oldTaints {
		existsInNew := false
		for _, taint := range *newTaints {
			if taint.MatchTaint(&oldTaints[i]) {
				existsInNew = true
				break
			}
		}
		if !existsInNew {
			*newTaints = append(*newTaints, oldTaint)
		}
	}
	return len(oldTaints) != len(*newTaints)
}

// checkIfTaintsAlreadyExists checks if the cluster already has taints that we want to add and returns a string with taint keys.
func checkIfTaintsAlreadyExists(oldTaints []corev1.Taint, taints []corev1.Taint) string {
	var existingTaintList = make([]string, 0)
	for _, taint := range taints {
		for _, oldTaint := range oldTaints {
			if taint.Key == oldTaint.Key && taint.Effect == oldTaint.Effect {
				existingTaintList = append(existingTaintList, taint.Key)
			}
		}
	}
	return strings.Join(existingTaintList, ",")
}

// deleteTaintsByKey removes all the taints that have the same key to given taintKey
func deleteTaintsByKey(taints []corev1.Taint, taintKey string) ([]corev1.Taint, bool) {
	newTaints := []corev1.Taint{}
	for i := range taints {
		if taintKey == taints[i].Key {
			continue
		}
		newTaints = append(newTaints, taints[i])
	}
	return newTaints, len(taints) != len(newTaints)
}

// deleteTaint removes all the taints that have the same key and effect to given taintToDelete.
func deleteTaint(taints []corev1.Taint, taintToDelete *corev1.Taint) ([]corev1.Taint, bool) {
	newTaints := []corev1.Taint{}
	for i := range taints {
		if taintToDelete.MatchTaint(&taints[i]) {
			continue
		}
		newTaints = append(newTaints, taints[i])
	}
	return newTaints, len(taints) != len(newTaints)
}
