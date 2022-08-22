package scheduler

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/scheduler/metrics"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

// addAllEventHandlers is a helper function used in Scheduler
// to add event handlers for various informers.
func (s *Scheduler) addAllEventHandlers() {
	bindingInformer := s.informerFactory.Work().V1alpha2().ResourceBindings().Informer()
	bindingInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: s.resourceBindingEventFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    s.onResourceBindingAdd,
			UpdateFunc: s.onResourceBindingUpdate,
		},
	})

	policyInformer := s.informerFactory.Policy().V1alpha1().PropagationPolicies().Informer()
	policyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: s.onPropagationPolicyUpdate,
	})

	clusterBindingInformer := s.informerFactory.Work().V1alpha2().ClusterResourceBindings().Informer()
	clusterBindingInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: s.resourceBindingEventFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    s.onResourceBindingAdd,
			UpdateFunc: s.onResourceBindingUpdate,
		},
	})

	clusterPolicyInformer := s.informerFactory.Policy().V1alpha1().ClusterPropagationPolicies().Informer()
	clusterPolicyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: s.onClusterPropagationPolicyUpdate,
	})

	memClusterInformer := s.informerFactory.Cluster().V1alpha1().Clusters().Informer()
	memClusterInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    s.addCluster,
			UpdateFunc: s.updateCluster,
			DeleteFunc: s.deleteCluster,
		},
	)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: s.KubeClient.CoreV1().Events("")})
	s.eventRecorder = eventBroadcaster.NewRecorder(gclient.NewSchema(), corev1.EventSource{Component: "karmada-scheduler"})
}

func (s *Scheduler) resourceBindingEventFilter(obj interface{}) bool {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return false
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
	key, err := cache.MetaNamespaceKeyFunc(cur)
	if err != nil {
		klog.Errorf("couldn't get key for object %#v: %v", cur, err)
		return
	}

	s.queue.Add(key)
	metrics.CountSchedulerBindings(metrics.BindingUpdate)
}

func (s *Scheduler) onPropagationPolicyUpdate(old, cur interface{}) {
	oldPropagationPolicy := old.(*policyv1alpha1.PropagationPolicy)
	curPropagationPolicy := cur.(*policyv1alpha1.PropagationPolicy)
	if equality.Semantic.DeepEqual(oldPropagationPolicy.Spec.Placement, curPropagationPolicy.Spec.Placement) {
		klog.V(2).Infof("Ignore PropagationPolicy(%s/%s) which placement unchanged.", oldPropagationPolicy.Namespace, oldPropagationPolicy.Name)
		return
	}

	selector := labels.SelectorFromSet(labels.Set{
		policyv1alpha1.PropagationPolicyNamespaceLabel: oldPropagationPolicy.Namespace,
		policyv1alpha1.PropagationPolicyNameLabel:      oldPropagationPolicy.Name,
	})

	err := s.requeueResourceBindings(selector)
	if err != nil {
		klog.Errorf("Failed to requeue ResourceBinding, error: %v", err)
		return
	}
}

// requeueClusterResourceBindings will retrieve all ClusterResourceBinding objects by the label selector and put them to queue.
func (s *Scheduler) requeueClusterResourceBindings(selector labels.Selector) error {
	referenceClusterResourceBindings, err := s.clusterBindingLister.List(selector)
	if err != nil {
		klog.Errorf("Failed to list ClusterResourceBinding by selector: %s, error: %v", selector.String(), err)
		return err
	}

	for _, clusterResourceBinding := range referenceClusterResourceBindings {
		key, err := cache.MetaNamespaceKeyFunc(clusterResourceBinding)
		if err != nil {
			klog.Errorf("couldn't get key for ClusterResourceBinding(%s): %v", clusterResourceBinding.Name, err)
			continue
		}
		klog.Infof("Requeue ClusterResourceBinding(%s) as placement changed.", clusterResourceBinding.Name)
		s.queue.Add(key)
		metrics.CountSchedulerBindings(metrics.PolicyChanged)
	}
	return nil
}

