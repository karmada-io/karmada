/*
Copyright 2023 The Karmada Authors.

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

package applicationfailover

import (
	"context"
	"math"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// RBApplicationFailoverControllerName is the controller name that will be used when reporting events and metrics.
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
	klog.V(4).InfoS("Reconciling ResourceBinding", "namespace", req.Namespace, "name", req.Name)

	binding := &workv1alpha2.ResourceBinding{}
	if err := c.Client.Get(ctx, req.NamespacedName, binding); err != nil {
		if apierrors.IsNotFound(err) {
			c.workloadUnhealthyMap.delete(req.NamespacedName)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	if !c.bindingFilter(binding) {
		c.workloadUnhealthyMap.delete(req.NamespacedName)
		return controllerruntime.Result{}, nil
	}

	retryDuration, err := c.syncBinding(ctx, binding)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	if retryDuration > 0 {
		klog.V(4).InfoS("Retry to check health status of the workload", "afterSeconds", retryDuration.Seconds())
		return controllerruntime.Result{RequeueAfter: retryDuration}, nil
	}
	return controllerruntime.Result{}, nil
}

func (c *RBApplicationFailoverController) detectFailure(clusters []string, tolerationSeconds *int32, key types.NamespacedName) (int32, []string) {
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

func (c *RBApplicationFailoverController) syncBinding(ctx context.Context, binding *workv1alpha2.ResourceBinding) (time.Duration, error) {
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
		klog.ErrorS(err, "Failed to evict binding", "namespace", binding.Namespace, "name", binding.Name)
		return 0, err
	}

	if len(needEvictClusters) != 0 {
		if err = c.updateBinding(ctx, binding, allClusters, needEvictClusters); err != nil {
			return 0, err
		}
	}

	// Cleanup clusters on which the application status is not unhealthy and clusters that have been evicted or removed in the workloadUnhealthyMap.
	c.workloadUnhealthyMap.deleteIrrelevantClusters(key, allClusters, others)

	return time.Duration(duration) * time.Second, nil
}

func (c *RBApplicationFailoverController) evictBinding(binding *workv1alpha2.ResourceBinding, clusters []string) error {
	clustersBeforeFailover := getClusterNamesFromTargetClusters(binding.Spec.Clusters)
	for _, cluster := range clusters {
		taskOpts, err := buildTaskOptions(binding.Spec.Failover.Application, binding.Status.AggregatedStatus, cluster, RBApplicationFailoverControllerName, clustersBeforeFailover)
		if err != nil {
			klog.ErrorS(err, "Failed to build TaskOptions for ResourceBinding under Cluster",
				"namespace", binding.Namespace,
				"name", binding.Name,
				"cluster", cluster)
			return err
		}
		binding.Spec.GracefulEvictCluster(cluster, workv1alpha2.NewTaskOptions(taskOpts...))
	}
	return nil
}

func (c *RBApplicationFailoverController) updateBinding(ctx context.Context, binding *workv1alpha2.ResourceBinding, allClusters sets.Set[string], needEvictClusters []string) error {
	if err := c.Update(ctx, binding); err != nil {
		for _, cluster := range needEvictClusters {
			helper.EmitClusterEvictionEventForResourceBinding(binding, cluster, c.EventRecorder, err)
		}
		klog.ErrorS(err, "Failed to update binding", "binding", klog.KObj(binding))
		return err
	}
	for _, cluster := range needEvictClusters {
		allClusters.Delete(cluster)
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
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(RBApplicationFailoverControllerName).
		For(&workv1alpha2.ResourceBinding{}, builder.WithPredicates(resourceBindingPredicateFn)).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
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
		klog.ErrorS(err, "Failed to construct clusterWideKey from binding", "namespace", rb.Namespace, "name", rb.Name)
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
