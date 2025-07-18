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

package execution

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/detector"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
)

const (
	// ControllerName is the controller name that will be used when reporting events and metrics.
	ControllerName = "execution-controller"
	// WorkSuspendDispatchingConditionMessage is the condition and event message when dispatching is suspended.
	WorkSuspendDispatchingConditionMessage = "Work dispatching is in a suspended state."
	// WorkDispatchingConditionMessage is the condition and event message when dispatching is not suspended.
	WorkDispatchingConditionMessage = "Work is being dispatched to member clusters."
	// workSuspendDispatchingConditionReason is the reason for the WorkDispatching condition when dispatching is suspended.
	workSuspendDispatchingConditionReason = "SuspendDispatching"
	// workDispatchingConditionReason is the reason for the WorkDispatching condition when dispatching is not suspended.
	workDispatchingConditionReason = "Dispatching"
)

// Controller is to sync Work.
type Controller struct {
	client.Client      // used to operate Work resources.
	EventRecorder      record.EventRecorder
	RESTMapper         meta.RESTMapper
	ObjectWatcher      objectwatcher.ObjectWatcher
	WorkPredicateFunc  predicate.Predicate
	InformerManager    genericmanager.MultiClusterInformerManager
	RateLimiterOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling Work", "work", req.NamespacedName.String())

	work := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, req.NamespacedName, work); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{}, err
	}

	clusterName, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.ErrorS(err, "Failed to get member cluster name for work", "namespace", work.Namespace, "name", work.Name)
		return controllerruntime.Result{}, err
	}

	cluster, err := util.GetCluster(c.Client, clusterName)
	if err != nil {
		klog.ErrorS(err, "Failed to get the given member cluster", "cluster", clusterName)
		return controllerruntime.Result{}, err
	}

	if !work.DeletionTimestamp.IsZero() {
		if err := c.handleWorkDelete(ctx, work, cluster); err != nil {
			return controllerruntime.Result{}, err
		}

		return c.removeFinalizer(ctx, work)
	}

	if err := c.updateWorkDispatchingConditionIfNeeded(ctx, work); err != nil {
		klog.ErrorS(err, "Failed to update work condition type", "type", workv1alpha1.WorkDispatching)
		return controllerruntime.Result{}, err
	}

	if util.IsWorkSuspendDispatching(work) {
		klog.V(4).InfoS("Skip syncing work for cluster as work dispatch is suspended.", "namespace", work.Namespace, "name", work.Name, "cluster", cluster.Name)
		return controllerruntime.Result{}, nil
	}

	if !util.IsClusterReady(&cluster.Status) {
		err := fmt.Errorf("cluster(%s) not ready", cluster.Name)
		klog.ErrorS(err, "Stop syncing the work for the cluster as cluster not ready.", "namespace", work.Namespace, "name", work.Name, "cluster", cluster.Name)
		return controllerruntime.Result{}, err
	}

	return c.syncWork(ctx, clusterName, work)
}

// SetupWithManager creates a controller and register to controller manager.
func (c *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	ctrlBuilder := controllerruntime.NewControllerManagedBy(mgr).Named(ControllerName).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions),
		})

	if c.WorkPredicateFunc != nil {
		ctrlBuilder.For(&workv1alpha1.Work{}, builder.WithPredicates(c.WorkPredicateFunc))
	} else {
		ctrlBuilder.For(&workv1alpha1.Work{})
	}

	return ctrlBuilder.Complete(c)
}

func (c *Controller) syncWork(ctx context.Context, clusterName string, work *workv1alpha1.Work) (controllerruntime.Result, error) {
	start := time.Now()
	err := c.syncToClusters(ctx, clusterName, work)
	metrics.ObserveSyncWorkloadLatency(err, start)
	if err != nil {
		msg := fmt.Sprintf("Failed to sync work(%s/%s) to cluster(%s), err: %v", work.Namespace, work.Name, clusterName, err)
		klog.Error(msg)
		c.EventRecorder.Event(work, corev1.EventTypeWarning, events.EventReasonSyncWorkloadFailed, msg)
		return controllerruntime.Result{}, err
	}
	msg := fmt.Sprintf("Sync work(%s/%s) to cluster(%s) successful.", work.Namespace, work.Name, clusterName)
	klog.V(4).Info(msg)
	c.EventRecorder.Event(work, corev1.EventTypeNormal, events.EventReasonSyncWorkloadSucceed, msg)
	return controllerruntime.Result{}, nil
}

