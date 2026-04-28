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

package federatedresourcequota

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/controllers/ctrlutil"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// SyncControllerName is the controller name that will be used when reporting events and metrics.
	SyncControllerName = "federated-resource-quota-sync-controller"
)

// SyncController is to sync FederatedResourceQuota.
type SyncController struct {
	client.Client      // used to operate Work resources.
	EventRecorder      record.EventRecorder
	RateLimiterOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The SyncController will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *SyncController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("FederatedResourceQuota sync controller reconciling", "namespacedName", req.NamespacedName.String())

	quota := &policyv1alpha1.FederatedResourceQuota{}
	if err := c.Client.Get(ctx, req.NamespacedName, quota); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).InfoS("Begin to cleanup works created by federatedResourceQuota", "namespacedName", req.NamespacedName.String())
			if err = c.cleanUpWorks(ctx, req.Namespace, req.Name); err != nil {
				klog.ErrorS(err, "Failed to cleanup works created by federatedResourceQuota", "namespacedName", req.NamespacedName.String())
				return controllerruntime.Result{}, err
			}
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	if err := c.cleanUpOrphanWorks(ctx, quota); err != nil {
		klog.ErrorS(err, "Failed to cleanup orphan works for federatedResourceQuota", "namespacedName", req.NamespacedName.String())
		return controllerruntime.Result{}, err
	}

	clusterList := &clusterv1alpha1.ClusterList{}
	if err := c.Client.List(ctx, clusterList); err != nil {
		klog.ErrorS(err, "Failed to list clusters")
		return controllerruntime.Result{}, err
	}

	if err := c.buildWorks(ctx, quota, clusterList.Items); err != nil {
		klog.ErrorS(err, "Failed to build works for federatedResourceQuota", "namespacedName", req.NamespacedName.String())
		c.EventRecorder.Eventf(quota, corev1.EventTypeWarning, events.EventReasonSyncFederatedResourceQuotaFailed, err.Error())
		return controllerruntime.Result{}, err
	}
	c.EventRecorder.Eventf(quota, corev1.EventTypeNormal, events.EventReasonSyncFederatedResourceQuotaSucceed, "Sync works for FederatedResourceQuota(%s) succeed.", req.NamespacedName.String())

	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *SyncController) SetupWithManager(mgr controllerruntime.Manager) error {
	fn := handler.MapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			var requests []reconcile.Request
			var clusterName string
			switch obj.(type) {
			case *clusterv1alpha1.Cluster:
				clusterName = obj.GetName()
			default:
				return nil
			}

			FederatedResourceQuotaList := &policyv1alpha1.FederatedResourceQuotaList{}
			if err := c.Client.List(ctx, FederatedResourceQuotaList); err != nil {
				klog.ErrorS(err, "Failed to list FederatedResourceQuota")
			}

			for _, federatedResourceQuota := range FederatedResourceQuotaList.Items {
				hard := extractClusterHardResourceList(federatedResourceQuota.Spec, clusterName)
				if hard == nil {
					continue
				}
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: federatedResourceQuota.Namespace,
						Name:      federatedResourceQuota.Name,
					},
				})
			}

			return requests
		},
	)

	clusterPredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	})

	frqPredicate := builder.WithPredicates(predicate.Funcs{
		// TODO(zhzhuang-zju): Should ignore the create event in case of the FRQ's .spec.staticAssignments is nil, as
		// no longer sync empty ResourceQuotas by default starting from v1.14. Here temporarily omitting this filter to
		// ensure that during upgrades from older versions, there is an opportunity to clean up legacy ResourceQuotas synced
		// from previous versions.
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			oldObj := updateEvent.ObjectOld.(*policyv1alpha1.FederatedResourceQuota)
			newObj := updateEvent.ObjectNew.(*policyv1alpha1.FederatedResourceQuota)
			return !reflect.DeepEqual(oldObj.Spec.StaticAssignments, newObj.Spec.StaticAssignments)
		},
		// TODO(zhzhuang-zju): Should ignore the delete event in case of the FRQ's .spec.staticAssignments is nil, as
		// no longer sync empty ResourceQuotas by default starting from v1.14. Here temporarily omitting this filter to
		// ensure that during upgrades from older versions, there is an opportunity to clean up legacy ResourceQuotas synced
		// from previous versions.
		DeleteFunc: func(event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
	})

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(SyncControllerName).
		For(&policyv1alpha1.FederatedResourceQuota{}, frqPredicate).
		Watches(&clusterv1alpha1.Cluster{}, handler.EnqueueRequestsFromMapFunc(fn), clusterPredicate).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions),
		}).
		Complete(c)
}

