package scheduler

import (
	"context"
	"encoding/json"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	policylister "github.com/karmada-io/karmada/pkg/generated/listers/policy/v1alpha1"
	worklister "github.com/karmada-io/karmada/pkg/generated/listers/work/v1alpha1"
	schedulercache "github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/core"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/clusteraffinity"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	// maxRetries is the number of times a service will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the
	// sequence of delays between successive queuings of a propagationbinding.
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15
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
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	schedulerCache := schedulercache.NewCache()
	// TODO: make plugins as a flag
	algorithm := core.NewGenericScheduler(schedulerCache, policyLister, []string{clusteraffinity.Name})
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
		util.PropagationPolicyNamespaceLabel: oldPropagationPolicy.Namespace,
		util.PropagationPolicyNameLabel:      oldPropagationPolicy.Name,
	})

	referenceBindings, err := s.bindingLister.List(selector)
	if err != nil {
		klog.Errorf("Failed to list ResourceBinding by selector: %s, error: %v", selector.String(), err)
		return
	}

	for _, binding := range referenceBindings {
		key, err := cache.MetaNamespaceKeyFunc(binding)
		if err != nil {
			klog.Errorf("couldn't get key for object %#v: %v", binding, err)
			return
		}
		klog.Infof("Requeue ResourceBinding(%s/%s) as placement changed.", binding.Namespace, binding.Name)
		s.queue.Add(key)
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
		util.ClusterPropagationPolicyLabel: oldClusterPropagationPolicy.Name,
	})

	referenceClusterResourceBindings, err := s.clusterBindingLister.List(selector)
	if err != nil {
		klog.Errorf("Failed to list ClusterResourceBinding by selector: %s, error: %v", selector.String(), err)
		return
	}

	for _, clusterResourceBinding := range referenceClusterResourceBindings {
		key, err := cache.MetaNamespaceKeyFunc(clusterResourceBinding)
		if err != nil {
			klog.Errorf("couldn't get key for object %#v: %v", clusterResourceBinding, err)
			return
		}
		klog.Infof("Requeue ClusterResourceBinding(%s) as placement changed.", clusterResourceBinding.Name)
		s.queue.Add(key)
	}
}

func (s *Scheduler) worker() {
	for s.scheduleNext() {
	}
}

func (s *Scheduler) scheduleNext() bool {
	key, shutdown := s.queue.Get()
	if shutdown {
		klog.Errorf("Fail to pop item from queue")
		return false
	}
	defer s.queue.Done(key)
	klog.Infof("Failover flag is: %v", Failover)

	err := s.scheduleOne(key.(string))
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

		clusterPolicyName := util.GetLabelValue(clusterResourceBinding.Labels, util.ClusterPropagationPolicyLabel)

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
	policyNamespace := util.GetLabelValue(resourceBinding.Labels, util.PropagationPolicyNamespaceLabel)
	policyName := util.GetLabelValue(resourceBinding.Labels, util.PropagationPolicyNameLabel)

	policy, err := s.policyLister.PropagationPolicies(policyNamespace).Get(policyName)
	if err != nil {
		return err
	}

	return s.scheduleResourceBinding(resourceBinding, policy)
}

func (s *Scheduler) scheduleResourceBinding(resourceBinding *workv1alpha1.ResourceBinding, policy *policyv1alpha1.PropagationPolicy) (err error) {
	scheduleResult, err := s.Algorithm.Schedule(context.TODO(), &policy.Spec.Placement)
	if err != nil {
		klog.V(2).Infof("failed scheduling ResourceBinding %s/%s: %v", resourceBinding.Namespace, resourceBinding.Name, err)
		return err
	}
	klog.V(4).Infof("ResourceBinding %s/%s scheduled to clusters %v", resourceBinding.Namespace, resourceBinding.Name, scheduleResult.SuggestedClusters)

	binding := resourceBinding.DeepCopy()
	targetClusters := make([]workv1alpha1.TargetCluster, len(scheduleResult.SuggestedClusters))
	for i, cluster := range scheduleResult.SuggestedClusters {
		targetClusters[i] = workv1alpha1.TargetCluster{Name: cluster}
	}
	binding.Spec.Clusters = targetClusters

	placement, err := json.Marshal(policy.Spec.Placement)
	if err != nil {
		klog.Errorf("Failed to marshal placement of propagationPolicy %s/%s, error: %v", policy.Namespace, policy.Name, err)
	}

	if binding.Annotations == nil {
		binding.Annotations = make(map[string]string)
	}
	binding.Annotations[util.PolicyPlacementAnnotation] = string(placement)

	_, err = s.KarmadaClient.WorkV1alpha1().ResourceBindings(binding.Namespace).Update(context.TODO(), binding, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (s *Scheduler) scheduleClusterResourceBinding(clusterResourceBinding *workv1alpha1.ClusterResourceBinding, policy *policyv1alpha1.ClusterPropagationPolicy) (err error) {
	scheduleResult, err := s.Algorithm.Schedule(context.TODO(), &policy.Spec.Placement)
	if err != nil {
		klog.V(2).Infof("failed scheduling ClusterResourceBinding %s: %v", clusterResourceBinding.Name, err)
		return err
	}
	klog.V(4).Infof("ClusterResourceBinding %s scheduled to clusters %v", clusterResourceBinding.Name, scheduleResult.SuggestedClusters)

	binding := clusterResourceBinding.DeepCopy()
	targetClusters := make([]workv1alpha1.TargetCluster, len(scheduleResult.SuggestedClusters))
	for i, cluster := range scheduleResult.SuggestedClusters {
		targetClusters[i] = workv1alpha1.TargetCluster{Name: cluster}
	}
	binding.Spec.Clusters = targetClusters

	placement, err := json.Marshal(policy.Spec.Placement)
	if err != nil {
		klog.Errorf("Failed to marshal placement of clusterPropagationPolicy %s, error: %v", policy.Name, err)
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
	if err == nil || errors.HasStatusCause(err, v1.NamespaceTerminatingCause) {
		s.queue.Forget(key)
		return
	}

	if s.queue.NumRequeues(key) < maxRetries {
		s.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	klog.V(2).Infof("Dropping propagationbinding %q out of the queue: %v", key, err)
	s.queue.Forget(key)
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
