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

package status

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// RBStatusControllerName is the controller name that will be used when reporting events and metrics.
const RBStatusControllerName = "resource-binding-status-controller"

// RBStatusController is to sync status of ResourceBinding
// and aggregate status to the resource template.
type RBStatusController struct {
	client.Client                                                   // used to operate ResourceBinding resources.
	DynamicClient       dynamic.Interface                           // used to fetch arbitrary resources from api server.
	InformerManager     genericmanager.SingleClusterInformerManager // used to fetch arbitrary resources from cache.
	ResourceInterpreter resourceinterpreter.ResourceInterpreter
	EventRecorder       record.EventRecorder
	RESTMapper          meta.RESTMapper
	RateLimiterOptions  ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *RBStatusController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling ResourceBinding", "namespace", req.Namespace, "name", req.Name)

	binding := &workv1alpha2.ResourceBinding{}
	if err := c.Client.Get(ctx, req.NamespacedName, binding); err != nil {
		// The rb no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{}, err
	}

	// The rb is being deleted, in which case we stop processing.
	if !binding.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	err := c.syncBindingStatus(ctx, binding)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *RBStatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	workMapFunc := handler.MapFunc(
		func(_ context.Context, workObj client.Object) []reconcile.Request {
			var requests []reconcile.Request

			annotations := workObj.GetAnnotations()
			namespace, nsExist := annotations[workv1alpha2.ResourceBindingNamespaceAnnotationKey]
			name, nameExist := annotations[workv1alpha2.ResourceBindingNameAnnotationKey]
			if nsExist && nameExist {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: namespace,
						Name:      name,
					},
				})
			}

			return requests
		})

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(RBStatusControllerName).
		For(&workv1alpha2.ResourceBinding{}, bindingPredicateFn).
		Watches(&workv1alpha1.Work{}, handler.EnqueueRequestsFromMapFunc(workMapFunc), workPredicateFn).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		Complete(c)
}

func (c *RBStatusController) syncBindingStatus(ctx context.Context, binding *workv1alpha2.ResourceBinding) error {
	err := helper.AggregateResourceBindingWorkStatus(ctx, c.Client, binding, c.EventRecorder)
	if err != nil {
		klog.ErrorS(err, "Failed to aggregate workStatues to ResourceBinding", "namespace", binding.Namespace, "name", binding.Name)
		return err
	}

	err = updateResourceStatus(ctx, c.DynamicClient, c.RESTMapper, c.ResourceInterpreter, c.EventRecorder, binding.Spec.Resource, binding.Status)
	if err != nil {
		return err
	}

	return nil
}
