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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// WorkStatusControllerName is the controller name that will be used when reporting events and metrics.
const WorkStatusControllerName = "work-status-controller"

// WorkStatusController is to sync status of Work.
type WorkStatusController struct {
	client.Client   // used to operate Work resources.
	EventRecorder   record.EventRecorder
	RESTMapper      meta.RESTMapper
	InformerManager genericmanager.MultiClusterInformerManager
	eventHandler    cache.ResourceEventHandler // eventHandler knows how to handle events from the member cluster.
	StopChan        <-chan struct{}
	worker          util.AsyncWorker // worker process resources periodic from rateLimitingQueue.
	// ConcurrentWorkStatusSyncs is the number of Work status that are allowed to sync concurrently.
	ConcurrentWorkStatusSyncs   int
	ObjectWatcher               objectwatcher.ObjectWatcher
	PredicateFunc               predicate.Predicate
	ClusterDynamicClientSetFunc func(clusterName string, client client.Client) (*util.DynamicClusterClient, error)
	ClusterCacheSyncTimeout     metav1.Duration
	RateLimiterOptions          ratelimiterflag.Options
	ResourceInterpreter         resourceinterpreter.ResourceInterpreter
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *WorkStatusController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling status of Work %s.", req.NamespacedName.String())

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
		klog.Errorf("Failed to get member cluster name by %s. Error: %v.", work.GetNamespace(), err)
		return controllerruntime.Result{}, err
	}

	cluster, err := util.GetCluster(c.Client, clusterName)
	if err != nil {
		klog.Errorf("Failed to the get given member cluster %s", clusterName)
		return controllerruntime.Result{}, err
	}

	if !util.IsClusterReady(&cluster.Status) {
		klog.Errorf("Stop syncing the Work(%s/%s) to the cluster(%s) as not ready.", work.Namespace, work.Name, cluster.Name)
		return controllerruntime.Result{}, fmt.Errorf("cluster(%s) not ready", cluster.Name)
	}

	return c.buildResourceInformers(cluster, work)
}

// buildResourceInformers builds informer dynamically for managed resources in member cluster.
// The created informer watches resource change and then sync to the relevant Work object.
func (c *WorkStatusController) buildResourceInformers(cluster *clusterv1alpha1.Cluster, work *workv1alpha1.Work) (controllerruntime.Result, error) {
	err := c.registerInformersAndStart(cluster, work)
	if err != nil {
		klog.Errorf("Failed to register informer for Work %s/%s. Error: %v.", work.GetNamespace(), work.GetName(), err)
		return controllerruntime.Result{}, err
	}
	return controllerruntime.Result{}, nil
}

// getEventHandler return callback function that knows how to handle events from the member cluster.
func (c *WorkStatusController) getEventHandler() cache.ResourceEventHandler {
	if c.eventHandler == nil {
		c.eventHandler = fedinformer.NewHandlerOnAllEvents(c.worker.Enqueue)
	}
	return c.eventHandler
}

// RunWorkQueue initializes worker and run it, worker will process resource asynchronously.
func (c *WorkStatusController) RunWorkQueue() {
	workerOptions := util.Options{
		Name:          "work-status",
		KeyFunc:       generateKey,
		ReconcileFunc: c.syncWorkStatus,
	}
	c.worker = util.NewAsyncWorker(workerOptions)
	c.worker.Run(c.ConcurrentWorkStatusSyncs, c.StopChan)
}

// generateKey generates a key from obj, the key contains cluster, GVK, namespace and name.
func generateKey(obj interface{}) (util.QueueKey, error) {
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
		klog.V(5).Infof("Ignore resource(kind=%s, %s/%s) which is not managed by Karmada.", resource.GetKind(), resource.GetNamespace(), resource.GetName())
		return "", nil
	}

	cluster, err := names.GetClusterName(workNamespace)
	if err != nil {
		klog.Errorf("Failed to get cluster name from work namespace: %s, error: %v.", workNamespace, err)
		return "", err
	}
	return cluster, nil
}