// requeueResourceBindings will retrieve all ResourceBinding objects by the label selector and put them to queue.
func (s *Scheduler) requeueResourceBindings(selector labels.Selector) error {
	referenceBindings, err := s.bindingLister.List(selector)
	if err != nil {
		klog.Errorf("Failed to list ResourceBinding by selector: %s, error: %v", selector.String(), err)
		return err
	}

	for _, binding := range referenceBindings {
		key, err := cache.MetaNamespaceKeyFunc(binding)
		if err != nil {
			klog.Errorf("couldn't get key for ResourceBinding(%s/%s): %v", binding.Namespace, binding.Name, err)
			continue
		}
		klog.Infof("Requeue ResourceBinding(%s/%s) as placement changed.", binding.Namespace, binding.Name)
		s.queue.Add(key)
		metrics.CountSchedulerBindings(metrics.PolicyChanged)
	}
	return nil
}

func (s *Scheduler) onClusterPropagationPolicyUpdate(old, cur interface{}) {
	oldClusterPropagationPolicy := old.(*policyv1alpha1.ClusterPropagationPolicy)
	curClusterPropagationPolicy := cur.(*policyv1alpha1.ClusterPropagationPolicy)
	if equality.Semantic.DeepEqual(oldClusterPropagationPolicy.Spec.Placement, curClusterPropagationPolicy.Spec.Placement) {
		klog.V(2).Infof("Ignore ClusterPropagationPolicy(%s) which placement unchanged.", oldClusterPropagationPolicy.Name)
		return
	}

	selector := labels.SelectorFromSet(labels.Set{
		policyv1alpha1.ClusterPropagationPolicyLabel: oldClusterPropagationPolicy.Name,
	})

	err := s.requeueClusterResourceBindings(selector)
	if err != nil {
		klog.Errorf("Failed to requeue ClusterResourceBinding, error: %v", err)
	}

	err = s.requeueResourceBindings(selector)
	if err != nil {
		klog.Errorf("Failed to requeue ResourceBinding, error: %v", err)
	}
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
		klog.Errorf("cannot convert oldObj to Cluster: %v", newObj)
		return
	}
	klog.V(3).Infof("Update event for cluster %s", newCluster.Name)

	if s.enableSchedulerEstimator {
		s.schedulerEstimatorWorker.Add(newCluster.Name)
	}

	switch {
	case !equality.Semantic.DeepEqual(oldCluster.Labels, newCluster.Labels):
		fallthrough
	case !equality.Semantic.DeepEqual(oldCluster.Spec, newCluster.Spec):
		s.enqueueAffectedPolicy(newCluster)
		s.enqueueAffectedClusterPolicy(newCluster)
	}
}

// enqueueAffectedPolicy find all propagation policies related to the cluster and reschedule the RBs
func (s *Scheduler) enqueueAffectedPolicy(newCluster *clusterv1alpha1.Cluster) {
	policies, _ := s.policyLister.List(labels.Everything())
	for _, policy := range policies {
		selector := labels.SelectorFromSet(labels.Set{
			policyv1alpha1.PropagationPolicyNamespaceLabel: policy.Namespace,
			policyv1alpha1.PropagationPolicyNameLabel:      policy.Name,
		})
		affinity := policy.Spec.Placement.ClusterAffinity
		switch {
		case affinity == nil:
			// If no clusters specified, add it in queue
			fallthrough
		case util.ClusterMatches(newCluster, *affinity):
			// If specific cluster matches the affinity. add it in queue
			err := s.requeueResourceBindings(selector)
			if err != nil {
				klog.Errorf("Failed to requeue ResourceBinding, error: %v", err)
			}
		}
	}
}

// enqueueAffectedClusterPolicy find all cluster propagation policies related to the cluster and reschedule the RBs/CRBs
func (s *Scheduler) enqueueAffectedClusterPolicy(newCluster *clusterv1alpha1.Cluster) {
	clusterPolicies, _ := s.clusterPolicyLister.List(labels.Everything())
	for _, policy := range clusterPolicies {
		selector := labels.SelectorFromSet(labels.Set{
			policyv1alpha1.ClusterPropagationPolicyLabel: policy.Name,
		})
		affinity := policy.Spec.Placement.ClusterAffinity
		switch {
		case affinity == nil:
			// If no clusters specified, add it in queue
			fallthrough
		case util.ClusterMatches(newCluster, *affinity):
			// If specific cluster matches the affinity. add it in queue
			err := s.requeueClusterResourceBindings(selector)
			if err != nil {
				klog.Errorf("Failed to requeue ClusterResourceBinding, error: %v", err)
			}
			err = s.requeueResourceBindings(selector)
			if err != nil {
				klog.Errorf("Failed to requeue ResourceBinding, error: %v", err)
			}
		}
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
