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
	"encoding/json"
	"reflect"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// StatusControllerName is the controller name that will be used when reporting events and metrics.
	StatusControllerName = "federated-resource-quota-status-controller"
)

// StatusController is to collect status from work to FederatedResourceQuota.
type StatusController struct {
	client.Client      // used to operate Work resources.
	EventRecorder      record.EventRecorder
	RateLimiterOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The SyncController will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *StatusController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("FederatedResourceQuota status controller reconciling", "namespacedName", req.NamespacedName.String())

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

	if err := c.collectQuotaStatus(ctx, quota); err != nil {
		klog.ErrorS(err, "Failed to collect status from works to federatedResourceQuota", "federatedResourceQuota", req.NamespacedName.String())
		c.EventRecorder.Eventf(quota, corev1.EventTypeWarning, events.EventReasonCollectFederatedResourceQuotaStatusFailed, err.Error())
		return controllerruntime.Result{}, err
	}
	c.EventRecorder.Eventf(quota, corev1.EventTypeNormal, events.EventReasonCollectFederatedResourceQuotaStatusSucceed, "Collect status of FederatedResourceQuota(%s) succeed.", req.NamespacedName.String())
	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *StatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	fn := handler.MapFunc(
		func(_ context.Context, obj client.Object) []reconcile.Request {
			var requests []reconcile.Request

			quotaNamespace, namespaceExist := obj.GetLabels()[util.FederatedResourceQuotaNamespaceLabel]
			quotaName, nameExist := obj.GetLabels()[util.FederatedResourceQuotaNameLabel]
			if namespaceExist && nameExist {
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

	workPredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			objOld := e.ObjectOld.(*workv1alpha1.Work)
			objNew := e.ObjectNew.(*workv1alpha1.Work)
			_, namespaceExist := objNew.GetLabels()[util.FederatedResourceQuotaNamespaceLabel]
			_, nameExist := objNew.GetLabels()[util.FederatedResourceQuotaNameLabel]
			if !namespaceExist || !nameExist {
				return false
			}
			return !reflect.DeepEqual(objOld.Status, objNew.Status)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			obj := deleteEvent.Object.(*workv1alpha1.Work)
			_, namespaceExist := obj.GetLabels()[util.FederatedResourceQuotaNamespaceLabel]
			_, nameExist := obj.GetLabels()[util.FederatedResourceQuotaNameLabel]
			if !namespaceExist || !nameExist {
				return false
			}
			return true
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	})
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(StatusControllerName).
		For(&policyv1alpha1.FederatedResourceQuota{}).
		Watches(&workv1alpha1.Work{}, handler.EnqueueRequestsFromMapFunc(fn), workPredicate).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions),
		}).
		Complete(c)
}

func (c *StatusController) collectQuotaStatus(ctx context.Context, quota *policyv1alpha1.FederatedResourceQuota) error {
	workList, err := helper.GetWorksByLabelsSet(ctx, c.Client, labels.Set{
		util.FederatedResourceQuotaNamespaceLabel: quota.Namespace,
		util.FederatedResourceQuotaNameLabel:      quota.Name,
	})
	if err != nil {
		klog.ErrorS(err, "Failed to list workList created by federatedResourceQuota", "federatedResourceQuota", klog.KObj(quota).String())
		return err
	}

	aggregatedStatuses, err := aggregatedStatusFormWorks(workList.Items)
	if err != nil {
		return err
	}

	quotaStatus := quota.Status.DeepCopy()
	quotaStatus.AggregatedStatus = aggregatedStatuses

	if !features.FeatureGate.Enabled(features.FederatedQuotaEnforcement) {
		quotaStatus.Overall = quota.Spec.Overall
		quotaStatus.OverallUsed = calculateUsed(aggregatedStatuses)
	}

	if reflect.DeepEqual(quota.Status, *quotaStatus) {
		klog.V(4).InfoS("New quotaStatus is equal with old federatedResourceQuota status, no update required.", "federatedResourceQuota", klog.KObj(quota).String())
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = helper.UpdateStatus(ctx, c.Client, quota, func() error {
			quota.Status = *quotaStatus
			return nil
		})
		return err
	})
}

func aggregatedStatusFormWorks(works []workv1alpha1.Work) ([]policyv1alpha1.ClusterQuotaStatus, error) {
	var aggregatedStatuses []policyv1alpha1.ClusterQuotaStatus
	for index := range works {
		work := works[index]
		var applied bool
		if cond := meta.FindStatusCondition(work.Status.Conditions, workv1alpha1.WorkApplied); cond != nil {
			switch cond.Status {
			case metav1.ConditionTrue:
				applied = true
			case metav1.ConditionUnknown:
				fallthrough
			case metav1.ConditionFalse:
				applied = false
			default: // should not happen unless the condition api changed.
				panic("unexpected status")
			}
		}
		if !applied {
			klog.Warningf("Work(%s) applied failed, skip aggregated status", klog.KObj(&work).String())
			continue
		}

		if len(work.Status.ManifestStatuses) == 0 {
			klog.Warningf("The ManifestStatuses length of work(%s) is zero", klog.KObj(&work).String())
			continue
		}

		clusterName, err := names.GetClusterName(work.Namespace)
		if err != nil {
			klog.ErrorS(err, "Failed to get clusterName from work namespace.", "workNamespace", work.Namespace)
			return nil, err
		}

		status := &corev1.ResourceQuotaStatus{}
		if err := json.Unmarshal(work.Status.ManifestStatuses[0].Status.Raw, status); err != nil {
			klog.ErrorS(err, "Failed to unmarshal work status to ResourceQuotaStatus", "work", klog.KObj(&work).String())
			return nil, err
		}

		aggregatedStatus := policyv1alpha1.ClusterQuotaStatus{
			ClusterName:         clusterName,
			ResourceQuotaStatus: *status,
		}
		aggregatedStatuses = append(aggregatedStatuses, aggregatedStatus)
	}

	sort.Slice(aggregatedStatuses, func(i, j int) bool {
		return aggregatedStatuses[i].ClusterName < aggregatedStatuses[j].ClusterName
	})
	return aggregatedStatuses, nil
}

func calculateUsed(aggregatedStatuses []policyv1alpha1.ClusterQuotaStatus) corev1.ResourceList {
	overallUsed := corev1.ResourceList{}
	for index := range aggregatedStatuses {
		used := aggregatedStatuses[index].Used
		for resourceName, quantity := range used {
			r, exist := overallUsed[resourceName]
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
