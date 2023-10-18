package scheduler

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/metrics"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
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
		},
	})
	if err != nil {
		klog.Errorf("Failed to add handlers for ResourceBindings: %v", err)
	}

	clusterBindingInformer := s.informerFactory.Work().V1alpha2().ClusterResourceBindings().Informer()
	_, err = clusterBindingInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: s.resourceBindingEventFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    s.onResourceBindingAdd,
			UpdateFunc: s.onResourceBindingUpdate,
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

func (s *Scheduler) resourceBindingEventFilter(obj interface{}) bool {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return false
	}

	switch t := obj.(type) {
	case *workv1alpha2.ResourceBinding:
		if !schedulerNameFilter(s.schedulerName, t.Spec.SchedulerName) {
			return false
		}
	case *workv1alpha2.ClusterResourceBinding:
		if !schedulerNameFilter(s.schedulerName, t.Spec.SchedulerName) {
			return false
		}
	}

	return util.GetLabelValue(accessor.GetLabels(), policyv1alpha1.PropagationPolicyNameLabel) != "" ||
		util.GetLabelValue(accessor.GetLabels(), policyv1alpha1.ClusterPropagationPolicyLabel) != ""
}

func (s *Scheduler) onResourceBindingAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("couldn't get key for object %#v: %v", obj, err)
		return
	}

	s.queue.Add(key)
	metrics.CountSchedulerBindings(metrics.BindingAdd)
}

func (s *Scheduler) onResourceBindingUpdate(old, cur interface{}) {
	unstructuredOldObj, err := helper.ToUnstructured(old)
	if err != nil {
		klog.Errorf("Failed to transform oldObj, error: %v", err)
		return
	}

	unstructuredNewObj, err := helper.ToUnstructured(cur)
	if err != nil {
		klog.Errorf("Failed to transform newObj, error: %v", err)
		return
	}

	if unstructuredOldObj.GetGeneration() == unstructuredNewObj.GetGeneration() {
		klog.V(4).Infof("Ignore update event of object (kind=%s, %s/%s) as specification no change", unstructuredOldObj.GetKind(), unstructuredOldObj.GetNamespace(), unstructuredOldObj.GetName())
		return
	}

	key, err := cache.MetaNamespaceKeyFunc(cur)
	if err != nil {
		klog.Errorf("couldn't get key for object %#v: %v", cur, err)
		return
	}

	s.queue.Add(key)
	metrics.CountSchedulerBindings(metrics.BindingUpdate)
}

func (s *Scheduler) onResourceBindingRequeue(binding *workv1alpha2.ResourceBinding, event string) {
	key, err := cache.MetaNamespaceKeyFunc(binding)
	if err != nil {
		klog.Errorf("couldn't get key for ResourceBinding(%s/%s): %v", binding.Namespace, binding.Name, err)
		return
	}
	klog.Infof("Requeue ResourceBinding(%s/%s) due to event(%s).", binding.Namespace, binding.Name, event)
	s.queue.Add(key)
	metrics.CountSchedulerBindings(event)
}

func (s *Scheduler) onClusterResourceBindingRequeue(clusterResourceBinding *workv1alpha2.ClusterResourceBinding, event string) {
	key, err := cache.MetaNamespaceKeyFunc(clusterResourceBinding)
	if err != nil {
		klog.Errorf("couldn't get key for ClusterResourceBinding(%s): %v", clusterResourceBinding.Name, err)
		return
	}
	klog.Infof("Requeue ClusterResourceBinding(%s) due to event(%s).", clusterResourceBinding.Name, event)
	s.queue.Add(key)
	metrics.CountSchedulerBindings(event)
}

func (s *Scheduler) addCluster(obj interface{}) {
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

func (s *Scheduler) updateCluster(oldObj, newObj interface{}) {
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
	case !equality.Semantic.DeepEqual(oldCluster.Labels, newCluster.Labels):
		fallthrough
	case oldCluster.Generation != newCluster.Generation:
		// To distinguish the obd and new cluster objects, we need to add the entire object
		// to the worker. Therefore, call Add func instead of Enqueue func.
		s.clusterReconcileWorker.Add(oldCluster)
		s.clusterReconcileWorker.Add(newCluster)
	}
}

func (s *Scheduler) deleteCluster(obj interface{}) {
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

		var affinity *policyv1alpha1.ClusterAffinity
		if placementPtr.ClusterAffinities != nil {
			if binding.Status.SchedulerObservedGeneration != binding.Generation {
				// Hit here means the binding maybe still in the queue waiting
				// for scheduling or its status has not been synced to the
				// cache. Just enqueue the binding to avoid missing the cluster
				// update event.
				if schedulerNameFilter(s.schedulerName, binding.Spec.SchedulerName) {
					s.onResourceBindingRequeue(binding, metrics.ClusterChanged)
				}
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
			if schedulerNameFilter(s.schedulerName, binding.Spec.SchedulerName) {
				s.onResourceBindingRequeue(binding, metrics.ClusterChanged)
			}
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

		var affinity *policyv1alpha1.ClusterAffinity
		if placementPtr.ClusterAffinities != nil {
			if binding.Status.SchedulerObservedGeneration != binding.Generation {
				// Hit here means the binding maybe still in the queue waiting
				// for scheduling or its status has not been synced to the
				// cache. Just enqueue the binding to avoid missing the cluster
				// update event.
				if schedulerNameFilter(s.schedulerName, binding.Spec.SchedulerName) {
					s.onClusterResourceBindingRequeue(binding, metrics.ClusterChanged)
				}
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
			if schedulerNameFilter(s.schedulerName, binding.Spec.SchedulerName) {
				s.onClusterResourceBindingRequeue(binding, metrics.ClusterChanged)
			}
		}
	}

	return nil
}
