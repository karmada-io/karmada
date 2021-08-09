package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	v1helper "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	policylister "github.com/karmada-io/karmada/pkg/generated/listers/policy/v1alpha1"
	worklister "github.com/karmada-io/karmada/pkg/generated/listers/work/v1alpha1"
	schedulercache "github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/core"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/apiinstalled"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/clusteraffinity"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/tainttoleration"
	"github.com/karmada-io/karmada/pkg/util"
)

// ScheduleType defines the schedule type of a binding object should be performed.
type ScheduleType string

const (
	// FirstSchedule means the binding object hasn't been scheduled.
	FirstSchedule ScheduleType = "FirstSchedule"

	// ReconcileSchedule means the binding object associated policy has been changed.
	ReconcileSchedule ScheduleType = "ReconcileSchedule"

	// ScaleSchedule means the replicas of binding object has been changed.
	ScaleSchedule ScheduleType = "ScaleSchedule"

	// FailoverSchedule means one of the cluster a binding object associated with becomes failure.
	FailoverSchedule ScheduleType = "FailoverSchedule"

	// AvoidSchedule means don't need to trigger scheduler.
	AvoidSchedule ScheduleType = "AvoidSchedule"

	// Unknown means can't detect the schedule type
	Unknown ScheduleType = "Unknown"
)

// Failover indicates if the scheduler should performs re-scheduler in case of cluster failure.
// TODO(RainbowMango): Remove the temporary solution by introducing feature flag
var Failover bool

// Scheduler is the scheduler schema, which is used to schedule a specific resource to specific clusters
type Scheduler struct {
	DynamicClient          dynamic.Interface
	KarmadaClient          karmadaclientset.Interface
	KubeClient             kubernetes.Interface
	bindingInformer        cache.SharedIndexInformer
	bindingLister          worklister.ResourceBindingLister
	policyInformer         cache.SharedIndexInformer
	policyLister           policylister.PropagationPolicyLister
	clusterBindingInformer cache.SharedIndexInformer
	clusterBindingLister   worklister.ClusterResourceBindingLister
	clusterPolicyInformer  cache.SharedIndexInformer
	clusterPolicyLister    policylister.ClusterPropagationPolicyLister
	clusterLister          clusterlister.ClusterLister
	informerFactory        informerfactory.SharedInformerFactory

	// TODO: implement a priority scheduling queue
	queue workqueue.RateLimitingInterface

	Algorithm      core.ScheduleAlgorithm
	schedulerCache schedulercache.Cache
}

// NewScheduler instantiates a scheduler
func NewScheduler(dynamicClient dynamic.Interface, karmadaClient karmadaclientset.Interface, kubeClient kubernetes.Interface) *Scheduler {
	factory := informerfactory.NewSharedInformerFactory(karmadaClient, 0)
	bindingInformer := factory.Work().V1alpha1().ResourceBindings().Informer()
	bindingLister := factory.Work().V1alpha1().ResourceBindings().Lister()
	policyInformer := factory.Policy().V1alpha1().PropagationPolicies().Informer()
	policyLister := factory.Policy().V1alpha1().PropagationPolicies().Lister()
	clusterBindingInformer := factory.Work().V1alpha1().ClusterResourceBindings().Informer()
	clusterBindingLister := factory.Work().V1alpha1().ClusterResourceBindings().Lister()
	clusterPolicyInformer := factory.Policy().V1alpha1().ClusterPropagationPolicies().Informer()
	clusterPolicyLister := factory.Policy().V1alpha1().ClusterPropagationPolicies().Lister()
	clusterLister := factory.Cluster().V1alpha1().Clusters().Lister()
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	schedulerCache := schedulercache.NewCache()
	// TODO: make plugins as a flag
	algorithm := core.NewGenericScheduler(schedulerCache, policyLister, []string{clusteraffinity.Name, tainttoleration.Name, apiinstalled.Name})
	sched := &Scheduler{
		DynamicClient:          dynamicClient,
		KarmadaClient:          karmadaClient,
		KubeClient:             kubeClient,
		bindingInformer:        bindingInformer,
		bindingLister:          bindingLister,
		policyInformer:         policyInformer,
		policyLister:           policyLister,
		clusterBindingInformer: clusterBindingInformer,
		clusterBindingLister:   clusterBindingLister,
		clusterPolicyInformer:  clusterPolicyInformer,
		clusterPolicyLister:    clusterPolicyLister,
		clusterLister:          clusterLister,
		informerFactory:        factory,
		queue:                  queue,
		Algorithm:              algorithm,
		schedulerCache:         schedulerCache,
	}

	bindingInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sched.onResourceBindingAdd,
		UpdateFunc: sched.onResourceBindingUpdate,
	})

	policyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: sched.onPropagationPolicyUpdate,
	})

	clusterBindingInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sched.onResourceBindingAdd,
		UpdateFunc: sched.onResourceBindingUpdate,
	})

	clusterPolicyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: sched.onClusterPropagationPolicyUpdate,
	})

	memclusterInformer := factory.Cluster().V1alpha1().Clusters().Informer()
	memclusterInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sched.addCluster,
			UpdateFunc: sched.updateCluster,
			DeleteFunc: sched.deleteCluster,
		},
	)

	return sched
}

