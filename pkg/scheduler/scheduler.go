package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
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
	"github.com/karmada-io/karmada/pkg/events"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	worklister "github.com/karmada-io/karmada/pkg/generated/listers/work/v1alpha2"
	schedulercache "github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/core"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
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

const (
	// DefaultScheduler defines the name of default scheduler.
	DefaultScheduler = "default-scheduler"
)

// Scheduler is the scheduler schema, which is used to schedule a specific resource to specific clusters
type Scheduler struct {
	DynamicClient        dynamic.Interface
	KarmadaClient        karmadaclientset.Interface
	KubeClient           kubernetes.Interface
	bindingLister        worklister.ResourceBindingLister
	clusterBindingLister worklister.ClusterResourceBindingLister
	clusterLister        clusterlister.ClusterLister
	informerFactory      informerfactory.SharedInformerFactory

	// clusterReconcileWorker reconciles cluster changes to trigger corresponding
	// ResourceBinding/ClusterResourceBinding rescheduling.
	clusterReconcileWorker util.AsyncWorker
	// TODO: implement a priority scheduling queue
	queue workqueue.RateLimitingInterface

	Algorithm      core.ScheduleAlgorithm
	schedulerCache schedulercache.Cache

	eventRecorder record.EventRecorder

	enableSchedulerEstimator            bool
	disableSchedulerEstimatorInPullMode bool
	schedulerEstimatorCache             *estimatorclient.SchedulerEstimatorCache
	schedulerEstimatorServicePrefix     string
	schedulerEstimatorPort              int
	schedulerEstimatorWorker            util.AsyncWorker
	schedulerName                       string

	enableEmptyWorkloadPropagation bool
}

