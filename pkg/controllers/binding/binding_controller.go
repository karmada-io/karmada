/*
Copyright 2020 The Karmada Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

// ControllerName is the controller name that will be used when reporting events and metrics.
const ControllerName = "binding-controller"

// ResourceBindingController is to sync ResourceBinding.
type ResourceBindingController struct {
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
func (c *ResourceBindingController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling ResourceBinding", "binding", req.NamespacedName.String())

	binding := &workv1alpha2.ResourceBinding{}
	if err := c.Client.Get(ctx, req.NamespacedName, binding); err != nil {
		// The resource no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{}, err
	}

	if !binding.DeletionTimestamp.IsZero() {
		klog.V(4).InfoS("Begin deleting works owned by ResourceBinding", "binding", req.NamespacedName.String())
		if err := helper.DeleteWorks(ctx, c.Client, req.Namespace, req.Name, binding.Labels[workv1alpha2.ResourceBindingPermanentIDLabel]); err != nil {
			klog.ErrorS(err, "Failed deleting works owned by ResourceBinding", "namespace", binding.GetNamespace(), "binding", binding.GetName())
			return controllerruntime.Result{}, err
		}
		return c.removeFinalizer(ctx, binding)
	}

	return c.syncBinding(ctx, binding)
}

// removeFinalizer removes finalizer from the given ResourceBinding
func (c *ResourceBindingController) removeFinalizer(ctx context.Context, rb *workv1alpha2.ResourceBinding) (controllerruntime.Result, error) {
	if !controllerutil.ContainsFinalizer(rb, util.BindingControllerFinalizer) {
		return controllerruntime.Result{}, nil
	}

	controllerutil.RemoveFinalizer(rb, util.BindingControllerFinalizer)
	err := c.Client.Update(ctx, rb)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	return controllerruntime.Result{}, nil
}

// syncBinding will sync resourceBinding to Works.
func (c *ResourceBindingController) syncBinding(ctx context.Context, binding *workv1alpha2.ResourceBinding) (controllerruntime.Result, error) {
	// Handle sequential rollout if enabled
	if binding.Spec.Placement != nil && binding.Spec.Placement.RolloutStrategy != nil &&
		binding.Spec.Placement.RolloutStrategy.Type == policyv1alpha1.RolloutStrategyTypeSequential {
		result, err := c.handleSequentialRollout(ctx, binding)
		if err != nil {
			return controllerruntime.Result{}, err
		}
		if result.RequeueAfter > 0 {
			return result, nil
		}
	}

	if err := c.removeOrphanWorks(ctx, binding); err != nil {
		return controllerruntime.Result{}, err
	}

	needWaitForCleanup, err := c.checkDirectPurgeOrphanWorks(ctx, binding)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	if needWaitForCleanup {
		msg := fmt.Sprintf("There are works in clusters with PurgeMode 'Directly' not deleted for ResourceBinding(%s/%s), skip syncing works",
			binding.Namespace, binding.Name)
		klog.V(4).InfoS(msg, "namespace", binding.GetNamespace(), "binding", binding.GetName())
		return controllerruntime.Result{RequeueAfter: requeueIntervalForDirectlyPurge}, nil
	}

	workload, err := helper.FetchResourceTemplate(ctx, c.DynamicClient, c.InformerManager, c.RESTMapper, binding.Spec.Resource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// It might happen when the resource template has been removed but the garbage collector hasn't removed
			// the ResourceBinding which dependent on resource template.
			// So, just return without retry(requeue) would save unnecessary loop.
			return controllerruntime.Result{}, nil
		}
		klog.ErrorS(err, "Failed to fetch workload for ResourceBinding", "namespace", binding.GetNamespace(), "binding", binding.GetName())
		return controllerruntime.Result{}, err
	}
	start := time.Now()
	err = ensureWork(ctx, c.Client, c.ResourceInterpreter, workload, c.OverrideManager, binding, apiextensionsv1.NamespaceScoped)
	metrics.ObserveSyncWorkLatency(err, start)
	if err != nil {
		klog.ErrorS(err, "Failed to transform ResourceBinding to works", "namespace", binding.GetNamespace(), "binding", binding.GetName())
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, events.EventReasonSyncWorkFailed, err.Error())
		c.EventRecorder.Event(workload, corev1.EventTypeWarning, events.EventReasonSyncWorkFailed, err.Error())
		return controllerruntime.Result{}, err
	}

	msg := fmt.Sprintf("Sync work of ResourceBinding(%s/%s) successful.", binding.Namespace, binding.Name)
	klog.V(4).InfoS(msg, "namespace", binding.GetNamespace(), "binding", binding.GetName())
	c.EventRecorder.Event(binding, corev1.EventTypeNormal, events.EventReasonSyncWorkSucceed, msg)
	c.EventRecorder.Event(workload, corev1.EventTypeNormal, events.EventReasonSyncWorkSucceed, msg)
	return controllerruntime.Result{}, nil
}

func (c *ResourceBindingController) removeOrphanWorks(ctx context.Context, binding *workv1alpha2.ResourceBinding) error {
	works, err := helper.FindOrphanWorks(ctx, c.Client, binding.Namespace, binding.Name,
		binding.Labels[workv1alpha2.ResourceBindingPermanentIDLabel], helper.ObtainBindingSpecExistingClusters(binding.Spec))
	if err != nil {
		klog.ErrorS(err, "Failed to find orphaned works by ResourceBinding", "namespace", binding.GetNamespace(), "binding", binding.GetName())
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, events.EventReasonCleanupWorkFailed, err.Error())
		return err
	}

	err = helper.RemoveOrphanWorks(ctx, c.Client, works)
	if err != nil {
		klog.ErrorS(err, "Failed to remove orphaned works by ResourceBinding", "namespace", binding.GetNamespace(), "binding", binding.GetName())
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, events.EventReasonCleanupWorkFailed, err.Error())
		return err
	}

	return nil
}

// checkDirectPurgeOrphanWorks checks whether there are orphan works in clusters with PurgeMode 'Directly'.
func (c *ResourceBindingController) checkDirectPurgeOrphanWorks(ctx context.Context, binding *workv1alpha2.ResourceBinding) (bool, error) {
	works, err := helper.FindWorksInClusters(ctx, c.Client, binding.Namespace, binding.Name,
		binding.Labels[workv1alpha2.ResourceBindingPermanentIDLabel], helper.ObtainClustersWithPurgeModeDirectly(binding.Spec))
	if err != nil {
		klog.ErrorS(err, "Failed to find orphaned works in clusters with PurgeMode 'Directly'", "namespace", binding.GetNamespace(), "binding", binding.GetName())
		return false, err
	}

	return len(works) > 0, nil
}

// handleSequentialRollout handles the sequential rollout logic for ResourceBinding.
func (c *ResourceBindingController) handleSequentialRollout(ctx context.Context, binding *workv1alpha2.ResourceBinding) (controllerruntime.Result, error) {
	// Check if rollout is complete
	if binding.Status.RolloutStatus != nil && binding.Status.RolloutStatus.Phase == workv1alpha2.RolloutPhaseCompleted {
		return controllerruntime.Result{}, nil
	}

	// Initialize rollout status if not present
	if binding.Status.RolloutStatus == nil {
		if binding.Annotations == nil {
			binding.Annotations = make(map[string]string)
		}
		binding.Annotations["rollout.karmada.io/stage"] = "0"
		binding.Annotations["rollout.karmada.io/startTime"] = time.Now().Format(time.RFC3339)

		binding.Status.RolloutStatus = &workv1alpha2.RolloutStatus{
			CurrentStage:       0,
			TotalStages:        int32(len(binding.Spec.Placement.RolloutStrategy.Sequential.Order)),
			Phase:              workv1alpha2.RolloutPhaseProgressing,
			Message:            "Rollout started",
			LastTransitionTime: &metav1.Time{Time: time.Now()},
		}

		if err := c.Client.Status().Update(ctx, binding); err != nil {
			return controllerruntime.Result{}, err
		}
		if err := c.Client.Update(ctx, binding); err != nil {
			return controllerruntime.Result{}, err
		}
		return controllerruntime.Result{}, nil
	}

	// Check if current cluster is healthy
	currentStage := binding.Status.RolloutStatus.CurrentStage
	if currentStage >= int32(len(binding.Spec.Placement.RolloutStrategy.Sequential.Order)) {
		// All stages completed
		now := metav1.Now()
		binding.Status.RolloutStatus.Phase = workv1alpha2.RolloutPhaseCompleted
		binding.Status.RolloutStatus.Message = "Rollout completed successfully"
		binding.Status.RolloutStatus.LastTransitionTime = &now
		if err := c.Client.Status().Update(ctx, binding); err != nil {
			return controllerruntime.Result{}, err
		}
		return controllerruntime.Result{}, nil
	}

	currentCluster := binding.Spec.Placement.RolloutStrategy.Sequential.Order[currentStage]
	healthy, err := c.checkClusterHealth(ctx, binding, currentCluster)
	if err != nil {
		return controllerruntime.Result{}, err
	}

	if !healthy {
		// Check timeout
		timeout := binding.Spec.Placement.RolloutStrategy.Sequential.HealthCheck.Timeout.Duration
		startTimeStr, exists := binding.Annotations["rollout.karmada.io/startTime"]
		if !exists {
			return controllerruntime.Result{}, fmt.Errorf("rollout start time not found")
		}
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return controllerruntime.Result{}, err
		}

		if time.Since(startTime) > timeout {
			// Handle timeout based on OnFailure policy
			return c.handleRolloutFailure(ctx, binding, int(currentStage), "timeout")
		}
		// Requeue to check again
		return controllerruntime.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Current cluster is healthy, advance to next stage
	return c.advanceRolloutStage(ctx, binding, int(currentStage))
}

// checkClusterHealth checks if a cluster is healthy based on Work status.
func (c *ResourceBindingController) checkClusterHealth(ctx context.Context, binding *workv1alpha2.ResourceBinding, clusterName string) (bool, error) {
	workNamespace := names.GenerateExecutionSpaceName(clusterName)
	workName := names.GenerateWorkName(binding.Namespace, binding.Name, binding.Labels[workv1alpha2.ResourceBindingPermanentIDLabel])

	work := &workv1alpha1.Work{}
	err := c.Client.Get(ctx, types.NamespacedName{Namespace: workNamespace, Name: workName}, work)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Work not created yet, not healthy
			return false, nil
		}
		return false, err
	}

	// Check health condition
	condition := binding.Spec.Placement.RolloutStrategy.Sequential.HealthCheck.Condition
	for _, cond := range work.Status.Conditions {
		if cond.Type == condition && cond.Status == metav1.ConditionTrue {
			return true, nil
		}
	}

	return false, nil
}

// advanceRolloutStage advances the rollout to the next stage.
func (c *ResourceBindingController) advanceRolloutStage(ctx context.Context, binding *workv1alpha2.ResourceBinding, currentStage int) (controllerruntime.Result, error) {
	nextStage := currentStage + 1
	if binding.Annotations == nil {
		binding.Annotations = make(map[string]string)
	}
	binding.Annotations["rollout.karmada.io/stage"] = fmt.Sprintf("%d", nextStage)

	// Update rollout status
	now := metav1.Now()
	binding.Status.RolloutStatus.CurrentStage = int32(nextStage)
	binding.Status.RolloutStatus.LastTransitionTime = &now

	if nextStage >= len(binding.Spec.Placement.RolloutStrategy.Sequential.Order) {
		binding.Status.RolloutStatus.Phase = workv1alpha2.RolloutPhaseCompleted
		binding.Status.RolloutStatus.Message = "Rollout completed successfully"
	} else {
		binding.Status.RolloutStatus.Message = fmt.Sprintf("Rollout progressing to stage %d", nextStage)
	}

	// Trigger rescheduling by updating RescheduleTriggeredAt
	binding.Spec.RescheduleTriggeredAt = &now

	if err := c.Client.Status().Update(ctx, binding); err != nil {
		return controllerruntime.Result{}, err
	}
	if err := c.Client.Update(ctx, binding); err != nil {
		return controllerruntime.Result{}, err
	}

	return controllerruntime.Result{}, nil
}

// handleRolloutFailure handles rollout failure based on OnFailure policy.
func (c *ResourceBindingController) handleRolloutFailure(ctx context.Context, binding *workv1alpha2.ResourceBinding, stage int, reason string) (controllerruntime.Result, error) {
	onFailure := binding.Spec.Placement.RolloutStrategy.Sequential.OnFailure

	switch onFailure {
	case policyv1alpha1.OnFailurePause:
		return c.markRolloutPaused(ctx, binding, stage, reason)
	case policyv1alpha1.OnFailureRollbackAll:
		// TODO: Implement rollback logic
		return c.markRolloutPaused(ctx, binding, stage, fmt.Sprintf("Rollback not yet implemented: %s", reason))
	case policyv1alpha1.OnFailureContinue:
		return c.advanceRolloutStage(ctx, binding, stage)
	default:
		return c.markRolloutPaused(ctx, binding, stage, reason)
	}
}

// markRolloutPaused marks the rollout as paused.
func (c *ResourceBindingController) markRolloutPaused(ctx context.Context, binding *workv1alpha2.ResourceBinding, stage int, reason string) (controllerruntime.Result, error) {
	now := metav1.Now()
	binding.Status.RolloutStatus.Phase = workv1alpha2.RolloutPhasePaused
	binding.Status.RolloutStatus.Message = fmt.Sprintf("Rollout paused at stage %d: %s", stage, reason)
	binding.Status.RolloutStatus.LastTransitionTime = &now

	if err := c.Client.Status().Update(ctx, binding); err != nil {
		return controllerruntime.Result{}, err
	}

	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ResourceBindingController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&workv1alpha2.ResourceBinding{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Watches(&policyv1alpha1.OverridePolicy{}, handler.EnqueueRequestsFromMapFunc(c.newOverridePolicyFunc())).
		Watches(&policyv1alpha1.ClusterOverridePolicy{}, handler.EnqueueRequestsFromMapFunc(c.newOverridePolicyFunc())).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		Complete(c)
}

func (c *ResourceBindingController) newOverridePolicyFunc() handler.MapFunc {
	return func(ctx context.Context, a client.Object) []reconcile.Request {
		var overrideRS []policyv1alpha1.ResourceSelector
		var namespace string
		switch t := a.(type) {
		case *policyv1alpha1.ClusterOverridePolicy:
			overrideRS = t.Spec.ResourceSelectors
		case *policyv1alpha1.OverridePolicy:
			overrideRS = t.Spec.ResourceSelectors
			namespace = t.Namespace
		default:
			return nil
		}

		readonlyBindingList := &workv1alpha2.ResourceBindingList{}
		listOption := &client.ListOptions{
			UnsafeDisableDeepCopy: ptr.To(true),
		}
		if len(namespace) > 0 {
			listOption = &client.ListOptions{
				Namespace:             namespace,
				UnsafeDisableDeepCopy: ptr.To(true),
			}
		}
		if err := c.Client.List(ctx, readonlyBindingList, listOption); err != nil {
			klog.ErrorS(err, "Failed to list ResourceBindings for policy", "namespace", a.GetNamespace(), "name", a.GetName())
			return nil
		}

		var requests []reconcile.Request
		for _, binding := range readonlyBindingList.Items {
			// Nil resourceSelectors means matching all resources.
			if len(overrideRS) == 0 {
				klog.V(2).InfoS("Enqueue ResourceBinding as override policy changes", "namespace", binding.Namespace,
					"binding", binding.Name, "policyNamespace", a.GetNamespace(), "policy", a.GetName())
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: binding.Namespace, Name: binding.Name}})
				continue
			}

			workload, err := helper.FetchResourceTemplate(ctx, c.DynamicClient, c.InformerManager, c.RESTMapper, binding.Spec.Resource)
			if err != nil {
				// If we cannot fetch resource template from binding, this may be due to the fact that the resource template has been deleted.
				// Just skip it so that it will not affect other bindings.
				klog.ErrorS(err, "Failed to fetch workload for ResourceBinding", "namespace", binding.Namespace, "binding", binding.Name)
				continue
			}

			for _, rs := range overrideRS {
				if util.ResourceMatches(workload, rs) {
					klog.V(2).InfoS("Enqueue ResourceBinding as override policy changes", "namespace", binding.Namespace,
						"binding", binding.Name, "policyNamespace", a.GetNamespace(), "policy", a.GetName())
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: binding.Namespace, Name: binding.Name}})
					break
				}
			}
		}
		return requests
	}
}
