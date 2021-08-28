package binding

import (
	"context"
	"reflect"
	"sort"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/kind/pkg/errors"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	karmadactlutil "github.com/karmada-io/karmada/pkg/controllers/util"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

var workPredicateFn = builder.WithPredicates(predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return false
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		var statusesOld, statusesNew []workv1alpha1.ManifestStatus

		switch oldWork := e.ObjectOld.(type) {
		case *workv1alpha1.Work:
			statusesOld = oldWork.Status.ManifestStatuses
		default:
			return false
		}

		switch newWork := e.ObjectNew.(type) {
		case *workv1alpha1.Work:
			statusesNew = newWork.Status.ManifestStatuses
		default:
			return false
		}

		return !reflect.DeepEqual(statusesOld, statusesNew)
	},
	DeleteFunc: func(event.DeleteEvent) bool {
		return false
	},
	GenericFunc: func(event.GenericEvent) bool {
		return false
	},
})

// ensureWork ensure Work to be created or updated.
func ensureWork(c client.Client, workload *unstructured.Unstructured, overrideManager overridemanager.OverrideManager, binding metav1.Object, scope apiextensionsv1.ResourceScope) error {
	var targetClusters []workv1alpha1.TargetCluster
	switch scope {
	case apiextensionsv1.NamespaceScoped:
		bindingObj := binding.(*workv1alpha1.ResourceBinding)
		targetClusters = bindingObj.Spec.Clusters
	case apiextensionsv1.ClusterScoped:
		bindingObj := binding.(*workv1alpha1.ClusterResourceBinding)
		targetClusters = bindingObj.Spec.Clusters
	}

	hasScheduledReplica, referenceRSP, desireReplicaInfos, err := getRSPAndReplicaInfos(c, workload, targetClusters)
	if err != nil {
		return err
	}

	for _, targetCluster := range targetClusters {
		clonedWorkload := workload.DeepCopy()
		cops, ops, err := overrideManager.ApplyOverridePolicies(clonedWorkload, targetCluster.Name)
		if err != nil {
			klog.Errorf("Failed to apply overrides for %s/%s/%s, err is: %v", clonedWorkload.GetKind(), clonedWorkload.GetNamespace(), clonedWorkload.GetName(), err)
			return err
		}

		workNamespace, err := names.GenerateExecutionSpaceName(targetCluster.Name)
		if err != nil {
			klog.Errorf("Failed to ensure Work for cluster: %s. Error: %v.", targetCluster.Name, err)
			return err
		}

		workLabel := mergeLabel(clonedWorkload, workNamespace, binding, scope)

		if clonedWorkload.GetKind() == util.DeploymentKind && (referenceRSP != nil || hasScheduledReplica) {
			err = applyReplicaSchedulingPolicy(clonedWorkload, desireReplicaInfos[targetCluster.Name])
			if err != nil {
				klog.Errorf("failed to apply ReplicaSchedulingPolicy for %s/%s/%s in cluster %s, err is: %v",
					clonedWorkload.GetKind(), clonedWorkload.GetNamespace(), clonedWorkload.GetName(), targetCluster.Name, err)
				return err
			}
		}

		annotations, err := recordAppliedOverrides(cops, ops)
		if err != nil {
			klog.Errorf("failed to record appliedOverrides, Error: %v", err)
			return err
		}

		workMeta := metav1.ObjectMeta{
			Name:        karmadactlutil.GenerateWorkName(clonedWorkload.GetKind(), clonedWorkload.GetName(), clonedWorkload.GetNamespace()),
			Namespace:   workNamespace,
			Finalizers:  []string{util.ExecutionControllerFinalizer},
			Labels:      workLabel,
			Annotations: annotations,
		}

		if err = karmadactlutil.CreateOrUpdateWork(c, workMeta, clonedWorkload); err != nil {
			return err
		}
	}
	return nil
}

