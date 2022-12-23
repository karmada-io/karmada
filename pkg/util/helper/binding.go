package helper

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// ClusterWeightInfo records the weight of a cluster
type ClusterWeightInfo struct {
	ClusterName string
	Weight      int64
}

// ClusterWeightInfoList is a slice of ClusterWeightInfo that implements sort.Interface to sort by Value.
type ClusterWeightInfoList []ClusterWeightInfo

func (p ClusterWeightInfoList) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p ClusterWeightInfoList) Len() int      { return len(p) }
func (p ClusterWeightInfoList) Less(i, j int) bool {
	if p[i].Weight != p[j].Weight {
		return p[i].Weight > p[j].Weight
	}
	return p[i].ClusterName < p[j].ClusterName
}

// SortClusterByWeight sort clusters by the weight
func SortClusterByWeight(m map[string]int64) ClusterWeightInfoList {
	p := make(ClusterWeightInfoList, len(m))
	i := 0
	for k, v := range m {
		p[i] = ClusterWeightInfo{k, v}
		i++
	}
	sort.Sort(p)
	return p
}

// GetWeightSum returns the sum of the weight info.
func (p ClusterWeightInfoList) GetWeightSum() int64 {
	var res int64
	for i := range p {
		res += p[i].Weight
	}
	return res
}

// Dispenser aims to divide replicas among clusters by different weights.
type Dispenser struct {
	// Target replicas, should be a positive integer.
	NumReplicas int32
	// Final result.
	Result []workv1alpha2.TargetCluster
}

// NewDispenser will construct a dispenser with target replicas and a prescribed initial result.
func NewDispenser(numReplicas int32, init []workv1alpha2.TargetCluster) *Dispenser {
	cp := make([]workv1alpha2.TargetCluster, len(init))
	copy(cp, init)
	return &Dispenser{NumReplicas: numReplicas, Result: cp}
}

// Done indicates whether finish dispensing.
func (a *Dispenser) Done() bool {
	return a.NumReplicas == 0 && len(a.Result) != 0
}

// TakeByWeight divide replicas by a weight list and merge the result into previous result.
func (a *Dispenser) TakeByWeight(w ClusterWeightInfoList) {
	if a.Done() {
		return
	}
	sum := w.GetWeightSum()
	if sum == 0 {
		return
	}

	sort.Sort(w)

	result := make([]workv1alpha2.TargetCluster, 0, w.Len())
	remain := a.NumReplicas
	for _, info := range w {
		replicas := int32(info.Weight * int64(a.NumReplicas) / sum)
		result = append(result, workv1alpha2.TargetCluster{
			Name:     info.ClusterName,
			Replicas: replicas,
		})
		remain -= replicas
	}
	// TODO(Garrybest): take rest replicas by fraction part
	for i := range result {
		if remain == 0 {
			break
		}
		result[i].Replicas++
		remain--
	}

	a.NumReplicas = remain
	a.Result = util.MergeTargetClusters(a.Result, result)
}

// GetStaticWeightInfoListByTargetClusters constructs a weight list by target cluster slice.
func GetStaticWeightInfoListByTargetClusters(tcs []workv1alpha2.TargetCluster) ClusterWeightInfoList {
	weightList := make(ClusterWeightInfoList, 0, len(tcs))
	for _, result := range tcs {
		weightList = append(weightList, ClusterWeightInfo{
			ClusterName: result.Name,
			Weight:      int64(result.Replicas),
		})
	}
	return weightList
}

// SpreadReplicasByTargetClusters divides replicas by the weight of a target cluster list.
func SpreadReplicasByTargetClusters(numReplicas int32, tcs, init []workv1alpha2.TargetCluster) []workv1alpha2.TargetCluster {
	weightList := GetStaticWeightInfoListByTargetClusters(tcs)
	disp := NewDispenser(numReplicas, init)
	disp.TakeByWeight(weightList)
	return disp.Result
}

// IsBindingScheduled will check if resourceBinding/clusterResourceBinding is successfully scheduled.
func IsBindingScheduled(status *workv1alpha2.ResourceBindingStatus) bool {
	return meta.IsStatusConditionTrue(status.Conditions, workv1alpha2.Scheduled)
}

// HasScheduledReplica checks if the scheduler has assigned replicas for a cluster.
func HasScheduledReplica(scheduleResult []workv1alpha2.TargetCluster) bool {
	for _, clusterResult := range scheduleResult {
		if clusterResult.Replicas > 0 {
			return true
		}
	}
	return false
}

// ObtainBindingSpecExistingClusters will obtain the cluster slice existing in the binding's spec field.
func ObtainBindingSpecExistingClusters(bindingSpec workv1alpha2.ResourceBindingSpec) sets.String {
	clusterNames := util.ConvertToClusterNames(bindingSpec.Clusters)
	for _, binding := range bindingSpec.RequiredBy {
		for _, targetCluster := range binding.Clusters {
			clusterNames.Insert(targetCluster.Name)
		}
	}

	for _, task := range bindingSpec.GracefulEvictionTasks {
		clusterNames.Insert(task.FromCluster)
	}

	return clusterNames
}

