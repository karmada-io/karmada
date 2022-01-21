package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/scheduler/app/options"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/features"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	policylister "github.com/karmada-io/karmada/pkg/generated/listers/policy/v1alpha1"
	worklister "github.com/karmada-io/karmada/pkg/generated/listers/work/v1alpha2"
	schedulercache "github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/core"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/apiinstalled"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/clusteraffinity"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/tainttoleration"
	"github.com/karmada-io/karmada/pkg/scheduler/metrics"
	"github.com/karmada-io/karmada/pkg/util"
)

// ScheduleType defines the schedule type of a binding object should be performed.
type ScheduleType string

const (
	// ReconcileSchedule means the binding object associated policy has been changed.
	ReconcileSchedule ScheduleType = "ReconcileSchedule"

	// ScaleSchedule means the replicas of binding object has been changed.
	ScaleSchedule ScheduleType = "ScaleSchedule"

	// FailoverSchedule means one of the cluster a binding object associated with becomes failure.
	FailoverSchedule ScheduleType = "FailoverSchedule"
)

const (
	scheduleSuccessReason  = "BindingScheduled"
	scheduleFailedReason   = "BindingFailedScheduling"
	scheduleSuccessMessage = "Binding has been scheduled"
)

// Scheduler is the scheduler schema, which is used to schedule a specific resource to specific clusters
type Scheduler struct {
	DynamicClient        dynamic.Interface
	KarmadaClient        karmadaclientset.Interface
	KubeClient           kubernetes.Interface
	bindingLister        worklister.ResourceBindingLister
	policyLister         policylister.PropagationPolicyLister
	clusterBindingLister worklister.ClusterResourceBindingLister
	clusterPolicyLister  policylister.ClusterPropagationPolicyLister
	clusterLister        clusterlister.ClusterLister
	informerFactory      informerfactory.SharedInformerFactory

	// TODO: implement a priority scheduling queue
	queue workqueue.RateLimitingInterface

	Algorithm      core.ScheduleAlgorithm
	schedulerCache schedulercache.Cache

	eventRecorder record.EventRecorder

	enableSchedulerEstimator bool
	schedulerEstimatorCache  *estimatorclient.SchedulerEstimatorCache
	schedulerEstimatorPort   int
	schedulerEstimatorWorker util.AsyncWorker
}

// NewScheduler instantiates a scheduler
func NewScheduler(dynamicClient dynamic.Interface, karmadaClient karmadaclientset.Interface, kubeClient kubernetes.Interface, opts *options.Options) *Scheduler {
	factory := informerfactory.NewSharedInformerFactory(karmadaClient, 0)
	bindingLister := factory.Work().V1alpha2().ResourceBindings().Lister()
	policyLister := factory.Policy().V1alpha1().PropagationPolicies().Lister()
	clusterBindingLister := factory.Work().V1alpha2().ClusterResourceBindings().Lister()
	clusterPolicyLister := factory.Policy().V1alpha1().ClusterPropagationPolicies().Lister()
	clusterLister := factory.Cluster().V1alpha1().Clusters().Lister()
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	schedulerCache := schedulercache.NewCache(clusterLister)
	// TODO: make plugins as a flag
	algorithm := core.NewGenericScheduler(schedulerCache, []string{clusteraffinity.Name, tainttoleration.Name, apiinstalled.Name})
	sched := &Scheduler{
		DynamicClient:            dynamicClient,
		KarmadaClient:            karmadaClient,
		KubeClient:               kubeClient,
		bindingLister:            bindingLister,
		policyLister:             policyLister,
		clusterBindingLister:     clusterBindingLister,
		clusterPolicyLister:      clusterPolicyLister,
		clusterLister:            clusterLister,
		informerFactory:          factory,
		queue:                    queue,
		Algorithm:                algorithm,
		schedulerCache:           schedulerCache,
		enableSchedulerEstimator: opts.EnableSchedulerEstimator,
	}
	if opts.EnableSchedulerEstimator {
		sched.schedulerEstimatorCache = estimatorclient.NewSchedulerEstimatorCache()
		sched.schedulerEstimatorPort = opts.SchedulerEstimatorPort
		sched.schedulerEstimatorWorker = util.NewAsyncWorker("scheduler-estimator", nil, sched.reconcileEstimatorConnection)
		schedulerEstimator := estimatorclient.NewSchedulerEstimator(sched.schedulerEstimatorCache, opts.SchedulerEstimatorTimeout.Duration)
		estimatorclient.RegisterSchedulerEstimator(schedulerEstimator)
	}

	metrics.Register()

	sched.addAllEventHandlers()
	return sched
}