// syncWorkStatus will collect status of object referencing by key and update to work which holds the object.
func (c *WorkStatusController) syncWorkStatus(key util.QueueKey) error {
	ctx := context.Background()
	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		klog.Errorf("Failed to sync status as invalid key: %v", key)
		return fmt.Errorf("invalid key")
	}

	observedObj, err := helper.GetObjectFromCache(c.RESTMapper, c.InformerManager, fedKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return c.handleDeleteEvent(ctx, fedKey)
		}
		return err
	}

	observedAnnotations := observedObj.GetAnnotations()
	workNamespace, nsExist := observedAnnotations[workv1alpha2.WorkNamespaceAnnotation]
	workName, nameExist := observedAnnotations[workv1alpha2.WorkNameAnnotation]
	if !nsExist || !nameExist {
		klog.Infof("Ignore object(%s) which not managed by Karmada.", fedKey.String())
		return nil
	}

	workObject := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: workNamespace, Name: workName}, workObject); err != nil {
		// Stop processing if the resource no longer exists.
		if apierrors.IsNotFound(err) {
			return nil
		}

		klog.Errorf("Failed to get Work(%s/%s) from cache: %v", workNamespace, workName, err)
		return err
	}

	// stop updating status if Work object in terminating state.
	if !workObject.DeletionTimestamp.IsZero() {
		return nil
	}

	if err := c.updateResource(ctx, observedObj, workObject, fedKey); err != nil {
		return err
	}

	klog.Infof("Reflecting the resource(kind=%s, %s/%s) status to the Work(%s/%s).", observedObj.GetKind(), observedObj.GetNamespace(), observedObj.GetName(), workNamespace, workName)
	return c.reflectStatus(ctx, workObject, observedObj)
}

func (c *WorkStatusController) updateResource(ctx context.Context, observedObj *unstructured.Unstructured, workObject *workv1alpha1.Work, fedKey keys.FederatedKey) error {
	if helper.IsWorkSuspendDispatching(workObject) {
		return nil
	}

	desiredObj, err := c.getRawManifest(workObject.Spec.Workload.Manifests, observedObj)
	if err != nil {
		return err
	}

	clusterName, err := names.GetClusterName(workObject.Namespace)
	if err != nil {
		klog.Errorf("Failed to get member cluster name: %v", err)
		return err
	}

	// we should check if the observed status is consistent with the declaration to prevent accidental changes made
	// in member clusters.
	needUpdate, err := c.ObjectWatcher.NeedsUpdate(clusterName, desiredObj, observedObj)
	if err != nil {
		return err
	}

	if needUpdate {
		updateErr := c.ObjectWatcher.Update(ctx, clusterName, desiredObj, observedObj)
		metrics.CountUpdateResourceToCluster(updateErr, desiredObj.GetAPIVersion(), desiredObj.GetKind(), clusterName)
		if updateErr != nil {
			klog.Errorf("Updating %s failed: %v", fedKey.String(), updateErr)
			return updateErr
		}
		// We can't return even after a success updates, because that might lose the chance to collect status.
		// Not all updates are real, they might be no change, in that case there will be no more event for this update,
		// this usually happens with those resources not enables 'metadata.generation', like 'Service'.
		// When a Service's status changes, it's 'metadata.resourceVersion' will be increased, but 'metadata.generation'
		// not increased(defaults to 0), the ObjectWatcher can't easily tell what happened to the object, so ObjectWatcher
		// also needs to update again. The update operation will be a non-operation if the event triggered by Service's
		// status changes.
	}
	return nil
}

func (c *WorkStatusController) handleDeleteEvent(ctx context.Context, key keys.FederatedKey) error {
	executionSpace := names.GenerateExecutionSpaceName(key.Cluster)

	// Given the workload might have been deleted from informer cache, so that we can't get work object by its label,
	// we have to get work by naming rule as the work's name is generated by the workload's kind, name and namespace.
	workName := names.GenerateWorkName(key.Kind, key.Name, key.Namespace)
	work := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: executionSpace, Name: workName}, work); err != nil {
		// stop processing as the work object has been removed, assume it's a normal delete operation.
		if apierrors.IsNotFound(err) {
			return nil
		}

		klog.Errorf("Failed to get Work from cache: %v", err)
		return err
	}

	// stop processing as the work object being deleting.
	if !work.DeletionTimestamp.IsZero() {
		return nil
	}

	if util.GetLabelValue(work.Labels, util.PropagationInstruction) == util.PropagationInstructionSuppressed {
		return nil
	}

	reCreateErr := c.recreateResourceIfNeeded(ctx, work, key)
	metrics.CountRecreateResourceToCluster(reCreateErr, key.GroupVersion().String(), key.Kind, key.Cluster)
	if reCreateErr != nil {
		c.updateAppliedCondition(ctx, work, metav1.ConditionFalse, "ReCreateFailed", reCreateErr.Error())
		return reCreateErr
	}
	c.updateAppliedCondition(ctx, work, metav1.ConditionTrue, "ReCreateSuccessful", "Manifest has been successfully applied")
	return nil
}