type schedulerOptions struct {
	// enableSchedulerEstimator represents whether the accurate scheduler estimator should be enabled.
	enableSchedulerEstimator bool
	// disableSchedulerEstimatorInPullMode represents whether to disable the scheduler estimator in pull mode.
	disableSchedulerEstimatorInPullMode bool
	// schedulerEstimatorTimeout specifies the timeout period of calling the accurate scheduler estimator service.
	schedulerEstimatorTimeout metav1.Duration
	// SchedulerEstimatorServicePrefix presents the prefix of the accurate scheduler estimator service name.
	schedulerEstimatorServicePrefix string
	// schedulerEstimatorPort is the port that the accurate scheduler estimator server serves at.
	schedulerEstimatorPort int
	// schedulerName is the name of the scheduler. Default is "default-scheduler".
	schedulerName string
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

// WithSchedulerEstimatorServicePrefix sets the schedulerEstimatorServicePrefix for scheduler
func WithSchedulerEstimatorServicePrefix(schedulerEstimatorServicePrefix string) Option {
	return func(o *schedulerOptions) {
		o.schedulerEstimatorServicePrefix = schedulerEstimatorServicePrefix
	}
}

// WithSchedulerEstimatorPort sets the schedulerEstimatorPort for scheduler
func WithSchedulerEstimatorPort(schedulerEstimatorPort int) Option {
	return func(o *schedulerOptions) {
		o.schedulerEstimatorPort = schedulerEstimatorPort
	}
}

// WithSchedulerName sets the schedulerName for scheduler
func WithSchedulerName(schedulerName string) Option {
	return func(o *schedulerOptions) {
		o.schedulerName = schedulerName
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
	clusterBindingLister := factory.Work().V1alpha2().ClusterResourceBindings().Lister()
	clusterLister := factory.Cluster().V1alpha1().Clusters().Lister()
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "scheduler-queue")
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
		clusterBindingLister: clusterBindingLister,
		clusterLister:        clusterLister,
		informerFactory:      factory,
		queue:                queue,
		Algorithm:            algorithm,
		schedulerCache:       schedulerCache,
	}

	sched.clusterReconcileWorker = util.NewAsyncWorker(util.Options{
		Name:          "ClusterReconcileWorker",
		ReconcileFunc: sched.reconcileCluster,
	})

	if options.enableSchedulerEstimator {
		sched.enableSchedulerEstimator = options.enableSchedulerEstimator
		sched.disableSchedulerEstimatorInPullMode = options.disableSchedulerEstimatorInPullMode
		sched.schedulerEstimatorServicePrefix = options.schedulerEstimatorServicePrefix
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
	sched.schedulerName = options.schedulerName

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

	s.clusterReconcileWorker.Run(1, stopCh)

	go wait.Until(s.worker, time.Second, stopCh)

	<-stopCh
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
	rb = rb.DeepCopy()

	if rb.Spec.Placement == nil {
		// never reach here
		err = fmt.Errorf("failed to get placement from resourceBinding(%s/%s)", rb.Namespace, rb.Name)
		klog.Error(err)
		return err
	}

	start := time.Now()
	appliedPlacementStr := util.GetLabelValue(rb.Annotations, util.PolicyPlacementAnnotation)
	if placementChanged(*rb.Spec.Placement, appliedPlacementStr, rb.Status.SchedulerObservedAffinityName) {
		// policy placement changed, need schedule
		klog.Infof("Start to schedule ResourceBinding(%s/%s) as placement changed", namespace, name)
		err = s.scheduleResourceBinding(rb)
		metrics.BindingSchedule(string(ReconcileSchedule), utilmetrics.DurationInSeconds(start), err)
		return err
	}
	if util.IsBindingReplicasChanged(&rb.Spec, rb.Spec.Placement.ReplicaScheduling) {
		// binding replicas changed, need reschedule
		klog.Infof("Reschedule ResourceBinding(%s/%s) as replicas scaled down or scaled up", namespace, name)
		err = s.scheduleResourceBinding(rb)
		metrics.BindingSchedule(string(ScaleSchedule), utilmetrics.DurationInSeconds(start), err)
		return err
	}
	if rb.Spec.Replicas == 0 ||
		rb.Spec.Placement.ReplicaSchedulingType() == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		// Duplicated resources should always be scheduled. Note: non-workload is considered as duplicated
		// even if scheduling type is divided.
		klog.V(3).Infof("Start to schedule ResourceBinding(%s/%s) as scheduling type is duplicated", namespace, name)
		err = s.scheduleResourceBinding(rb)
		metrics.BindingSchedule(string(ReconcileSchedule), utilmetrics.DurationInSeconds(start), err)
		return err
	}
	// TODO(dddddai): reschedule bindings on cluster change
	klog.V(3).Infof("Don't need to schedule ResourceBinding(%s/%s)", rb.Namespace, rb.Name)

	// If no scheduling is required, we need to ensure that binding.Generation is equal to
	// binding.Status.SchedulerObservedGeneration which means the current status of binding
	// is the latest status of successful scheduling.
	if rb.Generation != rb.Status.SchedulerObservedGeneration {
		updateRB := rb.DeepCopy()
		updateRB.Status.SchedulerObservedGeneration = updateRB.Generation
		return patchBindingStatus(s.KarmadaClient, rb, updateRB)
	}
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
	crb = crb.DeepCopy()

	if crb.Spec.Placement == nil {
		// never reach here
		err = fmt.Errorf("failed to get placement from clusterResourceBinding(%s)", crb.Name)
		klog.Error(err)
		return err
	}

	start := time.Now()
	appliedPlacementStr := util.GetLabelValue(crb.Annotations, util.PolicyPlacementAnnotation)
	if placementChanged(*crb.Spec.Placement, appliedPlacementStr, crb.Status.SchedulerObservedAffinityName) {
		// policy placement changed, need schedule
		klog.Infof("Start to schedule ClusterResourceBinding(%s) as placement changed", name)
		err = s.scheduleClusterResourceBinding(crb)
		metrics.BindingSchedule(string(ReconcileSchedule), utilmetrics.DurationInSeconds(start), err)
		return err
	}
	if util.IsBindingReplicasChanged(&crb.Spec, crb.Spec.Placement.ReplicaScheduling) {
		// binding replicas changed, need reschedule
		klog.Infof("Reschedule ClusterResourceBinding(%s) as replicas scaled down or scaled up", name)
		err = s.scheduleClusterResourceBinding(crb)
		metrics.BindingSchedule(string(ScaleSchedule), utilmetrics.DurationInSeconds(start), err)
		return err
	}
	if crb.Spec.Replicas == 0 ||
		crb.Spec.Placement.ReplicaSchedulingType() == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		// Duplicated resources should always be scheduled. Note: non-workload is considered as duplicated
		// even if scheduling type is divided.
		klog.V(3).Infof("Start to schedule ClusterResourceBinding(%s) as scheduling type is duplicated", name)
		err = s.scheduleClusterResourceBinding(crb)
		metrics.BindingSchedule(string(ReconcileSchedule), utilmetrics.DurationInSeconds(start), err)
		return err
	}
	// TODO(dddddai): reschedule bindings on cluster change
	klog.Infof("Don't need to schedule ClusterResourceBinding(%s)", name)

	// If no scheduling is required, we need to ensure that binding.Generation is equal to
	// binding.Status.SchedulerObservedGeneration which means the current status of binding
	// is the latest status of successful scheduling.
	if crb.Generation != crb.Status.SchedulerObservedGeneration {
		updateCRB := crb.DeepCopy()
		updateCRB.Status.SchedulerObservedGeneration = updateCRB.Generation
		return patchClusterResourceBindingStatus(s.KarmadaClient, crb, updateCRB)
	}
	return nil
}

