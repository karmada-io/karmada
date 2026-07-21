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

package status

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// WorkStatusControllerName is the controller name that will be used when reporting events and metrics.
const WorkStatusControllerName = "work-status-controller"

const memberInformerNotSyncedRequeueAfter = time.Minute

// WorkStatusController is to sync status of Work.
type WorkStatusController struct {
	client.Client    // used to operate Work resources.
	EventRecorder    record.EventRecorder
	RESTMapper       meta.RESTMapper
	InformerManager  genericmanager.MultiClusterInformerManager
	eventHandlerOnce sync.Once
	eventHandler     cache.ResourceEventHandler // eventHandler knows how to handle events from the member cluster.
	Context          context.Context
	worker           util.AsyncPriorityWorker // worker process resources periodic from rateLimitingQueue.
	// ConcurrentWorkStatusSyncs is the number of Work status that are allowed to sync concurrently.
	ConcurrentWorkStatusSyncs   int
	WorkPredicateFunc           predicate.Predicate
	ClusterDynamicClientSetFunc util.NewClusterDynamicClientSetFunc
	ClusterClientOption         *util.ClientOption
	RateLimiterOptions          ratelimiterflag.Options
	ResourceInterpreter         resourceinterpreter.ResourceInterpreter
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *WorkStatusController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling status of Work.", "namespace", req.Namespace, "name", req.Name)

	work := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, req.NamespacedName, work); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{}, err
	}

	if !work.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	if !helper.IsResourceApplied(&work.Status) {
		return controllerruntime.Result{}, nil
	}

	clusterName, err := names.GetClusterName(work.GetNamespace())
	if err != nil {
		klog.ErrorS(err, "Failed to get member cluster name from Work.", "namespace", work.GetNamespace())
		return controllerruntime.Result{}, err
	}

	cluster, err := util.GetCluster(c.Client, clusterName)
	if err != nil {
		klog.ErrorS(err, "Failed to get the given member cluster", "cluster", clusterName)
		return controllerruntime.Result{}, err
	}

	if !util.IsClusterReady(&cluster.Status) {
		err := fmt.Errorf("cluster(%s) not ready", cluster.Name)
		klog.ErrorS(err, "Stop syncing the Work to the cluster as not ready.", "namespace", work.Namespace, "name", work.Name, "cluster", cluster.Name)
		return controllerruntime.Result{}, err
	}

	return c.buildResourceInformers(cluster, work)
}

// buildResourceInformers builds informer dynamically for managed resources in member cluster.
// The created informer watches resource change and then sync to the relevant Work object.
func (c *WorkStatusController) buildResourceInformers(cluster *clusterv1alpha1.Cluster, work *workv1alpha1.Work) (controllerruntime.Result, error) {
	gvrTargets, err := util.GetGVRsFromWork(c.RESTMapper, work)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	informersSynced, err := helper.EnsureInformerHandlersReady(cluster, gvrTargets, c.getEventHandler(), c.InformerManager, c.ClusterDynamicClientSetFunc, c.Client, c.ClusterClientOption)
	if err != nil {
		klog.ErrorS(err, "Failed to register informer for Work.", "namespace", work.GetNamespace(), "name", work.GetName())
		return controllerruntime.Result{}, err
	}
	if !informersSynced {
		klog.V(4).InfoS("Member cluster informers are not synced yet.", "namespace", work.GetNamespace(), "name", work.GetName(), "cluster", cluster.Name)
		return controllerruntime.Result{RequeueAfter: memberInformerNotSyncedRequeueAfter}, nil
	}
	return controllerruntime.Result{}, nil
}

// getEventHandler return callback function that knows how to handle events from the member cluster.
func (c *WorkStatusController) getEventHandler() cache.ResourceEventHandler {
	c.eventHandlerOnce.Do(func() {
		if c.eventHandler == nil {
			c.eventHandler = fedinformer.NewHandlerOnEvents(c.onAdd, c.onUpdate, c.onDelete)
		}
	})
	return c.eventHandler
}

// RunWorkQueue initializes worker and run it, worker will process resource asynchronously.
func (c *WorkStatusController) RunWorkQueue() {
	workerOptions := util.Options{
		Name:             "work-status",
		KeyFunc:          generateKey,
		ReconcileFunc:    c.syncWorkStatus,
		UsePriorityQueue: features.FeatureGate.Enabled(features.ControllerPriorityQueue),
	}
	c.worker = util.NewAsyncWorker(workerOptions)
	c.worker.Run(c.Context, c.ConcurrentWorkStatusSyncs)
}

