/*
Copyright 2025 The Karmada Authors.

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
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

const (
	// QuotaEnforcementControllerName is the controller name that will be used when reporting events.
	QuotaEnforcementControllerName = "federated-resource-quota-enforcement-controller"
)

// QuotaEnforcementController reflects the status of the namespaced FederatedResourceQuotas.
// The controller will reconcile in two cases:
//  1. When a FederatedResourceQuota is CREATED or UPDATED
//  2. When a ResourceBinding is DELETED
//
// When reconciling the controller will update Overall and OverallUsed.
type QuotaEnforcementController struct {
	client.Client // used to operate FederatedResourceQuota and ResourceBinding resources.
	EventRecorder record.EventRecorder
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The SyncController will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *QuotaEnforcementController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("QuotaEnforcementController reconciling %s", req.NamespacedName.String())

	quota := &policyv1alpha1.FederatedResourceQuota{}
	if err := c.Get(ctx, req.NamespacedName, quota); err != nil {
		// FederatedResourceQuota may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		klog.V(4).Infof("Error fetching FederatedResourceQuota %s", req.NamespacedName.String())
		return controllerruntime.Result{}, err
	}

	if !quota.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	if err := c.collectQuotaStatus(quota); err != nil {
		klog.Errorf("Failed to collect status for FederatedResourceQuota(%s), error: %v", req.NamespacedName.String(), err)
		c.EventRecorder.Eventf(quota, corev1.EventTypeWarning, events.EventReasonCollectFederatedResourceQuotaOverallStatusFailed, err.Error())
		return controllerruntime.Result{}, err
	}

	c.EventRecorder.Eventf(quota, corev1.EventTypeNormal, events.EventReasonCollectFederatedResourceQuotaOverallStatusSucceed,
		"Collect status of FederatedResourceQuota(%s) succeed.", req.NamespacedName.String())
	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *QuotaEnforcementController) SetupWithManager(mgr controllerruntime.Manager) error {
	fn := handler.MapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			federatedResourceQuotaList := &policyv1alpha1.FederatedResourceQuotaList{}
			if err := c.Client.List(ctx, federatedResourceQuotaList); err != nil {
				klog.Errorf("Failed to list FederatedResourceQuota, error: %v", err)
			}

			var requests []reconcile.Request
			for _, federatedResourceQuota := range federatedResourceQuotaList.Items {
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

	federatedResourceQuotaPredicate := predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(obj event.UpdateEvent) bool {
			oldObj := obj.ObjectOld.(*policyv1alpha1.FederatedResourceQuota)
			newObj := obj.ObjectNew.(*policyv1alpha1.FederatedResourceQuota)
			return !equality.Semantic.DeepEqual(oldObj.Spec.Overall, newObj.Spec.Overall)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false // ignore deletes
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}

	resourceBindingPredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return false // ignore creates
		},
		UpdateFunc: func(_ event.UpdateEvent) bool {
			return false // ignore updates
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return true // only care about RB deletions
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	})
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&policyv1alpha1.FederatedResourceQuota{}, builder.WithPredicates(federatedResourceQuotaPredicate)).
		Watches(&workv1alpha2.ResourceBinding{}, handler.EnqueueRequestsFromMapFunc(fn), resourceBindingPredicate).
		Complete(c)
}

func (c *QuotaEnforcementController) collectQuotaStatus(quota *policyv1alpha1.FederatedResourceQuota) error {
	klog.V(4).Info("Collecting FederatedResourceQuota status using ResourceBindings.")

	// TODO: Consider adding filtering step to ResourceBinding list once scope is added to the quota
	bindingList, err := helper.GetResourceBindingsByNamespace(c.Client, quota.Namespace)
	if err != nil {
		klog.Errorf("Failed to list resourcebindings tracked by FederatedResourceQuota(%s), error: %v", klog.KObj(quota).String(), err)
		return err
	}

	quotaStatus := quota.Status.DeepCopy()
	quotaStatus.Overall = quota.Spec.Overall
	quotaStatus.OverallUsed = calculateUsedWithResourceBinding(bindingList.Items, quota.Spec.Overall)

	if reflect.DeepEqual(quota.Status, *quotaStatus) {
		klog.V(4).Infof("New quotaStatus is equal with old federatedResourceQuota(%s) status, no update required.", klog.KObj(quota).String())
		return nil
	}

	_, statuserr := helper.UpdateStatus(context.Background(), c.Client, quota, func() error {
		quota.Status = *quotaStatus
		return nil
	})

	return statuserr
}

func calculateUsedWithResourceBinding(resourceBindings []workv1alpha2.ResourceBinding, overall corev1.ResourceList) corev1.ResourceList {
	overallUsed := corev1.ResourceList{}
	for _, binding := range resourceBindings {
		if binding.Spec.ReplicaRequirements == nil || binding.Spec.Clusters == nil {
			continue
		}
		used := binding.Spec.ReplicaRequirements.ResourceRequest
		replicas := binding.Spec.Replicas

		for name, quantity := range used {
			// Update quantity to reflect number of replicas in resource
			q := quantity.DeepCopy()
			q.Mul(int64(replicas))

			// Update quantity to reflect the number of clusters resource is duplicated to
			if binding.Spec.Placement != nil && binding.Spec.Placement.ReplicaSchedulingType() == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
				q.Mul(int64(len(binding.Spec.Clusters)))
			}

			existing, found := overallUsed[name]
			if found {
				existing.Add(q)
				overallUsed[name] = existing
			} else {
				overallUsed[name] = q
			}
		}
	}
	return filterResourceListByOverall(overallUsed, overall)
}

// Filters source ResourceList using the keys provided by reference
func filterResourceListByOverall(source, reference corev1.ResourceList) corev1.ResourceList {
	filteredUsed := corev1.ResourceList{}
	for key := range reference {
		if quantity, exists := source[key]; exists {
			filteredUsed[key] = quantity
		}
	}
	return filteredUsed
}
