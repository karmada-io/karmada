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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
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

	// EnableAsyncWorkCreation enables asynchronous work creation for improved throughput.
	// When enabled, work creation tasks are submitted to an async queue and processed
	// by dedicated workers, similar to kube-scheduler's async binding pattern.
	EnableAsyncWorkCreation bool
	// AsyncWorkCreator handles asynchronous work creation
	AsyncWorkCreator *AsyncWorkCreator
	// RequeueAfterFailure is a channel used to requeue bindings after async work creation failure.
	// When AsyncWorkCreator fails to create a work, it sends the binding key to this channel
	// to trigger a re-reconciliation.
	RequeueAfterFailure chan string
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
//
//nolint:gocyclo
func (c *ResourceBindingController) syncBinding(ctx context.Context, binding *workv1alpha2.ResourceBinding) (controllerruntime.Result, error) {
	bindingKey := fmt.Sprintf("%s/%s", binding.Namespace, binding.Name)

	// Fast path: Check assume cache first - if works are being created asynchronously, skip this reconcile
	if c.EnableAsyncWorkCreation && c.AsyncWorkCreator != nil && c.AsyncWorkCreator.IsAssumed(bindingKey) {
		klog.V(4).InfoS("Skip reconcile, works are being created asynchronously", "binding", bindingKey)
		return controllerruntime.Result{}, nil
	}

	// Compute target clusters hash for async work creation optimization
	currentHash := helper.GetTargetClustersHash(binding.Spec.Clusters, binding.Spec.RequiredBy)
	lastHash := ""
	if binding.Annotations != nil {
		lastHash = binding.Annotations[workv1alpha2.TargetClustersHashAnnotation]
	}

	// Clear assume cache when clusters changed - need full re-sync
	if lastHash != currentHash && c.EnableAsyncWorkCreation && c.AsyncWorkCreator != nil {
		c.AsyncWorkCreator.ForgetBinding(bindingKey)
	}

	// Always check and remove orphan works to ensure correctness
	// This matches the original behavior and prevents edge cases where orphans are missed
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

	// Use async work creation if enabled
	if c.EnableAsyncWorkCreation && c.AsyncWorkCreator != nil {
		return c.syncBindingAsync(ctx, binding, workload, bindingKey, currentHash, lastHash)
	}

	// Fallback to synchronous work creation
	start := time.Now()
	err = ensureWork(ctx, c.Client, c.ResourceInterpreter, workload, c.OverrideManager, binding, apiextensionsv1.NamespaceScoped)
	metrics.ObserveSyncWorkLatency(err, start)
	if err != nil {
		klog.ErrorS(err, "Failed to transform ResourceBinding to works", "namespace", binding.GetNamespace(), "binding", binding.GetName())
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, events.EventReasonSyncWorkFailed, err.Error())
		c.EventRecorder.Event(workload, corev1.EventTypeWarning, events.EventReasonSyncWorkFailed, err.Error())
		return controllerruntime.Result{}, err
	}

	// Update target clusters hash annotation if changed
	if lastHash != currentHash {
		if err := c.updateTargetClustersHash(ctx, binding, currentHash); err != nil {
			klog.V(4).ErrorS(err, "Failed to update target clusters hash annotation", "namespace", binding.GetNamespace(), "binding", binding.GetName())
			// Don't return error, this is just an optimization hint
		}
	}

	msg := fmt.Sprintf("Sync work of ResourceBinding(%s/%s) successful.", binding.Namespace, binding.Name)
	klog.V(4).InfoS(msg, "namespace", binding.GetNamespace(), "binding", binding.GetName())
	c.EventRecorder.Event(binding, corev1.EventTypeNormal, events.EventReasonSyncWorkSucceed, msg)
	c.EventRecorder.Event(workload, corev1.EventTypeNormal, events.EventReasonSyncWorkSucceed, msg)
	return controllerruntime.Result{}, nil
}

// syncBindingAsync handles asynchronous work creation following the kube-scheduler pattern.
// It prepares work tasks and submits them to the async queue, returning immediately without
// waiting for the actual work creation to complete.
func (c *ResourceBindingController) syncBindingAsync(ctx context.Context, binding *workv1alpha2.ResourceBinding,
	workload *unstructured.Unstructured, bindingKey, currentHash, lastHash string) (controllerruntime.Result, error) {
	// Prepare work tasks
	tasks, err := c.prepareWorkTasks(ctx, binding, workload)
	if err != nil {
		klog.ErrorS(err, "Failed to prepare work tasks", "binding", bindingKey)
		c.EventRecorder.Event(binding, corev1.EventTypeWarning, events.EventReasonSyncWorkFailed, err.Error())
		return controllerruntime.Result{}, err
	}

	if len(tasks) == 0 {
		klog.V(4).InfoS("No work tasks to create", "binding", bindingKey)
		return controllerruntime.Result{}, nil
	}

	// Submit tasks to async queue
	if err := c.AsyncWorkCreator.Submit(bindingKey, tasks); err != nil {
		klog.ErrorS(err, "Failed to submit work tasks to async queue", "binding", bindingKey)
		// Queue is full, fall back to sync processing
		return controllerruntime.Result{Requeue: true}, err
	}

	klog.V(4).InfoS("Submitted work tasks to async queue", "binding", bindingKey, "taskCount", len(tasks))

	// Update target clusters hash annotation if changed
	if lastHash != currentHash {
		if err := c.updateTargetClustersHash(ctx, binding, currentHash); err != nil {
			klog.V(4).ErrorS(err, "Failed to update target clusters hash annotation", "binding", bindingKey)
		}
	}

	// Note: Success events will be recorded by AsyncWorkCreator when works are actually created
	return controllerruntime.Result{}, nil
}