// generateKey generates a key from obj, the key contains cluster, GVK, namespace and name.
func generateKey(obj any) (util.QueueKey, error) {
	resource := obj.(*unstructured.Unstructured)
	cluster, err := getClusterNameFromAnnotation(resource)
	if err != nil {
		return nil, err
	}
	// return a nil key when the obj not managed by Karmada, which will be discarded before putting to queue.
	if cluster == "" {
		return nil, nil
	}

	return keys.FederatedKeyFunc(cluster, obj)
}

// getClusterNameFromAnnotation gets cluster name from ownerLabel, if label not exist, means resource is not created by karmada.
func getClusterNameFromAnnotation(resource *unstructured.Unstructured) (string, error) {
	workNamespace, exist := resource.GetAnnotations()[workv1alpha2.WorkNamespaceAnnotation]
	if !exist {
		klog.V(5).InfoS("Ignore resource which is not managed by Karmada.", "kind", resource.GetKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())
		return "", nil
	}

	cluster, err := names.GetClusterName(workNamespace)
	if err != nil {
		klog.ErrorS(err, "Failed to get cluster name from Work.", "namespace", workNamespace)
		return "", err
	}
	return cluster, nil
}

// syncWorkStatus will collect status of object referencing by key and update to work which holds the object.
func (c *WorkStatusController) syncWorkStatus(key util.QueueKey) error {
	ctx := context.Background()
	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		err := fmt.Errorf("invalid key")
		klog.ErrorS(err, "Failed to sync status", "key", key)
		return err
	}

	observedObj, err := helper.GetObjectFromCache(c.RESTMapper, c.InformerManager, fedKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	observedAnnotations := observedObj.GetAnnotations()
	workNamespace, nsExist := observedAnnotations[workv1alpha2.WorkNamespaceAnnotation]
	workName, nameExist := observedAnnotations[workv1alpha2.WorkNameAnnotation]
	if !nsExist || !nameExist {
		klog.InfoS("Ignoring object which is not managed by Karmada.", "object", fedKey.String())
		return nil
	}

	workObject := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: workNamespace, Name: workName}, workObject); err != nil {
		// Stop processing if the resource no longer exists.
		if apierrors.IsNotFound(err) {
			return nil
		}

		klog.ErrorS(err, "Failed to get Work from cache", "namespace", workNamespace, "name", workName)
		return err
	}

	// stop updating status if Work object in terminating state.
	if !workObject.DeletionTimestamp.IsZero() {
		return nil
	}

	klog.InfoS("Reflecting resource status to Work.", "kind", observedObj.GetKind(), "resource", observedObj.GetNamespace()+"/"+observedObj.GetName(), "namespace", workNamespace, "name", workName)
	return c.reflectStatus(ctx, workObject, observedObj)
}

// reflectStatus grabs cluster object's running status then updates to its owner object(Work).
func (c *WorkStatusController) reflectStatus(ctx context.Context, work *workv1alpha1.Work, clusterObj *unstructured.Unstructured) error {
	statusRaw, err := c.ResourceInterpreter.ReflectStatus(clusterObj)
	if err != nil {
		klog.ErrorS(err, "Failed to reflect status for object with resourceInterpreter", "kind", clusterObj.GetKind(), "resource", clusterObj.GetNamespace()+"/"+clusterObj.GetName())
		c.EventRecorder.Eventf(work, corev1.EventTypeWarning, events.EventReasonReflectStatusFailed, "Reflect status for object(%s/%s/%s) failed, err: %s.", clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName(), err.Error())
		return err
	}
	c.EventRecorder.Eventf(work, corev1.EventTypeNormal, events.EventReasonReflectStatusSucceed, "Reflect status for object(%s/%s/%s) succeed.", clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName())

	resourceHealth := c.interpretHealth(clusterObj, work)

	identifier, err := c.buildStatusIdentifier(work, clusterObj)
	if err != nil {
		return err
	}

	manifestStatus := workv1alpha1.ManifestStatus{
		Identifier: *identifier,
		Status:     statusRaw,
		Health:     resourceHealth,
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = helper.UpdateStatus(ctx, c.Client, work, func() error {
			work.Status.ManifestStatuses = c.mergeStatus(work.Status.ManifestStatuses, manifestStatus)
			return nil
		})
		return err
	})
}

