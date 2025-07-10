/*
Copyright 2021 The Karmada Authors.

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

package binding

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

// ClusterResourceBindingControllerName is the controller name that will be used when reporting events and metrics.
const ClusterResourceBindingControllerName = "cluster-resource-binding-controller"

// ClusterResourceBindingController is to sync ClusterResourceBinding.
type ClusterResourceBindingController struct {
	client.Client                                                   // used to operate ClusterResourceBinding resources.
	DynamicClient       dynamic.Interface                           // used to fetch arbitrary resources from api server.
	InformerManager     genericmanager.SingleClusterInformerManager // used to fetch arbitrary resources from cache.
	EventRecorder       record.EventRecorder
	RESTMapper          meta.RESTMapper
	OverrideManager     overridemanager.OverrideManager
	ResourceInterpreter resourceinterpreter.ResourceInterpreter
	RateLimiterOptions  ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *ClusterResourceBindingController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling ClusterResourceBinding.", "ClusterResourceBinding", req.NamespacedName.Name)

	clusterResourceBinding := &workv1alpha2.ClusterResourceBinding{}
	if err := c.Client.Get(ctx, req.NamespacedName, clusterResourceBinding); err != nil {
		// The resource no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{}, err
	}

	if !clusterResourceBinding.DeletionTimestamp.IsZero() {
		klog.V(4).InfoS("Begin to delete works owned by ClusterResourceBinding.", "ClusterResourceBinding", clusterResourceBinding.Name)
		if err := helper.DeleteWorks(ctx, c.Client, "", req.Name, clusterResourceBinding.Labels[workv1alpha2.ClusterResourceBindingPermanentIDLabel]); err != nil {
			klog.ErrorS(err, "Failed to delete works related to ClusterResourceBinding.", "ClusterResourceBinding", clusterResourceBinding.Name)
			return controllerruntime.Result{}, err
		}
		return c.removeFinalizer(ctx, clusterResourceBinding)
	}

	return c.syncBinding(ctx, clusterResourceBinding)
}

// removeFinalizer removes finalizer from the given ClusterResourceBinding
func (c *ClusterResourceBindingController) removeFinalizer(ctx context.Context, crb *workv1alpha2.ClusterResourceBinding) (controllerruntime.Result, error) {
	if !controllerutil.ContainsFinalizer(crb, util.ClusterResourceBindingControllerFinalizer) {
		return controllerruntime.Result{}, nil
	}

	controllerutil.RemoveFinalizer(crb, util.ClusterResourceBindingControllerFinalizer)
	err := c.Client.Update(ctx, crb)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	return controllerruntime.Result{}, nil
}

// syncBinding will sync clusterResourceBinding to Works.
func (c *ClusterResourceBindingController) syncBinding(ctx context.Context, binding *workv1alpha2.ClusterResourceBinding) (controllerruntime.Result, error) {
	if err := c.removeOrphanWorks(ctx, binding); err != nil {
		return controllerruntime.Result{}, err
	}

	workload, err := helper.FetchResourceTemplate(ctx, c.DynamicClient, c.InformerManager, c.RESTMapper, binding.Spec.Resource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// It might happen when the resource template has been removed but the garbage collector hasn't removed
			// the ClusterResourceBinding which dependent on resource template.
			// So, just return without retry(requeue) would save unnecessary loop.
			return controllerruntime.Result{}, nil
		}
		klog.ErrorS(err, "Failed to fetch workload for ClusterResourceBinding.", "ClusterResourceBinding", binding.Name)
		return controllerruntime.Result{}, err
	}

	start := time.Now()
	err = ensureWork(ctx, c.Client, c.ResourceInterpreter, workload, c.OverrideManager, binding, apiextensionsv1.ClusterScoped)
	metrics.ObserveSyncWorkLatency(err, start)
	if err != nil {
		klog.ErrorS(err, "Failed to transform ClusterResourceBinding to works.", "ClusterResourceBinding", binding.Name)
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, events.EventReasonSyncWorkFailed, err.Error())
		c.EventRecorder.Event(workload, corev1.EventTypeWarning, events.EventReasonSyncWorkFailed, err.Error())
		return controllerruntime.Result{}, err
	}

	msg := fmt.Sprintf("Sync work of ClusterResourceBinding(%s) successful.", binding.Name)
	klog.V(4).InfoS("Sync work of ClusterResourceBinding successful.", "ClusterResourceBinding", binding.Name)
	c.EventRecorder.Event(binding, corev1.EventTypeNormal, events.EventReasonSyncWorkSucceed, msg)
	c.EventRecorder.Event(workload, corev1.EventTypeNormal, events.EventReasonSyncWorkSucceed, msg)
	return controllerruntime.Result{}, nil
}

func (c *ClusterResourceBindingController) removeOrphanWorks(ctx context.Context, binding *workv1alpha2.ClusterResourceBinding) error {
	works, err := helper.FindOrphanWorks(ctx, c.Client, "", binding.Name, binding.Labels[workv1alpha2.ClusterResourceBindingPermanentIDLabel], helper.ObtainBindingSpecExistingClusters(binding.Spec))
	if err != nil {
		klog.ErrorS(err, "Failed to find orphan works of ClusterResourceBinding.", "ClusterResourceBinding", binding.Name)
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, events.EventReasonCleanupWorkFailed, err.Error())
		return err
	}

	err = helper.RemoveOrphanWorks(ctx, c.Client, works)
	if err != nil {
		klog.ErrorS(err, "Failed to remove orphan works of ClusterResourceBinding.", "ClusterResourceBinding", binding.Name)
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, events.EventReasonCleanupWorkFailed, err.Error())
		return err
	}

	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ClusterResourceBindingController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ClusterResourceBindingControllerName).
		For(&workv1alpha2.ClusterResourceBinding{}).
		Watches(&policyv1alpha1.ClusterOverridePolicy{}, handler.EnqueueRequestsFromMapFunc(c.newOverridePolicyFunc())).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		Complete(c)
}

func (c *ClusterResourceBindingController) newOverridePolicyFunc() handler.MapFunc {
	return func(ctx context.Context, a client.Object) []reconcile.Request {
		var overrideRS []policyv1alpha1.ResourceSelector
		switch t := a.(type) {
		case *policyv1alpha1.ClusterOverridePolicy:
			overrideRS = t.Spec.ResourceSelectors
		default:
			return nil
		}

		readonlyBindingList := &workv1alpha2.ClusterResourceBindingList{}
		if err := c.Client.List(ctx, readonlyBindingList, &client.ListOptions{UnsafeDisableDeepCopy: ptr.To(true)}); err != nil {
			klog.ErrorS(err, "Failed to list ClusterResourceBindings")
			return nil
		}

		var requests []reconcile.Request
		for _, binding := range readonlyBindingList.Items {
			// Nil resourceSelectors means matching all resources.
			if len(overrideRS) == 0 {
				klog.V(2).InfoS("Enqueue ClusterResourceBinding as cluster override policy changes.", "ClusterResourceBinding", binding.Name, "ClusterOverridePolicy", a.GetName())
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: binding.Name}})
				continue
			}

			workload, err := helper.FetchResourceTemplate(ctx, c.DynamicClient, c.InformerManager, c.RESTMapper, binding.Spec.Resource)
			if err != nil {
				// If we cannot fetch resource template from binding, this may be due to the fact that the resource template has been deleted.
				// Just skip it so that it will not affect other bindings.
				klog.ErrorS(err, "Failed to fetch workload for ClusterResourceBinding.", "ClusterResourceBinding", binding.Name)
				continue
			}

			for _, rs := range overrideRS {
				if util.ResourceMatches(workload, rs) {
					klog.V(2).InfoS("Enqueue ClusterResourceBinding as cluster override policy changes.", "ClusterResourceBinding", binding.Name, "ClusterOverridePolicy", a.GetName())
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: binding.Name}})
					break
				}
			}
		}
		return requests
	}
}
