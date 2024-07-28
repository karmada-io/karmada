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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

const (
	// StatusControllerName is the controller name that will be used when reporting events.
	DefaultControllerName = "federated-resource-quota-default-status-controller"
)

// DefaultStatusController is to collect default status for the FederatedResourceQuota.
// The controller will determine resource usage based off the replicaRequirements defined in the relevant
// ResourceBindings found in the namespace where a FederatedResourceQuota is applied.
//
// The total sum of replicaRequirements cannot exceed the limits set by the quota.
type DefaultStatusController struct {
	client.Client // used to operate Work resources.
	EventRecorder record.EventRecorder
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The SyncController will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *DefaultStatusController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("FederatedResourceQuota status controller reconciling %s", req.NamespacedName.String())

	quota := &policyv1alpha1.FederatedResourceQuota{}

	if err := c.Get(ctx, req.NamespacedName, quota); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	if !quota.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	if quota.Spec.StaticAssignments != nil {
		klog.V(4).Infof("FederatedResourceQuota(%s) sets static assignments, not reconciling.", quota.Name)
		return controllerruntime.Result{}, nil
	}

	if err := c.collectQuotaStatus(quota); err != nil {
		klog.Errorf("Failed to collect status for FederatedResourceQuota(%s), error: %v", req.NamespacedName.String(), err)
		c.EventRecorder.Eventf(quota, corev1.EventTypeWarning, events.EventReasonCollectFederatedResourceQuotaStatusFailed, err.Error())
		return controllerruntime.Result{}, err
	}
	c.EventRecorder.Eventf(quota, corev1.EventTypeNormal, events.EventReasonCollectFederatedResourceQuotaStatusSucceed, "Collect status of FederatedResourceQuota(%s) succeed.", req.NamespacedName.String())
	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *DefaultStatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	fn := handler.MapFunc(
		func(_ context.Context, obj client.Object) []reconcile.Request {
			var requests []reconcile.Request
			binding := obj.(*workv1alpha2.ResourceBinding)
			quotaNamespace := binding.GetNamespace()
			quotaName := binding.Spec.FederatedResourceQuota
			// Only reconcile if the resourcebinding is limited by a ResourceQuota
			if quotaName != "" && quotaNamespace != "" {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: quotaNamespace,
						Name:      quotaName,
					},
				})
			}
			return requests
		},
	)

	resourceBindingPredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			binding := createEvent.Object.(*workv1alpha2.ResourceBinding)
			return binding.Spec.FederatedResourceQuota != ""
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			oldObj := updateEvent.ObjectOld.(*workv1alpha2.ResourceBinding)
			newObj := updateEvent.ObjectNew.(*workv1alpha2.ResourceBinding)
			// If either binding version has a quota, we should reconcile
			if oldObj.Spec.FederatedResourceQuota != "" || newObj.Spec.FederatedResourceQuota != "" {
				// If ResourceRequest is unchanged, ignore update
				return !reflect.DeepEqual(oldObj.Spec.ReplicaRequirements.ResourceRequest,
					newObj.Spec.ReplicaRequirements.ResourceRequest)
			}
			return false
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			binding := deleteEvent.Object.(*workv1alpha2.ResourceBinding)
			return binding.Spec.FederatedResourceQuota != ""
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	})
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&policyv1alpha1.FederatedResourceQuota{}).
		Watches(&workv1alpha2.ResourceBinding{}, handler.EnqueueRequestsFromMapFunc(fn), resourceBindingPredicate).
		Complete(c)
}

func (c *DefaultStatusController) collectQuotaStatus(quota *policyv1alpha1.FederatedResourceQuota) error {
	klog.V(4).Infof("Collecting FederatedResourceQuota status using ResourceBindings.")
	bindingList, err := helper.GetResourceBindingsByNamespace(c.Client, quota.Namespace)
	if err != nil {
		klog.Errorf("Failed to list resourcebindings tracked by FederatedResourceQuota(%s), error: %v", klog.KObj(quota).String(), err)
		return err
	}

	quotaStatus := quota.Status.DeepCopy()
	quotaStatus.Overall = quota.Spec.Overall
	quotaStatus.OverallUsed = calculateUsedWithResourceBinding(bindingList.Items)

	if reflect.DeepEqual(quota.Status, *quotaStatus) {
		klog.V(4).Infof("New quotaStatus are equal with old federatedResourceQuota(%s) status, no update required.", klog.KObj(quota).String())
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

func calculateUsedWithResourceBinding(resourceBindings []v1alpha2.ResourceBinding) corev1.ResourceList {
	overallUsed := corev1.ResourceList{}
	for index := range resourceBindings {
		// TODO: Consider skipping resourcebinding if it is not applied
		used := resourceBindings[index].Spec.ReplicaRequirements.ResourceRequest
		replicas := resourceBindings[index].Spec.Replicas
		for resourceName, quantity := range used {
			r, exist := overallUsed[resourceName]
			// Update quantity to reflect number of replicas in resource
			quantity.Mul(int64(replicas))
			if !exist {
				overallUsed[resourceName] = quantity
			} else {
				r.Add(quantity)
				overallUsed[resourceName] = r
			}
		}
	}
	return overallUsed
}