func (c *WorkStatusController) interpretHealth(clusterObj *unstructured.Unstructured, work *workv1alpha1.Work) workv1alpha1.ResourceHealth {
	// For kind that doesn't have health check, we treat it as healthy.
	if !c.ResourceInterpreter.HookEnabled(clusterObj.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretHealth) {
		klog.V(5).InfoS("skipping health assessment for object as customization missing; will treat it as healthy.", "kind", clusterObj.GroupVersionKind(), "resource", clusterObj.GetNamespace()+"/"+clusterObj.GetName())
		return workv1alpha1.ResourceHealthy
	}

	var resourceHealth workv1alpha1.ResourceHealth
	healthy, err := c.ResourceInterpreter.InterpretHealth(clusterObj)
	if err != nil {
		resourceHealth = workv1alpha1.ResourceUnknown
		c.EventRecorder.Eventf(work, corev1.EventTypeWarning, events.EventReasonInterpretHealthFailed, "Interpret health of object(%s/%s/%s) failed, err: %s.", clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName(), err.Error())
	} else if healthy {
		resourceHealth = workv1alpha1.ResourceHealthy
		c.EventRecorder.Eventf(work, corev1.EventTypeNormal, events.EventReasonInterpretHealthSucceed, "Interpret health of object(%s/%s/%s) as healthy.", clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName())
	} else {
		resourceHealth = workv1alpha1.ResourceUnhealthy
		c.EventRecorder.Eventf(work, corev1.EventTypeNormal, events.EventReasonInterpretHealthSucceed, "Interpret health of object(%s/%s/%s) as unhealthy.", clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName())
	}
	return resourceHealth
}

func (c *WorkStatusController) buildStatusIdentifier(work *workv1alpha1.Work, clusterObj *unstructured.Unstructured) (*workv1alpha1.ResourceIdentifier, error) {
	manifestRef := helper.ManifestReference{APIVersion: clusterObj.GetAPIVersion(), Kind: clusterObj.GetKind(),
		Namespace: clusterObj.GetNamespace(), Name: clusterObj.GetName()}
	ordinal, err := helper.GetManifestIndex(work.Spec.Workload.Manifests, &manifestRef)
	if err != nil {
		return nil, err
	}

	groupVersion, err := schema.ParseGroupVersion(clusterObj.GetAPIVersion())
	if err != nil {
		return nil, err
	}

	identifier := &workv1alpha1.ResourceIdentifier{
		Ordinal: ordinal,
		// TODO(RainbowMango): Consider merge Group and Version to APIVersion from Work API.
		Group:     groupVersion.Group,
		Version:   groupVersion.Version,
		Kind:      clusterObj.GetKind(),
		Namespace: clusterObj.GetNamespace(),
		Name:      clusterObj.GetName(),
	}

	return identifier, nil
}

func (c *WorkStatusController) mergeStatus(_ []workv1alpha1.ManifestStatus, newStatus workv1alpha1.ManifestStatus) []workv1alpha1.ManifestStatus {
	// TODO(RainbowMango): update 'statuses' if 'newStatus' already exist.
	// For now, we only have at most one manifest in Work, so just override current 'statuses'.
	return []workv1alpha1.ManifestStatus{newStatus}
}

// SetupWithManager creates a controller and register to controller manager.
func (c *WorkStatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	ctrlBuilder := controllerruntime.NewControllerManagedBy(mgr).Named(WorkStatusControllerName).
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

func (c *WorkStatusController) onAdd(obj any, isInInitialList bool) {
	curObj := obj.(runtime.Object)
	priority := util.ItemPriorityIfInInitialList(isInInitialList)
	c.worker.EnqueueWithOpts(util.AddOpts{Priority: priority}, curObj)
}

func (c *WorkStatusController) onUpdate(old, cur any) {
	curObj := cur.(runtime.Object)
	if !reflect.DeepEqual(old, cur) {
		c.worker.Enqueue(curObj)
	}
}

func (c *WorkStatusController) onDelete(old any) {
	if deleted, ok := old.(cache.DeletedFinalStateUnknown); ok {
		// This object might be stale but ok for our current usage.
		old = deleted.Obj
		if old == nil {
			return
		}
	}
	oldObj := old.(runtime.Object)
	c.worker.Enqueue(oldObj)
}