func (c *Controller) handleWorkDelete(ctx context.Context, work *workv1alpha1.Work, cluster *clusterv1alpha1.Cluster) error {
	if ptr.Deref(work.Spec.PreserveResourcesOnDeletion, false) {
		if err := c.cleanupPolicyClaimMetadata(ctx, work, cluster); err != nil {
			klog.ErrorS(err, "Failed to remove annotations and labels", "cluster", cluster.Name)
			return err
		}
		klog.V(4).InfoS("Preserving resource on deletion from work on cluster", "namespace", work.Namespace, "name", work.Name, "cluster", cluster.Name)
		return nil
	}

	// Abort deleting workload if cluster is unready when unjoining cluster, otherwise the unjoin process will be failed.
	if util.IsClusterReady(&cluster.Status) {
		err := c.tryDeleteWorkload(ctx, cluster.Name, work)
		if err != nil {
			klog.ErrorS(err, "Failed to delete work", "name", work.Name, "namespace", work.Namespace)
			return err
		}
	} else if cluster.DeletionTimestamp.IsZero() { // cluster is unready, but not terminating
		return fmt.Errorf("cluster(%s) not ready", cluster.Name)
	}

	return nil
}

func (c *Controller) cleanupPolicyClaimMetadata(ctx context.Context, work *workv1alpha1.Work, cluster *clusterv1alpha1.Cluster) error {
	for _, manifest := range work.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		if err := workload.UnmarshalJSON(manifest.Raw); err != nil {
			klog.ErrorS(err, "Failed to unmarshal workload from work", "namespace", work.GetNamespace(), "name", work.GetName())
			return err
		}

		fedKey, err := keys.FederatedKeyFunc(cluster.Name, workload)
		if err != nil {
			klog.ErrorS(err, "Failed to get the federated key resource from member cluster", "kind", workload.GetKind(), "namespace", workload.GetNamespace(), "name", workload.GetName(), "cluster", cluster.Name)
			return err
		}

		clusterObj, err := helper.GetObjectFromCache(c.RESTMapper, c.InformerManager, fedKey)
		if err != nil {
			klog.ErrorS(err, "Failed to get the resource from member cluster cache", "kind", workload.GetKind(), "namespace", workload.GetNamespace(), "name", workload.GetName(), "cluster", cluster.Name)
			return err
		}

		if workload.GetNamespace() == corev1.NamespaceAll {
			detector.CleanupCPPClaimMetadata(workload)
		} else {
			detector.CleanupPPClaimMetadata(workload)
		}
		util.RemoveLabels(workload, util.ManagedResourceLabels...)
		util.RemoveAnnotations(workload, util.ManagedResourceAnnotations...)

		operationResult, err := c.ObjectWatcher.Update(ctx, cluster.Name, workload, clusterObj)
		metrics.CountUpdateResourceToCluster(err, workload.GetAPIVersion(), workload.GetKind(), cluster.Name, string(operationResult))
		if err != nil {
			klog.ErrorS(err, "Failed to update metadata in the given member cluster", "cluster", cluster.Name)
			return err
		}
	}

	return nil
}

// tryDeleteWorkload tries to delete resources in the given member cluster.
func (c *Controller) tryDeleteWorkload(ctx context.Context, clusterName string, work *workv1alpha1.Work) error {
	for _, manifest := range work.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.ErrorS(err, "Failed to unmarshal workload")
			return err
		}

		err = c.ObjectWatcher.Delete(ctx, clusterName, workload)
		metrics.CountDeleteResourceFromCluster(err, workload.GetAPIVersion(), workload.GetKind(), clusterName)
		if err != nil {
			klog.ErrorS(err, "Failed to delete resource in the given member cluster", "cluster", clusterName)
			return err
		}
	}

	return nil
}

// removeFinalizer remove finalizer from the given Work
func (c *Controller) removeFinalizer(ctx context.Context, work *workv1alpha1.Work) (controllerruntime.Result, error) {
	if !controllerutil.ContainsFinalizer(work, util.ExecutionControllerFinalizer) {
		return controllerruntime.Result{}, nil
	}

	controllerutil.RemoveFinalizer(work, util.ExecutionControllerFinalizer)
	err := c.Client.Update(ctx, work)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	return controllerruntime.Result{}, nil
}