// Run runs the scheduler
func (s *Scheduler) Run(ctx context.Context) {
	stopCh := ctx.Done()
	klog.Infof("Starting karmada scheduler")
	defer klog.Infof("Shutting down karmada scheduler")

	// Establish all connections first and then begin scheduling.
	if s.enableSchedulerEstimator {
		s.establishEstimatorConnections()
		s.schedulerEstimatorWorker.Run(1, stopCh)
	}

	s.informerFactory.Start(stopCh)
	s.informerFactory.WaitForCacheSync(stopCh)

	go wait.Until(s.worker, time.Second, stopCh)

	<-stopCh
}

func (s *Scheduler) worker() {
	for s.scheduleNext() {
	}
}

func (s *Scheduler) getPlacement(resourceBinding *workv1alpha2.ResourceBinding) (policyv1alpha1.Placement, string, error) {
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

func (s *Scheduler) getClusterPlacement(crb *workv1alpha2.ClusterResourceBinding) (policyv1alpha1.Placement, string, error) {
	var placement policyv1alpha1.Placement
	policyName := util.GetLabelValue(crb.Labels, policyv1alpha1.ClusterPropagationPolicyLabel)

	policy, err := s.clusterPolicyLister.Get(policyName)
	if err != nil {
		return placement, "", err
	}

	placement = policy.Spec.Placement
	placementBytes, err := json.Marshal(placement)
	if err != nil {
		klog.Errorf("Failed to marshal placement of propagationPolicy %s/%s, error: %v", policy.Namespace, policy.Name, err)
		return placement, "", err
	}
	return placement, string(placementBytes), nil
}

func (s *Scheduler) scheduleNext() bool {
	key, shutdown := s.queue.Get()
	if shutdown {
		klog.Errorf("Fail to pop item from queue")
		return false
	}
	defer s.queue.Done(key)

	err := s.doSchedule(key.(string))
	s.handleErr(err, key)
	return true
}

func (s *Scheduler) doSchedule(key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	if len(ns) > 0 {
		return s.doScheduleBinding(ns, name)
	}
	return s.doScheduleClusterBinding(name)
}

func (s *Scheduler) doScheduleBinding(namespace, name string) (err error) {
	rb, err := s.bindingLister.ResourceBindings(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// the binding does not exist, do nothing
			return nil
		}
		return err
	}

	// Update "Scheduled" condition according to schedule result.
	defer func() {
		s.recordScheduleResultEventForResourceBinding(rb, err)
		var condition metav1.Condition
		if err == nil {
			condition = util.NewCondition(workv1alpha2.Scheduled, scheduleSuccessReason, scheduleSuccessMessage, metav1.ConditionTrue)
		} else {
			condition = util.NewCondition(workv1alpha2.Scheduled, scheduleFailedReason, err.Error(), metav1.ConditionFalse)
		}
		if updateErr := s.updateBindingScheduledConditionIfNeeded(rb, condition); updateErr != nil {
			klog.Errorf("Failed update condition(%s) for ResourceBinding(%s/%s)", workv1alpha2.Scheduled, rb.Namespace, rb.Name)
			if err == nil {
				// schedule succeed but update condition failed, return err in order to retry in next loop.
				err = updateErr
			}
		}
	}()

	start := time.Now()
	policyPlacement, policyPlacementStr, err := s.getPlacement(rb)
	if err != nil {
		return err
	}
	appliedPlacement := util.GetLabelValue(rb.Annotations, util.PolicyPlacementAnnotation)
	if policyPlacementStr != appliedPlacement {
		// policy placement changed, need schedule
		klog.Infof("Start to schedule ResourceBinding(%s/%s) as placement changed", namespace, name)
		err = s.scheduleResourceBinding(rb)
		metrics.BindingSchedule(string(ReconcileSchedule), metrics.SinceInSeconds(start), err)
		return err
	}
	if policyPlacement.ReplicaScheduling != nil && util.IsBindingReplicasChanged(&rb.Spec, policyPlacement.ReplicaScheduling) {
		// binding replicas changed, need reschedule
		klog.Infof("Reschedule ResourceBinding(%s/%s) as replicas scaled down or scaled up", namespace, name)
		err = s.scheduleResourceBinding(rb)
		metrics.BindingSchedule(string(ScaleSchedule), metrics.SinceInSeconds(start), err)
		return err
	}
	exist, ready := s.allClustersInReadyState(rb.Spec.Clusters)
	if !exist {
		klog.Infof("Reschedule ResourceBinding(%s/%s) as cluster unjoined", namespace, name)
		err = s.rescheduleResourceBinding(rb)
		// TODO(dddddai): add metrics
		return err
	}
	if !ready && features.FeatureGate.Enabled(features.Failover) {
		klog.Infof("Reschedule ResourceBinding(%s/%s) as cluster failure", namespace, name)
		err = s.rescheduleResourceBinding(rb)
		metrics.BindingSchedule(string(FailoverSchedule), metrics.SinceInSeconds(start), err)
		return err
	}
	klog.Infof("Don't need to schedule ResourceBinding(%s/%s)", namespace, name)
	return nil
}