func getRSPAndReplicaInfos(c client.Client, workload *unstructured.Unstructured, targetClusters []workv1alpha1.TargetCluster) (bool, *v1alpha1.ReplicaSchedulingPolicy, map[string]int64, error) {
	if hasScheduledReplica(targetClusters) {
		return true, nil, transScheduleResultToMap(targetClusters), nil
	}

	referenceRSP, desireReplicaInfos, err := calculateReplicasIfNeeded(c, workload, getBindingClusterNames(targetClusters))
	if err != nil {
		klog.Errorf("Failed to get ReplicaSchedulingPolicy for %s/%s/%s, err is: %v", workload.GetKind(), workload.GetNamespace(), workload.GetName(), err)
		return false, nil, nil, err
	}

	return false, referenceRSP, desireReplicaInfos, nil
}

func applyReplicaSchedulingPolicy(workload *unstructured.Unstructured, desireReplica int64) error {
	_, ok, err := unstructured.NestedInt64(workload.Object, util.SpecField, util.ReplicasField)
	if err != nil {
		return err
	}
	if ok {
		err := unstructured.SetNestedField(workload.Object, desireReplica, util.SpecField, util.ReplicasField)
		if err != nil {
			return err
		}
	}
	return nil
}

func mergeLabel(workload *unstructured.Unstructured, workNamespace string, binding metav1.Object, scope apiextensionsv1.ResourceScope) map[string]string {
	var workLabel = make(map[string]string)
	util.MergeLabel(workload, workv1alpha1.WorkNamespaceLabel, workNamespace)
	util.MergeLabel(workload, workv1alpha1.WorkNameLabel, karmadactlutil.GenerateWorkName(workload.GetKind(), workload.GetName(), workload.GetNamespace()))

	if scope == apiextensionsv1.NamespaceScoped {
		util.MergeLabel(workload, workv1alpha1.ResourceBindingNamespaceLabel, binding.GetNamespace())
		util.MergeLabel(workload, workv1alpha1.ResourceBindingNameLabel, binding.GetName())
		workLabel[workv1alpha1.ResourceBindingNamespaceLabel] = binding.GetNamespace()
		workLabel[workv1alpha1.ResourceBindingNameLabel] = binding.GetName()
	} else {
		util.MergeLabel(workload, workv1alpha1.ClusterResourceBindingLabel, binding.GetName())
		workLabel[workv1alpha1.ClusterResourceBindingLabel] = binding.GetName()
	}

	return workLabel
}

func recordAppliedOverrides(cops *overridemanager.AppliedOverrides, ops *overridemanager.AppliedOverrides) (map[string]string, error) {
	annotations := make(map[string]string)

	if cops != nil {
		appliedBytes, err := cops.MarshalJSON()
		if err != nil {
			return nil, err
		}
		if appliedBytes != nil {
			annotations[util.AppliedClusterOverrides] = string(appliedBytes)
		}
	}

	if ops != nil {
		appliedBytes, err := ops.MarshalJSON()
		if err != nil {
			return nil, err
		}
		if appliedBytes != nil {
			annotations[util.AppliedOverrides] = string(appliedBytes)
		}
	}

	return annotations, nil
}

func transScheduleResultToMap(scheduleResult []workv1alpha1.TargetCluster) map[string]int64 {
	var desireReplicaInfos = make(map[string]int64, len(scheduleResult))
	for _, clusterInfo := range scheduleResult {
		desireReplicaInfos[clusterInfo.Name] = int64(clusterInfo.Replicas)
	}
	return desireReplicaInfos
}

func calculateReplicasIfNeeded(c client.Client, workload *unstructured.Unstructured, clusterNames []string) (*v1alpha1.ReplicaSchedulingPolicy, map[string]int64, error) {
	var err error
	var referenceRSP *v1alpha1.ReplicaSchedulingPolicy
	var desireReplicaInfos = make(map[string]int64)

	if workload.GetKind() == util.DeploymentKind {
		referenceRSP, err = matchReplicaSchedulingPolicy(c, workload)
		if err != nil {
			return nil, nil, err
		}
		if referenceRSP != nil {
			desireReplicaInfos, err = calculateReplicas(c, referenceRSP, clusterNames)
			if err != nil {
				klog.Errorf("Failed to get desire replicas for %s/%s/%s, err is: %v", workload.GetKind(), workload.GetNamespace(), workload.GetName(), err)
				return nil, nil, err
			}
			klog.V(4).Infof("DesireReplicaInfos with replica scheduling policies(%s/%s) is %v", referenceRSP.Namespace, referenceRSP.Name, desireReplicaInfos)
		}
	}
	return referenceRSP, desireReplicaInfos, nil
}