// syncToClusters ensures that the state of the given object is synchronized to member clusters.
func (c *Controller) syncToClusters(ctx context.Context, clusterName string, work *workv1alpha1.Work) error {
	var errs []error
	syncSucceedNum := 0
	for _, manifest := range work.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.ErrorS(err, "Failed to unmarshal workload of the work", "namespace", work.GetNamespace(), "name", work.GetName())
			errs = append(errs, err)
			continue
		}

		if err = c.tryCreateOrUpdateWorkload(ctx, clusterName, workload); err != nil {
			klog.ErrorS(err, "Failed to create or update resource in the given member cluster", "namespace", workload.GetNamespace(), "name", workload.GetName(), "cluster", clusterName)
			c.eventf(workload, corev1.EventTypeWarning, events.EventReasonSyncWorkloadFailed, "Failed to create or update resource(%s) in member cluster(%s): %v", klog.KObj(workload), clusterName, err)
			errs = append(errs, err)
			continue
		}
		c.eventf(workload, corev1.EventTypeNormal, events.EventReasonSyncWorkloadSucceed, "Successfully applied resource(%v/%v) to cluster %s", workload.GetNamespace(), workload.GetName(), clusterName)
		syncSucceedNum++
	}

	if len(errs) > 0 {
		total := len(work.Spec.Workload.Manifests)
		message := fmt.Sprintf("Failed to apply all manifests (%d/%d): %s", syncSucceedNum, total, errors.NewAggregate(errs).Error())
		err := c.updateAppliedCondition(ctx, work, metav1.ConditionFalse, "AppliedFailed", message)
		if err != nil {
			klog.ErrorS(err, "Failed to update applied status for given work", "name", work.Name, "namespace", work.Namespace)
			errs = append(errs, err)
		}
		return errors.NewAggregate(errs)
	}

	err := c.updateAppliedCondition(ctx, work, metav1.ConditionTrue, "AppliedSuccessful", "Manifest has been successfully applied")
	if err != nil {
		klog.ErrorS(err, "Failed to update applied status for given work", "name", work.Name, "namespace", work.Namespace)
		return err
	}

	return nil
}

func (c *Controller) tryCreateOrUpdateWorkload(ctx context.Context, clusterName string, workload *unstructured.Unstructured) error {
	fedKey, err := keys.FederatedKeyFunc(clusterName, workload)
	if err != nil {
		klog.ErrorS(err, "Failed to get FederatedKey", "name", workload.GetName())
		return err
	}

	clusterObj, err := helper.GetObjectFromCache(c.RESTMapper, c.InformerManager, fedKey)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.ErrorS(err, "Failed to get the resource from member cluster", "kind", workload.GetKind(), "namespace", workload.GetNamespace(), "name", workload.GetName(), "cluster", clusterName)
			return err
		}
		err = c.ObjectWatcher.Create(ctx, clusterName, workload)
		metrics.CountCreateResourceToCluster(err, workload.GetAPIVersion(), workload.GetKind(), clusterName, false)
		if err != nil {
			return err
		}
		return nil
	}

	operationResult, err := c.ObjectWatcher.Update(ctx, clusterName, workload, clusterObj)
	metrics.CountUpdateResourceToCluster(err, workload.GetAPIVersion(), workload.GetKind(), clusterName, string(operationResult))
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) updateWorkDispatchingConditionIfNeeded(ctx context.Context, work *workv1alpha1.Work) error {
	newWorkDispatchingCondition := metav1.Condition{
		Type:               workv1alpha1.WorkDispatching,
		LastTransitionTime: metav1.Now(),
	}

	if util.IsWorkSuspendDispatching(work) {
		newWorkDispatchingCondition.Status = metav1.ConditionFalse
		newWorkDispatchingCondition.Reason = workSuspendDispatchingConditionReason
		newWorkDispatchingCondition.Message = WorkSuspendDispatchingConditionMessage
	} else {
		newWorkDispatchingCondition.Status = metav1.ConditionTrue
		newWorkDispatchingCondition.Reason = workDispatchingConditionReason
		newWorkDispatchingCondition.Message = WorkDispatchingConditionMessage
	}

	if meta.IsStatusConditionPresentAndEqual(work.Status.Conditions, newWorkDispatchingCondition.Type, newWorkDispatchingCondition.Status) {
		return nil
	}

	if err := c.setStatusCondition(ctx, work, newWorkDispatchingCondition); err != nil {
		return err
	}

	obj, err := helper.ToUnstructured(work)
	if err != nil {
		return err
	}

	c.eventf(obj, corev1.EventTypeNormal, events.EventReasonWorkDispatching, newWorkDispatchingCondition.Message)
	return nil
}

// updateAppliedCondition updates the applied condition for the given Work.
func (c *Controller) updateAppliedCondition(ctx context.Context, work *workv1alpha1.Work, status metav1.ConditionStatus, reason, message string) error {
	newWorkAppliedCondition := metav1.Condition{
		Type:               workv1alpha1.WorkApplied,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	return c.setStatusCondition(ctx, work, newWorkAppliedCondition)
}

func (c *Controller) setStatusCondition(ctx context.Context, work *workv1alpha1.Work, statusCondition metav1.Condition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = helper.UpdateStatus(ctx, c.Client, work, func() error {
			meta.SetStatusCondition(&work.Status.Conditions, statusCondition)
			return nil
		})
		return err
	})
}

func (c *Controller) eventf(object *unstructured.Unstructured, eventType, reason, messageFmt string, args ...interface{}) {
	ref, err := util.GenEventRef(object)
	if err != nil {
		klog.ErrorS(err, "Ignore event as failed to build event reference", "reason", reason, "kind", object.GetKind(), "object", klog.KObj(object))
		return
	}
	c.EventRecorder.Eventf(ref, eventType, reason, messageFmt, args...)
}