func (c *SyncController) cleanUpWorks(ctx context.Context, namespace, name string) error {
	var errs []error
	workList := &workv1alpha1.WorkList{}
	if err := c.List(ctx, workList, client.MatchingLabels{
		util.FederatedResourceQuotaNamespaceLabel: namespace,
		util.FederatedResourceQuotaNameLabel:      name,
	}); err != nil {
		klog.ErrorS(err, "Failed to list works")
		return err
	}

	for index := range workList.Items {
		work := &workList.Items[index]
		if err := c.Delete(ctx, work); err != nil && !apierrors.IsNotFound(err) {
			klog.ErrorS(err, "Failed to delete work", "work", klog.KObj(work).String())
			errs = append(errs, err)
		}
	}

	return errors.NewAggregate(errs)
}

func (c *SyncController) cleanUpOrphanWorks(ctx context.Context, quota *policyv1alpha1.FederatedResourceQuota) error {
	var errs []error
	workList := &workv1alpha1.WorkList{}
	if err := c.List(ctx, workList, client.MatchingLabels{
		util.FederatedResourceQuotaNamespaceLabel: quota.GetNamespace(),
		util.FederatedResourceQuotaNameLabel:      quota.GetName(),
	}); err != nil {
		klog.ErrorS(err, "Failed to list works")
		return err
	}

	for index := range workList.Items {
		work := &workList.Items[index]
		if !isOrphanQuotaWork(work, quota.Spec) {
			continue
		}
		if err := c.Delete(ctx, work); err != nil && !apierrors.IsNotFound(err) {
			klog.ErrorS(err, "Failed to delete work", "work", klog.KObj(work).String())
			errs = append(errs, err)
		}
	}

	return errors.NewAggregate(errs)
}

func (c *SyncController) buildWorks(ctx context.Context, quota *policyv1alpha1.FederatedResourceQuota, clusters []clusterv1alpha1.Cluster) error {
	var errs []error
	for _, cluster := range clusters {
		hard := extractClusterHardResourceList(quota.Spec, cluster.Name)
		if hard == nil {
			continue
		}

		resourceQuota := &corev1.ResourceQuota{}
		resourceQuota.APIVersion = "v1"
		resourceQuota.Kind = "ResourceQuota"
		resourceQuota.Namespace = quota.Namespace
		resourceQuota.Name = quota.Name
		resourceQuota.Spec.Hard = hard

		resourceQuotaObj, err := helper.ToUnstructured(resourceQuota)
		if err != nil {
			klog.ErrorS(err, "Failed to transform resourceQuota", "resourceQuota", klog.KObj(resourceQuota).String())
			errs = append(errs, err)
			continue
		}

		objectMeta := metav1.ObjectMeta{
			Namespace:  names.GenerateExecutionSpaceName(cluster.Name),
			Name:       names.GenerateWorkName(resourceQuota.Kind, quota.Name, quota.Namespace),
			Finalizers: []string{util.ExecutionControllerFinalizer},
			Labels: map[string]string{
				util.FederatedResourceQuotaNamespaceLabel: quota.Namespace,
				util.FederatedResourceQuotaNameLabel:      quota.Name,
			},
		}

		err = ctrlutil.CreateOrUpdateWork(ctx, c.Client, objectMeta, resourceQuotaObj)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.NewAggregate(errs)
}

func extractClusterHardResourceList(spec policyv1alpha1.FederatedResourceQuotaSpec, cluster string) corev1.ResourceList {
	for index := range spec.StaticAssignments {
		if spec.StaticAssignments[index].ClusterName == cluster {
			return spec.StaticAssignments[index].Hard
		}
	}
	return nil
}

func isOrphanQuotaWork(work *workv1alpha1.Work, spec policyv1alpha1.FederatedResourceQuotaSpec) bool {
	for _, assignment := range spec.StaticAssignments {
		if names.GenerateExecutionSpaceName(assignment.ClusterName) == work.GetNamespace() {
			return false
		}
	}

	return true
}
