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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1alpha1 "github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName = "workload-rebalancer"
)

// RebalancerController is to handle a rebalance to workloads selected by WorkloadRebalancer object.
type RebalancerController struct {
	Client client.Client
}

// SetupWithManager creates a controller and register to controller manager.
func (c *RebalancerController) SetupWithManager(mgr controllerruntime.Manager) error {
	var predicateFunc = predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&appsv1alpha1.WorkloadRebalancer{}, builder.WithPredicates(predicateFunc)).
		Complete(c)
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *RebalancerController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling for WorkloadRebalancer %s", req.Name)

	// 1. get latest WorkloadRebalancer
	rebalancer := &appsv1alpha1.WorkloadRebalancer{}
	if err := c.Client.Get(ctx, req.NamespacedName, rebalancer); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("no need to reconcile WorkloadRebalancer for it not found")
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	// 2. get and update referenced binding to trigger a rescheduling
	newStatus, successNum, retryNum := c.doWorkloadRebalance(ctx, rebalancer)

	// 3. update status of WorkloadRebalancer
	if err := c.updateWorkloadRebalancerStatus(ctx, rebalancer, newStatus); err != nil {
		return controllerruntime.Result{}, err
	}
	klog.Infof("Finish handling WorkloadRebalancer (%s), %d/%d resource success in all, while %d resource need retry",
		rebalancer.Name, successNum, len(newStatus.ObservedWorkloads), retryNum)

	if retryNum > 0 {
		return controllerruntime.Result{}, fmt.Errorf("%d resource reschedule triggered failed and need retry", retryNum)
	}
	return controllerruntime.Result{}, nil
}

func (c *RebalancerController) buildWorkloadRebalancerStatus(rebalancer *appsv1alpha1.WorkloadRebalancer) *appsv1alpha1.WorkloadRebalancerStatus {
	resourceList := make([]appsv1alpha1.ObservedWorkload, 0)
	for _, resource := range rebalancer.Spec.Workloads {
		resourceList = append(resourceList, appsv1alpha1.ObservedWorkload{
			Workload: resource,
		})
	}
	return &appsv1alpha1.WorkloadRebalancerStatus{
		ObservedWorkloads: resourceList,
	}
}

func (c *RebalancerController) doWorkloadRebalance(ctx context.Context, rebalancer *appsv1alpha1.WorkloadRebalancer) (
	newStatus *appsv1alpha1.WorkloadRebalancerStatus, successNum int64, retryNum int64) {
	// get previous status and update basing on it
	newStatus = rebalancer.Status.DeepCopy()
	if len(newStatus.ObservedWorkloads) == 0 {
		newStatus = c.buildWorkloadRebalancerStatus(rebalancer)
	}

	successNum, retryNum = int64(0), int64(0)
	for i, resource := range newStatus.ObservedWorkloads {
		if resource.Result == appsv1alpha1.RebalanceSuccessful {
			successNum++
			continue
		}
		if resource.Result == appsv1alpha1.RebalanceFailed && resource.Reason == appsv1alpha1.RebalanceObjectNotFound {
			continue
		}

		bindingName := names.GenerateBindingName(resource.Workload.Kind, resource.Workload.Name)
		// resource with empty namespace represents it is a cluster wide resource.
		if resource.Workload.Namespace != "" {
			binding := &workv1alpha2.ResourceBinding{}
			if err := c.Client.Get(ctx, client.ObjectKey{Namespace: resource.Workload.Namespace, Name: bindingName}, binding); err != nil {
				klog.Errorf("get binding for resource %+v failed: %+v", resource.Workload, err)
				c.recordAndCountRebalancerFailed(&newStatus.ObservedWorkloads[i], &retryNum, err)
				continue
			}
			// update spec.rescheduleTriggeredAt of referenced fetchTargetRefBindings to trigger a rescheduling
			if c.needTriggerReschedule(rebalancer.CreationTimestamp, binding.Spec.RescheduleTriggeredAt) {
				binding.Spec.RescheduleTriggeredAt = &rebalancer.CreationTimestamp

				if err := c.Client.Update(ctx, binding); err != nil {
					klog.Errorf("update binding for resource %+v failed: %+v", resource.Workload, err)
					c.recordAndCountRebalancerFailed(&newStatus.ObservedWorkloads[i], &retryNum, err)
					continue
				}
			}
			c.recordAndCountRebalancerSuccess(&newStatus.ObservedWorkloads[i], &successNum)
		} else {
			clusterbinding := &workv1alpha2.ClusterResourceBinding{}
			if err := c.Client.Get(ctx, client.ObjectKey{Name: bindingName}, clusterbinding); err != nil {
				klog.Errorf("get cluster binding for resource %+v failed: %+v", resource.Workload, err)
				c.recordAndCountRebalancerFailed(&newStatus.ObservedWorkloads[i], &retryNum, err)
				continue
			}
			// update spec.rescheduleTriggeredAt of referenced clusterbinding to trigger a rescheduling
			if c.needTriggerReschedule(rebalancer.CreationTimestamp, clusterbinding.Spec.RescheduleTriggeredAt) {
				clusterbinding.Spec.RescheduleTriggeredAt = &rebalancer.CreationTimestamp

				if err := c.Client.Update(ctx, clusterbinding); err != nil {
					klog.Errorf("update cluster binding for resource %+v failed: %+v", resource.Workload, err)
					c.recordAndCountRebalancerFailed(&newStatus.ObservedWorkloads[i], &retryNum, err)
					continue
				}
			}
			c.recordAndCountRebalancerSuccess(&newStatus.ObservedWorkloads[i], &successNum)
		}
	}
	return
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
	rebalancerPatch := client.MergeFrom(rebalancer)
	modifiedRebalancer := rebalancer.DeepCopy()
	modifiedRebalancer.Status = *newStatus

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		klog.V(4).Infof("Start to patch WorkloadRebalancer(%s) status", rebalancer.Name)
		if err = c.Client.Status().Patch(ctx, modifiedRebalancer, rebalancerPatch); err != nil {
			klog.Errorf("Failed to patch WorkloadRebalancer (%s) status, err: %+v", rebalancer.Name, err)
			return err
		}
		return nil
	})
}
