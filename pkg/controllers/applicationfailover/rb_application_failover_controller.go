package applicationfailover

import (
	"context"
	"fmt"
	"math"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// RBApplicationFailoverControllerName is the controller name that will be used when reporting events.
const RBApplicationFailoverControllerName = "resource-binding-application-failover-controller"

// RBApplicationFailoverController is to sync ResourceBinding's application failover behavior.
type RBApplicationFailoverController struct {
	client.Client
	EventRecorder      record.EventRecorder
	RateLimiterOptions ratelimiterflag.Options

	// workloadUnhealthyMap records which clusters the specific resource is in an unhealthy state
	workloadUnhealthyMap *workloadUnhealthyMap
	ResourceInterpreter  resourceinterpreter.ResourceInterpreter
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *RBApplicationFailoverController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling ResourceBinding %s.", req.NamespacedName.String())

	binding := &workv1alpha2.ResourceBinding{}
	if err := c.Client.Get(ctx, req.NamespacedName, binding); err != nil {
		if apierrors.IsNotFound(err) {
			resource, err := helper.ConstructClusterWideKey(binding.Spec.Resource)
			if err != nil {
				return controllerruntime.Result{Requeue: true}, err
			}
			c.workloadUnhealthyMap.delete(resource)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{Requeue: true}, err
	}

	if !c.bindingFilter(binding) {
		resource, err := helper.ConstructClusterWideKey(binding.Spec.Resource)
		if err != nil {
			return controllerruntime.Result{Requeue: true}, err
		}
		c.workloadUnhealthyMap.delete(resource)
		return controllerruntime.Result{}, nil
	}

	retryDuration, err := c.syncBinding(binding)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	if retryDuration > 0 {
		klog.V(4).Infof("Retry to check health status of the workload after %v minutes.", retryDuration.Minutes())
		return controllerruntime.Result{RequeueAfter: retryDuration}, nil
	}
	return controllerruntime.Result{}, nil
}

func (c *RBApplicationFailoverController) detectFailure(clusters []string, tolerationSeconds *int32, resource keys.ClusterWideKey) (int32, []string) {
	var needEvictClusters []string
	duration := int32(math.MaxInt32)

	for _, cluster := range clusters {
		if !c.workloadUnhealthyMap.hasWorkloadBeenUnhealthy(resource, cluster) {
			c.workloadUnhealthyMap.setTimeStamp(resource, cluster)
			if duration > *tolerationSeconds {
				duration = *tolerationSeconds
			}
			continue
		}
		// When the workload in a cluster is in an unhealthy state for more than the tolerance time,
		// and the cluster has not been in the GracefulEvictionTasks,
		// the cluster will be added to the list that needs to be evicted.
		unHealthyTimeStamp := c.workloadUnhealthyMap.getTimeStamp(resource, cluster)
		timeNow := metav1.Now()
		if timeNow.After(unHealthyTimeStamp.Add(time.Duration(*tolerationSeconds) * time.Second)) {
			needEvictClusters = append(needEvictClusters, cluster)
		} else {
			if duration > *tolerationSeconds-int32(timeNow.Sub(unHealthyTimeStamp.Time).Seconds()) {
				duration = *tolerationSeconds - int32(timeNow.Sub(unHealthyTimeStamp.Time).Seconds())
			}
		}
	}

	if duration == int32(math.MaxInt32) {
		duration = 0
	}
	return duration, needEvictClusters
}

func (c *RBApplicationFailoverController) syncBinding(binding *workv1alpha2.ResourceBinding) (time.Duration, error) {
	tolerationSeconds := binding.Spec.Failover.Application.DecisionConditions.TolerationSeconds
	resource, err := helper.ConstructClusterWideKey(binding.Spec.Resource)
	if err != nil {
		klog.Errorf("failed to get key from binding(%s)'s resource", binding.Name)
		return 0, err
	}

	allClusters := sets.New[string]()
	for _, cluster := range binding.Spec.Clusters {
		allClusters.Insert(cluster.Name)
	}

	unhealthyClusters := filterIrrelevantClusters(binding.Status.AggregatedStatus, binding.Spec)
	duration, needEvictClusters := c.detectFailure(unhealthyClusters, tolerationSeconds, resource)

	err = c.evictBinding(binding, needEvictClusters)
	if err != nil {
		klog.Errorf("Failed to evict binding(%s/%s), err: %v.", binding.Namespace, binding.Name, err)
		return 0, err
	}

	if len(needEvictClusters) != 0 {
		if err = c.updateBinding(binding, allClusters, needEvictClusters); err != nil {
			return 0, err
		}
	}

	// Cleanup clusters that have been evicted or removed in the workloadUnhealthyMap
	c.workloadUnhealthyMap.deleteIrrelevantClusters(resource, allClusters)

	return time.Duration(duration) * time.Second, nil
}

func (c *RBApplicationFailoverController) evictBinding(binding *workv1alpha2.ResourceBinding, clusters []string) error {
	for _, cluster := range clusters {
		switch binding.Spec.Failover.Application.PurgeMode {
		case policyv1alpha1.Graciously:
			if features.FeatureGate.Enabled(features.GracefulEviction) {
				binding.Spec.GracefulEvictCluster(cluster, workv1alpha2.NewTaskOptions(workv1alpha2.WithProducer(RBApplicationFailoverControllerName),
					workv1alpha2.WithReason(workv1alpha2.EvictionReasonApplicationFailure), workv1alpha2.WithGracePeriodSeconds(binding.Spec.Failover.Application.GracePeriodSeconds)))
			} else {
				err := fmt.Errorf("GracefulEviction featureGate must be enabled when purgeMode is %s", policyv1alpha1.Graciously)
				klog.Error(err)
				return err
			}
		case policyv1alpha1.Never:
			if features.FeatureGate.Enabled(features.GracefulEviction) {
				binding.Spec.GracefulEvictCluster(cluster, workv1alpha2.NewTaskOptions(workv1alpha2.WithProducer(RBApplicationFailoverControllerName),
					workv1alpha2.WithReason(workv1alpha2.EvictionReasonApplicationFailure), workv1alpha2.WithSuppressDeletion(pointer.Bool(true))))
			} else {
				err := fmt.Errorf("GracefulEviction featureGate must be enabled when purgeMode is %s", policyv1alpha1.Never)
				klog.Error(err)
				return err
			}
		case policyv1alpha1.Immediately:
			binding.Spec.RemoveCluster(cluster)
		}
	}

	return nil
}

func (c *RBApplicationFailoverController) updateBinding(binding *workv1alpha2.ResourceBinding, allClusters sets.Set[string], needEvictClusters []string) error {
	if err := c.Update(context.TODO(), binding); err != nil {
		for _, cluster := range needEvictClusters {
			helper.EmitClusterEvictionEventForResourceBinding(binding, cluster, c.EventRecorder, err)
		}
		klog.ErrorS(err, "Failed to update binding", "binding", klog.KObj(binding))
		return err
	}
	for _, cluster := range needEvictClusters {
		allClusters.Delete(cluster)
	}
	if !features.FeatureGate.Enabled(features.GracefulEviction) {
		for _, cluster := range needEvictClusters {
			helper.EmitClusterEvictionEventForResourceBinding(binding, cluster, c.EventRecorder, nil)
		}
	}

	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *RBApplicationFailoverController) SetupWithManager(mgr controllerruntime.Manager) error {
	c.workloadUnhealthyMap = newWorkloadUnhealthyMap()
	resourceBindingPredicateFn := predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			obj := createEvent.Object.(*workv1alpha2.ResourceBinding)
			if obj.Spec.Failover == nil || obj.Spec.Failover.Application == nil {
				return false
			}
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			oldObj := updateEvent.ObjectOld.(*workv1alpha2.ResourceBinding)
			newObj := updateEvent.ObjectNew.(*workv1alpha2.ResourceBinding)
			if (oldObj.Spec.Failover == nil || oldObj.Spec.Failover.Application == nil) &&
				(newObj.Spec.Failover == nil || newObj.Spec.Failover.Application == nil) {
				return false
			}
			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			obj := deleteEvent.Object.(*workv1alpha2.ResourceBinding)
			if obj.Spec.Failover == nil || obj.Spec.Failover.Application == nil {
				return false
			}
			return true
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool { return false },
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&workv1alpha2.ResourceBinding{}, builder.WithPredicates(resourceBindingPredicateFn)).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions)}).
		Complete(c)
}

func (c *RBApplicationFailoverController) bindingFilter(rb *workv1alpha2.ResourceBinding) bool {
	if rb.Spec.Failover == nil || rb.Spec.Failover.Application == nil {
		return false
	}

	if len(rb.Status.AggregatedStatus) == 0 {
		return false
	}

	resourceKey, err := helper.ConstructClusterWideKey(rb.Spec.Resource)
	if err != nil {
		// Never reach
		klog.Errorf("Failed to construct clusterWideKey from binding(%s/%s)", rb.Namespace, rb.Name)
		return false
	}

	if !c.ResourceInterpreter.HookEnabled(resourceKey.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretHealth) {
		return false
	}

	if !rb.Spec.PropagateDeps {
		return false
	}
	return true
}