func (s *Scheduler) scheduleResourceBinding(rb *workv1alpha2.ResourceBinding) (err error) {
	defer func() {
		var condition metav1.Condition
		if err == nil {
			condition = util.NewCondition(workv1alpha2.Scheduled, scheduleSuccessReason, scheduleSuccessMessage, metav1.ConditionTrue)
		} else {
			condition = util.NewCondition(workv1alpha2.Scheduled, scheduleFailedReason, err.Error(), metav1.ConditionFalse)
		}
		if updateErr := patchBindingStatusCondition(s.KarmadaClient, rb, condition); updateErr != nil {
			klog.Errorf("Failed to patch schedule status to ResourceBinding(%s/%s): %v", rb.Namespace, rb.Name, err)
			if err == nil {
				// schedule succeed but update status failed, return err in order to retry in next loop.
				err = updateErr
			}
		}
	}()

	if rb.Spec.Placement.ClusterAffinities != nil {
		return s.scheduleResourceBindingWithClusterAffinities(rb)
	}
	return s.scheduleResourceBindingWithClusterAffinity(rb)
}

func (s *Scheduler) scheduleResourceBindingWithClusterAffinity(rb *workv1alpha2.ResourceBinding) error {
	klog.V(4).InfoS("Begin scheduling resource binding with ClusterAffinity", "resourceBinding", klog.KObj(rb))
	defer klog.V(4).InfoS("End scheduling resource binding with ClusterAffinity", "resourceBinding", klog.KObj(rb))

	placementBytes, err := json.Marshal(*rb.Spec.Placement)
	if err != nil {
		klog.V(4).ErrorS(err, "Failed to marshal binding placement", "resourceBinding", klog.KObj(rb))
		return err
	}

	scheduleResult, err := s.Algorithm.Schedule(context.TODO(), &rb.Spec, &rb.Status, &core.ScheduleAlgorithmOption{EnableEmptyWorkloadPropagation: s.enableEmptyWorkloadPropagation})
	var noClusterFit *framework.FitError
	// in case of no cluster fit, can not return but continue to patch(cleanup) the result.
	if err != nil && !errors.As(err, &noClusterFit) {
		s.recordScheduleResultEventForResourceBinding(rb, err)
		klog.Errorf("Failed scheduling ResourceBinding(%s/%s): %v", rb.Namespace, rb.Name, err)
		return err
	}

	klog.V(4).Infof("ResourceBinding(%s/%s) scheduled to clusters %v", rb.Namespace, rb.Name, scheduleResult.SuggestedClusters)
	patchErr := s.patchScheduleResultForResourceBinding(rb, string(placementBytes), scheduleResult.SuggestedClusters)
	s.recordScheduleResultEventForResourceBinding(rb, utilerrors.NewAggregate([]error{err, patchErr}))

	// only care about the patch result,
	// for FitError already recorded by event
	return patchErr
}

