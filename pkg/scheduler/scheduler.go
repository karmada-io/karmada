package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	policylister "github.com/karmada-io/karmada/pkg/generated/listers/policy/v1alpha1"
	worklister "github.com/karmada-io/karmada/pkg/generated/listers/work/v1alpha2"
	schedulercache "github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/core"
	frameworkplugins "github.com/karmada-io/karmada/pkg/scheduler/framework/plugins"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/runtime"
	"github.com/karmada-io/karmada/pkg/scheduler/metrics"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	utilmetrics "github.com/karmada-io/karmada/pkg/util/metrics"
)

// ScheduleType defines the schedule type of a binding object should be performed.
type ScheduleType string

const (
	// ReconcileSchedule means the binding object associated policy has been changed.
	ReconcileSchedule ScheduleType = "ReconcileSchedule"

	// ScaleSchedule means the replicas of binding object has been changed.
	ScaleSchedule ScheduleType = "ScaleSchedule"
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

	enableSchedulerEstimator            bool
	disableSchedulerEstimatorInPullMode bool
	schedulerEstimatorCache             *estimatorclient.SchedulerEstimatorCache
	schedulerEstimatorPort              int
	schedulerEstimatorWorker            util.AsyncWorker

	enableEmptyWorkloadPropagation bool
}

type schedulerOptions struct {
	// enableSchedulerEstimator represents whether the accurate scheduler estimator should be enabled.
	enableSchedulerEstimator bool
	// disableSchedulerEstimatorInPullMode represents whether to disable the scheduler estimator in pull mode.
	disableSchedulerEstimatorInPullMode bool
	// schedulerEstimatorTimeout specifies the timeout period of calling the accurate scheduler estimator service.
	schedulerEstimatorTimeout metav1.Duration
	// schedulerEstimatorPort is the port that the accurate scheduler estimator server serves at.
	schedulerEstimatorPort int
	//enableEmptyWorkloadPropagation represents whether allow workload with replicas 0 propagated to member clusters should be enabled
	enableEmptyWorkloadPropagation bool
	// outOfTreeRegistry represents the registry of out-of-tree plugins
	outOfTreeRegistry runtime.Registry
	// plugins is the list of plugins to enable or disable
	plugins []string
}

// Option configures a Scheduler
type Option func(*schedulerOptions)

// WithEnableSchedulerEstimator sets the enableSchedulerEstimator for scheduler
func WithEnableSchedulerEstimator(enableSchedulerEstimator bool) Option {
	return func(o *schedulerOptions) {
		o.enableSchedulerEstimator = enableSchedulerEstimator
	}
}

// WithDisableSchedulerEstimatorInPullMode sets the disableSchedulerEstimatorInPullMode for scheduler
func WithDisableSchedulerEstimatorInPullMode(disableSchedulerEstimatorInPullMode bool) Option {
	return func(o *schedulerOptions) {
		o.disableSchedulerEstimatorInPullMode = disableSchedulerEstimatorInPullMode
	}
}

// WithSchedulerEstimatorTimeout sets the schedulerEstimatorTimeout for scheduler
func WithSchedulerEstimatorTimeout(schedulerEstimatorTimeout metav1.Duration) Option {
	return func(o *schedulerOptions) {
		o.schedulerEstimatorTimeout = schedulerEstimatorTimeout
	}
}

// WithSchedulerEstimatorPort sets the schedulerEstimatorPort for scheduler
func WithSchedulerEstimatorPort(schedulerEstimatorPort int) Option {
	return func(o *schedulerOptions) {
		o.schedulerEstimatorPort = schedulerEstimatorPort
	}
}

// WithEnableEmptyWorkloadPropagation sets the enablePropagateEmptyWorkLoad for scheduler
func WithEnableEmptyWorkloadPropagation(enableEmptyWorkloadPropagation bool) Option {
	return func(o *schedulerOptions) {
		o.enableEmptyWorkloadPropagation = enableEmptyWorkloadPropagation
	}
}

// WithEnableSchedulerPlugin sets the scheduler-plugin for scheduler
func WithEnableSchedulerPlugin(plugins []string) Option {
	return func(o *schedulerOptions) {
		o.plugins = plugins
	}
}

