package applicationfailover

import (
	"context"
	"fmt"
	"math"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// CRBApplicationFailoverControllerName is the controller name that will be used when reporting events.
const CRBApplicationFailoverControllerName = "cluster-resource-binding-application-failover-controller"

// CRBApplicationFailoverController is to sync ClusterResourceBinding's application failover behavior.
type CRBApplicationFailoverController struct {
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
func (c *CRBApplicationFailoverController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling ClusterResourceBinding %s.", req.Name)

	binding := &workv1alpha2.ClusterResourceBinding{}
	if err := c.Client.Get(ctx, req.NamespacedName, binding); err != nil {
		if apierrors.IsNotFound(err) {
			c.workloadUnhealthyMap.delete(req.NamespacedName)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{Requeue: true}, err
	}

	if !c.clusterResourceBindingFilter(binding) {
		c.workloadUnhealthyMap.delete(req.NamespacedName)
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

func (c *CRBApplicationFailoverController) detectFailure(clusters []string, tolerationSeconds *int32, key types.NamespacedName) (int32, []string) {
	var needEvictClusters []string
	duration := int32(math.MaxInt32)

	for _, cluster := range clusters {
		if !c.workloadUnhealthyMap.hasWorkloadBeenUnhealthy(key, cluster) {
			c.workloadUnhealthyMap.setTimeStamp(key, cluster)
			if duration > *tolerationSeconds {
				duration = *tolerationSeconds
			}
			continue
		}
		// When the workload in a cluster is in an unhealthy state for more than the tolerance time,
		// and the cluster has not been in the GracefulEvictionTasks,
		// the cluster will be added to the list that needs to be evicted.
		unHealthyTimeStamp := c.workloadUnhealthyMap.getTimeStamp(key, cluster)
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

func (c *CRBApplicationFailoverController) syncBinding(binding *workv1alpha2.ClusterResourceBinding) (time.Duration, error) {
	key := types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}
	tolerationSeconds := binding.Spec.Failover.Application.DecisionConditions.TolerationSeconds

	allClusters := sets.New[string]()
	for _, cluster := range binding.Spec.Clusters {
		allClusters.Insert(cluster.Name)
	}

	unhealthyClusters, others := distinguishUnhealthyClustersWithOthers(binding.Status.AggregatedStatus, binding.Spec)
	duration, needEvictClusters := c.detectFailure(unhealthyClusters, tolerationSeconds, key)

	err := c.evictBinding(binding, needEvictClusters)
	if err != nil {
		klog.Errorf("Failed to evict binding(%s), err: %v.", binding.Name, err)
		return 0, err
	}

	if len(needEvictClusters) != 0 {
		if err = c.updateBinding(binding, allClusters, needEvictClusters); err != nil {
			return 0, err
		}
	}

	// Cleanup clusters on which the application status is not unhealthy and clusters that have been evicted or removed in the workloadUnhealthyMap.
	c.workloadUnhealthyMap.deleteIrrelevantClusters(key, allClusters, others)

	return time.Duration(duration) * time.Second, nil
}

func (c *CRBApplicationFailoverController) evictBinding(binding *workv1alpha2.ClusterResourceBinding, clusters []string) error {
	for _, cluster := range clusters {
		switch binding.Spec.Failover.Application.PurgeMode {
		case policyv1alpha1.Graciously:
			if features.FeatureGate.Enabled(features.GracefulEviction) {
				binding.Spec.GracefulEvictCluster(cluster, workv1alpha2.NewTaskOptions(workv1alpha2.WithProducer(CRBApplicationFailoverControllerName), workv1alpha2.WithReason(workv1alpha2.EvictionReasonApplicationFailure), workv1alpha2.WithGracePeriodSeconds(binding.Spec.Failover.Application.GracePeriodSeconds)))
			} else {
				err := fmt.Errorf("GracefulEviction featureGate must be enabled when purgeMode is %s", policyv1alpha1.Graciously)
				klog.Error(err)
				return err
			}
		case policyv1alpha1.Never:
			if features.FeatureGate.Enabled(features.GracefulEviction) {
				binding.Spec.GracefulEvictCluster(cluster, workv1alpha2.NewTaskOptions(workv1alpha2.WithProducer(CRBApplicationFailoverControllerName), workv1alpha2.WithReason(workv1alpha2.EvictionReasonApplicationFailure), workv1alpha2.WithSuppressDeletion(pointer.Bool(true))))
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

func (c *CRBApplicationFailoverController) updateBinding(binding *workv1alpha2.ClusterResourceBinding, allClusters sets.Set[string], needEvictClusters []string) error {
	if err := c.Update(context.TODO(), binding); err != nil {
		for _, cluster := range needEvictClusters {
			helper.EmitClusterEvictionEventForClusterResourceBinding(binding, cluster, c.EventRecorder, err)
		}
		klog.ErrorS(err, "Failed to update binding", "binding", klog.KObj(binding))
		return err
	}
	for _, cluster := range needEvictClusters {
		allClusters.Delete(cluster)
	}
	if !features.FeatureGate.Enabled(features.GracefulEviction) {
		for _, cluster := range needEvictClusters {
			helper.EmitClusterEvictionEventForClusterResourceBinding(binding, cluster, c.EventRecorder, nil)
		}
	}

	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *CRBApplicationFailoverController) SetupWithManager(mgr controllerruntime.Manager) error {
	c.workloadUnhealthyMap = newWorkloadUnhealthyMap()
	clusterResourceBindingPredicateFn := predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			obj := createEvent.Object.(*workv1alpha2.ClusterResourceBinding)
			if obj.Spec.Failover == nil || obj.Spec.Failover.Application == nil {
				return false
			}
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			oldObj := updateEvent.ObjectOld.(*workv1alpha2.ClusterResourceBinding)
			newObj := updateEvent.ObjectNew.(*workv1alpha2.ClusterResourceBinding)
			if (oldObj.Spec.Failover == nil || oldObj.Spec.Failover.Application == nil) &&
				(newObj.Spec.Failover == nil || newObj.Spec.Failover.Application == nil) {
				return false
			}
			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			obj := deleteEvent.Object.(*workv1alpha2.ClusterResourceBinding)
			if obj.Spec.Failover == nil || obj.Spec.Failover.Application == nil {
				return false
			}
			return true
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool { return false },
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&workv1alpha2.ClusterResourceBinding{}, builder.WithPredicates(clusterResourceBindingPredicateFn)).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions)}).
		Complete(c)
}

func (c *CRBApplicationFailoverController) clusterResourceBindingFilter(crb *workv1alpha2.ClusterResourceBinding) bool {
	if crb.Spec.Failover == nil || crb.Spec.Failover.Application == nil {
		return false
	}

	if len(crb.Status.AggregatedStatus) == 0 {
		return false
	}

	resourceKey, err := helper.ConstructClusterWideKey(crb.Spec.Resource)
	if err != nil {
		// Never reach
		klog.Errorf("Failed to construct clusterWideKey from clusterResourceBinding(%s)", crb.Name)
		return false
	}

	if !c.ResourceInterpreter.HookEnabled(resourceKey.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretHealth) {
		return false
	}

	return true
}