func matchReplicaSchedulingPolicy(c client.Client, workload *unstructured.Unstructured) (*v1alpha1.ReplicaSchedulingPolicy, error) {
	// get all namespace-scoped replica scheduling policies
	policyList := &v1alpha1.ReplicaSchedulingPolicyList{}
	if err := c.List(context.TODO(), policyList, &client.ListOptions{Namespace: workload.GetNamespace()}); err != nil {
		klog.Errorf("Failed to list replica scheduling policies from namespace: %s, error: %v", workload.GetNamespace(), err)
		return nil, err
	}

	if len(policyList.Items) == 0 {
		return nil, nil
	}

	matchedPolicies := getMatchedReplicaSchedulingPolicy(policyList.Items, workload)
	if len(matchedPolicies) == 0 {
		klog.V(2).Infof("No replica scheduling policy for resource: %s/%s", workload.GetNamespace(), workload.GetName())
		return nil, nil
	}

	return &matchedPolicies[0], nil
}

func getMatchedReplicaSchedulingPolicy(policies []v1alpha1.ReplicaSchedulingPolicy, resource *unstructured.Unstructured) []v1alpha1.ReplicaSchedulingPolicy {
	// select policy in which at least one resource selector matches target resource.
	resourceMatches := make([]v1alpha1.ReplicaSchedulingPolicy, 0)
	for _, policy := range policies {
		if util.ResourceMatchSelectors(resource, policy.Spec.ResourceSelectors...) {
			resourceMatches = append(resourceMatches, policy)
		}
	}

	// Sort by policy names.
	sort.Slice(resourceMatches, func(i, j int) bool {
		return resourceMatches[i].Name < resourceMatches[j].Name
	})

	return resourceMatches
}

func calculateReplicas(c client.Client, policy *v1alpha1.ReplicaSchedulingPolicy, clusterNames []string) (map[string]int64, error) {
	weightSum := int64(0)
	matchClusters := make(map[string]int64)
	desireReplicaInfos := make(map[string]int64)

	// found out clusters matched the given ReplicaSchedulingPolicy
	for _, clusterName := range clusterNames {
		clusterObj := &clusterv1alpha1.Cluster{}
		if err := c.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
			klog.Errorf("Failed to get member cluster: %s, error: %v", clusterName, err)
			return nil, err
		}
		for _, staticWeightRule := range policy.Spec.Preferences.StaticWeightList {
			if util.ClusterMatches(clusterObj, staticWeightRule.TargetCluster) {
				weightSum += staticWeightRule.Weight
				matchClusters[clusterName] = staticWeightRule.Weight
				break
			}
		}
	}

	if weightSum == 0 {
		return desireReplicaInfos, nil
	}

	allocatedReplicas := int32(0)
	for clusterName, weight := range matchClusters {
		desireReplicaInfos[clusterName] = weight * int64(policy.Spec.TotalReplicas) / weightSum
		allocatedReplicas += int32(desireReplicaInfos[clusterName])
	}

	if remainReplicas := policy.Spec.TotalReplicas - allocatedReplicas; remainReplicas > 0 {
		sortedClusters := helper.SortClusterByWeight(matchClusters)
		for i := 0; remainReplicas > 0; i++ {
			desireReplicaInfos[sortedClusters[i].ClusterName]++
			remainReplicas--
			if i == len(desireReplicaInfos) {
				i = 0
			}
		}
	}

	for _, clusterName := range clusterNames {
		if _, exist := matchClusters[clusterName]; !exist {
			desireReplicaInfos[clusterName] = 0
		}
	}

	return desireReplicaInfos, nil
}