// FindOrphanWorks retrieves all works that labeled with current binding(ResourceBinding or ClusterResourceBinding) objects,
// then pick the works that not meet current binding declaration.
func FindOrphanWorks(c client.Client, bindingNamespace, bindingName string, expectClusters sets.String) ([]workv1alpha1.Work, error) {
	var needJudgeWorks []workv1alpha1.Work
	workList, err := GetWorksByBindingNamespaceName(c, bindingNamespace, bindingName)
	if err != nil {
		klog.Errorf("Failed to get works by binding object (%s/%s): %v", bindingNamespace, bindingName, err)
		return nil, err
	}
	needJudgeWorks = append(needJudgeWorks, workList.Items...)

	var orphanWorks []workv1alpha1.Work
	for _, work := range needJudgeWorks {
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

// RemoveOrphanWorks will remove orphan works.
func RemoveOrphanWorks(c client.Client, works []workv1alpha1.Work) error {
	var errs []error
	for workIndex, work := range works {
		err := c.Delete(context.TODO(), &works[workIndex])
		if err != nil {
			klog.Errorf("Failed to delete orphan work %s/%s, err is %v", work.GetNamespace(), work.GetName(), err)
			errs = append(errs, err)
			continue
		}
		klog.Infof("Delete orphan work %s/%s successfully.", work.GetNamespace(), work.GetName())
	}
	return errors.NewAggregate(errs)
}

// FetchWorkload fetches the kubernetes resource to be propagated.
func FetchWorkload(dynamicClient dynamic.Interface, informerManager genericmanager.SingleClusterInformerManager,
	restMapper meta.RESTMapper, resource workv1alpha2.ObjectReference) (*unstructured.Unstructured, error) {
	dynamicResource, err := restmapper.GetGroupVersionResource(restMapper,
		schema.FromAPIVersionAndKind(resource.APIVersion, resource.Kind))
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK %s %s. Error: %v", resource.APIVersion,
			resource.Kind, err)
		return nil, err
	}

	var workload runtime.Object

	if len(resource.Namespace) == 0 {
		// cluster-scoped resource
		workload, err = informerManager.Lister(dynamicResource).Get(resource.Name)
	} else {
		workload, err = informerManager.Lister(dynamicResource).ByNamespace(resource.Namespace).Get(resource.Name)
	}
	if err != nil {
		// fall back to call api server in case the cache has not been synchronized yet
		klog.Warningf("Failed to get workload from cache, kind: %s, namespace: %s, name: %s. Error: %v. Fall back to call api server",
			resource.Kind, resource.Namespace, resource.Name, err)
		workload, err = dynamicClient.Resource(dynamicResource).Namespace(resource.Namespace).Get(context.TODO(),
			resource.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Failed to get workload from api server, kind: %s, namespace: %s, name: %s. Error: %v",
				resource.Kind, resource.Namespace, resource.Name, err)
			return nil, err
		}
	}

	unstructuredWorkLoad, err := ToUnstructured(workload)
	if err != nil {
		klog.Errorf("Failed to transform object(%s/%s): %v", resource.Namespace, resource.Name, err)
		return nil, err
	}

	return unstructuredWorkLoad, nil
}

// GetClusterResourceBindings returns a ClusterResourceBindingList by labels.
func GetClusterResourceBindings(c client.Client, ls labels.Set) (*workv1alpha2.ClusterResourceBindingList, error) {
	bindings := &workv1alpha2.ClusterResourceBindingList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(ls)}

	return bindings, c.List(context.TODO(), bindings, listOpt)
}

// GetResourceBindings returns a ResourceBindingList by labels
func GetResourceBindings(c client.Client, ls labels.Set) (*workv1alpha2.ResourceBindingList, error) {
	bindings := &workv1alpha2.ResourceBindingList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(ls)}

	return bindings, c.List(context.TODO(), bindings, listOpt)
}

// DeleteWorkByRBNamespaceAndName will delete all Work objects by ResourceBinding namespace and name.
func DeleteWorkByRBNamespaceAndName(c client.Client, namespace, name string) error {
	return DeleteWorks(c, namespace, name)
}

// DeleteWorkByCRBName will delete all Work objects by ClusterResourceBinding name.
func DeleteWorkByCRBName(c client.Client, name string) error {
	return DeleteWorks(c, "", name)
}

// DeleteWorks will delete all Work objects by labels.
func DeleteWorks(c client.Client, namespace, name string) error {
	workList, err := GetWorksByBindingNamespaceName(c, namespace, name)
	if err != nil {
		klog.Errorf("Failed to get works by ResourceBinding(%s/%s) : %v", namespace, name, err)
		return err
	}

	var errs []error
	for index, work := range workList.Items {
		if err := c.Delete(context.TODO(), &workList.Items[index]); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			klog.Errorf("Failed to delete work(%s/%s): %v", work.Namespace, work.Name, err)
			errs = append(errs, err)
		}
	}
	return errors.NewAggregate(errs)
}