func (s *Scheduler) scheduleResourceBindingWithClusterAffinities(rb *workv1alpha2.ResourceBinding) error {
	klog.V(4).InfoS("Begin scheduling resourceBinding with ClusterAffinities", "resourceBinding", klog.KObj(rb))
	defer klog.V(4).InfoS("End scheduling resourceBinding with ClusterAffinities", "resourceBinding", klog.KObj(rb))

	placementBytes, err := json.Marshal(*rb.Spec.Placement)
	if err != nil {
		klog.V(4).ErrorS(err, "Failed to marshal binding placement", "resourceBinding", klog.KObj(rb))
		return err
	}

	var (
		scheduleResult core.ScheduleResult
		firstErr       error
	)

	affinityIndex := getAffinityIndex(rb.Spec.Placement.ClusterAffinities, rb.Status.SchedulerObservedAffinityName)
	updatedStatus := rb.Status.DeepCopy()
	for affinityIndex < len(rb.Spec.Placement.ClusterAffinities) {
		klog.V(4).Infof("Schedule ResourceBinding(%s/%s) with clusterAffiliates index(%d)", rb.Namespace, rb.Name, affinityIndex)
		updatedStatus.SchedulerObservedAffinityName = rb.Spec.Placement.ClusterAffinities[affinityIndex].AffinityName
		scheduleResult, err = s.Algorithm.Schedule(context.TODO(), &rb.Spec, updatedStatus, &core.ScheduleAlgorithmOption{EnableEmptyWorkloadPropagation: s.enableEmptyWorkloadPropagation})
		if err == nil {
			break
		}

		// obtain to err of the first scheduling
		if firstErr == nil {
			firstErr = err
		}

		err = fmt.Errorf("failed to schedule ResourceBinding(%s/%s) with clusterAffiliates index(%d): %v", rb.Namespace, rb.Name, affinityIndex, err)
		klog.Error(err)
		s.recordScheduleResultEventForResourceBinding(rb, err)
		affinityIndex++
	}

	if affinityIndex >= len(rb.Spec.Placement.ClusterAffinities) {
		klog.Errorf("Failed to schedule ResourceBinding(%s/%s) with all ClusterAffinities.", rb.Namespace, rb.Name)

		updatedStatus.SchedulerObservedAffinityName = rb.Status.SchedulerObservedAffinityName

		var noClusterFit *framework.FitError
		if !errors.As(firstErr, &noClusterFit) {
			return firstErr
		}

		klog.V(4).Infof("ResourceBinding(%s/%s) scheduled to clusters %v", rb.Namespace, rb.Name, nil)
		patchErr := s.patchScheduleResultForResourceBinding(rb, string(placementBytes), nil)
		s.recordScheduleResultEventForResourceBinding(rb, patchErr)
		return patchErr
	}

	klog.V(4).Infof("ResourceBinding(%s/%s) scheduled to clusters %v", rb.Namespace, rb.Name, scheduleResult.SuggestedClusters)
	patchErr := s.patchScheduleResultForResourceBinding(rb, string(placementBytes), scheduleResult.SuggestedClusters)
	patchStatusErr := patchBindingStatusWithAffinityName(s.KarmadaClient, rb, updatedStatus.SchedulerObservedAffinityName)
	scheduleErr := utilerrors.NewAggregate([]error{patchErr, patchStatusErr})
	s.recordScheduleResultEventForResourceBinding(rb, scheduleErr)
	return scheduleErr
}

func (s *Scheduler) patchScheduleResultForResourceBinding(oldBinding *workv1alpha2.ResourceBinding, placement string, scheduleResult []workv1alpha2.TargetCluster) error {
	newBinding := oldBinding.DeepCopy()
	if newBinding.Annotations == nil {
		newBinding.Annotations = make(map[string]string)
	}
	newBinding.Annotations[util.PolicyPlacementAnnotation] = placement
	newBinding.Spec.Clusters = scheduleResult

	patchBytes, err := helper.GenMergePatch(oldBinding, newBinding)
	if err != nil {
		return err
	}
	if len(patchBytes) == 0 {
		return nil
	}

	_, err = s.KarmadaClient.WorkV1alpha2().ResourceBindings(newBinding.Namespace).Patch(context.TODO(), newBinding.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("Failed to patch schedule to ResourceBinding(%s/%s): %v", oldBinding.Namespace, oldBinding.Name, err)
		return err
	}

	klog.V(4).Infof("Patch schedule to ResourceBinding(%s/%s) succeed", oldBinding.Namespace, oldBinding.Name)
	return nil
}

func (s *Scheduler) scheduleClusterResourceBinding(crb *workv1alpha2.ClusterResourceBinding) (err error) {
	defer func() {
		var condition metav1.Condition
		if err == nil {
			condition = util.NewCondition(workv1alpha2.Scheduled, scheduleSuccessReason, scheduleSuccessMessage, metav1.ConditionTrue)
		} else {
			condition = util.NewCondition(workv1alpha2.Scheduled, scheduleFailedReason, err.Error(), metav1.ConditionFalse)
		}
		if updateErr := patchClusterBindingStatusCondition(s.KarmadaClient, crb, condition); updateErr != nil {
			klog.Errorf("Failed to patch schedule status to ClusterResourceBinding(%s): %v", crb.Name, err)
			if err == nil {
				// schedule succeed but update status failed, return err in order to retry in next loop.
				err = updateErr
			}
		}
	}()

	if crb.Spec.Placement.ClusterAffinities != nil {
		return s.scheduleClusterResourceBindingWithClusterAffinities(crb)
	}
	return s.scheduleClusterResourceBindingWithClusterAffinity(crb)
}