// Run runs the scheduler
func (s *Scheduler) Run(ctx context.Context) {
	stopCh := ctx.Done()
	klog.Infof("Starting karmada scheduler")
	defer klog.Infof("Shutting down karmada scheduler")
	s.informerFactory.Start(stopCh)
	if !cache.WaitForCacheSync(stopCh, s.bindingInformer.HasSynced) {
		return
	}

	go wait.Until(s.worker, time.Second, stopCh)

	<-stopCh
}

func (s *Scheduler) onResourceBindingAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("couldn't get key for object %#v: %v", obj, err)
		return
	}

	s.queue.Add(key)
}

func (s *Scheduler) onResourceBindingUpdate(old, cur interface{}) {
	s.onResourceBindingAdd(cur)
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

func (s *Scheduler) worker() {
	for s.scheduleNext() {
	}
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
	}
	return nil
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
	}
	return nil
}

func (s *Scheduler) getPlacement(resourceBinding *workv1alpha1.ResourceBinding) (policyv1alpha1.Placement, string, error) {
	var placement policyv1alpha1.Placement
	var clusterPolicyName string
	var policyName string
	var policyNamespace string
	var err error
	if clusterPolicyName = util.GetLabelValue(resourceBinding.Labels, policyv1alpha1.ClusterPropagationPolicyLabel); clusterPolicyName != "" {
		var clusterPolicy *policyv1alpha1.ClusterPropagationPolicy
		clusterPolicy, err = s.clusterPolicyLister.Get(clusterPolicyName)
		if err != nil {
			return placement, "", err
		}

		placement = clusterPolicy.Spec.Placement
	}

	if policyName = util.GetLabelValue(resourceBinding.Labels, policyv1alpha1.PropagationPolicyNameLabel); policyName != "" {
		policyNamespace = util.GetLabelValue(resourceBinding.Labels, policyv1alpha1.PropagationPolicyNamespaceLabel)
		var policy *policyv1alpha1.PropagationPolicy
		policy, err = s.policyLister.PropagationPolicies(policyNamespace).Get(policyName)
		if err != nil {
			return placement, "", err
		}

		placement = policy.Spec.Placement
	}

	var placementBytes []byte
	placementBytes, err = json.Marshal(placement)
	if err != nil {
		return placement, "", err
	}

	defer func() {
		if err != nil {
			if clusterPolicyName != "" {
				klog.Errorf("Failed to get placement of clusterPropagationPolicy %s, error: %v", clusterPolicyName, err)
			} else {
				klog.Errorf("Failed to get placement of propagationPolicy %s/%s, error: %v", policyNamespace, policyName, err)
			}
		}
	}()

	return placement, string(placementBytes), nil
}

