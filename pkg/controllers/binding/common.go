package binding

import (
	"context"
	"reflect"
	"sort"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
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
			Name:        names.GenerateWorkName(clonedWorkload.GetKind(), clonedWorkload.GetName(), clonedWorkload.GetNamespace()),
			Namespace:   workNamespace,
			Finalizers:  []string{util.ExecutionControllerFinalizer},
			Labels:      workLabel,
			Annotations: annotations,
		}

		if err = helper.CreateOrUpdateWork(c, workMeta, clonedWorkload); err != nil {
			return err
		}
	}
	return nil
}

func getRSPAndReplicaInfos(c client.Client, workload *unstructured.Unstructured, targetClusters []workv1alpha1.TargetCluster) (bool, *v1alpha1.ReplicaSchedulingPolicy, map[string]int64, error) {
	if helper.HasScheduledReplica(targetClusters) {
		return true, nil, transScheduleResultToMap(targetClusters), nil
	}

	referenceRSP, desireReplicaInfos, err := calculateReplicasIfNeeded(c, workload, helper.GetBindingClusterNames(targetClusters))
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
	util.MergeLabel(workload, workv1alpha1.WorkNameLabel, names.GenerateWorkName(workload.GetKind(), workload.GetName(), workload.GetNamespace()))

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