// WithOutOfTreeRegistry sets the registry for out-of-tree plugins. Those plugins
// will be appended to the default in-tree registry.
func WithOutOfTreeRegistry(registry runtime.Registry) Option {
	return func(o *schedulerOptions) {
		o.outOfTreeRegistry = registry
	}
}

// NewScheduler instantiates a scheduler
func NewScheduler(dynamicClient dynamic.Interface, karmadaClient karmadaclientset.Interface, kubeClient kubernetes.Interface, opts ...Option) (*Scheduler, error) {
	factory := informerfactory.NewSharedInformerFactory(karmadaClient, 0)
	bindingLister := factory.Work().V1alpha2().ResourceBindings().Lister()
	policyLister := factory.Policy().V1alpha1().PropagationPolicies().Lister()
	clusterBindingLister := factory.Work().V1alpha2().ClusterResourceBindings().Lister()
	clusterPolicyLister := factory.Policy().V1alpha1().ClusterPropagationPolicies().Lister()
	clusterLister := factory.Cluster().V1alpha1().Clusters().Lister()
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	schedulerCache := schedulercache.NewCache(clusterLister)

	options := schedulerOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	registry := frameworkplugins.NewInTreeRegistry()
	if err := registry.Merge(options.outOfTreeRegistry); err != nil {
		return nil, err
	}
	registry = registry.Filter(options.plugins)
	algorithm, err := core.NewGenericScheduler(schedulerCache, registry)
	if err != nil {
		return nil, err
	}

	sched := &Scheduler{
		DynamicClient:        dynamicClient,
		KarmadaClient:        karmadaClient,
		KubeClient:           kubeClient,
		bindingLister:        bindingLister,
		policyLister:         policyLister,
		clusterBindingLister: clusterBindingLister,
		clusterPolicyLister:  clusterPolicyLister,
		clusterLister:        clusterLister,
		informerFactory:      factory,
		queue:                queue,
		Algorithm:            algorithm,
		schedulerCache:       schedulerCache,
	}

	if options.enableSchedulerEstimator {
		sched.enableSchedulerEstimator = options.enableSchedulerEstimator
		sched.disableSchedulerEstimatorInPullMode = options.disableSchedulerEstimatorInPullMode
		sched.schedulerEstimatorPort = options.schedulerEstimatorPort
		sched.schedulerEstimatorCache = estimatorclient.NewSchedulerEstimatorCache()
		schedulerEstimatorWorkerOptions := util.Options{
			Name:          "scheduler-estimator",
			KeyFunc:       nil,
			ReconcileFunc: sched.reconcileEstimatorConnection,
		}
		sched.schedulerEstimatorWorker = util.NewAsyncWorker(schedulerEstimatorWorkerOptions)
		schedulerEstimator := estimatorclient.NewSchedulerEstimator(sched.schedulerEstimatorCache, options.schedulerEstimatorTimeout.Duration)
		estimatorclient.RegisterSchedulerEstimator(schedulerEstimator)
	}
	sched.enableEmptyWorkloadPropagation = options.enableEmptyWorkloadPropagation

	sched.addAllEventHandlers()
	return sched, nil
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
		if updateErr := s.patchBindingScheduleStatus(rb, condition); updateErr != nil {
			klog.Errorf("Failed to patch schedule status to ResourceBinding(%s/%s): %v", rb.Namespace, rb.Name, err)
			if err == nil {
				// schedule succeed but update status failed, return err in order to retry in next loop.
				err = updateErr
			}
		}
	}()

	start := time.Now()
	policyPlacement, policyPlacementStr, err := s.getPlacement(rb)
	if err != nil {
		return err
	}
	if appliedPlacement := util.GetLabelValue(rb.Annotations, util.PolicyPlacementAnnotation); policyPlacementStr != appliedPlacement {
		// policy placement changed, need schedule
		klog.Infof("Start to schedule ResourceBinding(%s/%s) as placement changed", namespace, name)
		err = s.scheduleResourceBinding(rb)
		metrics.BindingSchedule(string(ReconcileSchedule), utilmetrics.DurationInSeconds(start), err)
		return err
	}
	if policyPlacement.ReplicaScheduling != nil && util.IsBindingReplicasChanged(&rb.Spec, policyPlacement.ReplicaScheduling) {
		// binding replicas changed, need reschedule
		klog.Infof("Reschedule ResourceBinding(%s/%s) as replicas scaled down or scaled up", namespace, name)
		err = s.scheduleResourceBinding(rb)
		metrics.BindingSchedule(string(ScaleSchedule), utilmetrics.DurationInSeconds(start), err)
		return err
	}
	if rb.Spec.Replicas == 0 ||
		policyPlacement.ReplicaScheduling == nil ||
		policyPlacement.ReplicaScheduling.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		// Duplicated resources should always be scheduled. Note: non-workload is considered as duplicated
		// even if scheduling type is divided.
		klog.V(3).Infof("Start to schedule ResourceBinding(%s/%s) as scheduling type is duplicated", namespace, name)
		err = s.scheduleResourceBinding(rb)
		metrics.BindingSchedule(string(ReconcileSchedule), utilmetrics.DurationInSeconds(start), err)
		return err
	}
	// TODO(dddddai): reschedule bindings on cluster change
	klog.V(3).Infof("Don't need to schedule ResourceBinding(%s/%s)", namespace, name)
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
		if updateErr := s.patchClusterBindingScheduleStatus(crb, condition); updateErr != nil {
			klog.Errorf("Failed to patch schedule status to ClusterResourceBinding(%s): %v", crb.Name, err)
			if err == nil {
				// schedule succeed but update status failed, return err in order to retry in next loop.
				err = updateErr
			}
		}
	}()

	start := time.Now()
	policyPlacement, policyPlacementStr, err := s.getClusterPlacement(crb)
	if err != nil {
		return err
	}
	if appliedPlacement := util.GetLabelValue(crb.Annotations, util.PolicyPlacementAnnotation); policyPlacementStr != appliedPlacement {
		// policy placement changed, need schedule
		klog.Infof("Start to schedule ClusterResourceBinding(%s) as placement changed", name)
		err = s.scheduleClusterResourceBinding(crb)
		metrics.BindingSchedule(string(ReconcileSchedule), utilmetrics.DurationInSeconds(start), err)
		return err
	}
	if policyPlacement.ReplicaScheduling != nil && util.IsBindingReplicasChanged(&crb.Spec, policyPlacement.ReplicaScheduling) {
		// binding replicas changed, need reschedule
		klog.Infof("Reschedule ClusterResourceBinding(%s) as replicas scaled down or scaled up", name)
		err = s.scheduleClusterResourceBinding(crb)
		metrics.BindingSchedule(string(ScaleSchedule), utilmetrics.DurationInSeconds(start), err)
		return err
	}
	if crb.Spec.Replicas == 0 ||
		policyPlacement.ReplicaScheduling == nil ||
		policyPlacement.ReplicaScheduling.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		// Duplicated resources should always be scheduled. Note: non-workload is considered as duplicated
		// even if scheduling type is divided.
		klog.V(3).Infof("Start to schedule ClusterResourceBinding(%s) as scheduling type is duplicated", name)
		err = s.scheduleClusterResourceBinding(crb)
		metrics.BindingSchedule(string(ReconcileSchedule), utilmetrics.DurationInSeconds(start), err)
		return err
	}
	// TODO(dddddai): reschedule bindings on cluster change
	klog.Infof("Don't need to schedule ClusterResourceBinding(%s)", name)
	return nil
}