func (s *Scheduler) doScheduleClusterBinding(name string) (err error) {
	crb, err := s.clusterBindingLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// the binding does not exist, do nothing
			return nil
		}
		return err
	}

	// Update "Scheduled" condition according to schedule result.
	defer func() {
		s.recordScheduleResultEventForClusterResourceBinding(crb, err)
		var condition metav1.Condition
		if err == nil {
			condition = util.NewCondition(workv1alpha2.Scheduled, scheduleSuccessReason, scheduleSuccessMessage, metav1.ConditionTrue)
		} else {
			condition = util.NewCondition(workv1alpha2.Scheduled, scheduleFailedReason, err.Error(), metav1.ConditionFalse)
		}
		if updateErr := s.updateClusterBindingScheduledConditionIfNeeded(crb, condition); updateErr != nil {
			klog.Errorf("Failed update condition(%s) for ClusterResourceBinding(%s)", workv1alpha2.Scheduled, crb.Name)
			if err == nil {
				// schedule succeed but update condition failed, return err in order to retry in next loop.
				err = updateErr
			}
		}
	}()

	start := time.Now()
	policyPlacement, policyPlacementStr, err := s.getClusterPlacement(crb)
	if err != nil {
		return err
	}
	appliedPlacement := util.GetLabelValue(crb.Annotations, util.PolicyPlacementAnnotation)
	if policyPlacementStr != appliedPlacement {
		// policy placement changed, need schedule
		klog.Infof("Start to schedule ClusterResourceBinding(%s) as placement changed", name)
		err = s.scheduleClusterResourceBinding(crb)
		metrics.BindingSchedule(string(ReconcileSchedule), metrics.SinceInSeconds(start), err)
		return err
	}
	if policyPlacement.ReplicaScheduling != nil && util.IsBindingReplicasChanged(&crb.Spec, policyPlacement.ReplicaScheduling) {
		// binding replicas changed, need reschedule
		klog.Infof("Reschedule ClusterResourceBinding(%s) as replicas scaled down or scaled up", name)
		err = s.scheduleClusterResourceBinding(crb)
		metrics.BindingSchedule(string(ScaleSchedule), metrics.SinceInSeconds(start), err)
		return err
	}
	exist, ready := s.allClustersInReadyState(crb.Spec.Clusters)
	if !exist {
		klog.Infof("Reschedule ClusterResourceBinding(%s) as cluster unjoined", name)
		err = s.rescheduleClusterResourceBinding(crb)
		// TODO(dddddai): add metrics
		return err
	}
	if !ready && features.FeatureGate.Enabled(features.Failover) {
		klog.Infof("Reschedule ClusterResourceBinding(%s) as cluster failure", name)
		err = s.rescheduleClusterResourceBinding(crb)
		metrics.BindingSchedule(string(FailoverSchedule), metrics.SinceInSeconds(start), err)
		return err
	}
	return nil
}

