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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

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
	Recalculation QuotaRecalculation
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The SyncController will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *QuotaEnforcementController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("QuotaEnforcementController reconciling", "namespacedName", req.NamespacedName.String())

	quota := &policyv1alpha1.FederatedResourceQuota{}
	if err := c.Get(ctx, req.NamespacedName, quota); err != nil {
		// FederatedResourceQuota may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		klog.ErrorS(err, "Error fetching FederatedResourceQuota", "federatedResourceQuota", req.NamespacedName.String())
		return controllerruntime.Result{}, err
	}

	if !quota.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	if err := c.collectQuotaStatus(quota); err != nil {
		klog.ErrorS(err, "Failed to collect status for FederatedResourceQuota", "federatedResourceQuota", req.NamespacedName.String())
		c.EventRecorder.Eventf(quota, corev1.EventTypeWarning, events.EventReasonCollectFederatedResourceQuotaOverallStatusFailed, err.Error())
		return controllerruntime.Result{}, err
	}

	c.EventRecorder.Eventf(quota, corev1.EventTypeNormal, events.EventReasonCollectFederatedResourceQuotaOverallStatusSucceed,
		"Collect status of FederatedResourceQuota(%s) succeed.", req.NamespacedName.String())
	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *QuotaEnforcementController) SetupWithManager(mgr controllerruntime.Manager) error {
	enqueueEffectedFRQ := handler.MapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			rb, ok := obj.(*workv1alpha2.ResourceBinding)
			if !ok {
				klog.ErrorS(fmt.Errorf("unexpected type: %T", obj), "Failed to convert object to ResourceBinding", "object", obj)
				return []reconcile.Request{}
			}

			federatedResourceQuotaList := &policyv1alpha1.FederatedResourceQuotaList{}
			if err := c.Client.List(ctx, federatedResourceQuotaList, &client.ListOptions{Namespace: rb.GetNamespace()}); err != nil {
				klog.ErrorS(err, "Failed to list FederatedResourceQuota")
				return []reconcile.Request{}
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

	// enqueueAll defines a mapping function that triggers reconciliation for all FederatedResourceQuota resources.
	// It is invoked periodically(controlled by ResyncPeriod).
	enqueueAll := handler.MapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			federatedResourceQuotaList := &policyv1alpha1.FederatedResourceQuotaList{}
			if err := c.Client.List(ctx, federatedResourceQuotaList); err != nil {
				klog.ErrorS(err, "Failed to list FederatedResourceQuota")
				return []reconcile.Request{}
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

	controller, err := controllerruntime.NewControllerManagedBy(mgr).
		For(&policyv1alpha1.FederatedResourceQuota{}, builder.WithPredicates(federatedResourceQuotaPredicate)).
		Watches(&workv1alpha2.ResourceBinding{}, handler.EnqueueRequestsFromMapFunc(enqueueEffectedFRQ), resourceBindingPredicate).
		Build(c)
	if err != nil {
		return err
	}

	// Creates a resync event channel to trigger periodic synchronization.
	// Because controller.Watch requires a non-nil Source channel, we first initialize the resyncEvent here.
	c.Recalculation.resyncEvent = make(chan event.GenericEvent)
	return utilerrors.NewAggregate([]error{
		mgr.Add(&c.Recalculation),
		controller.Watch(source.Channel(c.Recalculation.resyncEvent, handler.EnqueueRequestsFromMapFunc(enqueueAll))),
	})
}

// QuotaRecalculation holds the configuration for periodically recalculating the status of FederatedResourceQuota resources.
type QuotaRecalculation struct {
	// ResyncPeriod defines the interval for periodic full resynchronization of FederatedResourceQuota resources.
	// This ensures quota recalculations occur at regular intervals to correct potential inaccuracies,
	// particularly when webhook validation side effects.
	ResyncPeriod metav1.Duration
	resyncEvent  chan event.GenericEvent
}

// Start starts the quota recalculation process.
// If the resync period is configured (>0):
// - Fires a blank event at each resync interval
// - Uses time.Ticker for precise periodic scheduling
// The empty GenericEvent acts as a signal to force reconciliation.
func (q *QuotaRecalculation) Start(ctx context.Context) error {
	if q.resyncEvent == nil {
		return fmt.Errorf("resyncEvent channel is not initialized, cannot start quota recalculation")
	}
	defer close(q.resyncEvent)

	if q.ResyncPeriod.Duration > 0 {
		klog.InfoS("Starting FederatedResourceQuota recalculation process with period", "duration", q.ResyncPeriod.Duration.String())
		ticker := time.NewTicker(q.ResyncPeriod.Duration)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				q.resyncEvent <- event.GenericEvent{}
			}
		}
	}

	for {
		<-ctx.Done()
		return nil
	}
}

func (c *QuotaEnforcementController) collectQuotaStatus(quota *policyv1alpha1.FederatedResourceQuota) error {
	klog.V(4).Info("Collecting FederatedResourceQuota status using ResourceBindings.")

	// TODO: Consider adding filtering step to ResourceBinding list once scope is added to the quota
	bindingList, err := helper.GetResourceBindingsByNamespace(c.Client, quota.Namespace)
	if err != nil {
		klog.ErrorS(err, "Failed to list resourcebindings tracked by FederatedResourceQuota", "federatedResourceQuota", klog.KObj(quota).String())
		return err
	}

	quotaStatus := quota.Status.DeepCopy()
	quotaStatus.Overall = quota.Spec.Overall
	quotaStatus.OverallUsed = calculateUsedWithResourceBinding(bindingList.Items, quota.Spec.Overall)

	if reflect.DeepEqual(quota.Status, *quotaStatus) {
		klog.V(4).InfoS("New quotaStatus is equal with old federatedResourceQuota status, no update required.", "federatedResourceQuota", klog.KObj(quota).String())
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
		usage := calculateResourceUsage(&binding)
		for name, quantity := range usage {
			existing, found := overallUsed[name]
			if found {
				existing.Add(quantity)
				overallUsed[name] = existing
			} else {
				overallUsed[name] = quantity
			}
		}
	}
	return filterResourceListByOverall(overallUsed, overall)
}

// calculateResourceUsage calculates the total resource usage based on the ResourceBinding.
func calculateResourceUsage(rb *workv1alpha2.ResourceBinding) corev1.ResourceList {
	if rb == nil || rb.Spec.ReplicaRequirements == nil || len(rb.Spec.ReplicaRequirements.ResourceRequest) == 0 || len(rb.Spec.Clusters) == 0 {
		return corev1.ResourceList{}
	}

	totalReplicas := int32(0)
	for _, cluster := range rb.Spec.Clusters {
		totalReplicas += cluster.Replicas
	}

	if totalReplicas == 0 {
		return corev1.ResourceList{}
	}

	usage := corev1.ResourceList{}
	replicaCount := int64(totalReplicas)

	for resourceName, quantityPerReplica := range rb.Spec.ReplicaRequirements.ResourceRequest {
		if quantityPerReplica.IsZero() {
			continue
		}

		totalQuantity := quantityPerReplica.DeepCopy()
		totalQuantity.Mul(replicaCount)

		usage[resourceName] = totalQuantity
	}

	return usage
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