func (s *Scheduler) scheduleResourceBinding(resourceBinding *workv1alpha2.ResourceBinding) (err error) {
	klog.V(4).InfoS("Begin scheduling resource binding", "resourceBinding", klog.KObj(resourceBinding))
	defer klog.V(4).InfoS("End scheduling resource binding", "resourceBinding", klog.KObj(resourceBinding))

	placement, placementStr, err := s.getPlacement(resourceBinding)
	if err != nil {
		return err
	}

	scheduleResult, err := s.Algorithm.Schedule(context.TODO(), &placement, &resourceBinding.Spec, &core.ScheduleAlgorithmOption{EnableEmptyWorkloadPropagation: s.enableEmptyWorkloadPropagation})
	if err != nil {
		klog.Errorf("Failed scheduling ResourceBinding %s/%s: %v", resourceBinding.Namespace, resourceBinding.Name, err)
		return err
	}

	klog.V(4).Infof("ResourceBinding %s/%s scheduled to clusters %v", resourceBinding.Namespace, resourceBinding.Name, scheduleResult.SuggestedClusters)
	return s.patchScheduleResultForResourceBinding(resourceBinding, placementStr, scheduleResult.SuggestedClusters)
}

func (s *Scheduler) patchScheduleResultForResourceBinding(oldBinding *workv1alpha2.ResourceBinding, placement string, scheduleResult []workv1alpha2.TargetCluster) error {
	newBinding := oldBinding.DeepCopy()
	if newBinding.Annotations == nil {
		newBinding.Annotations = make(map[string]string)
	}
	newBinding.Annotations[util.PolicyPlacementAnnotation] = placement
	newBinding.Spec.Clusters = scheduleResult

	oldData, err := json.Marshal(oldBinding)
	if err != nil {
		return fmt.Errorf("failed to marshal the existing resource binding(%s/%s): %v", oldBinding.Namespace, oldBinding.Name, err)
	}
	newData, err := json.Marshal(newBinding)
	if err != nil {
		return fmt.Errorf("failed to marshal the new resource binding(%s/%s): %v", newBinding.Namespace, newBinding.Name, err)
	}
	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return fmt.Errorf("failed to create a merge patch: %v", err)
	}

	_, err = s.KarmadaClient.WorkV1alpha2().ResourceBindings(newBinding.Namespace).Patch(context.TODO(), newBinding.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	return err
}

