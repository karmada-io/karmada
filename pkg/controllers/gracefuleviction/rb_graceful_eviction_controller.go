/*
Copyright 2022 The Karmada Authors.

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

package gracefuleviction

import (
	"context"
	"reflect"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// RBGracefulEvictionControllerName is the controller name that will be used when reporting events and metrics.
const RBGracefulEvictionControllerName = "resource-binding-graceful-eviction-controller"

// RBGracefulEvictionController is to sync ResourceBinding.spec.gracefulEvictionTasks.
type RBGracefulEvictionController struct {
	client.Client
	EventRecorder           record.EventRecorder
	RateLimiterOptions      ratelimiterflag.Options
	GracefulEvictionTimeout time.Duration
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *RBGracefulEvictionController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling ResourceBinding", "namespace", req.Namespace, "name", req.Name)

	retryDuration, err := c.syncBinding(ctx, req.NamespacedName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}
	if retryDuration > 0 {
		klog.V(4).InfoS("Retry to evict task after minutes", "retryAfterMinutes", retryDuration.Minutes())
		return controllerruntime.Result{RequeueAfter: retryDuration}, nil
	}
	return controllerruntime.Result{}, nil
}

// syncBinding wraps the read-modify-patch in retry.RetryOnConflict so
// optimistic-lock collisions with the taint-manager's binding-eviction
// worker (which also writes Spec.GracefulEvictionTasks via Update) get a
// tight inline refetch+retry instead of bouncing through the controller's
// exponential workqueue backoff. The binding is fetched fresh inside the
// closure on each attempt so the patch's MergeFromWithOptimisticLock base
// always carries the latest resourceVersion.
func (c *RBGracefulEvictionController) syncBinding(ctx context.Context, name types.NamespacedName) (time.Duration, error) {
	var (
		retryAfter      time.Duration
		evictedClusters []string
		bindingForEvent *workv1alpha2.ResourceBinding
	)
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		binding := &workv1alpha2.ResourceBinding{}
		if getErr := c.Client.Get(ctx, name, binding); getErr != nil {
			return getErr
		}
		if !binding.DeletionTimestamp.IsZero() {
			retryAfter = 0
			evictedClusters = nil
			bindingForEvent = nil
			return nil
		}

		keptTask, evictedCluster := assessEvictionTasks(binding.Spec.GracefulEvictionTasks, metav1.Now(), assessmentOption{
			timeout:        c.GracefulEvictionTimeout,
			scheduleResult: binding.Spec.Clusters,
			observedStatus: binding.Status.AggregatedStatus,
			hasScheduled:   binding.Status.SchedulerObservedGeneration == binding.Generation,
		})
		if reflect.DeepEqual(binding.Spec.GracefulEvictionTasks, keptTask) {
			retryAfter = nextRetry(keptTask, c.GracefulEvictionTimeout, metav1.Now().Time)
			evictedClusters = nil
			bindingForEvent = nil
			return nil
		}

		objPatch := client.MergeFromWithOptions(binding, client.MergeFromWithOptimisticLock{})
		modifiedObj := binding.DeepCopy()
		modifiedObj.Spec.GracefulEvictionTasks = keptTask
		if patchErr := c.Client.Patch(ctx, modifiedObj, objPatch); patchErr != nil {
			return patchErr
		}
		evictedClusters = evictedCluster
		bindingForEvent = modifiedObj
		retryAfter = nextRetry(keptTask, c.GracefulEvictionTimeout, metav1.Now().Time)
		return nil
	})
	if err != nil {
		return 0, err
	}

	for _, cluster := range evictedClusters {
		klog.V(2).InfoS("Evicted cluster from ResourceBinding gracefulEvictionTasks", "cluster", cluster,
			"namespace", bindingForEvent.Namespace, "name", bindingForEvent.Name)
		helper.EmitClusterEvictionEventForResourceBinding(bindingForEvent, cluster, c.EventRecorder, nil)
	}
	return retryAfter, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *RBGracefulEvictionController) SetupWithManager(mgr controllerruntime.Manager) error {
	resourceBindingPredicateFn := predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			newObj := createEvent.Object.(*workv1alpha2.ResourceBinding)
			// When the current component is restarted and there are still tasks in the
			// GracefulEvictionTasks queue, we need to continue the procession.
			return len(newObj.Spec.GracefulEvictionTasks) != 0
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			newObj := updateEvent.ObjectNew.(*workv1alpha2.ResourceBinding)
			return len(newObj.Spec.GracefulEvictionTasks) != 0
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(RBGracefulEvictionControllerName).
		For(&workv1alpha2.ResourceBinding{}, builder.WithPredicates(resourceBindingPredicateFn)).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		Complete(c)
}