func (c *WorkStatusController) recreateResourceIfNeeded(ctx context.Context, work *workv1alpha1.Work, workloadKey keys.FederatedKey) error {
	for _, rawManifest := range work.Spec.Workload.Manifests {
		manifest := &unstructured.Unstructured{}
		if err := manifest.UnmarshalJSON(rawManifest.Raw); err != nil {
			return err
		}

		desiredGVK := schema.FromAPIVersionAndKind(manifest.GetAPIVersion(), manifest.GetKind())
		if reflect.DeepEqual(desiredGVK, workloadKey.GroupVersionKind()) &&
			manifest.GetNamespace() == workloadKey.Namespace &&
			manifest.GetName() == workloadKey.Name {
			klog.Infof("Recreating resource(%s).", workloadKey.String())
			err := c.ObjectWatcher.Create(ctx, workloadKey.Cluster, manifest)
			if err != nil {
				c.eventf(manifest, corev1.EventTypeWarning, events.EventReasonSyncWorkloadFailed, "Failed to create or update resource(%s/%s) in member cluster(%s): %v", manifest.GetNamespace(), manifest.GetName(), workloadKey.Cluster, err)
				return err
			}
			c.eventf(manifest, corev1.EventTypeNormal, events.EventReasonSyncWorkloadSucceed, "Successfully applied resource(%s/%s) to cluster %s", manifest.GetNamespace(), manifest.GetName(), workloadKey.Cluster)
			return nil
		}
	}
	return nil
}

// updateAppliedCondition update the condition for the given Work
func (c *WorkStatusController) updateAppliedCondition(ctx context.Context, work *workv1alpha1.Work, status metav1.ConditionStatus, reason, message string) {
	newWorkAppliedCondition := metav1.Condition{
		Type:               workv1alpha1.WorkApplied,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = helper.UpdateStatus(ctx, c.Client, work, func() error {
			meta.SetStatusCondition(&work.Status.Conditions, newWorkAppliedCondition)
			return nil
		})
		return err
	})

	if err != nil {
		klog.Errorf("Failed to update condition of work %s/%s: %s", work.Namespace, work.Name, err.Error())
	}
}