func (s *Scheduler) scheduleResourceBinding(resourceBinding *workv1alpha2.ResourceBinding) (err error) {
	klog.V(4).InfoS("Begin scheduling resource binding", "resourceBinding", klog.KObj(resourceBinding))
	defer klog.V(4).InfoS("End scheduling resource binding", "resourceBinding", klog.KObj(resourceBinding))

	placement, placementStr, err := s.getPlacement(resourceBinding)
	if err != nil {
		return err
	}
	scheduleResult, err := s.Algorithm.Schedule(context.TODO(), &placement, &resourceBinding.Spec)
	if err != nil {
		klog.Errorf("Failed scheduling ResourceBinding %s/%s: %v", resourceBinding.Namespace, resourceBinding.Name, err)
		return err
	}
	klog.V(4).Infof("ResourceBinding %s/%s scheduled to clusters %v", resourceBinding.Namespace, resourceBinding.Name, scheduleResult.SuggestedClusters)

	binding := resourceBinding.DeepCopy()
	binding.Spec.Clusters = scheduleResult.SuggestedClusters

	if binding.Annotations == nil {
		binding.Annotations = make(map[string]string)
	}
	binding.Annotations[util.PolicyPlacementAnnotation] = placementStr

	_, err = s.KarmadaClient.WorkV1alpha2().ResourceBindings(binding.Namespace).Update(context.TODO(), binding, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (s *Scheduler) scheduleClusterResourceBinding(clusterResourceBinding *workv1alpha2.ClusterResourceBinding) (err error) {
	klog.V(4).InfoS("Begin scheduling cluster resource binding", "clusterResourceBinding", klog.KObj(clusterResourceBinding))
	defer klog.V(4).InfoS("End scheduling cluster resource binding", "clusterResourceBinding", klog.KObj(clusterResourceBinding))

	clusterPolicyName := util.GetLabelValue(clusterResourceBinding.Labels, policyv1alpha1.ClusterPropagationPolicyLabel)
	policy, err := s.clusterPolicyLister.Get(clusterPolicyName)
	if err != nil {
		return err
	}
	scheduleResult, err := s.Algorithm.Schedule(context.TODO(), &policy.Spec.Placement, &clusterResourceBinding.Spec)
	if err != nil {
		klog.V(2).Infof("Failed scheduling ClusterResourceBinding %s: %v", clusterResourceBinding.Name, err)
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

	_, err = s.KarmadaClient.WorkV1alpha2().ClusterResourceBindings().Update(context.TODO(), binding, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (s *Scheduler) handleErr(err error, key interface{}) {
	if err == nil || apierrors.HasStatusCause(err, corev1.NamespaceTerminatingCause) {
		s.queue.Forget(key)
		return
	}

	s.queue.AddRateLimited(key)
	metrics.CountSchedulerBindings(metrics.ScheduleAttemptFailure)
}

func (s *Scheduler) rescheduleClusterResourceBinding(clusterResourceBinding *workv1alpha2.ClusterResourceBinding) error {
	klog.V(4).InfoS("Begin rescheduling cluster resource binding", "clusterResourceBinding", klog.KObj(clusterResourceBinding))
	defer klog.V(4).InfoS("End rescheduling cluster resource binding", "clusterResourceBinding", klog.KObj(clusterResourceBinding))

	policyName := util.GetLabelValue(clusterResourceBinding.Labels, policyv1alpha1.ClusterPropagationPolicyLabel)
	policy, err := s.clusterPolicyLister.Get(policyName)
	if err != nil {
		klog.Errorf("Failed to get policy by policyName(%s): Error: %v", policyName, err)
		return err
	}
	reScheduleResult, err := s.Algorithm.ReSchedule(context.TODO(), &policy.Spec.Placement, &clusterResourceBinding.Spec)
	if err != nil {
		return err
	}
	if len(reScheduleResult.SuggestedClusters) == 0 {
		return nil
	}

	clusterResourceBinding.Spec.Clusters = reScheduleResult.SuggestedClusters
	klog.Infof("The final binding.Spec.Cluster values are: %v\n", clusterResourceBinding.Spec.Clusters)

	_, err = s.KarmadaClient.WorkV1alpha2().ClusterResourceBindings().Update(context.TODO(), clusterResourceBinding, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (s *Scheduler) rescheduleResourceBinding(resourceBinding *workv1alpha2.ResourceBinding) error {
	klog.V(4).InfoS("Begin rescheduling resource binding", "resourceBinding", klog.KObj(resourceBinding))
	defer klog.V(4).InfoS("End rescheduling resource binding", "resourceBinding", klog.KObj(resourceBinding))

	placement, _, err := s.getPlacement(resourceBinding)
	if err != nil {
		klog.Errorf("Failed to get placement by resourceBinding(%s/%s): Error: %v", resourceBinding.Namespace, resourceBinding.Name, err)
		return err
	}
	reScheduleResult, err := s.Algorithm.ReSchedule(context.TODO(), &placement, &resourceBinding.Spec)
	if err != nil {
		return err
	}
	if len(reScheduleResult.SuggestedClusters) == 0 {
		return nil
	}

	resourceBinding.Spec.Clusters = reScheduleResult.SuggestedClusters
	klog.Infof("The final binding.Spec.Cluster values are: %v\n", resourceBinding.Spec.Clusters)

	_, err = s.KarmadaClient.WorkV1alpha2().ResourceBindings(resourceBinding.Namespace).Update(context.TODO(), resourceBinding, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// allClustersInReadyState reports if all target clusters exist and stay ready
func (s *Scheduler) allClustersInReadyState(tcs []workv1alpha2.TargetCluster) (exist, ready bool) {
	clusters := s.schedulerCache.Snapshot().GetClusters()
	count := 0
	ready = true
	for i := range tcs {
		for _, c := range clusters {
			if c.Cluster().Name == tcs[i].Name {
				if meta.IsStatusConditionFalse(c.Cluster().Status.Conditions, clusterv1alpha1.ClusterConditionReady) {
					ready = false
				}
				if c.Cluster().DeletionTimestamp.IsZero() {
					count++
				}
				continue
			}
		}
	}
	return count == len(tcs), ready
}

func (s *Scheduler) reconcileEstimatorConnection(key util.QueueKey) error {
	name, ok := key.(string)
	if !ok {
		return fmt.Errorf("failed to reconcile estimator connection as invalid key: %v", key)
	}

	_, err := s.clusterLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			s.schedulerEstimatorCache.DeleteCluster(name)
			return nil
		}
		return err
	}
	return estimatorclient.EstablishConnection(name, s.schedulerEstimatorCache, s.schedulerEstimatorPort)
}

func (s *Scheduler) establishEstimatorConnections() {
	clusterList, err := s.KarmadaClient.ClusterV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Cannot list all clusters when establish all cluster estimator connections: %v", err)
		return
	}
	for i := range clusterList.Items {
		if err = estimatorclient.EstablishConnection(clusterList.Items[i].Name, s.schedulerEstimatorCache, s.schedulerEstimatorPort); err != nil {
			klog.Error(err)
		}
	}
}

// updateBindingScheduledConditionIfNeeded sets the scheduled condition of ResourceBinding if needed
func (s *Scheduler) updateBindingScheduledConditionIfNeeded(rb *workv1alpha2.ResourceBinding, newScheduledCondition metav1.Condition) error {
	if rb == nil {
		return nil
	}

	oldScheduledCondition := meta.FindStatusCondition(rb.Status.Conditions, workv1alpha2.Scheduled)
	if oldScheduledCondition != nil {
		if util.IsConditionsEqual(newScheduledCondition, *oldScheduledCondition) {
			klog.V(4).Infof("No need to update scheduled condition")
			return nil
		}
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		meta.SetStatusCondition(&rb.Status.Conditions, newScheduledCondition)
		_, updateErr := s.KarmadaClient.WorkV1alpha2().ResourceBindings(rb.Namespace).UpdateStatus(context.TODO(), rb, metav1.UpdateOptions{})
		if updateErr == nil {
			return nil
		}

		if updated, err := s.KarmadaClient.WorkV1alpha2().ResourceBindings(rb.Namespace).Get(context.TODO(), rb.Name, metav1.GetOptions{}); err == nil {
			// make a copy so we don't mutate the shared cache
			rb = updated.DeepCopy()
		} else {
			klog.Errorf("failed to get updated resource binding %s/%s: %v", rb.Namespace, rb.Name, err)
		}

		return updateErr
	})
}

// updateClusterBindingScheduledConditionIfNeeded sets the scheduled condition of ClusterResourceBinding if needed
func (s *Scheduler) updateClusterBindingScheduledConditionIfNeeded(crb *workv1alpha2.ClusterResourceBinding, newScheduledCondition metav1.Condition) error {
	if crb == nil {
		return nil
	}

	oldScheduledCondition := meta.FindStatusCondition(crb.Status.Conditions, workv1alpha2.Scheduled)
	if oldScheduledCondition != nil {
		if util.IsConditionsEqual(newScheduledCondition, *oldScheduledCondition) {
			klog.V(4).Infof("No need to update scheduled condition")
			return nil
		}
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		meta.SetStatusCondition(&crb.Status.Conditions, newScheduledCondition)
		_, updateErr := s.KarmadaClient.WorkV1alpha2().ClusterResourceBindings().UpdateStatus(context.TODO(), crb, metav1.UpdateOptions{})
		if updateErr == nil {
			return nil
		}

		if updated, err := s.KarmadaClient.WorkV1alpha2().ClusterResourceBindings().Get(context.TODO(), crb.Name, metav1.GetOptions{}); err == nil {
			// make a copy so we don't mutate the shared cache
			crb = updated.DeepCopy()
		} else {
			klog.Errorf("failed to get updated cluster resource binding %s: %v", crb.Name, err)
		}

		return updateErr
	})
}

func (s *Scheduler) recordScheduleResultEventForResourceBinding(rb *workv1alpha2.ResourceBinding, schedulerErr error) {
	if rb == nil {
		return
	}

	ref := &corev1.ObjectReference{
		Kind:       rb.Spec.Resource.Kind,
		APIVersion: rb.Spec.Resource.APIVersion,
		Namespace:  rb.Spec.Resource.Namespace,
		Name:       rb.Spec.Resource.Name,
		UID:        rb.Spec.Resource.UID,
	}

	if schedulerErr == nil {
		s.eventRecorder.Event(rb, corev1.EventTypeNormal, workv1alpha2.EventReasonScheduleBindingSucceed, scheduleSuccessMessage)
		s.eventRecorder.Event(ref, corev1.EventTypeNormal, workv1alpha2.EventReasonScheduleBindingSucceed, scheduleSuccessMessage)
	} else {
		s.eventRecorder.Event(rb, corev1.EventTypeWarning, workv1alpha2.EventReasonScheduleBindingFailed, schedulerErr.Error())
		s.eventRecorder.Event(ref, corev1.EventTypeWarning, workv1alpha2.EventReasonScheduleBindingFailed, schedulerErr.Error())
	}
}

func (s *Scheduler) recordScheduleResultEventForClusterResourceBinding(crb *workv1alpha2.ClusterResourceBinding, schedulerErr error) {
	if crb == nil {
		return
	}

	ref := &corev1.ObjectReference{
		Kind:       crb.Spec.Resource.Kind,
		APIVersion: crb.Spec.Resource.APIVersion,
		Namespace:  crb.Spec.Resource.Namespace,
		Name:       crb.Spec.Resource.Name,
		UID:        crb.Spec.Resource.UID,
	}

	if schedulerErr == nil {
		s.eventRecorder.Event(crb, corev1.EventTypeNormal, workv1alpha2.EventReasonScheduleBindingSucceed, scheduleSuccessMessage)
		s.eventRecorder.Event(ref, corev1.EventTypeNormal, workv1alpha2.EventReasonScheduleBindingSucceed, scheduleSuccessMessage)
	} else {
		s.eventRecorder.Event(crb, corev1.EventTypeWarning, workv1alpha2.EventReasonScheduleBindingFailed, schedulerErr.Error())
		s.eventRecorder.Event(ref, corev1.EventTypeWarning, workv1alpha2.EventReasonScheduleBindingFailed, schedulerErr.Error())
	}
}
