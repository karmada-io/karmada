/*
Copyright 2023 The Karmada Authors.

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

package multiclusterservice

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/controllers/ctrlutil"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// EndpointSliceCollectController collects EndpointSlice from member clusters and reports them to control-plane.
type EndpointSliceCollectController struct {
	client.Client
	RESTMapper                  meta.RESTMapper
	Context                     context.Context
	InformerManager             genericmanager.MultiClusterInformerManager
	WorkerNumber                int                 // WorkerNumber is the number of worker goroutines
	PredicateFunc               predicate.Predicate // PredicateFunc is the function that filters events before enqueuing the keys.
	ClusterDynamicClientSetFunc util.NewClusterDynamicClientSetFunc
	ClusterClientOption         *util.ClientOption
	// eventHandlers holds the handlers which used to handle events reported from member clusters.
	// Each handler takes the cluster name as key and takes the handler function as the value, e.g.
	// "member1": instance of ResourceEventHandler
	eventHandlers sync.Map
	worker        util.AsyncWorker // worker process resources periodic from rateLimitingQueue.

	ClusterCacheSyncTimeout metav1.Duration
	RateLimiterOptions      ratelimiterflag.Options
}

var (
	endpointSliceGVR       = discoveryv1.SchemeGroupVersion.WithResource("endpointslices")
	multiClusterServiceGVK = networkingv1alpha1.SchemeGroupVersion.WithKind("MultiClusterService")
)

// EndpointSliceCollectControllerName is the controller name that will be used when reporting events and metrics.
const EndpointSliceCollectControllerName = "endpointslice-collect-controller"

// Reconcile performs a full reconciliation for the object referred to by the Request.
func (c *EndpointSliceCollectController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling Work", "namespace", req.Namespace, "name", req.Name)

	work := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, req.NamespacedName, work); err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	if !util.IsWorkContains(work.Spec.Workload.Manifests, multiClusterServiceGVK) {
		return controllerruntime.Result{}, nil
	}

	if !work.DeletionTimestamp.IsZero() {
		// The Provider Clusters' EndpointSlice will be deleted by multiclusterservice-controller, let's just ignore it
		return controllerruntime.Result{}, nil
	}

	clusterName, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.ErrorS(err, "Failed to get cluster name for work", "namespace", work.Namespace, "name", work.Name)
		return controllerruntime.Result{}, err
	}

	if err = c.buildResourceInformers(clusterName); err != nil {
		return controllerruntime.Result{}, err
	}

	if err = c.collectTargetEndpointSlice(ctx, work, clusterName); err != nil {
		return controllerruntime.Result{}, err
	}

	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *EndpointSliceCollectController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(EndpointSliceCollectControllerName).
		For(&workv1alpha1.Work{}, builder.WithPredicates(c.PredicateFunc)).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		Complete(c)
}

// RunWorkQueue initializes worker and run it, worker will process resource asynchronously.
func (c *EndpointSliceCollectController) RunWorkQueue() {
	workerOptions := util.Options{
		Name:          "endpointslice-collect",
		KeyFunc:       nil,
		ReconcileFunc: c.collectEndpointSlice,
	}
	c.worker = util.NewAsyncWorker(workerOptions)
	c.worker.Run(c.Context, c.WorkerNumber)
}

func (c *EndpointSliceCollectController) collectEndpointSlice(key util.QueueKey) error {
	ctx := context.Background()
	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		var ErrInvalidKey = errors.New("invalid key")
		klog.ErrorS(ErrInvalidKey, "Failed to collect endpointslice as invalid key", "key", key)
		return fmt.Errorf("invalid key")
	}

	klog.V(4).InfoS("Begin to collect", "kind", fedKey.Kind, "namespaceKey", fedKey.NamespaceKey())
	if err := c.handleEndpointSliceEvent(ctx, fedKey); err != nil {
		klog.ErrorS(err, "Failed to handle endpointSlice event", "namespaceKey",
			fedKey.NamespaceKey())
		return err
	}

	return nil
}

func (c *EndpointSliceCollectController) buildResourceInformers(clusterName string) error {
	cluster, err := util.GetCluster(c.Client, clusterName)
	if err != nil {
		klog.ErrorS(err, "Failed to get the given member cluster", "cluster", clusterName)
		return err
	}

	if !util.IsClusterReady(&cluster.Status) {
		var ErrClusterNotReady = errors.New("cluster not ready")
		klog.ErrorS(ErrClusterNotReady, "Stop collect endpointslice for cluster as cluster not ready.", "cluster", cluster.Name)
		return fmt.Errorf("cluster(%s) not ready", cluster.Name)
	}

	if err := c.registerInformersAndStart(cluster); err != nil {
		klog.ErrorS(err, "Failed to register informer for Cluster", "cluster", cluster.Name)
		return err
	}

	return nil
}

// registerInformersAndStart builds informer manager for cluster if it doesn't exist, then constructs informers for gvr
// and start it.
func (c *EndpointSliceCollectController) registerInformersAndStart(cluster *clusterv1alpha1.Cluster) error {
	singleClusterInformerManager := c.InformerManager.GetSingleClusterManager(cluster.Name)
	if singleClusterInformerManager == nil {
		dynamicClusterClient, err := c.ClusterDynamicClientSetFunc(cluster.Name, c.Client, c.ClusterClientOption)
		if err != nil {
			klog.ErrorS(err, "Failed to build dynamic cluster client for cluster", "cluster", cluster.Name)
			return err
		}
		singleClusterInformerManager = c.InformerManager.ForCluster(dynamicClusterClient.ClusterName, dynamicClusterClient.DynamicClientSet, 0)
	}

	gvrTargets := []schema.GroupVersionResource{
		endpointSliceGVR,
	}

	allSynced := true
	for _, gvr := range gvrTargets {
		if !singleClusterInformerManager.IsInformerSynced(gvr) || !singleClusterInformerManager.IsHandlerExist(gvr, c.getEventHandler(cluster.Name)) {
			allSynced = false
			singleClusterInformerManager.ForResource(gvr, c.getEventHandler(cluster.Name))
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
		for _, gvr := range gvrTargets {
			if !synced[gvr] {
				return fmt.Errorf("informer for %s hasn't synced", gvr)
			}
		}
		return nil
	}(); err != nil {
		klog.ErrorS(err, "Failed to sync cache for cluster", "cluster", cluster.Name)
		c.InformerManager.Stop(cluster.Name)
		return err
	}

	return nil
}

// getEventHandler return callback function that knows how to handle events from the member cluster.
func (c *EndpointSliceCollectController) getEventHandler(clusterName string) cache.ResourceEventHandler {
	if value, exists := c.eventHandlers.Load(clusterName); exists {
		return value.(cache.ResourceEventHandler)
	}

	eventHandler := fedinformer.NewHandlerOnEvents(c.genHandlerAddFunc(clusterName), c.genHandlerUpdateFunc(clusterName),
		c.genHandlerDeleteFunc(clusterName))
	c.eventHandlers.Store(clusterName, eventHandler)
	return eventHandler
}

func (c *EndpointSliceCollectController) genHandlerAddFunc(clusterName string) func(obj interface{}) {
	return func(obj interface{}) {
		curObj := obj.(runtime.Object)
		key, err := keys.FederatedKeyFunc(clusterName, curObj)
		if err != nil {
			klog.ErrorS(err, "Failed to generate key for obj", "gvk", curObj.GetObjectKind().GroupVersionKind())
			return
		}
		c.worker.Add(key)
	}
}

func (c *EndpointSliceCollectController) genHandlerUpdateFunc(clusterName string) func(oldObj, newObj interface{}) {
	return func(oldObj, newObj interface{}) {
		curObj := newObj.(runtime.Object)
		if !reflect.DeepEqual(oldObj, newObj) {
			key, err := keys.FederatedKeyFunc(clusterName, curObj)
			if err != nil {
				klog.ErrorS(err, "Failed to generate key for obj", "gvk", curObj.GetObjectKind().GroupVersionKind())
				return
			}
			c.worker.Add(key)
		}
	}
}

func (c *EndpointSliceCollectController) genHandlerDeleteFunc(clusterName string) func(obj interface{}) {
	return func(obj interface{}) {
		if deleted, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			// This object might be stale but ok for our current usage.
			obj = deleted.Obj
			if obj == nil {
				return
			}
		}
		oldObj := obj.(runtime.Object)
		key, err := keys.FederatedKeyFunc(clusterName, oldObj)
		if err != nil {
			klog.ErrorS(err, "Failed to generate key for obj", "gvk", oldObj.GetObjectKind().GroupVersionKind())
			return
		}
		c.worker.Add(key)
	}
}

// handleEndpointSliceEvent syncs EndPointSlice objects to control-plane according to EndpointSlice event.
// For EndpointSlice create or update event, reports the EndpointSlice when referencing service has been exported.
// For EndpointSlice delete event, cleanup the previously reported EndpointSlice.
func (c *EndpointSliceCollectController) handleEndpointSliceEvent(ctx context.Context, endpointSliceKey keys.FederatedKey) error {
	endpointSliceObj, err := helper.GetObjectFromCache(c.RESTMapper, c.InformerManager, endpointSliceKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return cleanupWorkWithEndpointSliceDelete(ctx, c.Client, endpointSliceKey)
		}
		return err
	}

	if util.GetLabelValue(endpointSliceObj.GetLabels(), discoveryv1.LabelManagedBy) == util.EndpointSliceDispatchControllerLabelValue {
		return nil
	}

	workList := &workv1alpha1.WorkList{}
	if err := c.Client.List(ctx, workList, &client.ListOptions{
		Namespace: names.GenerateExecutionSpaceName(endpointSliceKey.Cluster),
		LabelSelector: labels.SelectorFromSet(labels.Set{
			util.MultiClusterServiceNamespaceLabel: endpointSliceKey.Namespace,
			util.MultiClusterServiceNameLabel:      util.GetLabelValue(endpointSliceObj.GetLabels(), discoveryv1.LabelServiceName),
		})}); err != nil {
		klog.ErrorS(err, "Failed to list workList reported by endpointSlice", "namespace", endpointSliceKey.Namespace, "name", endpointSliceKey.Name)
		return err
	}

	mcsExists := false
	for _, work := range workList.Items {
		if util.IsWorkContains(work.Spec.Workload.Manifests, multiClusterServiceGVK) {
			mcsExists = true
			break
		}
	}
	if !mcsExists {
		return nil
	}

	if err = c.reportEndpointSliceWithEndpointSliceCreateOrUpdate(ctx, endpointSliceKey.Cluster, endpointSliceObj); err != nil {
		klog.ErrorS(err, "Failed to handle endpointSlice event", "namespaceKey",
			endpointSliceKey.NamespaceKey())
		return err
	}

	return nil
}

func (c *EndpointSliceCollectController) collectTargetEndpointSlice(ctx context.Context, work *workv1alpha1.Work, clusterName string) error {
	manager := c.InformerManager.GetSingleClusterManager(clusterName)
	if manager == nil {
		err := fmt.Errorf("failed to get informer manager for cluster %s", clusterName)
		klog.ErrorS(err, "Failed to get informer manager for cluster")
		return err
	}

	svcNamespace := util.GetLabelValue(work.Labels, util.MultiClusterServiceNamespaceLabel)
	svcName := util.GetLabelValue(work.Labels, util.MultiClusterServiceNameLabel)
	selector := labels.SelectorFromSet(labels.Set{
		discoveryv1.LabelServiceName: svcName,
	})
	epsList, err := manager.Lister(discoveryv1.SchemeGroupVersion.WithResource("endpointslices")).ByNamespace(svcNamespace).List(selector)
	if err != nil {
		klog.ErrorS(err, "Failed to list EndpointSlice for Service in a cluster", "namespace", svcNamespace, "name", svcName, "cluster", clusterName)
		return err
	}
	for _, epsObj := range epsList {
		eps := &discoveryv1.EndpointSlice{}
		if err = helper.ConvertToTypedObject(epsObj, eps); err != nil {
			klog.ErrorS(err, "Failed to convert object to EndpointSlice")
			return err
		}
		if util.GetLabelValue(eps.GetLabels(), discoveryv1.LabelManagedBy) == util.EndpointSliceDispatchControllerLabelValue {
			continue
		}
		epsUnstructured, err := helper.ToUnstructured(eps)
		if err != nil {
			klog.ErrorS(err, "Failed to convert EndpointSlice to unstructured", "namespace", eps.GetNamespace(), "name", eps.GetName())
			return err
		}
		if err = c.reportEndpointSliceWithEndpointSliceCreateOrUpdate(ctx, clusterName, epsUnstructured); err != nil {
			return err
		}
	}

	return nil
}

// reportEndpointSliceWithEndpointSliceCreateOrUpdate reports the EndpointSlice when referencing service has been exported.
func (c *EndpointSliceCollectController) reportEndpointSliceWithEndpointSliceCreateOrUpdate(ctx context.Context, clusterName string, endpointSlice *unstructured.Unstructured) error {
	if err := reportEndpointSlice(ctx, c.Client, endpointSlice, clusterName); err != nil {
		return fmt.Errorf("failed to report EndpointSlice(%s/%s) from cluster(%s) to control-plane",
			endpointSlice.GetNamespace(), endpointSlice.GetName(), clusterName)
	}

	return nil
}

// reportEndpointSlice report EndPointSlice objects to control-plane.
func reportEndpointSlice(ctx context.Context, c client.Client, endpointSlice *unstructured.Unstructured, clusterName string) error {
	executionSpace := names.GenerateExecutionSpaceName(clusterName)
	workName := names.GenerateWorkName(endpointSlice.GetKind(), endpointSlice.GetName(), endpointSlice.GetNamespace())

	workMeta, err := getEndpointSliceWorkMeta(ctx, c, executionSpace, workName, endpointSlice)
	if err != nil {
		return err
	}

	// indicate the Work should be not propagated since it's collected resource.
	if err := ctrlutil.CreateOrUpdateWork(ctx, c, workMeta, endpointSlice, ctrlutil.WithSuspendDispatching(true)); err != nil {
		klog.ErrorS(err, "Failed to create or update work", "namespace", workMeta.Namespace, "name", workMeta.Name)
		return err
	}

	return nil
}

func getEndpointSliceWorkMeta(ctx context.Context, c client.Client, ns string, workName string, endpointSlice *unstructured.Unstructured) (metav1.ObjectMeta, error) {
	existWork := &workv1alpha1.Work{}
	var err error
	if err = c.Get(ctx, types.NamespacedName{
		Namespace: ns,
		Name:      workName,
	}, existWork); err != nil && !apierrors.IsNotFound(err) {
		klog.ErrorS(err, "Get EndpointSlice work", "namespace", ns, "name", workName)
		return metav1.ObjectMeta{}, err
	}

	existFinalizers := existWork.GetFinalizers()
	finalizersToAdd := []string{util.MCSEndpointSliceDispatchControllerFinalizer}
	newFinalizers := util.MergeFinalizers(existFinalizers, finalizersToAdd)

	ls := map[string]string{
		util.MultiClusterServiceNamespaceLabel: endpointSlice.GetNamespace(),
		util.MultiClusterServiceNameLabel:      endpointSlice.GetLabels()[discoveryv1.LabelServiceName],
		util.EndpointSliceWorkManagedByLabel:   util.MultiClusterServiceKind,
	}
	if existWork.Labels == nil || (err != nil && apierrors.IsNotFound(err)) {
		workMeta := metav1.ObjectMeta{
			Name:       workName,
			Namespace:  ns,
			Labels:     ls,
			Finalizers: newFinalizers,
		}
		return workMeta, nil
	}

	ls = util.DedupeAndMergeLabels(ls, existWork.Labels)
	if value, ok := existWork.Labels[util.EndpointSliceWorkManagedByLabel]; ok {
		controllerSet := sets.New[string]()
		controllerSet.Insert(strings.Split(value, ".")...)
		controllerSet.Insert(util.MultiClusterServiceKind)
		ls[util.EndpointSliceWorkManagedByLabel] = strings.Join(controllerSet.UnsortedList(), ".")
	}
	return metav1.ObjectMeta{
		Name:       workName,
		Namespace:  ns,
		Labels:     ls,
		Finalizers: newFinalizers,
	}, nil
}

func cleanupWorkWithEndpointSliceDelete(ctx context.Context, c client.Client, endpointSliceKey keys.FederatedKey) error {
	executionSpace := names.GenerateExecutionSpaceName(endpointSliceKey.Cluster)

	workNamespaceKey := types.NamespacedName{
		Namespace: executionSpace,
		Name:      names.GenerateWorkName(endpointSliceKey.Kind, endpointSliceKey.Name, endpointSliceKey.Namespace),
	}
	work := &workv1alpha1.Work{}
	if err := c.Get(ctx, workNamespaceKey, work); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.ErrorS(err, "Failed to get work in executionSpace", "namespaceKey", workNamespaceKey.String(), "executionSpace", executionSpace)
		return err
	}

	return cleanProviderClustersEndpointSliceWork(ctx, c, work.DeepCopy())
}

// TBD: Currently, the EndpointSlice work can be handled by both service-export-controller and endpointslice-collect-controller.
// To indicate this, we've introduced the label endpointslice.karmada.io/managed-by. Therefore,
// if managed by both controllers, simply remove each controller's respective labels.
// If managed solely by its own controller, delete the work accordingly.
// This logic should be deleted after the conflict is fixed.
func cleanProviderClustersEndpointSliceWork(ctx context.Context, c client.Client, work *workv1alpha1.Work) error {
	controllers := util.GetLabelValue(work.Labels, util.EndpointSliceWorkManagedByLabel)
	controllerSet := sets.New[string]()
	controllerSet.Insert(strings.Split(controllers, ".")...)
	controllerSet.Delete(util.MultiClusterServiceKind)
	if controllerSet.Len() > 0 {
		delete(work.Labels, util.MultiClusterServiceNameLabel)
		delete(work.Labels, util.MultiClusterServiceNamespaceLabel)
		work.Labels[util.EndpointSliceWorkManagedByLabel] = strings.Join(controllerSet.UnsortedList(), ".")

		if err := c.Update(ctx, work); err != nil {
			klog.ErrorS(err, "Failed to update work", "namespace", work.Namespace, "name", work.Name)
			return err
		}
		return nil
	}

	if err := c.Delete(ctx, work); err != nil {
		klog.ErrorS(err, "Failed to delete work", "namespace", work.Namespace, "name", work.Name)
		return err
	}

	return nil
}