//nolint:gocyclo
// Note: ignore the cyclomatic complexity issue to get gocyclo on board. Tracked by: https://github.com/karmada-io/karmada/issues/460
func (s *Scheduler) getScheduleType(key string) ScheduleType {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return Unknown
	}

	// ResourceBinding object
	if len(ns) > 0 {
		resourceBinding, err := s.bindingLister.ResourceBindings(ns).Get(name)
		if errors.IsNotFound(err) {
			return Unknown
		}

		if len(resourceBinding.Spec.Clusters) == 0 {
			return FirstSchedule
		}

		policyPlacement, policyPlacementStr, err := s.getPlacement(resourceBinding)
		if err != nil {
			return Unknown
		}

		appliedPlacement := util.GetLabelValue(resourceBinding.Annotations, util.PolicyPlacementAnnotation)

		if policyPlacementStr != appliedPlacement {
			return ReconcileSchedule
		}

		if policyPlacement.ReplicaScheduling != nil && util.IsBindingReplicasChanged(&resourceBinding.Spec, policyPlacement.ReplicaScheduling) {
			return ScaleSchedule
		}

		clusters := s.schedulerCache.Snapshot().GetClusters()
		for _, tc := range resourceBinding.Spec.Clusters {
			boundCluster := tc.Name
			for _, c := range clusters {
				if c.Cluster().Name == boundCluster {
					if meta.IsStatusConditionPresentAndEqual(c.Cluster().Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse) {
						return FailoverSchedule
					}
				}
			}
		}
	} else { // ClusterResourceBinding
		binding, err := s.clusterBindingLister.Get(name)
		if errors.IsNotFound(err) {
			return Unknown
		}

		if len(binding.Spec.Clusters) == 0 {
			return FirstSchedule
		}

		policyName := util.GetLabelValue(binding.Labels, policyv1alpha1.ClusterPropagationPolicyLabel)

		policy, err := s.clusterPolicyLister.Get(policyName)
		if err != nil {
			return Unknown
		}
		placement, err := json.Marshal(policy.Spec.Placement)
		if err != nil {
			klog.Errorf("Failed to marshal placement of propagationPolicy %s/%s, error: %v", policy.Namespace, policy.Name, err)
			return Unknown
		}
		policyPlacementStr := string(placement)

		appliedPlacement := util.GetLabelValue(binding.Annotations, util.PolicyPlacementAnnotation)

		if policyPlacementStr != appliedPlacement {
			return ReconcileSchedule
		}

		if policy.Spec.Placement.ReplicaScheduling != nil && util.IsBindingReplicasChanged(&binding.Spec, policy.Spec.Placement.ReplicaScheduling) {
			return ScaleSchedule
		}

		clusters := s.schedulerCache.Snapshot().GetClusters()
		for _, tc := range binding.Spec.Clusters {
			boundCluster := tc.Name
			for _, c := range clusters {
				if c.Cluster().Name == boundCluster {
					if meta.IsStatusConditionPresentAndEqual(c.Cluster().Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse) {
						return FailoverSchedule
					}
				}
			}
		}
	}

	return AvoidSchedule
}

func (s *Scheduler) scheduleNext() bool {
	key, shutdown := s.queue.Get()
	if shutdown {
		klog.Errorf("Fail to pop item from queue")
		return false
	}
	defer s.queue.Done(key)

	var err error
	switch s.getScheduleType(key.(string)) {
	case FirstSchedule:
		err = s.scheduleOne(key.(string))
		klog.Infof("Start scheduling binding(%s)", key.(string))
	case ReconcileSchedule: // share same logic with first schedule
		err = s.scheduleOne(key.(string))
		klog.Infof("Reschedule binding(%s) as placement changed", key.(string))
	case ScaleSchedule:
		err = s.scaleScheduleOne(key.(string))
		klog.Infof("Reschedule binding(%s) as replicas scaled down or scaled up", key.(string))
	case FailoverSchedule:
		if Failover {
			err = s.rescheduleOne(key.(string))
			klog.Infof("Reschedule binding(%s) as cluster failure", key.(string))
		}
	case AvoidSchedule:
		klog.Infof("Don't need to schedule binding(%s)", key.(string))
	default:
		err = fmt.Errorf("unknow schedule type")
		klog.Warningf("Failed to identify scheduler type for binding(%s)", key.(string))
	}

	s.handleErr(err, key)
	return true
}