func (s *Scheduler) scheduleClusterResourceBinding(clusterResourceBinding *workv1alpha2.ClusterResourceBinding) (err error) {
	klog.V(4).InfoS("Begin scheduling cluster resource binding", "clusterResourceBinding", klog.KObj(clusterResourceBinding))
	defer klog.V(4).InfoS("End scheduling cluster resource binding", "clusterResourceBinding", klog.KObj(clusterResourceBinding))

	clusterPolicyName := util.GetLabelValue(clusterResourceBinding.Labels, policyv1alpha1.ClusterPropagationPolicyLabel)
	policy, err := s.clusterPolicyLister.Get(clusterPolicyName)
	if err != nil {
		return err
	}

	placement, err := json.Marshal(policy.Spec.Placement)
	if err != nil {
		klog.Errorf("Failed to marshal placement of clusterPropagationPolicy %s, error: %v", policy.Name, err)
		return err
	}

	scheduleResult, err := s.Algorithm.Schedule(context.TODO(), &policy.Spec.Placement, &clusterResourceBinding.Spec, &core.ScheduleAlgorithmOption{EnableEmptyWorkloadPropagation: s.enableEmptyWorkloadPropagation})
	if err != nil {
		klog.V(2).Infof("Failed scheduling ClusterResourceBinding %s: %v", clusterResourceBinding.Name, err)
		return err
	}

	klog.V(4).Infof("ClusterResourceBinding %s scheduled to clusters %v", clusterResourceBinding.Name, scheduleResult.SuggestedClusters)
	return s.patchScheduleResultForClusterResourceBinding(clusterResourceBinding, string(placement), scheduleResult.SuggestedClusters)
}

func (s *Scheduler) patchScheduleResultForClusterResourceBinding(oldBinding *workv1alpha2.ClusterResourceBinding, placement string, scheduleResult []workv1alpha2.TargetCluster) error {
	newBinding := oldBinding.DeepCopy()
	if newBinding.Annotations == nil {
		newBinding.Annotations = make(map[string]string)
	}
	newBinding.Annotations[util.PolicyPlacementAnnotation] = placement
	newBinding.Spec.Clusters = scheduleResult

	oldData, err := json.Marshal(oldBinding)
	if err != nil {
		return fmt.Errorf("failed to marshal the existing cluster resource binding(%s): %v", oldBinding.Name, err)
	}
	newData, err := json.Marshal(newBinding)
	if err != nil {
		return fmt.Errorf("failed to marshal the new resource binding(%s): %v", newBinding.Name, err)
	}
	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return fmt.Errorf("failed to create a merge patch: %v", err)
	}

	_, err = s.KarmadaClient.WorkV1alpha2().ClusterResourceBindings().Patch(context.TODO(), newBinding.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	return err
}

func (s *Scheduler) handleErr(err error, key interface{}) {
	if err == nil || apierrors.HasStatusCause(err, corev1.NamespaceTerminatingCause) {
		s.queue.Forget(key)
		return
	}

	s.queue.AddRateLimited(key)
	metrics.CountSchedulerBindings(metrics.ScheduleAttemptFailure)
}