// reflectStatus grabs cluster object's running status then updates to its owner object(Work).
func (c *WorkStatusController) reflectStatus(ctx context.Context, work *workv1alpha1.Work, clusterObj *unstructured.Unstructured) error {
	statusRaw, err := c.ResourceInterpreter.ReflectStatus(clusterObj)
	if err != nil {
		klog.Errorf("Failed to reflect status for object(%s/%s/%s) with resourceInterpreter, err: %v",
			clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName(), err)
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
		klog.V(5).Infof("skipping health assessment for object: %v %s/%s as missing customization and will treat it as healthy.", clusterObj.GroupVersionKind(), clusterObj.GetNamespace(), clusterObj.GetName())
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

func (c *WorkStatusController) getRawManifest(manifests []workv1alpha1.Manifest, clusterObj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	for _, rawManifest := range manifests {
		manifest := &unstructured.Unstructured{}
		if err := manifest.UnmarshalJSON(rawManifest.Raw); err != nil {
			return nil, err
		}

		if manifest.GetAPIVersion() == clusterObj.GetAPIVersion() &&
			manifest.GetKind() == clusterObj.GetKind() &&
			manifest.GetNamespace() == clusterObj.GetNamespace() &&
			manifest.GetName() == clusterObj.GetName() {
			return manifest, nil
		}
	}

	return nil, fmt.Errorf("no such manifest exist")
}

// registerInformersAndStart builds informer manager for cluster if it doesn't exist, then constructs informers for gvr
// and start it.
func (c *WorkStatusController) registerInformersAndStart(cluster *clusterv1alpha1.Cluster, work *workv1alpha1.Work) error {
	singleClusterInformerManager, err := c.getSingleClusterManager(cluster)
	if err != nil {
		return err
	}

	gvrTargets, err := c.getGVRsFromWork(work)
	if err != nil {
		return err
	}

	allSynced := true
	for gvr := range gvrTargets {
		if !singleClusterInformerManager.IsInformerSynced(gvr) || !singleClusterInformerManager.IsHandlerExist(gvr, c.getEventHandler()) {
			allSynced = false
			singleClusterInformerManager.ForResource(gvr, c.getEventHandler())
		}
	}
	if allSynced {
		return nil
	}

	c.InformerManager.Start(cluster.Name)

	if err := func() error {
		synced := c.InformerManager.WaitForCacheSyncWithTimeout(cluster.Name, c.ClusterCacheSyncTimeout.Duration)
		if synced == nil {
			return fmt.Errorf("no informerFactory for cluster %s exist", cluster.Name)
		}
		for gvr := range gvrTargets {
			if !synced[gvr] {
				return fmt.Errorf("informer for %s hasn't synced", gvr)
			}
		}
		return nil
	}(); err != nil {
		klog.Errorf("Failed to sync cache for cluster: %s, error: %v", cluster.Name, err)
		c.InformerManager.Stop(cluster.Name)
		return err
	}

	return nil
}

// getGVRsFromWork traverses the manifests in work to find groupVersionResource list.
func (c *WorkStatusController) getGVRsFromWork(work *workv1alpha1.Work) (map[schema.GroupVersionResource]bool, error) {
	gvrTargets := map[schema.GroupVersionResource]bool{}
	for _, manifest := range work.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("Failed to unmarshal workload. Error: %v.", err)
			return nil, err
		}
		gvr, err := restmapper.GetGroupVersionResource(c.RESTMapper, workload.GroupVersionKind())
		if err != nil {
			klog.Errorf("Failed to get GVR from GVK for resource %s/%s. Error: %v.", workload.GetNamespace(), workload.GetName(), err)
			return nil, err
		}
		gvrTargets[gvr] = true
	}
	return gvrTargets, nil
}

// getSingleClusterManager gets singleClusterInformerManager with clusterName.
// If manager is not exist, create it, otherwise gets it from map.
func (c *WorkStatusController) getSingleClusterManager(cluster *clusterv1alpha1.Cluster) (genericmanager.SingleClusterInformerManager, error) {
	// TODO(chenxianpao): If cluster A is removed, then a new cluster that name also is A joins karmada,
	//  the cache in informer manager should be updated.
	singleClusterInformerManager := c.InformerManager.GetSingleClusterManager(cluster.Name)
	if singleClusterInformerManager == nil {
		dynamicClusterClient, err := c.ClusterDynamicClientSetFunc(cluster.Name, c.Client)
		if err != nil {
			klog.Errorf("Failed to build dynamic cluster client for cluster %s.", cluster.Name)
			return nil, err
		}
		singleClusterInformerManager = c.InformerManager.ForCluster(dynamicClusterClient.ClusterName, dynamicClusterClient.DynamicClientSet, 0)
	}
	return singleClusterInformerManager, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *WorkStatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(WorkStatusControllerName).
		For(&workv1alpha1.Work{}, builder.WithPredicates(c.PredicateFunc)).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions),
		}).Complete(c)
}

func (c *WorkStatusController) eventf(object *unstructured.Unstructured, eventType, reason, messageFmt string, args ...interface{}) {
	ref, err := helper.GenEventRef(object)
	if err != nil {
		klog.Errorf("Ignore event(%s) as failing to build event reference for: kind=%s, %s due to %v", reason, object.GetKind(), klog.KObj(object), err)
		return
	}
	c.EventRecorder.Eventf(ref, eventType, reason, messageFmt, args...)
}