func (s *Scheduler) scheduleOne(key string) (err error) {
	klog.V(4).Infof("begin scheduling ResourceBinding %s", key)
	defer klog.V(4).Infof("end scheduling ResourceBinding %s: %v", key, err)

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	if ns == "" {
		clusterResourceBinding, err := s.clusterBindingLister.Get(name)
		if errors.IsNotFound(err) {
			return nil
		}

		clusterPolicyName := util.GetLabelValue(clusterResourceBinding.Labels, policyv1alpha1.ClusterPropagationPolicyLabel)

		clusterPolicy, err := s.clusterPolicyLister.Get(clusterPolicyName)
		if err != nil {
			return err
		}

		return s.scheduleClusterResourceBinding(clusterResourceBinding, clusterPolicy)
	}
	resourceBinding, err := s.bindingLister.ResourceBindings(ns).Get(name)
	if errors.IsNotFound(err) {
		return nil
	}

	return s.scheduleResourceBinding(resourceBinding)
}

func (s *Scheduler) scheduleResourceBinding(resourceBinding *workv1alpha1.ResourceBinding) (err error) {
	placement, placementStr, err := s.getPlacement(resourceBinding)
	if err != nil {
		return err
	}

	scheduleResult, err := s.Algorithm.Schedule(context.TODO(), &placement, &resourceBinding.Spec.Resource)
	if err != nil {
		klog.V(2).Infof("failed scheduling ResourceBinding %s/%s: %v", resourceBinding.Namespace, resourceBinding.Name, err)
		return err
	}
	klog.V(4).Infof("ResourceBinding %s/%s scheduled to clusters %v", resourceBinding.Namespace, resourceBinding.Name, scheduleResult.SuggestedClusters)

	binding := resourceBinding.DeepCopy()
	binding.Spec.Clusters = scheduleResult.SuggestedClusters

	if binding.Annotations == nil {
		binding.Annotations = make(map[string]string)
	}
	binding.Annotations[util.PolicyPlacementAnnotation] = placementStr

	_, err = s.KarmadaClient.WorkV1alpha1().ResourceBindings(binding.Namespace).Update(context.TODO(), binding, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (s *Scheduler) scheduleClusterResourceBinding(clusterResourceBinding *workv1alpha1.ClusterResourceBinding, policy *policyv1alpha1.ClusterPropagationPolicy) (err error) {
	scheduleResult, err := s.Algorithm.Schedule(context.TODO(), &policy.Spec.Placement, &clusterResourceBinding.Spec.Resource)
	if err != nil {
		klog.V(2).Infof("failed scheduling ClusterResourceBinding %s: %v", clusterResourceBinding.Name, err)
		return err
	}
	klog.V(4).Infof("ClusterResourceBinding %s scheduled to clusters %v", clusterResourceBinding.Name, scheduleResult.SuggestedClusters)

	binding := clusterResourceBinding.DeepCopy()
	binding.Spec.Clusters = scheduleResult.SuggestedClusters

	placement, err := json.Marshal(policy.Spec.Placement)
	if err != nil {
		klog.Errorf("Failed to marshal placement of clusterPropagationPolicy %s, error: %v", policy.Name, err)
		return err
	}

	if binding.Annotations == nil {
		binding.Annotations = make(map[string]string)
	}
	binding.Annotations[util.PolicyPlacementAnnotation] = string(placement)

	_, err = s.KarmadaClient.WorkV1alpha1().ClusterResourceBindings().Update(context.TODO(), binding, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (s *Scheduler) handleErr(err error, key interface{}) {
	if err == nil || errors.HasStatusCause(err, corev1.NamespaceTerminatingCause) {
		s.queue.Forget(key)
		return
	}

	s.queue.AddRateLimited(key)
}

func (s *Scheduler) addCluster(obj interface{}) {
	cluster, ok := obj.(*clusterv1alpha1.Cluster)
	if !ok {
		klog.Errorf("cannot convert to Cluster: %v", obj)
		return
	}
	klog.V(3).Infof("add event for cluster %s", cluster.Name)

	s.schedulerCache.AddCluster(cluster)
}

func (s *Scheduler) updateCluster(_, newObj interface{}) {
	newCluster, ok := newObj.(*clusterv1alpha1.Cluster)
	if !ok {
		klog.Errorf("cannot convert newObj to Cluster: %v", newObj)
		return
	}
	klog.V(3).Infof("update event for cluster %s", newCluster.Name)
	s.schedulerCache.UpdateCluster(newCluster)

	// Check if cluster becomes failure
	if meta.IsStatusConditionPresentAndEqual(newCluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse) {
		klog.Infof("Found cluster(%s) failure and failover flag is %v", newCluster.Name, Failover)

		if Failover { // Trigger reschedule on cluster failure only when flag is true.
			s.enqueueAffectedBinding(newCluster.Name)
			s.enqueueAffectedClusterBinding(newCluster.Name)
			return
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
	klog.V(3).Infof("delete event for cluster %s", cluster.Name)
	s.schedulerCache.DeleteCluster(cluster)
}

// enqueueAffectedBinding will find all ResourceBindings which are related to the current NotReady cluster and add them in queue.
func (s *Scheduler) enqueueAffectedBinding(notReadyClusterName string) {
	bindings, _ := s.bindingLister.List(labels.Everything())
	klog.Infof("Start traveling all ResourceBindings")
	for _, binding := range bindings {
		clusters := binding.Spec.Clusters
		for _, bindingCluster := range clusters {
			if bindingCluster.Name == notReadyClusterName {
				rescheduleKey, err := cache.MetaNamespaceKeyFunc(binding)
				if err != nil {
					klog.Errorf("couldn't get rescheduleKey for ResourceBinding %#v: %v", bindingCluster.Name, err)
					return
				}
				s.queue.Add(rescheduleKey)
				klog.Infof("Add expired ResourceBinding in queue successfully")
			}
		}
	}
}

// enqueueAffectedClusterBinding will find all cluster resource bindings which are related to the current NotReady cluster and add them in queue.
func (s *Scheduler) enqueueAffectedClusterBinding(notReadyClusterName string) {
	bindings, _ := s.clusterBindingLister.List(labels.Everything())
	klog.Infof("Start traveling all ClusterResourceBindings")
	for _, binding := range bindings {
		clusters := binding.Spec.Clusters
		for _, bindingCluster := range clusters {
			if bindingCluster.Name == notReadyClusterName {
				rescheduleKey, err := cache.MetaNamespaceKeyFunc(binding)
				if err != nil {
					klog.Errorf("couldn't get rescheduleKey for ClusterResourceBinding %s: %v", bindingCluster.Name, err)
					return
				}
				s.queue.Add(rescheduleKey)
				klog.Infof("Add expired ClusterResourceBinding in queue successfully")
			}
		}
	}
}

// rescheduleOne.
func (s *Scheduler) rescheduleOne(key string) (err error) {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	// ClusterResourceBinding object
	if ns == "" {
		klog.Infof("begin rescheduling ClusterResourceBinding %s", name)
		defer klog.Infof("end rescheduling ClusterResourceBinding %s", name)

		clusterResourceBinding, err := s.clusterBindingLister.Get(name)
		if errors.IsNotFound(err) {
			return nil
		}
		crBinding := clusterResourceBinding.DeepCopy()
		return s.rescheduleClusterResourceBinding(crBinding)
	}

	// ResourceBinding object
	if len(ns) > 0 {
		klog.Infof("begin rescheduling ResourceBinding %s %s", ns, name)
		defer klog.Infof("end rescheduling ResourceBinding %s: %s", ns, name)

		resourceBinding, err := s.bindingLister.ResourceBindings(ns).Get(name)
		if errors.IsNotFound(err) {
			return nil
		}
		binding := resourceBinding.DeepCopy()
		return s.rescheduleResourceBinding(binding)
	}
	return nil
}

func (s *Scheduler) rescheduleClusterResourceBinding(clusterResourceBinding *workv1alpha1.ClusterResourceBinding) error {
	policyName := util.GetLabelValue(clusterResourceBinding.Labels, policyv1alpha1.ClusterPropagationPolicyLabel)
	policy, err := s.clusterPolicyLister.Get(policyName)
	if err != nil {
		klog.Errorf("Failed to get policy by policyName(%s): Error: %v", policyName, err)
		return err
	}

	targetClusters, err := s.obtainTargetCluster(clusterResourceBinding.Spec.Clusters, policy.Spec.Placement)
	if err != nil {
		return err
	}
	if targetClusters == nil {
		return nil
	}

	// TODO(tinyma123) Check if the final result meets the spread constraints.

	clusterResourceBinding.Spec.Clusters = nil
	for cluster := range targetClusters {
		clusterResourceBinding.Spec.Clusters = append(clusterResourceBinding.Spec.Clusters, workv1alpha1.TargetCluster{Name: cluster})
	}
	klog.Infof("The final binding.Spec.Cluster values are: %v\n", clusterResourceBinding.Spec.Clusters)

	_, err = s.KarmadaClient.WorkV1alpha1().ClusterResourceBindings().Update(context.TODO(), clusterResourceBinding, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (s *Scheduler) rescheduleResourceBinding(resourceBinding *workv1alpha1.ResourceBinding) error {
	placement, _, err := s.getPlacement(resourceBinding)
	if err != nil {
		klog.Errorf("Failed to get placement by resourceBinding(%s/%s): Error: %v", resourceBinding.Namespace, resourceBinding.Name, err)
		return err
	}

	targetClusters, err := s.obtainTargetCluster(resourceBinding.Spec.Clusters, placement)
	if err != nil {
		return err
	}
	if targetClusters == nil {
		return nil
	}

	// TODO(tinyma123) Check if the final result meets the spread constraints.

	resourceBinding.Spec.Clusters = nil
	for cluster := range targetClusters {
		resourceBinding.Spec.Clusters = append(resourceBinding.Spec.Clusters, workv1alpha1.TargetCluster{Name: cluster})
	}
	klog.Infof("The final binding.Spec.Cluster values are: %v\n", resourceBinding.Spec.Clusters)

	_, err = s.KarmadaClient.WorkV1alpha1().ResourceBindings(resourceBinding.Namespace).Update(context.TODO(), resourceBinding, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// calcReservedCluster eliminates the not-ready clusters from the 'bindClusters'.
func calcReservedCluster(bindClusters, readyClusters sets.String) sets.String {
	return bindClusters.Difference(bindClusters.Difference(readyClusters))
}

// calcAvailableCluster returns a list of ready clusters that not in 'bindClusters'.
func calcAvailableCluster(bindCluster, readyClusters sets.String) sets.String {
	return readyClusters.Difference(bindCluster)
}

func (s *Scheduler) obtainTargetCluster(bindingClusters []workv1alpha1.TargetCluster, placement policyv1alpha1.Placement) (sets.String, error) {
	readyClusters := s.schedulerCache.Snapshot().GetReadyClusterNames()
	totalClusters := util.ConvertToClusterNames(bindingClusters)

	reservedClusters := calcReservedCluster(totalClusters, readyClusters)
	availableClusters := calcAvailableCluster(totalClusters, readyClusters)

	filterPredicate := func(t *corev1.Taint) bool {
		// now only interested in NoSchedule taint which means do not allow new resource to schedule onto the cluster unless they tolerate the taint
		// todo: supprot NoExecute taint
		return t.Effect == corev1.TaintEffectNoSchedule
	}

	candidateClusters := sets.NewString()
	for clusterName := range availableClusters {
		clusterObj, err := s.clusterLister.Get(clusterName)
		if err != nil {
			klog.Errorf("Failed to get clusterObj by clusterName: %s", clusterName)
			return nil, err
		}

		if placement.ClusterAffinity != nil && !util.ClusterMatches(clusterObj, *placement.ClusterAffinity) {
			continue
		}

		_, isUntolerated := v1helper.FindMatchingUntoleratedTaint(clusterObj.Spec.Taints, placement.ClusterTolerations, filterPredicate)
		if !isUntolerated {
			candidateClusters.Insert(clusterName)
		}
	}

	klog.V(4).Infof("Reserved bindingClusters : %v", reservedClusters.List())
	klog.V(4).Infof("Candidate bindingClusters: %v", candidateClusters.List())

	// TODO: should schedule as much as possible?
	deltaLen := len(bindingClusters) - len(reservedClusters)
	if len(candidateClusters) < deltaLen {
		klog.Warningf("ignore reschedule binding as insufficient available cluster")
		return nil, nil
	}

	targetClusters := reservedClusters
	clusterList := candidateClusters.List()
	for i := 0; i < deltaLen; i++ {
		targetClusters.Insert(clusterList[i])
	}

	return targetClusters, nil
}

func (s *Scheduler) scaleScheduleOne(key string) (err error) {
	klog.V(4).Infof("begin scale scheduling ResourceBinding %s", key)
	defer klog.V(4).Infof("end scale scheduling ResourceBinding %s: %v", key, err)

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	if ns == "" {
		clusterResourceBinding, err := s.clusterBindingLister.Get(name)
		if errors.IsNotFound(err) {
			return nil
		}

		clusterPolicyName := util.GetLabelValue(clusterResourceBinding.Labels, policyv1alpha1.ClusterPropagationPolicyLabel)

		clusterPolicy, err := s.clusterPolicyLister.Get(clusterPolicyName)
		if err != nil {
			return err
		}

		return s.scaleScheduleClusterResourceBinding(clusterResourceBinding, clusterPolicy)
	}

	resourceBinding, err := s.bindingLister.ResourceBindings(ns).Get(name)
	if errors.IsNotFound(err) {
		return nil
	}

	return s.scaleScheduleResourceBinding(resourceBinding)
}

func (s *Scheduler) scaleScheduleResourceBinding(resourceBinding *workv1alpha1.ResourceBinding) (err error) {
	placement, placementStr, err := s.getPlacement(resourceBinding)
	if err != nil {
		return err
	}

	scheduleResult, err := s.Algorithm.ScaleSchedule(context.TODO(), &placement, &resourceBinding.Spec)
	if err != nil {
		klog.V(2).Infof("failed rescheduling ResourceBinding %s/%s after replicas changes: %v", resourceBinding.Namespace, resourceBinding.Name, err)
		return err
	}

	klog.V(4).Infof("ResourceBinding %s/%s scheduled to clusters %v", resourceBinding.Namespace, resourceBinding.Name, scheduleResult.SuggestedClusters)

	binding := resourceBinding.DeepCopy()
	binding.Spec.Clusters = scheduleResult.SuggestedClusters

	if binding.Annotations == nil {
		binding.Annotations = make(map[string]string)
	}
	binding.Annotations[util.PolicyPlacementAnnotation] = placementStr

	_, err = s.KarmadaClient.WorkV1alpha1().ResourceBindings(binding.Namespace).Update(context.TODO(), binding, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (s *Scheduler) scaleScheduleClusterResourceBinding(clusterResourceBinding *workv1alpha1.ClusterResourceBinding,
	policy *policyv1alpha1.ClusterPropagationPolicy) (err error) {
	scheduleResult, err := s.Algorithm.ScaleSchedule(context.TODO(), &policy.Spec.Placement, &clusterResourceBinding.Spec)
	if err != nil {
		klog.V(2).Infof("failed rescheduling ClusterResourceBinding %s after replicas scaled down: %v", clusterResourceBinding.Name, err)
		return err
	}

	klog.V(4).Infof("ClusterResourceBinding %s scheduled to clusters %v", clusterResourceBinding.Name, scheduleResult.SuggestedClusters)

	binding := clusterResourceBinding.DeepCopy()
	binding.Spec.Clusters = scheduleResult.SuggestedClusters

	placement, err := json.Marshal(policy.Spec.Placement)
	if err != nil {
		klog.Errorf("Failed to marshal placement of propagationPolicy %s/%s, error: %v", policy.Namespace, policy.Name, err)
		return err
	}

	if binding.Annotations == nil {
		binding.Annotations = make(map[string]string)
	}
	binding.Annotations[util.PolicyPlacementAnnotation] = string(placement)

	_, err = s.KarmadaClient.WorkV1alpha1().ClusterResourceBindings().Update(context.TODO(), binding, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