func (s *Scheduler) scheduleClusterResourceBindingWithClusterAffinity(crb *workv1alpha2.ClusterResourceBinding) error {
	klog.V(4).InfoS("Begin scheduling clusterResourceBinding with ClusterAffinity", "clusterResourceBinding", klog.KObj(crb))
	defer klog.V(4).InfoS("End scheduling clusterResourceBinding with ClusterAffinity", "clusterResourceBinding", klog.KObj(crb))

	placementBytes, err := json.Marshal(*crb.Spec.Placement)
	if err != nil {
		klog.V(4).ErrorS(err, "Failed to marshal binding placement", "clusterResourceBinding", klog.KObj(crb))
		return err
	}

	scheduleResult, err := s.Algorithm.Schedule(context.TODO(), &crb.Spec, &crb.Status, &core.ScheduleAlgorithmOption{EnableEmptyWorkloadPropagation: s.enableEmptyWorkloadPropagation})
	var noClusterFit *framework.FitError
	// in case of no cluster fit, can not return but continue to patch(cleanup) the result.
	if err != nil && !errors.As(err, &noClusterFit) {
		s.recordScheduleResultEventForClusterResourceBinding(crb, err)
		klog.Errorf("Failed scheduling clusterResourceBinding(%s): %v", crb.Name, err)
		return err
	}

	klog.V(4).Infof("clusterResourceBinding(%s) scheduled to clusters %v", crb.Name, scheduleResult.SuggestedClusters)
	patchErr := s.patchScheduleResultForClusterResourceBinding(crb, string(placementBytes), scheduleResult.SuggestedClusters)
	s.recordScheduleResultEventForClusterResourceBinding(crb, utilerrors.NewAggregate([]error{err, patchErr}))

	// only care about the patch result,
	// for FitError already recorded by event
	return patchErr
}

func (s *Scheduler) scheduleClusterResourceBindingWithClusterAffinities(crb *workv1alpha2.ClusterResourceBinding) error {
	klog.V(4).InfoS("Begin scheduling clusterResourceBinding with ClusterAffinities", "clusterResourceBinding", klog.KObj(crb))
	defer klog.V(4).InfoS("End scheduling clusterResourceBinding with ClusterAffinities", "clusterResourceBinding", klog.KObj(crb))

	placementBytes, err := json.Marshal(*crb.Spec.Placement)
	if err != nil {
		klog.V(4).ErrorS(err, "Failed to marshal binding placement", "clusterResourceBinding", klog.KObj(crb))
		return err
	}

	var (
		scheduleResult core.ScheduleResult
		firstErr       error
	)

	affinityIndex := getAffinityIndex(crb.Spec.Placement.ClusterAffinities, crb.Status.SchedulerObservedAffinityName)
	updatedStatus := crb.Status.DeepCopy()
	for affinityIndex < len(crb.Spec.Placement.ClusterAffinities) {
		klog.V(4).Infof("Schedule ClusterResourceBinding(%s) with clusterAffiliates index(%d)", crb.Name, affinityIndex)
		updatedStatus.SchedulerObservedAffinityName = crb.Spec.Placement.ClusterAffinities[affinityIndex].AffinityName
		scheduleResult, err = s.Algorithm.Schedule(context.TODO(), &crb.Spec, updatedStatus, &core.ScheduleAlgorithmOption{EnableEmptyWorkloadPropagation: s.enableEmptyWorkloadPropagation})
		if err == nil {
			break
		}

		// obtain to err of the first scheduling
		if firstErr == nil {
			firstErr = err
		}

		err = fmt.Errorf("failed to schedule ClusterResourceBinding(%s) with clusterAffiliates index(%d): %v", crb.Name, affinityIndex, err)
		klog.Error(err)
		s.recordScheduleResultEventForClusterResourceBinding(crb, err)
		affinityIndex++
	}

	if affinityIndex >= len(crb.Spec.Placement.ClusterAffinities) {
		klog.Errorf("Failed to schedule ClusterResourceBinding(%s) with all ClusterAffinities.", crb.Name)

		updatedStatus.SchedulerObservedAffinityName = crb.Status.SchedulerObservedAffinityName

		var noClusterFit *framework.FitError
		if !errors.As(firstErr, &noClusterFit) {
			return firstErr
		}

		klog.V(4).Infof("ClusterResourceBinding(%s) scheduled to clusters %v", crb.Name, nil)
		patchErr := s.patchScheduleResultForClusterResourceBinding(crb, string(placementBytes), nil)
		s.recordScheduleResultEventForClusterResourceBinding(crb, patchErr)
		return patchErr
	}

	klog.V(4).Infof("ClusterResourceBinding(%s) scheduled to clusters %v", crb.Name, scheduleResult.SuggestedClusters)
	patchErr := s.patchScheduleResultForClusterResourceBinding(crb, string(placementBytes), scheduleResult.SuggestedClusters)
	patchStatusErr := patchClusterBindingStatusWithAffinityName(s.KarmadaClient, crb, updatedStatus.SchedulerObservedAffinityName)
	scheduleErr := utilerrors.NewAggregate([]error{patchErr, patchStatusErr})
	s.recordScheduleResultEventForClusterResourceBinding(crb, scheduleErr)
	return scheduleErr
}