// aggregateResourceBindingWorkStatus will collect all work statuses with current ResourceBinding objects,
// then aggregate status info to current ResourceBinding status.
func aggregateResourceBindingWorkStatus(c client.Client, binding *workv1alpha1.ResourceBinding, workload *unstructured.Unstructured) error {
	aggregatedStatuses, err := assembleWorkStatus(c, labels.SelectorFromSet(labels.Set{
		workv1alpha1.ResourceBindingNamespaceLabel: binding.Namespace,
		workv1alpha1.ResourceBindingNameLabel:      binding.Name,
	}), workload)
	if err != nil {
		return err
	}

	if reflect.DeepEqual(binding.Status.AggregatedStatus, aggregatedStatuses) {
		klog.V(4).Infof("New aggregatedStatuses are equal with old resourceBinding(%s/%s) AggregatedStatus, no update required.",
			binding.Namespace, binding.Name)
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		if err = c.Get(context.TODO(), client.ObjectKey{Namespace: binding.Namespace, Name: binding.Name}, binding); err != nil {
			return err
		}
		binding.Status.AggregatedStatus = aggregatedStatuses
		return c.Status().Update(context.TODO(), binding)
	})
}

// aggregateClusterResourceBindingWorkStatus will collect all work statuses with current ClusterResourceBinding objects,
// then aggregate status info to current ClusterResourceBinding status.
func aggregateClusterResourceBindingWorkStatus(c client.Client, binding *workv1alpha1.ClusterResourceBinding, workload *unstructured.Unstructured) error {
	aggregatedStatuses, err := assembleWorkStatus(c, labels.SelectorFromSet(labels.Set{
		workv1alpha1.ClusterResourceBindingLabel: binding.Name,
	}), workload)
	if err != nil {
		return err
	}

	if reflect.DeepEqual(binding.Status.AggregatedStatus, aggregatedStatuses) {
		klog.Infof("New aggregatedStatuses are equal with old clusterResourceBinding(%s) AggregatedStatus, no update required.", binding.Name)
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		if err = c.Get(context.TODO(), client.ObjectKey{Name: binding.Name}, binding); err != nil {
			return err
		}
		binding.Status.AggregatedStatus = aggregatedStatuses
		return c.Status().Update(context.TODO(), binding)
	})
}

// assemble workStatuses from workList which list by selector and match with workload.
func assembleWorkStatus(c client.Client, selector labels.Selector, workload *unstructured.Unstructured) ([]workv1alpha1.AggregatedStatusItem, error) {
	workList := &workv1alpha1.WorkList{}
	if err := c.List(context.TODO(), workList, &client.ListOptions{LabelSelector: selector}); err != nil {
		return nil, err
	}

	statuses := make([]workv1alpha1.AggregatedStatusItem, 0)
	for _, work := range workList.Items {
		identifierIndex, err := karmadactlutil.GetManifestIndex(work.Spec.Workload.Manifests, workload)
		if err != nil {
			klog.Errorf("Failed to get manifestIndex of workload in work.Spec.Workload.Manifests. Error: %v.", err)
			return nil, err
		}
		clusterName, err := names.GetClusterName(work.Namespace)
		if err != nil {
			klog.Errorf("Failed to get clusterName from work namespace %s. Error: %v.", work.Namespace, err)
			return nil, err
		}

		// if sync work to member cluster failed, then set status back to resource binding.
		var applied bool
		var appliedMsg string
		if cond := meta.FindStatusCondition(work.Status.Conditions, workv1alpha1.WorkApplied); cond != nil {
			switch cond.Status {
			case metav1.ConditionTrue:
				applied = true
			case metav1.ConditionUnknown:
				fallthrough
			case metav1.ConditionFalse:
				applied = false
				appliedMsg = cond.Message
			default: // should not happen unless the condition api changed.
				panic("unexpected status")
			}
		}
		if !applied {
			aggregatedStatus := workv1alpha1.AggregatedStatusItem{
				ClusterName:    clusterName,
				Applied:        applied,
				AppliedMessage: appliedMsg,
			}
			statuses = append(statuses, aggregatedStatus)
			return statuses, nil
		}

		for _, manifestStatus := range work.Status.ManifestStatuses {
			equal, err := equalIdentifier(&manifestStatus.Identifier, identifierIndex, workload)
			if err != nil {
				return nil, err
			}
			if equal {
				aggregatedStatus := workv1alpha1.AggregatedStatusItem{
					ClusterName: clusterName,
					Status:      manifestStatus.Status,
					Applied:     applied,
				}
				statuses = append(statuses, aggregatedStatus)
				break
			}
		}
	}

	return statuses, nil
}

