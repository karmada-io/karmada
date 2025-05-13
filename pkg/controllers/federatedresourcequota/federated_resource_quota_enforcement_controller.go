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
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

const (
	// QuotaEnforcementControllerName is the controller name that will be used when reporting events.
	QuotaEnforcementControllerName = "federated-resource-quota-enforcement-controller"
)

// QuotaEnforcementController reflects the overall resource usage in namespaced FederatedResourceQuotas.
// The controller will determine resource usage based off the replicaRequirements defined in the relevant
// ResourceBindings found in the namespace where a FederatedResourceQuota is applied.
//
// The total sum of replicaRequirements cannot exceed the limits set by the quota.
type QuotaEnforcementController struct {
	client.Client // used to operate Work resources.
	EventRecorder record.EventRecorder
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The SyncController will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *QuotaEnforcementController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.V(4).Infof("QuotaEnforcementController reconciling %s", req.NamespacedName.String())

	// First try fetching FederatedResourceQuota
	quota := &policyv1alpha1.FederatedResourceQuota{}
	if err := c.Get(ctx, req.NamespacedName, quota); err == nil {
		if !quota.DeletionTimestamp.IsZero() {
			return ctrl.Result{}, nil
		}

		if err := c.collectQuotaStatus(quota); err != nil {
			klog.Errorf("Failed to collect status for FederatedResourceQuota(%s), error: %v", req.NamespacedName.String(), err)
			c.EventRecorder.Eventf(quota, corev1.EventTypeWarning, events.EventReasonCollectFederatedResourceQuotaStatusFailed, err.Error())
			return ctrl.Result{}, err
		}
	} else if apierrors.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		return ctrl.Result{}, nil
	}

	// If not FederatedResourceQuota, check if its a deleted ResourceBinding
	binding := &workv1alpha2.ResourceBinding{}
	if err := c.Get(ctx, req.NamespacedName, binding); err != nil {
		if apierrors.IsNotFound(err) {
			c.collectQuotaStatuses(ctx, binding.Namespace)
		} else {
			return ctrl.Result{}, err
		}
	}

	c.EventRecorder.Eventf(quota, corev1.EventTypeNormal, events.EventReasonCollectFederatedResourceQuotaStatusSucceed, "Collect status of FederatedResourceQuota(%s) succeed.", req.NamespacedName.String())
	return ctrl.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *QuotaEnforcementController) SetupWithManager(mgr ctrl.Manager) error {
	fn := handler.MapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			var quotaList policyv1alpha1.FederatedResourceQuotaList
			if err := c.Client.List(ctx, &quotaList, client.InNamespace(obj.GetNamespace())); err != nil {
				return nil
			}

			if len(quotaList.Items) == 0 {
				return nil // no quotas in namespace, skip reconcile
			}

			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Namespace: obj.GetNamespace(),
					Name:      obj.GetName(),
				},
			}}
		},
	)

	resourceBindingPredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return false // ignore creates
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return false // ignore updates
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return true // only care about RB deletions
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	})
	return ctrl.NewControllerManagedBy(mgr).
		For(&policyv1alpha1.FederatedResourceQuota{}).
		Watches(&workv1alpha2.ResourceBinding{}, handler.EnqueueRequestsFromMapFunc(fn), resourceBindingPredicate).
		Complete(c)
}

func (c *QuotaEnforcementController) collectQuotaStatuses(ctx context.Context, namespace string) error {
	var quotaList policyv1alpha1.FederatedResourceQuotaList
	if err := c.Client.List(ctx, &quotaList, client.InNamespace(namespace)); err != nil {
		return err
	}

	for _, quota := range quotaList.Items {
		err := c.collectQuotaStatus(&quota)
		if err != nil {
			fmt.Printf("failed to collect status for FederatedResourceQuota(%s), error: %v", klog.KObj(&quota).String(), err)
		}
	}

	return nil
}

func (c *QuotaEnforcementController) collectQuotaStatus(quota *policyv1alpha1.FederatedResourceQuota) error {
	klog.V(4).Infof("Collecting FederatedResourceQuota status using ResourceBindings.")

	// TODO: Consider adding filtering step to ResourceBinding list once scope is added to the quota
	bindingList, err := helper.GetResourceBindingsByNamespace(c.Client, quota.Namespace)
	if err != nil {
		klog.Errorf("Failed to list resourcebindings tracked by FederatedResourceQuota(%s), error: %v", klog.KObj(quota).String(), err)
		return err
	}

	quotaStatus := quota.Status.DeepCopy()
	quotaStatus.Overall = quota.Spec.Overall
	quotaStatus.OverallUsed = calculateUsedWithResourceBinding(bindingList.Items)

	if reflect.DeepEqual(quota.Status, *quotaStatus) {
		klog.V(4).Infof("New quotaStatus is equal with old federatedResourceQuota(%s) status, no update required.", klog.KObj(quota).String())
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = helper.UpdateStatus(context.Background(), c.Client, quota, func() error {
			quota.Status = *quotaStatus
			return nil
		})
		return err
	})
}

func calculateUsedWithResourceBinding(resourceBindings []workv1alpha2.ResourceBinding) corev1.ResourceList {
	overallUsed := corev1.ResourceList{}
	for _, binding := range resourceBindings {
		// TODO: Consider skipping resourcebinding if it is not applied
		if binding.Spec.ReplicaRequirements == nil {
			continue
		}
		used := binding.Spec.ReplicaRequirements.ResourceRequest
		replicas := binding.Spec.Replicas

		for name, quantity := range used {
			// Update quantity to reflect number of replicas in resource
			q := quantity.DeepCopy()
			q.Mul(int64(replicas))

			// Update quantity to reflect the number of clusters resource is duplicated to
			if binding.Spec.Placement != nil && binding.Spec.Placement.ReplicaSchedulingType() == v1alpha1.ReplicaSchedulingTypeDuplicated {
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
	return overallUsed
}