// prepareWorkTasks prepares work creation tasks for all target clusters.
// This reuses the existing prepareWorkTask logic from common.go and converts results to WorkTask.
func (c *ResourceBindingController) prepareWorkTasks(_ context.Context, binding *workv1alpha2.ResourceBinding,
	workload *unstructured.Unstructured) ([]*WorkTask, error) {
	bindingSpec := binding.Spec
	targetClusters := mergeTargetClusters(bindingSpec.Clusters, bindingSpec.RequiredBy)

	if len(targetClusters) == 0 {
		return nil, nil
	}

	var jobCompletions []workv1alpha2.TargetCluster
	var err error
	if workload.GetKind() == util.JobKind && needReviseJobCompletions(bindingSpec.Replicas, bindingSpec.Placement) {
		jobCompletions, err = divideReplicasByJobCompletions(workload, targetClusters)
		if err != nil {
			return nil, err
		}
	}

	tasks := make([]*WorkTask, 0, len(targetClusters))
	numClusters := len(targetClusters)

	for i, targetCluster := range targetClusters {
		// Deep copy workload for each cluster
		clonedWorkload := workload.DeepCopy()

		// Use existing prepareWorkTask from common.go
		wt, err := prepareWorkTask(PrepareWorkTaskArgs{
			ResourceInterpreter: c.ResourceInterpreter,
			ClonedWorkload:      clonedWorkload,
			OverrideManager:     c.OverrideManager,
			Binding:             binding,
			Scope:               apiextensionsv1.NamespaceScoped,
			BindingSpec:         bindingSpec,
			TargetCluster:       targetCluster,
			ClusterIndex:        i,
			JobCompletions:      jobCompletions,
			TotalClusters:       numClusters,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to prepare work task for cluster %s: %w", targetCluster.Name, err)
		}

		// Convert workTask to WorkTask for async processing
		tasks = append(tasks, &WorkTask{
			WorkMeta:                    wt.workMeta,
			Workload:                    wt.workload,
			SuspendDispatching:          ptr.To(wt.suspendDisp),
			PreserveResourcesOnDeletion: ptr.To(wt.preserveOnDel),
			Binding:                     binding,
		})
	}

	return tasks, nil
}

// updateTargetClustersHash updates the target clusters hash annotation on the binding.
func (c *ResourceBindingController) updateTargetClustersHash(ctx context.Context, binding *workv1alpha2.ResourceBinding, hash string) error {
	bindingCopy := binding.DeepCopy()
	if bindingCopy.Annotations == nil {
		bindingCopy.Annotations = make(map[string]string)
	}
	bindingCopy.Annotations[workv1alpha2.TargetClustersHashAnnotation] = hash
	return c.Client.Update(ctx, bindingCopy)
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

// SetupWithManager creates a controller and register to controller manager.
func (c *ResourceBindingController) SetupWithManager(mgr controllerruntime.Manager) error {
	builder := controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&workv1alpha2.ResourceBinding{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Watches(&policyv1alpha1.OverridePolicy{}, handler.EnqueueRequestsFromMapFunc(c.newOverridePolicyFunc())).
		Watches(&policyv1alpha1.ClusterOverridePolicy{}, handler.EnqueueRequestsFromMapFunc(c.newOverridePolicyFunc())).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)})

	// Add channel source for requeue after async work creation failure
	if c.EnableAsyncWorkCreation && c.RequeueAfterFailure != nil {
		requeueEventChan := make(chan event.GenericEvent, 1000)
		// Start goroutine to convert string keys to GenericEvents
		go c.runRequeueListener(requeueEventChan)
		builder = builder.WatchesRawSource(source.Channel(requeueEventChan, &handler.EnqueueRequestForObject{}))
	}

	return builder.Complete(c)
}

// runRequeueListener listens on RequeueAfterFailure channel and converts binding keys to GenericEvents
func (c *ResourceBindingController) runRequeueListener(eventChan chan<- event.GenericEvent) {
	for bindingKey := range c.RequeueAfterFailure {
		parts := strings.SplitN(bindingKey, "/", 2)
		if len(parts) != 2 {
			klog.Warningf("Invalid binding key format: %s", bindingKey)
			continue
		}
		namespace, name := parts[0], parts[1]
		klog.V(4).Infof("Requeuing ResourceBinding %s/%s after async work creation failure", namespace, name)

		// Create a minimal ResourceBinding object for the event
		// The controller will fetch the full object during reconciliation
		rb := &workv1alpha2.ResourceBinding{}
		rb.SetNamespace(namespace)
		rb.SetName(name)

		select {
		case eventChan <- event.GenericEvent{Object: rb}:
		default:
			klog.Warningf("Requeue event channel is full, dropping requeue for %s", bindingKey)
		}
	}
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
