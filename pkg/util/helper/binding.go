/*
Copyright 2021 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helper

import (
	"context"
	"crypto/rand"
	"math/big"
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

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// ClusterWeightInfo records the weight of a cluster
type ClusterWeightInfo struct {
	ClusterName  string
	Weight       int64
	LastReplicas int32
}

// ClusterWeightInfoList is a slice of ClusterWeightInfo that implements sort.Interface to sort by Value.
type ClusterWeightInfoList []ClusterWeightInfo

func (p ClusterWeightInfoList) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p ClusterWeightInfoList) Len() int      { return len(p) }
func (p ClusterWeightInfoList) Less(i, j int) bool {
	if p[i].Weight != p[j].Weight {
		return p[i].Weight > p[j].Weight
	}
	// when weights is equal, sort by last scheduling replicas result,
	// more last scheduling replicas means the remainders of the last scheduling were randomized to such clusters,
	// so in order to keep the inertia in this scheduling, such clusters should also be prioritized
	if p[i].LastReplicas != p[j].LastReplicas {
		return p[i].LastReplicas > p[j].LastReplicas
	}
	// when last scheduling replicas is also equal, sort by random,
	// first generate a random number within [0, 100) range,
	// then return < if the actual number is in [0, 50) range, return > if is in [50, 100) range
	const maxRandomNum = 100
	randomNum, err := rand.Int(rand.Reader, big.NewInt(maxRandomNum))
	return err == nil && randomNum.Cmp(big.NewInt(maxRandomNum/2)) >= 0
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
		replicas := int32(info.Weight * int64(a.NumReplicas) / sum) // #nosec G115: integer overflow conversion int64 -> int32
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
func GetStaticWeightInfoListByTargetClusters(tcs, scheduled []workv1alpha2.TargetCluster) ClusterWeightInfoList {
	weightList := make(ClusterWeightInfoList, 0, len(tcs))
	for _, targetCluster := range tcs {
		var lastReplicas int32
		for _, scheduledCluster := range scheduled {
			if targetCluster.Name == scheduledCluster.Name {
				lastReplicas = scheduledCluster.Replicas
				break
			}
		}
		weightList = append(weightList, ClusterWeightInfo{
			ClusterName:  targetCluster.Name,
			Weight:       int64(targetCluster.Replicas),
			LastReplicas: lastReplicas,
		})
	}
	return weightList
}

// SpreadReplicasByTargetClusters divides replicas by the weight of a target cluster list.
func SpreadReplicasByTargetClusters(numReplicas int32, tcs, init []workv1alpha2.TargetCluster) []workv1alpha2.TargetCluster {
	weightList := GetStaticWeightInfoListByTargetClusters(tcs, init)
	disp := NewDispenser(numReplicas, init)
	disp.TakeByWeight(weightList)
	return disp.Result
}

// IsBindingScheduled will check if resourceBinding/clusterResourceBinding is successfully scheduled.
func IsBindingScheduled(status *workv1alpha2.ResourceBindingStatus) bool {
	return meta.IsStatusConditionTrue(status.Conditions, workv1alpha2.Scheduled)
}

// ObtainBindingSpecExistingClusters will obtain the cluster slice existing in the binding's spec field.
func ObtainBindingSpecExistingClusters(bindingSpec workv1alpha2.ResourceBindingSpec) sets.Set[string] {
	clusterNames := util.ConvertToClusterNames(bindingSpec.Clusters)
	for _, binding := range bindingSpec.RequiredBy {
		for _, targetCluster := range binding.Clusters {
			clusterNames.Insert(targetCluster.Name)
		}
	}

	for _, task := range bindingSpec.GracefulEvictionTasks {
		// EvictionTasks with Immediately PurgeMode should not be treated as existing clusters
		// Work on those clusters should be removed immediately and treated as orphans
		if task.PurgeMode != policyv1alpha1.Immediately {
			clusterNames.Insert(task.FromCluster)
		}
	}

	return clusterNames
}

// FindOrphanWorks retrieves all works that labeled with current binding(ResourceBinding or ClusterResourceBinding) objects,
// then pick the works that not meet current binding declaration.
func FindOrphanWorks(ctx context.Context, c client.Client, bindingNamespace, bindingName, bindingID string, expectClusters sets.Set[string]) ([]workv1alpha1.Work, error) {
	var needJudgeWorks []workv1alpha1.Work
	workList, err := GetWorksByBindingID(ctx, c, bindingID, bindingNamespace != "")
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
func RemoveOrphanWorks(ctx context.Context, c client.Client, works []workv1alpha1.Work) error {
	var errs []error
	for workIndex, work := range works {
		err := c.Delete(ctx, &works[workIndex])
		if err != nil {
			klog.Errorf("Failed to delete orphan work %s/%s, err is %v", work.GetNamespace(), work.GetName(), err)
			errs = append(errs, err)
			continue
		}
		klog.Infof("Delete orphan work %s/%s successfully.", work.GetNamespace(), work.GetName())
	}
	return errors.NewAggregate(errs)
}

// FetchResourceTemplate fetches the resource template to be propagated.
// Any updates to this resource template are not recommended as it may come from the informer cache.
// We should abide by the principle of making a deep copy first and then modifying it.
// See issue: https://github.com/karmada-io/karmada/issues/3878.
func FetchResourceTemplate(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	informerManager genericmanager.SingleClusterInformerManager,
	restMapper meta.RESTMapper,
	resource workv1alpha2.ObjectReference,
) (*unstructured.Unstructured, error) {
	gvr, err := restmapper.GetGroupVersionResource(restMapper, schema.FromAPIVersionAndKind(resource.APIVersion, resource.Kind))
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK(%s/%s), Error: %v", resource.APIVersion, resource.Kind, err)
		return nil, err
	}

	var object runtime.Object

	if len(resource.Namespace) == 0 {
		// cluster-scoped resource
		object, err = informerManager.Lister(gvr).Get(resource.Name)
	} else {
		object, err = informerManager.Lister(gvr).ByNamespace(resource.Namespace).Get(resource.Name)
	}
	if err != nil {
		// fall back to call api server in case the cache has not been synchronized yet
		klog.Warningf("Failed to get resource template (%s/%s/%s) from cache, Error: %v. Fall back to call api server.",
			resource.Kind, resource.Namespace, resource.Name, err)
		object, err = dynamicClient.Resource(gvr).Namespace(resource.Namespace).Get(ctx, resource.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Failed to get resource template (%s/%s/%s) from api server, Error: %v",
				resource.Kind, resource.Namespace, resource.Name, err)
			return nil, err
		}
	}

	unstructuredObj, err := ToUnstructured(object)
	if err != nil {
		klog.Errorf("Failed to transform object(%s/%s), Error: %v", resource.Namespace, resource.Name, err)
		return nil, err
	}

	return unstructuredObj, nil
}

// FetchResourceTemplatesByLabelSelector fetches the resource templates by label selector to be propagated.
// Any updates to this resource template are not recommended as it may come from the informer cache.
// We should abide by the principle of making a deep copy first and then modifying it.
// See issue: https://github.com/karmada-io/karmada/issues/3878.
func FetchResourceTemplatesByLabelSelector(
	dynamicClient dynamic.Interface,
	informerManager genericmanager.SingleClusterInformerManager,
	restMapper meta.RESTMapper,
	resource workv1alpha2.ObjectReference,
	selector labels.Selector,
) ([]*unstructured.Unstructured, error) {
	gvr, err := restmapper.GetGroupVersionResource(restMapper, schema.FromAPIVersionAndKind(resource.APIVersion, resource.Kind))
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK(%s/%s), Error: %v", resource.APIVersion, resource.Kind, err)
		return nil, err
	}
	var objectList []runtime.Object
	if len(resource.Namespace) == 0 {
		// cluster-scoped resource
		objectList, err = informerManager.Lister(gvr).List(selector)
	} else {
		objectList, err = informerManager.Lister(gvr).ByNamespace(resource.Namespace).List(selector)
	}
	var objects []*unstructured.Unstructured
	if err != nil || len(objectList) == 0 {
		// fall back to call api server in case the cache has not been synchronized yet
		klog.Warningf("Failed to get resource template (%s/%s/%s) from cache, Error: %v. Fall back to call api server.",
			resource.Kind, resource.Namespace, resource.Name, err)
		unstructuredList, err := dynamicClient.Resource(gvr).Namespace(resource.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			klog.Errorf("Failed to get resource template (%s/%s/%s) from api server, Error: %v",
				resource.Kind, resource.Namespace, resource.Name, err)
			return nil, err
		}
		for i := range unstructuredList.Items {
			objects = append(objects, &unstructuredList.Items[i])
		}
	}

	for i := range objectList {
		unstructuredObj, err := ToUnstructured(objectList[i])
		if err != nil {
			klog.Errorf("Failed to transform object(%s/%s), Error: %v", resource.Namespace, resource.Name, err)
			return nil, err
		}
		objects = append(objects, unstructuredObj)
	}
	return objects, nil
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

// GetResourceBindingsByNamespace returns a ResourceBindingList by namespace
func GetResourceBindingsByNamespace(c client.Client, namespace string) (*workv1alpha2.ResourceBindingList, error) {
	bindings := &workv1alpha2.ResourceBindingList{}
	listOpt := &client.ListOptions{Namespace: namespace}

	return bindings, c.List(context.TODO(), bindings, listOpt)
}

// DeleteWorks will delete all Work objects by labels.
func DeleteWorks(ctx context.Context, c client.Client, namespace, name, bindingID string) error {
	workList, err := GetWorksByBindingID(ctx, c, bindingID, namespace != "")
	if err != nil {
		klog.Errorf("Failed to get works by (Cluster)ResourceBinding(%s/%s) : %v", namespace, name, err)
		return err
	}

	var errs []error
	for index, work := range workList.Items {
		if err := c.Delete(ctx, &workList.Items[index]); err != nil {
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
		replicaRequirements := &workv1alpha2.ReplicaRequirements{
			NodeClaim:       nodeClaim,
			ResourceRequest: resourceRequest,
		}
		if features.FeatureGate.Enabled(features.ResourceQuotaEstimate) {
			replicaRequirements.Namespace = podTemplate.Namespace
			// PriorityClassName is set from podTemplate
			// If it is not set from podTemplate, it is default to an empty string
			replicaRequirements.PriorityClassName = podTemplate.Spec.PriorityClassName
		}
		return replicaRequirements
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

// ConstructObjectReference constructs ObjectReference from ResourceSelector.
func ConstructObjectReference(rs policyv1alpha1.ResourceSelector) workv1alpha2.ObjectReference {
	return workv1alpha2.ObjectReference{
		APIVersion: rs.APIVersion,
		Kind:       rs.Kind,
		Namespace:  rs.Namespace,
		Name:       rs.Name,
	}
}