// GenerateNodeClaimByPodSpec will return a NodeClaim from PodSpec.
func GenerateNodeClaimByPodSpec(podSpec *corev1.PodSpec) *workv1alpha2.NodeClaim {
	nodeClaim := &workv1alpha2.NodeClaim{
		NodeSelector: podSpec.NodeSelector,
		Tolerations:  podSpec.Tolerations,
	}
	hasAffinity := podSpec.Affinity != nil && podSpec.Affinity.NodeAffinity != nil && podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil
	if hasAffinity {
		nodeClaim.HardNodeAffinity = podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	}
	if nodeClaim.NodeSelector == nil && nodeClaim.HardNodeAffinity == nil && len(nodeClaim.Tolerations) == 0 {
		return nil
	}
	return nodeClaim
}

// GenerateReplicaRequirements generates replica requirements for node and resources.
func GenerateReplicaRequirements(podTemplate *corev1.PodTemplateSpec) *workv1alpha2.ReplicaRequirements {
	nodeClaim := GenerateNodeClaimByPodSpec(&podTemplate.Spec)
	resourceRequest := util.EmptyResource().AddPodTemplateRequest(&podTemplate.Spec).ResourceList()

	if nodeClaim != nil || resourceRequest != nil {
		return &workv1alpha2.ReplicaRequirements{
			NodeClaim:       nodeClaim,
			ResourceRequest: resourceRequest,
		}
	}

	return nil
}

// ConstructClusterWideKey construct resource ClusterWideKey from binding's objectReference.
func ConstructClusterWideKey(resource workv1alpha2.ObjectReference) (keys.ClusterWideKey, error) {
	gv, err := schema.ParseGroupVersion(resource.APIVersion)
	if err != nil {
		return keys.ClusterWideKey{}, err
	}
	return keys.ClusterWideKey{
		Group:     gv.Group,
		Version:   gv.Version,
		Kind:      resource.Kind,
		Namespace: resource.Namespace,
		Name:      resource.Name,
	}, nil
}

// EmitClusterEvictionEventForResourceBinding records the eviction event for resourceBinding and its objectReference.
func EmitClusterEvictionEventForResourceBinding(binding *workv1alpha2.ResourceBinding, cluster string, eventRecorder record.EventRecorder, err error) {
	if binding == nil {
		return
	}

	ref := &corev1.ObjectReference{
		Kind:       binding.Spec.Resource.Kind,
		APIVersion: binding.Spec.Resource.APIVersion,
		Namespace:  binding.Spec.Resource.Namespace,
		Name:       binding.Spec.Resource.Name,
		UID:        binding.Spec.Resource.UID,
	}

	if err != nil {
		eventRecorder.Eventf(binding, corev1.EventTypeWarning, events.EventReasonEvictWorkloadFromClusterFailed, "Evict from cluster %s failed.", cluster)
		eventRecorder.Eventf(ref, corev1.EventTypeWarning, events.EventReasonEvictWorkloadFromClusterFailed, "Evict from cluster %s failed.", cluster)
	} else {
		eventRecorder.Eventf(binding, corev1.EventTypeNormal, events.EventReasonEvictWorkloadFromClusterSucceed, "Evict from cluster %s succeed.", cluster)
		eventRecorder.Eventf(ref, corev1.EventTypeNormal, events.EventReasonEvictWorkloadFromClusterSucceed, "Evict from cluster %s succeed.", cluster)
	}
}

// EmitClusterEvictionEventForClusterResourceBinding records the eviction event for clusterResourceBinding and its objectReference.
func EmitClusterEvictionEventForClusterResourceBinding(binding *workv1alpha2.ClusterResourceBinding, cluster string, eventRecorder record.EventRecorder, err error) {
	if binding == nil {
		return
	}

	ref := &corev1.ObjectReference{
		Kind:       binding.Spec.Resource.Kind,
		APIVersion: binding.Spec.Resource.APIVersion,
		Namespace:  binding.Spec.Resource.Namespace,
		Name:       binding.Spec.Resource.Name,
		UID:        binding.Spec.Resource.UID,
	}

	if err != nil {
		eventRecorder.Eventf(binding, corev1.EventTypeWarning, events.EventReasonEvictWorkloadFromClusterFailed, "Evict from cluster %s failed.", cluster)
		eventRecorder.Eventf(ref, corev1.EventTypeWarning, events.EventReasonEvictWorkloadFromClusterFailed, "Evict from cluster %s failed.", cluster)
	} else {
		eventRecorder.Eventf(binding, corev1.EventTypeNormal, events.EventReasonEvictWorkloadFromClusterSucceed, "Evict from cluster %s succeed.", cluster)
		eventRecorder.Eventf(ref, corev1.EventTypeNormal, events.EventReasonEvictWorkloadFromClusterSucceed, "Evict from cluster %s succeed.", cluster)
	}
}
