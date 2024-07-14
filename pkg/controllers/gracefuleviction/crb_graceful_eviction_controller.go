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
	"k8s.io/client-go/tools/record"
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

// CRBGracefulEvictionControllerName is the controller name that will be used when reporting events.
const CRBGracefulEvictionControllerName = "cluster-resource-binding-graceful-eviction-controller"

// CRBGracefulEvictionController is to sync ClusterResourceBinding.spec.gracefulEvictionTasks.
type CRBGracefulEvictionController struct {
	client.Client
	EventRecorder           record.EventRecorder
	RateLimiterOptions      ratelimiterflag.Options
	GracefulEvictionTimeout time.Duration
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *CRBGracefulEvictionController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling ClusterResourceBinding %s.", req.NamespacedName.String())

	binding := &workv1alpha2.ClusterResourceBinding{}
	if err := c.Client.Get(ctx, req.NamespacedName, binding); err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	if !binding.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	retryDuration, err := c.syncBinding(binding)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	if retryDuration > 0 {
		klog.V(4).Infof("Retry to evict task after %v minutes.", retryDuration.Minutes())
		return controllerruntime.Result{RequeueAfter: retryDuration}, nil
	}
	return controllerruntime.Result{}, nil
}

func (c *CRBGracefulEvictionController) syncBinding(binding *workv1alpha2.ClusterResourceBinding) (time.Duration, error) {
	keptTask, evictedClusters := assessEvictionTasks(binding.Spec, binding.Status.AggregatedStatus, c.GracefulEvictionTimeout, metav1.Now())
	if reflect.DeepEqual(binding.Spec.GracefulEvictionTasks, keptTask) {
		return nextRetry(keptTask, c.GracefulEvictionTimeout, metav1.Now().Time), nil
	}

	objPatch := client.MergeFrom(binding)
	modifiedObj := binding.DeepCopy()
	modifiedObj.Spec.GracefulEvictionTasks = keptTask
	err := c.Client.Patch(context.TODO(), modifiedObj, objPatch)
	if err != nil {
		return 0, err
	}

	for _, cluster := range evictedClusters {
		klog.V(2).Infof("Success to evict Cluster(%s) from ClusterResourceBinding(%s) gracefulEvictionTasks",
			cluster, binding.Name)
		helper.EmitClusterEvictionEventForClusterResourceBinding(binding, cluster, c.EventRecorder, err)
	}
	return nextRetry(keptTask, c.GracefulEvictionTimeout, metav1.Now().Time), nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *CRBGracefulEvictionController) SetupWithManager(mgr controllerruntime.Manager) error {
	clusterResourceBindingPredicateFn := predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			newObj := createEvent.Object.(*workv1alpha2.ClusterResourceBinding)
			if len(newObj.Spec.GracefulEvictionTasks) == 0 {
				return false
			}
			// When the current component is restarted and there are still tasks in the
			// GracefulEvictionTasks queue, we need to continue the procession.
			return newObj.Status.SchedulerObservedGeneration == newObj.Generation
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			newObj := updateEvent.ObjectNew.(*workv1alpha2.ClusterResourceBinding)

			if len(newObj.Spec.GracefulEvictionTasks) == 0 {
				return false
			}

			return newObj.Status.SchedulerObservedGeneration == newObj.Generation
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&workv1alpha2.ClusterResourceBinding{}, builder.WithPredicates(clusterResourceBindingPredicateFn)).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions)}).
		Complete(c)
}