func (s *Scheduler) patchScheduleResultForClusterResourceBinding(oldBinding *workv1alpha2.ClusterResourceBinding, placement string, scheduleResult []workv1alpha2.TargetCluster) error {
	newBinding := oldBinding.DeepCopy()
	if newBinding.Annotations == nil {
		newBinding.Annotations = make(map[string]string)
	}
	newBinding.Annotations[util.PolicyPlacementAnnotation] = placement
	newBinding.Spec.Clusters = scheduleResult

	patchBytes, err := helper.GenMergePatch(oldBinding, newBinding)
	if err != nil {
		return err
	}
	if len(patchBytes) == 0 {
		return nil
	}

	_, err = s.KarmadaClient.WorkV1alpha2().ClusterResourceBindings().Patch(context.TODO(), newBinding.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("Failed to patch schedule to ClusterResourceBinding(%s): %v", oldBinding.Name, err)
		return err
	}

	klog.V(4).Infof("Patch schedule to ClusterResourceBinding(%s) succeed", oldBinding.Name)
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

	return estimatorclient.EstablishConnection(s.KubeClient, name, s.schedulerEstimatorCache, s.schedulerEstimatorServicePrefix, s.schedulerEstimatorPort)
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
		if err = estimatorclient.EstablishConnection(s.KubeClient, clusterList.Items[i].Name, s.schedulerEstimatorCache, s.schedulerEstimatorServicePrefix, s.schedulerEstimatorPort); err != nil {
			klog.Error(err)
		}
	}
}

// patchBindingStatusCondition patches schedule status condition of ResourceBinding when necessary.
func patchBindingStatusCondition(karmadaClient karmadaclientset.Interface, rb *workv1alpha2.ResourceBinding, newScheduledCondition metav1.Condition) error {
	klog.V(4).Infof("Begin to patch status condition to ResourceBinding(%s/%s)", rb.Namespace, rb.Name)

	updateRB := rb.DeepCopy()
	meta.SetStatusCondition(&updateRB.Status.Conditions, newScheduledCondition)
	// Postpone setting observed generation until schedule succeed, assume scheduler will retry and
	// will succeed eventually.
	if newScheduledCondition.Status == metav1.ConditionTrue {
		updateRB.Status.SchedulerObservedGeneration = rb.Generation
	}

	if reflect.DeepEqual(rb.Status, updateRB.Status) {
		return nil
	}
	return patchBindingStatus(karmadaClient, rb, updateRB)
}

// patchBindingStatusWithAffinityName patches schedule status with affinityName of ResourceBinding when necessary.
func patchBindingStatusWithAffinityName(karmadaClient karmadaclientset.Interface, rb *workv1alpha2.ResourceBinding, affinityName string) error {
	if rb.Status.SchedulerObservedAffinityName == affinityName {
		return nil
	}

	klog.V(4).Infof("Begin to patch status with affinityName(%s) to ResourceBinding(%s/%s).", affinityName, rb.Namespace, rb.Name)
	updateRB := rb.DeepCopy()
	updateRB.Status.SchedulerObservedAffinityName = affinityName
	return patchBindingStatus(karmadaClient, rb, updateRB)
}