func equalIdentifier(targetIdentifier *workv1alpha1.ResourceIdentifier, ordinal int, workload *unstructured.Unstructured) (bool, error) {
	groupVersion, err := schema.ParseGroupVersion(workload.GetAPIVersion())
	if err != nil {
		return false, err
	}

	if targetIdentifier.Ordinal == ordinal &&
		targetIdentifier.Group == groupVersion.Group &&
		targetIdentifier.Version == groupVersion.Version &&
		targetIdentifier.Kind == workload.GetKind() &&
		targetIdentifier.Namespace == workload.GetNamespace() &&
		targetIdentifier.Name == workload.GetName() {
		return true, nil
	}

	return false, nil
}

// isBindingReady will check if resourceBinding/clusterResourceBinding is ready to build Work.
func isBindingReady(targetClusters []workv1alpha1.TargetCluster) bool {
	return len(targetClusters) != 0
}

// hasScheduledReplica checks if the scheduler has assigned replicas for each cluster.
func hasScheduledReplica(scheduleResult []workv1alpha1.TargetCluster) bool {
	for _, clusterResult := range scheduleResult {
		if clusterResult.Replicas > 0 {
			return true
		}
	}
	return false
}

// getBindingClusterNames will get clusterName list from bind clusters field
func getBindingClusterNames(targetClusters []workv1alpha1.TargetCluster) []string {
	var clusterNames []string
	for _, targetCluster := range targetClusters {
		clusterNames = append(clusterNames, targetCluster.Name)
	}
	return clusterNames
}

// findOrphanWorks retrieves all works that labeled with current binding(ResourceBinding or ClusterResourceBinding) objects,
// then pick the works that not meet current binding declaration.
func findOrphanWorks(c client.Client, bindingNamespace, bindingName string, clusterNames []string, scope apiextensionsv1.ResourceScope) ([]workv1alpha1.Work, error) {
	workList := &workv1alpha1.WorkList{}
	if scope == apiextensionsv1.NamespaceScoped {
		selector := labels.SelectorFromSet(labels.Set{
			workv1alpha1.ResourceBindingNamespaceLabel: bindingNamespace,
			workv1alpha1.ResourceBindingNameLabel:      bindingName,
		})

		if err := c.List(context.TODO(), workList, &client.ListOptions{LabelSelector: selector}); err != nil {
			return nil, err
		}
	} else {
		selector := labels.SelectorFromSet(labels.Set{
			workv1alpha1.ClusterResourceBindingLabel: bindingName,
		})

		if err := c.List(context.TODO(), workList, &client.ListOptions{LabelSelector: selector}); err != nil {
			return nil, err
		}
	}

	var orphanWorks []workv1alpha1.Work
	expectClusters := sets.NewString(clusterNames...)
	for _, work := range workList.Items {
		workTargetCluster, err := names.GetClusterName(work.GetNamespace())
		if err != nil {
			klog.Errorf("Failed to get cluster name which Work %s/%s belongs to. Error: %v.",
				work.GetNamespace(), work.GetName(), err)
			return nil, err
		}
		if !expectClusters.Has(workTargetCluster) {
			orphanWorks = append(orphanWorks, work)
		}
	}
	return orphanWorks, nil
}

// removeOrphanWorks will remove orphan works.
func removeOrphanWorks(c client.Client, works []workv1alpha1.Work) error {
	for workIndex, work := range works {
		err := c.Delete(context.TODO(), &works[workIndex])
		if err != nil {
			return err
		}
		klog.Infof("Delete orphan work %s/%s successfully.", work.GetNamespace(), work.GetName())
	}
	return nil
}

// deleteWorks will delete all Work objects by labels.
func deleteWorks(c client.Client, selector labels.Set) (controllerruntime.Result, error) {
	workList, err := helper.GetWorks(c, selector)
	if err != nil {
		klog.Errorf("Failed to get works by label %v: %v", selector, err)
		return controllerruntime.Result{Requeue: true}, err
	}

	var errs []error
	for index, work := range workList.Items {
		if err := c.Delete(context.TODO(), &workList.Items[index]); err != nil {
			klog.Errorf("Failed to delete work(%s/%s): %v", work.Namespace, work.Name, err)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return controllerruntime.Result{Requeue: true}, errors.NewAggregate(errs)
	}

	return controllerruntime.Result{}, nil
}
