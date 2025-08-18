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

// CRBApplicationFailoverControllerName is the controller name that will be used when reporting events and metrics.
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
	klog.V(4).InfoS("Reconciling ClusterResourceBinding", "name", req.Name)

	binding := &workv1alpha2.ClusterResourceBinding{}
	if err := c.Client.Get(ctx, req.NamespacedName, binding); err != nil {
		if apierrors.IsNotFound(err) {
			c.workloadUnhealthyMap.delete(req.NamespacedName)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	filter, err := c.clusterResourceBindingFilter(binding)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	if !filter {
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

func (c *CRBApplicationFailoverController) syncBinding(ctx context.Context, binding *workv1alpha2.ClusterResourceBinding) (time.Duration, error) {
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
		klog.ErrorS(err, "Failed to evict binding", "name", binding.Name)
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

func (c *CRBApplicationFailoverController) evictBinding(binding *workv1alpha2.ClusterResourceBinding, clusters []string) error {
	clustersBeforeFailover := getClusterNamesFromTargetClusters(binding.Spec.Clusters)
	for _, cluster := range clusters {
		taskOpts, err := buildTaskOptions(binding.Spec.Failover.Application, binding.Status.AggregatedStatus, cluster, CRBApplicationFailoverControllerName, clustersBeforeFailover)
		if err != nil {
			klog.ErrorS(err, "Failed to build TaskOptions for ClusterResourceBinding under Cluster",
				"name", binding.Name,
				"cluster", cluster)
			return err
		}
		binding.Spec.GracefulEvictCluster(cluster, workv1alpha2.NewTaskOptions(taskOpts...))
	}
	return nil
}

func (c *CRBApplicationFailoverController) updateBinding(ctx context.Context, binding *workv1alpha2.ClusterResourceBinding, allClusters sets.Set[string], needEvictClusters []string) error {
	if err := c.Update(ctx, binding); err != nil {
		for _, cluster := range needEvictClusters {
			helper.EmitClusterEvictionEventForClusterResourceBinding(binding, cluster, c.EventRecorder, err)
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
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(CRBApplicationFailoverControllerName).
		For(&workv1alpha2.ClusterResourceBinding{}, builder.WithPredicates(clusterResourceBindingPredicateFn)).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		Complete(c)
}

func (c *CRBApplicationFailoverController) clusterResourceBindingFilter(crb *workv1alpha2.ClusterResourceBinding) (bool, error) {
	if crb.Spec.Failover == nil || crb.Spec.Failover.Application == nil {
		return false, nil
	}

	if len(crb.Status.AggregatedStatus) == 0 {
		return false, nil
	}

	resourceKey, err := helper.ConstructClusterWideKey(crb.Spec.Resource)
	if err != nil {
		// Never reach
		klog.ErrorS(err, "Failed to construct clusterWideKey from clusterResourceBinding", "name", crb.Name)
		return false, err
	}

	enabled, err := c.ResourceInterpreter.HookEnabled(resourceKey.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretHealth)
	if err != nil {
		klog.Errorf("Failed to check if the interpret health hook is enabled for resource(kind=%s, %s/%s): %v",
			resourceKey.GroupVersionKind().Kind, resourceKey.Namespace, resourceKey.Name, err)
		return false, err
	}

	return enabled, nil
}
