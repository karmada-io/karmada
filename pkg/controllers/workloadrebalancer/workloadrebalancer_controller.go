/*
Copyright 2024 The Karmada Authors.

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

package workloadrebalancer

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1alpha1 "github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// ControllerName is the controller name that will be used when reporting events and metrics.
	ControllerName = "workload-rebalancer"
)

// RebalancerController is to handle a rebalance to workloads selected by WorkloadRebalancer object.
type RebalancerController struct {
	Client             client.Client
	RateLimiterOptions ratelimiterflag.Options
}

// SetupWithManager creates a controller and register to controller manager.
func (c *RebalancerController) SetupWithManager(mgr controllerruntime.Manager) error {
	var predicateFunc = predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj := e.ObjectOld.(*appsv1alpha1.WorkloadRebalancer)
			newObj := e.ObjectNew.(*appsv1alpha1.WorkloadRebalancer)
			return !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&appsv1alpha1.WorkloadRebalancer{}, builder.WithPredicates(predicateFunc)).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		Complete(c)
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *RebalancerController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling for WorkloadRebalancer %s", req.Name)

	// 1. get latest WorkloadRebalancer
	rebalancer := &appsv1alpha1.WorkloadRebalancer{}
	if err := c.Client.Get(ctx, req.NamespacedName, rebalancer); err != nil {
		if apierrors.IsNotFound(err) {
			klog.InfoS("no need to reconcile WorkloadRebalancer for it not found")
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	// 2. list latest target workloads and trigger its rescheduling by updating its referenced binding.
	newStatus, err := c.handleWorkloadRebalance(ctx, rebalancer)
	if err != nil {
		return controllerruntime.Result{}, err
	}

	// 3. when all workloads finished, judge whether the rebalancer needs cleanup.
	rebalancer.Status = *newStatus
	if rebalancer.Spec.TTLSecondsAfterFinished != nil {
		if rebalancer.Status.FinishTime == nil {
			return controllerruntime.Result{}, fmt.Errorf("finish time should not be nil")
		}
		remainingTTL := timeLeft(rebalancer)
		if remainingTTL > 0 {
			return controllerruntime.Result{RequeueAfter: remainingTTL}, nil
		}
		if err := c.deleteWorkloadRebalancer(ctx, rebalancer); err != nil {
			return controllerruntime.Result{}, err
		}
	}

	return controllerruntime.Result{}, nil
}

// When spec filed of WorkloadRebalancer updated, we shall refresh the workload list in status.observedWorkloads:
//  1. a new workload added to spec list, just add it into status list and do the rebalance.
//  2. a workload deleted from previous spec list, keep it in status list if already success, and remove it if not.
//  3. a workload is modified, just regard it as deleted an old one and inserted a new one.
//  4. just list order is disrupted, no additional action.
func (c *RebalancerController) syncWorkloadsFromSpecToStatus(rebalancer *appsv1alpha1.WorkloadRebalancer) *appsv1alpha1.WorkloadRebalancerStatus {
	observedWorkloads := make([]appsv1alpha1.ObservedWorkload, 0)

	specWorkloadSet := sets.New[appsv1alpha1.ObjectReference]()
	for _, workload := range rebalancer.Spec.Workloads {
		specWorkloadSet.Insert(workload)
	}

	for _, item := range rebalancer.Status.ObservedWorkloads {
		// if item still exist in `spec`, keep it in `status` and remove it from `specWorkloadSet`.
		// if item no longer exist in `spec`, keep it in `status` if it already success, otherwise remove it from `status`.
		if specWorkloadSet.Has(item.Workload) {
			observedWorkloads = append(observedWorkloads, item)
			specWorkloadSet.Delete(item.Workload)
		} else if item.Result == appsv1alpha1.RebalanceSuccessful {
			observedWorkloads = append(observedWorkloads, item)
		}
	}

	// since item exist in both `spec` and `status` has been removed, the left means the newly added workload,
	// add them into `status`.
	for workload := range specWorkloadSet {
		observedWorkloads = append(observedWorkloads, appsv1alpha1.ObservedWorkload{Workload: workload})
	}

	sort.Slice(observedWorkloads, func(i, j int) bool {
		wi := observedWorkloads[i].Workload
		wj := observedWorkloads[j].Workload
		stri := fmt.Sprintf("%s %s %s %s", wi.APIVersion, wi.Kind, wi.Namespace, wi.Name)
		strj := fmt.Sprintf("%s %s %s %s", wj.APIVersion, wj.Kind, wj.Namespace, wj.Name)
		return stri < strj
	})

	return &appsv1alpha1.WorkloadRebalancerStatus{ObservedWorkloads: observedWorkloads, ObservedGeneration: rebalancer.Generation}
}

func (c *RebalancerController) handleWorkloadRebalance(ctx context.Context, rebalancer *appsv1alpha1.WorkloadRebalancer) (*appsv1alpha1.WorkloadRebalancerStatus, error) {
	// 1. get previous status and update basing on it
	newStatus := c.syncWorkloadsFromSpecToStatus(rebalancer)

	// 2. trigger the rescheduling of target workloads listed in newStatus
	newStatus, retryNum := c.triggerReschedule(ctx, rebalancer.ObjectMeta, newStatus)

	// 3. judge whether status changed and update finishTime field if so
	statusChanged := false
	newStatus.FinishTime = rebalancer.Status.FinishTime
	// compare whether newStatus equals to old status except finishTime field.
	if !reflect.DeepEqual(rebalancer.Status, *newStatus) {
		statusChanged = true
		// retryNum is 0 means the latest rebalancer has finished, which should update finishTime.
		if retryNum == 0 {
			finishTime := metav1.Now()
			newStatus.FinishTime = &finishTime
		}
	}

	// 4. update status of WorkloadRebalancer if statusChanged
	if statusChanged {
		if err := c.updateWorkloadRebalancerStatus(ctx, rebalancer, newStatus); err != nil {
			return newStatus, fmt.Errorf("update status failed: %w", err)
		}
	}

	if retryNum > 0 {
		return newStatus, fmt.Errorf("%d resource reschedule triggered failed and need retry", retryNum)
	}
	return newStatus, nil
}

func (c *RebalancerController) triggerReschedule(ctx context.Context, metadata metav1.ObjectMeta, newStatus *appsv1alpha1.WorkloadRebalancerStatus) (
	*appsv1alpha1.WorkloadRebalancerStatus, int64) {
	successNum, retryNum := int64(0), int64(0)
	for i, resource := range newStatus.ObservedWorkloads {
		if resource.Result == appsv1alpha1.RebalanceSuccessful {
			successNum++
			continue
		}
		if resource.Result == appsv1alpha1.RebalanceFailed {
			continue
		}

		bindingName := names.GenerateBindingName(resource.Workload.Kind, resource.Workload.Name)
		// resource with empty namespace represents it is a cluster wide resource.
		if resource.Workload.Namespace != "" {
			binding := &workv1alpha2.ResourceBinding{}
			if err := c.Client.Get(ctx, client.ObjectKey{Namespace: resource.Workload.Namespace, Name: bindingName}, binding); err != nil {
				klog.ErrorS(err, "get binding for resource failed", "resource", resource.Workload)
				c.recordAndCountRebalancerFailed(&newStatus.ObservedWorkloads[i], &retryNum, err)
				continue
			}
			// update spec.rescheduleTriggeredAt of referenced fetchTargetRefBindings to trigger a rescheduling
			if c.needTriggerReschedule(metadata.CreationTimestamp, binding.Spec.RescheduleTriggeredAt) {
				binding.Spec.RescheduleTriggeredAt = &metadata.CreationTimestamp

				if err := c.Client.Update(ctx, binding); err != nil {
					klog.ErrorS(err, "update binding for resource failed", "resource", resource.Workload)
					c.recordAndCountRebalancerFailed(&newStatus.ObservedWorkloads[i], &retryNum, err)
					continue
				}
			}
			c.recordAndCountRebalancerSuccess(&newStatus.ObservedWorkloads[i], &successNum)
		} else {
			clusterbinding := &workv1alpha2.ClusterResourceBinding{}
			if err := c.Client.Get(ctx, client.ObjectKey{Name: bindingName}, clusterbinding); err != nil {
				klog.ErrorS(err, "get cluster binding for resource failed", "resource", resource.Workload)
				c.recordAndCountRebalancerFailed(&newStatus.ObservedWorkloads[i], &retryNum, err)
				continue
			}
			// update spec.rescheduleTriggeredAt of referenced clusterbinding to trigger a rescheduling
			if c.needTriggerReschedule(metadata.CreationTimestamp, clusterbinding.Spec.RescheduleTriggeredAt) {
				clusterbinding.Spec.RescheduleTriggeredAt = &metadata.CreationTimestamp

				if err := c.Client.Update(ctx, clusterbinding); err != nil {
					klog.ErrorS(err, "update cluster binding for resource failed", "resource", resource.Workload)
					c.recordAndCountRebalancerFailed(&newStatus.ObservedWorkloads[i], &retryNum, err)
					continue
				}
			}
			c.recordAndCountRebalancerSuccess(&newStatus.ObservedWorkloads[i], &successNum)
		}
	}

	klog.V(4).InfoS(fmt.Sprintf("Finish handling WorkloadRebalancer, %d/%d resource success in all, while %d resource need retry",
		successNum, len(newStatus.ObservedWorkloads), retryNum), "workloadRebalancer", metadata.Name, "successNum", successNum, "totalNum", len(newStatus.ObservedWorkloads), "retryNum", retryNum)
	return newStatus, retryNum
}

func (c *RebalancerController) needTriggerReschedule(creationTimestamp metav1.Time, rescheduleTriggeredAt *metav1.Time) bool {
	return rescheduleTriggeredAt == nil || creationTimestamp.After(rescheduleTriggeredAt.Time)
}

func (c *RebalancerController) recordAndCountRebalancerSuccess(resource *appsv1alpha1.ObservedWorkload, successNum *int64) {
	resource.Result = appsv1alpha1.RebalanceSuccessful
	*successNum++
}

func (c *RebalancerController) recordAndCountRebalancerFailed(resource *appsv1alpha1.ObservedWorkload, retryNum *int64, err error) {
	reason := apierrors.ReasonForError(err)
	if reason == metav1.StatusReasonNotFound {
		resource.Result = appsv1alpha1.RebalanceFailed
		resource.Reason = appsv1alpha1.RebalanceObjectNotFound
	} else {
		*retryNum++
	}
}

func (c *RebalancerController) updateWorkloadRebalancerStatus(ctx context.Context, rebalancer *appsv1alpha1.WorkloadRebalancer,
	newStatus *appsv1alpha1.WorkloadRebalancerStatus) error {
	modifiedRebalancer := rebalancer.DeepCopy()
	modifiedRebalancer.Status = *newStatus

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		klog.V(4).InfoS("Start to patch WorkloadRebalancer status", "workloadRebalancer", rebalancer.Name)
		if err = c.Client.Status().Patch(ctx, modifiedRebalancer, client.MergeFrom(rebalancer)); err != nil {
			klog.ErrorS(err, "Failed to patch WorkloadRebalancer status", "workloadRebalancer", rebalancer.Name)
			return err
		}
		klog.V(4).InfoS("Patch WorkloadRebalancer successful", "workloadRebalancer", rebalancer.Name)
		return nil
	})
}

func (c *RebalancerController) deleteWorkloadRebalancer(ctx context.Context, rebalancer *appsv1alpha1.WorkloadRebalancer) error {
	klog.V(4).InfoS("Start to clean up WorkloadRebalancer", "workloadRebalancer", rebalancer.Name)

	options := &client.DeleteOptions{Preconditions: &metav1.Preconditions{ResourceVersion: &rebalancer.ResourceVersion}}
	if err := c.Client.Delete(ctx, rebalancer, options); err != nil {
		klog.ErrorS(err, "Cleaning up WorkloadRebalancer failed", "workloadRebalancer", rebalancer.Name)
		return err
	}

	klog.V(4).InfoS("Cleaning up WorkloadRebalancer successful", "workloadRebalancer", rebalancer.Name)
	return nil
}

func timeLeft(r *appsv1alpha1.WorkloadRebalancer) time.Duration {
	expireAt := r.Status.FinishTime.Add(time.Duration(*r.Spec.TTLSecondsAfterFinished) * time.Second)
	remainingTTL := time.Until(expireAt)

	klog.V(4).InfoS("Check remaining TTL", "workloadRebalancer", r.Name, "FinishTime", r.Status.FinishTime.UTC(), "remainingTTL", remainingTTL)
	return remainingTTL
}