func patchBindingStatus(karmadaClient karmadaclientset.Interface, rb, updateRB *workv1alpha2.ResourceBinding) error {
	patchBytes, err := helper.GenMergePatch(rb, updateRB)
	if err != nil {
		return err
	}
	if len(patchBytes) == 0 {
		return nil
	}

	_, err = karmadaClient.WorkV1alpha2().ResourceBindings(rb.Namespace).Patch(context.TODO(), rb.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		klog.Errorf("Failed to patch schedule status ResourceBinding(%s/%s): %v", rb.Namespace, rb.Name, err)
		return err
	}

	klog.V(4).Infof("Patch schedule status to ResourceBinding(%s/%s) succeed", rb.Namespace, rb.Name)
	return nil
}

// patchClusterBindingStatusCondition patches schedule status condition of ClusterResourceBinding when necessary
func patchClusterBindingStatusCondition(karmadaClient karmadaclientset.Interface, crb *workv1alpha2.ClusterResourceBinding, newScheduledCondition metav1.Condition) error {
	klog.V(4).Infof("Begin to patch status condition to ClusterResourceBinding(%s)", crb.Name)

	updateCRB := crb.DeepCopy()
	meta.SetStatusCondition(&updateCRB.Status.Conditions, newScheduledCondition)
	// Postpone setting observed generation until schedule succeed, assume scheduler will retry and
	// will succeed eventually.
	if newScheduledCondition.Status == metav1.ConditionTrue {
		updateCRB.Status.SchedulerObservedGeneration = crb.Generation
	}

	if reflect.DeepEqual(crb.Status, updateCRB.Status) {
		return nil
	}
	return patchClusterResourceBindingStatus(karmadaClient, crb, updateCRB)
}

// patchClusterBindingStatusWithAffinityName patches schedule status with affinityName of ClusterResourceBinding when necessary.
func patchClusterBindingStatusWithAffinityName(karmadaClient karmadaclientset.Interface, crb *workv1alpha2.ClusterResourceBinding, affinityName string) error {
	if crb.Status.SchedulerObservedAffinityName == affinityName {
		return nil
	}

	klog.V(4).Infof("Begin to patch status with affinityName(%s) to ClusterResourceBinding(%s).", affinityName, crb.Name)
	updateCRB := crb.DeepCopy()
	updateCRB.Status.SchedulerObservedAffinityName = affinityName
	return patchClusterResourceBindingStatus(karmadaClient, crb, updateCRB)
}

func patchClusterResourceBindingStatus(karmadaClient karmadaclientset.Interface, crb, updateCRB *workv1alpha2.ClusterResourceBinding) error {
	patchBytes, err := helper.GenMergePatch(crb, updateCRB)
	if err != nil {
		return err
	}
	if len(patchBytes) == 0 {
		return nil
	}

	_, err = karmadaClient.WorkV1alpha2().ClusterResourceBindings().Patch(context.TODO(), crb.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
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
		s.eventRecorder.Event(rb, corev1.EventTypeNormal, events.EventReasonScheduleBindingSucceed, scheduleSuccessMessage)
		s.eventRecorder.Event(ref, corev1.EventTypeNormal, events.EventReasonScheduleBindingSucceed, scheduleSuccessMessage)
	} else {
		s.eventRecorder.Event(rb, corev1.EventTypeWarning, events.EventReasonScheduleBindingFailed, schedulerErr.Error())
		s.eventRecorder.Event(ref, corev1.EventTypeWarning, events.EventReasonScheduleBindingFailed, schedulerErr.Error())
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
		s.eventRecorder.Event(crb, corev1.EventTypeNormal, events.EventReasonScheduleBindingSucceed, scheduleSuccessMessage)
		s.eventRecorder.Event(ref, corev1.EventTypeNormal, events.EventReasonScheduleBindingSucceed, scheduleSuccessMessage)
	} else {
		s.eventRecorder.Event(crb, corev1.EventTypeWarning, events.EventReasonScheduleBindingFailed, schedulerErr.Error())
		s.eventRecorder.Event(ref, corev1.EventTypeWarning, events.EventReasonScheduleBindingFailed, schedulerErr.Error())
	}
}