func (s *Scheduler) reconcileEstimatorConnection(key util.QueueKey) error {
	name, ok := key.(string)
	if !ok {
		return fmt.Errorf("failed to reconcile estimator connection as invalid key: %v", key)
	}

	cluster, err := s.clusterLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			s.schedulerEstimatorCache.DeleteCluster(name)
			return nil
		}
		return err
	}
	if cluster.Spec.SyncMode == clusterv1alpha1.Pull && s.disableSchedulerEstimatorInPullMode {
		return nil
	}

	return estimatorclient.EstablishConnection(s.KubeClient, name, s.schedulerEstimatorCache, s.schedulerEstimatorPort)
}

func (s *Scheduler) establishEstimatorConnections() {
	clusterList, err := s.KarmadaClient.ClusterV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Cannot list all clusters when establish all cluster estimator connections: %v", err)
		return
	}
	for i := range clusterList.Items {
		if clusterList.Items[i].Spec.SyncMode == clusterv1alpha1.Pull && s.disableSchedulerEstimatorInPullMode {
			continue
		}
		if err = estimatorclient.EstablishConnection(s.KubeClient, clusterList.Items[i].Name, s.schedulerEstimatorCache, s.schedulerEstimatorPort); err != nil {
			klog.Error(err)
		}
	}
}

// patchBindingScheduleStatus patches schedule status of ResourceBinding when necessary.
func (s *Scheduler) patchBindingScheduleStatus(rb *workv1alpha2.ResourceBinding, newScheduledCondition metav1.Condition) error {
	if rb == nil {
		return nil
	}

	modifiedObj := rb.DeepCopy()
	meta.SetStatusCondition(&modifiedObj.Status.Conditions, newScheduledCondition)
	// Postpone setting observed generation until schedule succeed, assume scheduler will retry and
	// will succeed eventually.
	if newScheduledCondition.Status == metav1.ConditionTrue {
		modifiedObj.Status.SchedulerObservedGeneration = modifiedObj.Generation
	}

	// Short path, ignore patch if no change.
	if reflect.DeepEqual(rb.Status, modifiedObj.Status) {
		return nil
	}

	patchBytes, err := helper.GenMergePatch(rb, modifiedObj)
	if err != nil {
		return fmt.Errorf("failed to create a merge patch: %v", err)
	}

	_, err = s.KarmadaClient.WorkV1alpha2().ResourceBindings(rb.Namespace).Patch(context.TODO(), rb.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		klog.Errorf("Failed to patch schedule status to ResourceBinding(%s/%s): %v", rb.Namespace, rb.Name, err)
		return err
	}

	klog.V(4).Infof("Patch schedule status to ResourceBinding(%s/%s) succeed", rb.Namespace, rb.Name)
	return nil
}

// patchClusterBindingScheduleStatus patches schedule status of ClusterResourceBinding when necessary
func (s *Scheduler) patchClusterBindingScheduleStatus(crb *workv1alpha2.ClusterResourceBinding, newScheduledCondition metav1.Condition) error {
	if crb == nil {
		return nil
	}

	modifiedObj := crb.DeepCopy()
	meta.SetStatusCondition(&modifiedObj.Status.Conditions, newScheduledCondition)
	// Postpone setting observed generation until schedule succeed, assume scheduler will retry and
	// will succeed eventually
	if newScheduledCondition.Status == metav1.ConditionTrue {
		modifiedObj.Status.SchedulerObservedGeneration = modifiedObj.Generation
	}

	// Short path, ignore patch if no change.
	if reflect.DeepEqual(crb.Status, modifiedObj.Status) {
		return nil
	}

	patchBytes, err := helper.GenMergePatch(crb, modifiedObj)
	if err != nil {
		return fmt.Errorf("failed to create a merge patch: %v", err)
	}

	_, err = s.KarmadaClient.WorkV1alpha2().ClusterResourceBindings().Patch(context.TODO(), crb.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		klog.Errorf("Failed to patch schedule status to ClusterResourceBinding(%s): %v", crb.Name, err)
		return err
	}

	klog.V(4).Infof("Patch schedule status to ClusterResourceBinding(%s) succeed", crb.Name)
	return nil
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
