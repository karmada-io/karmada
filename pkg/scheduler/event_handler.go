/*
Copyright 2022 The Karmada Authors.

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

package scheduler

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	internalqueue "github.com/karmada-io/karmada/pkg/scheduler/internal/queue"
	"github.com/karmada-io/karmada/pkg/scheduler/metrics"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// addAllEventHandlers is a helper function used in Scheduler
// to add event handlers for various informers.
func (s *Scheduler) addAllEventHandlers() {
	bindingInformer := s.informerFactory.Work().V1alpha2().ResourceBindings().Informer()
	_, err := bindingInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: s.resourceBindingEventFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    s.onResourceBindingAdd,
			UpdateFunc: s.onResourceBindingUpdate,
			DeleteFunc: s.onResourceBindingDelete,
		},
	})
	if err != nil {
		klog.Errorf("Failed to add handlers for ResourceBindings: %v", err)
	}

	clusterBindingInformer := s.informerFactory.Work().V1alpha2().ClusterResourceBindings().Informer()
	_, err = clusterBindingInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: s.resourceBindingEventFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    s.onClusterResourceBindingAdd,
			UpdateFunc: s.onClusterResourceBindingUpdate,
		},
	})
	if err != nil {
		klog.Errorf("Failed to add handlers for ClusterResourceBindings: %v", err)
	}

	memClusterInformer := s.informerFactory.Cluster().V1alpha1().Clusters().Informer()
	_, err = memClusterInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    s.addCluster,
			UpdateFunc: s.updateCluster,
			DeleteFunc: s.deleteCluster,
		},
	)
	if err != nil {
		klog.Errorf("Failed to add handlers for Clusters: %v", err)
	}

	// ignore the error here because the informers haven't been started
	_ = bindingInformer.SetTransform(fedinformer.StripUnusedFields)
	_ = clusterBindingInformer.SetTransform(fedinformer.StripUnusedFields)
	_ = memClusterInformer.SetTransform(fedinformer.StripUnusedFields)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: s.KubeClient.CoreV1().Events(metav1.NamespaceAll)})
	s.eventRecorder = eventBroadcaster.NewRecorder(gclient.NewSchema(), corev1.EventSource{Component: s.schedulerName})
}

func (s *Scheduler) resourceBindingEventFilter(obj any) bool {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return false
	}

	switch t := obj.(type) {
	case *workv1alpha2.ResourceBinding:
		if !schedulerNameFilter(s.schedulerName, t.Spec.SchedulerName) {
			return false
		}
		if t.Spec.SchedulingSuspended() {
			return false
		}
	case *workv1alpha2.ClusterResourceBinding:
		if !schedulerNameFilter(s.schedulerName, t.Spec.SchedulerName) {
			return false
		}
		if t.Spec.SchedulingSuspended() {
			return false
		}
	}

	return util.GetLabelValue(accessor.GetLabels(), policyv1alpha1.PropagationPolicyPermanentIDLabel) != "" ||
		util.GetLabelValue(accessor.GetLabels(), policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel) != "" ||
		util.GetLabelValue(accessor.GetLabels(), workv1alpha2.BindingManagedByLabel) != ""
}

// newQueuedBindingInfoFromRB converts a ResourceBinding to a QueuedBindingInfo.
func newQueuedBindingInfoFromRB(rb *workv1alpha2.ResourceBinding) *internalqueue.QueuedBindingInfo {
	return &internalqueue.QueuedBindingInfo{
		NamespacedKey: cache.ObjectName{Namespace: rb.GetNamespace(), Name: rb.GetName()}.String(),
		Priority:      rb.Spec.SchedulePriorityValue(),
	}
}

// newQueuedBindingInfoFromCRB converts a ClusterResourceBinding to a QueuedBindingInfo.
func newQueuedBindingInfoFromCRB(crb *workv1alpha2.ClusterResourceBinding) *internalqueue.QueuedBindingInfo {
	return &internalqueue.QueuedBindingInfo{
		NamespacedKey: crb.GetName(),
		Priority:      crb.Spec.SchedulePriorityValue(),
	}
}

// onResourceBindingAdd is called when an add event for a ResourceBinding is received from the Informer.
func (s *Scheduler) onResourceBindingAdd(obj any) {
	rb, ok := obj.(*workv1alpha2.ResourceBinding)
	if !ok {
		klog.Errorf("onResourceBindingAdd: unexpected object type %T", obj)
		return
	}

	if features.FeatureGate.Enabled(features.PriorityBasedScheduling) {
		s.priorityQueue.Push(newQueuedBindingInfoFromRB(rb))
	} else {
		key, err := cache.MetaNamespaceKeyFunc(rb)
		if err != nil {
			klog.Errorf("couldn't get key for object %#v: %v", rb, err)
			return
		}
		s.queue.Add(key)
	}
	metrics.CountSchedulerBindings(metrics.BindingAdd)
}

// onClusterResourceBindingAdd is called when an add event for a ClusterResourceBinding is received from the Informer.
func (s *Scheduler) onClusterResourceBindingAdd(obj any) {
	crb, ok := obj.(*workv1alpha2.ClusterResourceBinding)
	if !ok {
		klog.Errorf("onClusterResourceBindingAdd: unexpected object type %T", obj)
		return
	}

	if features.FeatureGate.Enabled(features.PriorityBasedScheduling) {
		s.priorityQueue.Push(newQueuedBindingInfoFromCRB(crb))
	} else {
		key, err := cache.MetaNamespaceKeyFunc(crb)
		if err != nil {
			klog.Errorf("couldn't get key for object %#v: %v", crb, err)
			return
		}
		s.queue.Add(key)
	}
	metrics.CountSchedulerBindings(metrics.BindingAdd)
}

// onResourceBindingUpdate is called when an update event for a ResourceBinding is received from the Informer.
func (s *Scheduler) onResourceBindingUpdate(old, cur any) {
	oldRB, ok1 := old.(*workv1alpha2.ResourceBinding)
	newRB, ok2 := cur.(*workv1alpha2.ResourceBinding)
	if !ok1 || !ok2 {
		klog.Errorf("onResourceBindingUpdate: unexpected object type, old=%T cur=%T", old, cur)
		return
	}

	if features.FeatureGate.Enabled(features.WorkloadAffinity) {
		s.schedulerCache.AssigningResourceBindings().OnBindingUpdate(newRB)
	}

	// Release per-cluster assumptions for any cluster whose workload just became Healthy.
	// This must run before the generation check below because AggregatedStatus is a status
	// field that does not bump the generation.
	s.releaseHealthyClusterAssumptions(oldRB, newRB)

	if oldRB.GetGeneration() == newRB.GetGeneration() {
		klog.V(4).Infof("Ignore update event of resourceBinding %s/%s as specification no change", newRB.GetNamespace(), newRB.GetName())
		return
	}

	if features.FeatureGate.Enabled(features.PriorityBasedScheduling) {
		s.priorityQueue.Push(newQueuedBindingInfoFromRB(newRB))
	} else {
		key, err := cache.MetaNamespaceKeyFunc(newRB)
		if err != nil {
			klog.Errorf("couldn't get key for object %#v: %v", newRB, err)
			return
		}
		s.queue.Add(key)
	}
	metrics.CountSchedulerBindings(metrics.BindingUpdate)
}

// onClusterResourceBindingUpdate is called when an update event for a ClusterResourceBinding is received from the Informer.
func (s *Scheduler) onClusterResourceBindingUpdate(old, cur any) {
	oldCRB, ok1 := old.(*workv1alpha2.ClusterResourceBinding)
	newCRB, ok2 := cur.(*workv1alpha2.ClusterResourceBinding)
	if !ok1 || !ok2 {
		klog.Errorf("onClusterResourceBindingUpdate: unexpected object type, old=%T cur=%T", old, cur)
		return
	}

	if oldCRB.GetGeneration() == newCRB.GetGeneration() {
		klog.V(4).Infof("Ignore update event of clusterResourceBinding %s as specification no change", newCRB.GetName())
		return
	}

	if features.FeatureGate.Enabled(features.PriorityBasedScheduling) {
		s.priorityQueue.Push(newQueuedBindingInfoFromCRB(newCRB))
	} else {
		key, err := cache.MetaNamespaceKeyFunc(newCRB)
		if err != nil {
			klog.Errorf("couldn't get key for object %#v: %v", newCRB, err)
			return
		}
		s.queue.Add(key)
	}
	metrics.CountSchedulerBindings(metrics.BindingUpdate)
}

// onResourceBindingDelete is called when a delete event for a ResourceBinding is received from the Informer.
func (s *Scheduler) onResourceBindingDelete(obj any) {
	if features.FeatureGate.Enabled(features.WorkloadAffinity) {
		var binding *workv1alpha2.ResourceBinding
		switch t := obj.(type) {
		case *workv1alpha2.ResourceBinding:
			binding = t
		case cache.DeletedFinalStateUnknown:
			var ok bool
			binding, ok = t.Obj.(*workv1alpha2.ResourceBinding)
			if !ok {
				klog.Errorf("onResourceBindingDelete: cannot convert DeletedFinalStateUnknown.Obj to *workv1alpha2.ResourceBinding: %v", t.Obj)
				return
			}
		default:
			klog.Errorf("onResourceBindingDelete: unexpected object type %T", obj)
			return
		}

		s.schedulerCache.AssigningResourceBindings().OnBindingDelete(binding)
	}
}

func (s *Scheduler) onResourceBindingRequeue(binding *workv1alpha2.ResourceBinding, event string) {
	klog.Infof("Requeue ResourceBinding(%s/%s) due to event(%s).", binding.Namespace, binding.Name, event)
	if features.FeatureGate.Enabled(features.PriorityBasedScheduling) {
		s.priorityQueue.Push(&internalqueue.QueuedBindingInfo{
			NamespacedKey: cache.ObjectName{Namespace: binding.Namespace, Name: binding.Name}.String(),
			Priority:      binding.Spec.SchedulePriorityValue(),
		})
	} else {
		key, err := cache.MetaNamespaceKeyFunc(binding)
		if err != nil {
			klog.Errorf("couldn't get key for ResourceBinding(%s/%s): %v", binding.Namespace, binding.Name, err)
			return
		}
		klog.Infof("Requeue ResourceBinding(%s/%s) due to event(%s).", binding.Namespace, binding.Name, event)
		s.queue.Add(key)
	}
	metrics.CountSchedulerBindings(event)
}

func (s *Scheduler) onClusterResourceBindingRequeue(clusterResourceBinding *workv1alpha2.ClusterResourceBinding, event string) {
	klog.Infof("Requeue ClusterResourceBinding(%s) due to event(%s).", clusterResourceBinding.Name, event)
	if features.FeatureGate.Enabled(features.PriorityBasedScheduling) {
		s.priorityQueue.Push(&internalqueue.QueuedBindingInfo{
			NamespacedKey: clusterResourceBinding.Name,
			Priority:      clusterResourceBinding.Spec.SchedulePriorityValue(),
		})
	} else {
		key, err := cache.MetaNamespaceKeyFunc(clusterResourceBinding)
		if err != nil {
			klog.Errorf("couldn't get key for ClusterResourceBinding(%s): %v", clusterResourceBinding.Name, err)
			return
		}
		klog.Infof("Requeue ClusterResourceBinding(%s) due to event(%s).", clusterResourceBinding.Name, event)
		s.queue.Add(key)
	}
	metrics.CountSchedulerBindings(event)
}

func (s *Scheduler) addCluster(obj any) {
	cluster, ok := obj.(*clusterv1alpha1.Cluster)
	if !ok {
		klog.Errorf("cannot convert to Cluster: %v", obj)
		return
	}
	klog.V(3).Infof("Add event for cluster %s", cluster.Name)
	if s.enableSchedulerEstimator {
		s.schedulerEstimatorWorker.Add(cluster.Name)
	}
}

func (s *Scheduler) updateCluster(oldObj, newObj any) {
	newCluster, ok := newObj.(*clusterv1alpha1.Cluster)
	if !ok {
		klog.Errorf("cannot convert newObj to Cluster: %v", newObj)
		return
	}
	oldCluster, ok := oldObj.(*clusterv1alpha1.Cluster)
	if !ok {
		klog.Errorf("cannot convert oldObj to Cluster: %v", oldObj)
		return
	}
	klog.V(3).Infof("Update event for cluster %s", newCluster.Name)

	if s.enableSchedulerEstimator {
		s.schedulerEstimatorWorker.Add(newCluster.Name)
	}

	switch {
	case oldCluster.DeletionTimestamp.IsZero() && !newCluster.DeletionTimestamp.IsZero():
		s.clusterReconcileWorker.Add(newCluster)
	case !equality.Semantic.DeepEqual(oldCluster.Labels, newCluster.Labels):
		fallthrough
	case oldCluster.Generation != newCluster.Generation:
		// To distinguish the old and new cluster objects, we need to add the entire object
		// to the worker. Therefore, call Add func instead of Enqueue func.
		s.clusterReconcileWorker.Add(oldCluster)
		s.clusterReconcileWorker.Add(newCluster)
	}
}

func (s *Scheduler) deleteCluster(obj any) {
	var cluster *clusterv1alpha1.Cluster
	switch t := obj.(type) {
	case *clusterv1alpha1.Cluster:
		cluster = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		cluster, ok = t.Obj.(*clusterv1alpha1.Cluster)
		if !ok {
			klog.Errorf("cannot convert to clusterv1alpha1.Cluster: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("cannot convert to clusterv1alpha1.Cluster: %v", t)
		return
	}

	klog.V(3).Infof("Delete event for cluster %s", cluster.Name)

	if s.enableSchedulerEstimator {
		s.schedulerEstimatorWorker.Add(cluster.Name)
	}
}

func schedulerNameFilter(schedulerNameFromOptions, schedulerName string) bool {
	if schedulerName == "" {
		schedulerName = DefaultScheduler
	}

	return schedulerNameFromOptions == schedulerName
}

func (s *Scheduler) reconcileCluster(key util.QueueKey) error {
	cluster, ok := key.(*clusterv1alpha1.Cluster)
	if !ok {
		return fmt.Errorf("invalid cluster key: %s", key)
	}
	return utilerrors.NewAggregate([]error{
		s.enqueueAffectedBindings(cluster),
		s.enqueueAffectedCRBs(cluster)},
	)
}

// enqueueAffectedBindings find all RBs related to the cluster and reschedule them
func (s *Scheduler) enqueueAffectedBindings(cluster *clusterv1alpha1.Cluster) error {
	klog.V(4).Infof("Enqueue affected ResourceBinding with cluster %s", cluster.Name)

	bindings, _ := s.bindingLister.List(labels.Everything())
	for _, binding := range bindings {
		placementPtr := binding.Spec.Placement
		if placementPtr == nil {
			// never reach here
			continue
		}
		if !schedulerNameFilter(s.schedulerName, binding.Spec.SchedulerName) {
			continue
		}
		if binding.Spec.SchedulingSuspended() {
			continue
		}

		var affinity *policyv1alpha1.ClusterAffinity
		if placementPtr.ClusterAffinities != nil {
			if binding.Status.SchedulerObservedGeneration != binding.Generation {
				// Hit here means the binding maybe still in the queue waiting
				// for scheduling or its status has not been synced to the
				// cache. Just enqueue the binding to avoid missing the cluster
				// update event.
				s.onResourceBindingRequeue(binding, metrics.ClusterChanged)
				continue
			}
			affinityIndex := getAffinityIndex(placementPtr.ClusterAffinities, binding.Status.SchedulerObservedAffinityName)
			affinity = &placementPtr.ClusterAffinities[affinityIndex].ClusterAffinity
		} else {
			affinity = placementPtr.ClusterAffinity
		}

		switch {
		case affinity == nil:
			// If no clusters specified, add it to the queue
			fallthrough
		case util.ClusterMatches(cluster, *affinity):
			// If the cluster manifest match the affinity, add it to the queue, trigger rescheduling
			s.onResourceBindingRequeue(binding, metrics.ClusterChanged)
		}
	}

	return nil
}

// enqueueAffectedCRBs find all CRBs related to the cluster and reschedule them
func (s *Scheduler) enqueueAffectedCRBs(cluster *clusterv1alpha1.Cluster) error {
	klog.V(4).Infof("Enqueue affected ClusterResourceBinding with cluster %s", cluster.Name)

	clusterBindings, _ := s.clusterBindingLister.List(labels.Everything())
	for _, binding := range clusterBindings {
		placementPtr := binding.Spec.Placement
		if placementPtr == nil {
			// never reach here
			continue
		}
		if !schedulerNameFilter(s.schedulerName, binding.Spec.SchedulerName) {
			continue
		}
		if binding.Spec.SchedulingSuspended() {
			continue
		}

		var affinity *policyv1alpha1.ClusterAffinity
		if placementPtr.ClusterAffinities != nil {
			if binding.Status.SchedulerObservedGeneration != binding.Generation {
				// Hit here means the binding maybe still in the queue waiting
				// for scheduling or its status has not been synced to the
				// cache. Just enqueue the binding to avoid missing the cluster
				// update event.
				s.onClusterResourceBindingRequeue(binding, metrics.ClusterChanged)
				continue
			}
			affinityIndex := getAffinityIndex(placementPtr.ClusterAffinities, binding.Status.SchedulerObservedAffinityName)
			affinity = &placementPtr.ClusterAffinities[affinityIndex].ClusterAffinity
		} else {
			affinity = placementPtr.ClusterAffinity
		}

		switch {
		case affinity == nil:
			// If no clusters specified, add it to the queue
			fallthrough
		case util.ClusterMatches(cluster, *affinity):
			// If the cluster manifest match the affinity, add it to the queue, trigger rescheduling
			s.onClusterResourceBindingRequeue(binding, metrics.ClusterChanged)
		}
	}

	return nil
}

// releaseHealthyClusterAssumptions releases per-cluster assumptions for any cluster
// whose workload transitioned to Healthy between the old and new ResourceBinding.
// It is a no-op when the assumption cache is not initialized or old is not a ResourceBinding.
func (s *Scheduler) releaseHealthyClusterAssumptions(oldRB, newRB *workv1alpha2.ResourceBinding) {
	// Defensive check: schedulerCache is expected to be always initialized.
	if s.schedulerCache == nil || s.schedulerCache.AssigningResourceBindings() == nil {
		return
	}

	// Skip non-workload resources.
	if !oldRB.Spec.IsWorkload() {
		return
	}

	bindingKey := names.NamespacedKey(newRB.Namespace, newRB.Name)
	// Skip when the target assumption does not exist.
	if !s.schedulerCache.AssigningResourceBindings().IsAssumptionExist(bindingKey) {
		return
	}

	for _, clusterName := range newlyHealthyClusters(oldRB.Status.AggregatedStatus, newRB.Status.AggregatedStatus) {
		s.schedulerCache.AssigningResourceBindings().ReleaseClusterAssumption(bindingKey, clusterName)
		klog.V(4).Infof("Released assumption for ResourceBinding(%s) on cluster(%s): workload became Healthy", bindingKey, clusterName)
	}
}

// newlyHealthyClusters returns the cluster names that transitioned to Healthy
// in newStatus compared to oldStatus. It is used to release per-cluster
// assumptions once the workload has reached a stable running state.
func newlyHealthyClusters(oldStatus, newStatus []workv1alpha2.AggregatedStatusItem) []string {
	oldHealthy := sets.New[string]()
	for _, item := range oldStatus {
		if item.Health == workv1alpha2.ResourceHealthy {
			oldHealthy.Insert(item.ClusterName)
		}
	}
	var result []string
	for _, item := range newStatus {
		if item.Health == workv1alpha2.ResourceHealthy {
			if !oldHealthy.Has(item.ClusterName) {
				result = append(result, item.ClusterName)
			}
		}
	}
	return result
}
