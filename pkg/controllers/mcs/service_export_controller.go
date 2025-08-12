/*
Copyright 2021 The Karmada Authors.

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

package mcs

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/controllers/ctrlutil"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// ServiceExportControllerName is the controller name that will be used when reporting events and metrics.
const ServiceExportControllerName = "service-export-controller"

// ServiceExportController is to sync ServiceExport and report EndpointSlices of exported service to control-plane.
type ServiceExportController struct {
	client.Client
	EventRecorder               record.EventRecorder
	RESTMapper                  meta.RESTMapper
	Context                     context.Context
	InformerManager             genericmanager.MultiClusterInformerManager
	WorkerNumber                int                 // WorkerNumber is the number of worker goroutines
	PredicateFunc               predicate.Predicate // PredicateFunc is the function that filters events before enqueuing the keys.
	ClusterDynamicClientSetFunc util.NewClusterDynamicClientSetFunc
	ClusterClientOption         *util.ClientOption
	ClusterCacheSyncTimeout     metav1.Duration

	// eventHandlers holds the handlers which used to handle events reported from member clusters.
	// Each handler takes the cluster name as key and takes the handler function as the value, e.g.
	// "member1": instance of ResourceEventHandler
	eventHandlers sync.Map
	// worker process resources periodic from rateLimitingQueue.
	worker             util.AsyncWorker
	RateLimiterOptions ratelimiterflag.Options
}

var (
	serviceExportGVR = mcsv1alpha1.SchemeGroupVersion.WithResource("serviceexports")
	serviceExportGVK = mcsv1alpha1.SchemeGroupVersion.WithKind(util.ServiceExportKind)
	endpointSliceGVR = discoveryv1.SchemeGroupVersion.WithResource("endpointslices")
	endpointSliceGVK = discoveryv1.SchemeGroupVersion.WithKind(util.EndpointSliceKind)
)

// Reconcile performs a full reconciliation for the object referred to by the Request.
func (c *ServiceExportController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling Work", "namespace", req.NamespacedName.Namespace, "name", req.NamespacedName.Name)

	work := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, req.NamespacedName, work); err != nil {
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

	if !util.IsWorkContains(work.Spec.Workload.Manifests, serviceExportGVK) {
		return controllerruntime.Result{}, nil
	}

	clusterName, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.ErrorS(err, "Failed to get member cluster name for work", "namespace", work.Namespace, "workName", work.Name)
		return controllerruntime.Result{}, err
	}

	cluster, err := util.GetCluster(c.Client, clusterName)
	if err != nil {
		klog.ErrorS(err, "Failed to get the given member cluster", "cluster", clusterName)
		return controllerruntime.Result{}, err
	}

	if !util.IsClusterReady(&cluster.Status) {
		err := fmt.Errorf("cluster(%s) not ready", cluster.Name)
		klog.ErrorS(err, "Stop sync work for cluster as cluster not ready.", "namespace", work.Namespace, "workName", work.Name, "cluster", cluster.Name)
		return controllerruntime.Result{}, err
	}

	return controllerruntime.Result{}, c.buildResourceInformers(cluster)
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ServiceExportController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).Named(ServiceExportControllerName).For(&workv1alpha1.Work{}, builder.WithPredicates(c.PredicateFunc)).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions),
		}).
		Complete(c)
}

// RunWorkQueue initializes worker and run it, worker will process resource asynchronously.
func (c *ServiceExportController) RunWorkQueue() {
	workerOptions := util.Options{
		Name:          "service-export",
		KeyFunc:       nil,
		ReconcileFunc: c.syncServiceExportOrEndpointSlice,
	}
	c.worker = util.NewAsyncWorker(workerOptions)
	c.worker.Run(c.Context, c.WorkerNumber)

	go c.enqueueReportedEpsServiceExport()
}

func (c *ServiceExportController) enqueueReportedEpsServiceExport() {
	workList := &workv1alpha1.WorkList{}
	err := wait.PollUntilContextCancel(context.TODO(), 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		err = c.List(ctx, workList, client.MatchingFields{
			indexregistry.WorkIndexByFieldSuspendDispatching: "true",
		})
		if err != nil {
			klog.ErrorS(err, "Failed to list collected EndpointSlices Work from member clusters")
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return
	}

	for index := range workList.Items {
		work := workList.Items[index]
		if !util.IsWorkContains(work.Spec.Workload.Manifests, endpointSliceGVK) {
			continue
		}

		managedByStr := work.Labels[util.EndpointSliceWorkManagedByLabel]
		if !strings.Contains(managedByStr, serviceExportGVK.Kind) {
			continue
		}

		clusterName, err := names.GetClusterName(work.GetNamespace())
		if err != nil {
			continue
		}

		key := keys.FederatedKey{
			Cluster: clusterName,
			ClusterWideKey: keys.ClusterWideKey{
				Group:     serviceExportGVK.Group,
				Version:   serviceExportGVK.Version,
				Kind:      serviceExportGVK.Kind,
				Namespace: work.Labels[util.ServiceNamespaceLabel],
				Name:      work.Labels[util.ServiceNameLabel],
			},
		}
		c.worker.Add(key)
	}
}

func (c *ServiceExportController) syncServiceExportOrEndpointSlice(key util.QueueKey) error {
	ctx := context.Background()
	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		err := fmt.Errorf("invalid key")
		klog.ErrorS(err, "Failed to sync serviceExport as invalid key", "key", key)
		return err
	}

	klog.V(4).InfoS("Begin to sync", "key", fedKey)

	switch fedKey.Kind {
	case util.ServiceExportKind:
		if err := c.handleServiceExportEvent(ctx, fedKey); err != nil {
			klog.ErrorS(err, "Failed to handle serviceExport event", "serviceExport", fedKey.NamespaceKey())
			return err
		}
	case util.EndpointSliceKind:
		if err := c.handleEndpointSliceEvent(ctx, fedKey); err != nil {
			klog.ErrorS(err, "Failed to handle endpointSlice event", "endpointSlice", fedKey.NamespaceKey())
			return err
		}
	}

	return nil
}

func (c *ServiceExportController) buildResourceInformers(cluster *clusterv1alpha1.Cluster) error {
	err := c.registerInformersAndStart(cluster)
	if err != nil {
		klog.ErrorS(err, "Failed to register informer for Cluster", "cluster", cluster.Name)
		return err
	}
	return nil
}

// registerInformersAndStart builds informer manager for cluster if it doesn't exist, then constructs informers for gvr
// and start it.
func (c *ServiceExportController) registerInformersAndStart(cluster *clusterv1alpha1.Cluster) error {
	singleClusterInformerManager := c.InformerManager.GetSingleClusterManager(cluster.Name)
	if singleClusterInformerManager == nil {
		dynamicClusterClient, err := c.ClusterDynamicClientSetFunc(cluster.Name, c.Client, c.ClusterClientOption)
		if err != nil {
			klog.ErrorS(err, "Failed to build dynamic cluster client for cluster.", "cluster", cluster.Name)
			return err
		}
		singleClusterInformerManager = c.InformerManager.ForCluster(dynamicClusterClient.ClusterName, dynamicClusterClient.DynamicClientSet, 0)
	}

	gvrTargets := []schema.GroupVersionResource{
		serviceExportGVR,
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
func (c *ServiceExportController) getEventHandler(clusterName string) cache.ResourceEventHandler {
	if value, exists := c.eventHandlers.Load(clusterName); exists {
		return value.(cache.ResourceEventHandler)
	}

	eventHandler := fedinformer.NewHandlerOnEvents(c.genHandlerAddFunc(clusterName), c.genHandlerUpdateFunc(clusterName),
		c.genHandlerDeleteFunc(clusterName))
	c.eventHandlers.Store(clusterName, eventHandler)
	return eventHandler
}

func (c *ServiceExportController) genHandlerAddFunc(clusterName string) func(obj interface{}) {
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

func (c *ServiceExportController) genHandlerUpdateFunc(clusterName string) func(oldObj, newObj interface{}) {
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

func (c *ServiceExportController) genHandlerDeleteFunc(clusterName string) func(obj interface{}) {
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

// handleServiceExportEvent syncs EndPointSlice objects to control-plane according to ServiceExport event.
// For ServiceExport create or update event, reports the referencing service's EndpointSlice.
// For ServiceExport delete event, cleanup the previously reported EndpointSlice.
func (c *ServiceExportController) handleServiceExportEvent(ctx context.Context, serviceExportKey keys.FederatedKey) error {
	_, err := helper.GetObjectFromCache(c.RESTMapper, c.InformerManager, serviceExportKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return cleanupWorkWithServiceExportDelete(ctx, c.Client, serviceExportKey)
		}
		return err
	}

	// Even though the EndpointSlice will be synced when dealing with EndpointSlice events, thus the 'report' here may
	// be redundant, but it helps to avoid a corner case:
	// If skip report here, after ServiceExport deletion and re-creation, if no EndpointSlice changes, we didn't get a
	// change to sync.
	if err = c.reportEndpointSliceWithServiceExportCreate(ctx, serviceExportKey); err != nil {
		klog.ErrorS(err, "Failed to handle ServiceExport event", "namespace", serviceExportKey.Namespace, "name", serviceExportKey.Name)
		return err
	}

	return nil
}

// handleEndpointSliceEvent syncs EndPointSlice objects to control-plane according to EndpointSlice event.
// For EndpointSlice create or update event, reports the EndpointSlice when referencing service has been exported.
// For EndpointSlice delete event, cleanup the previously reported EndpointSlice.
func (c *ServiceExportController) handleEndpointSliceEvent(ctx context.Context, endpointSliceKey keys.FederatedKey) error {
	endpointSliceObj, err := helper.GetObjectFromCache(c.RESTMapper, c.InformerManager, endpointSliceKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return cleanupWorkWithEndpointSliceDelete(ctx, c.Client, endpointSliceKey)
		}
		return err
	}

	if err = c.reportEndpointSliceWithEndpointSliceCreateOrUpdate(ctx, endpointSliceKey.Cluster, endpointSliceObj); err != nil {
		klog.ErrorS(err, "Failed to handle endpointSlice event", "namespace", endpointSliceKey.Namespace, "name", endpointSliceKey.Name)
		return err
	}

	return nil
}

// reportEndpointSliceWithServiceExportCreate reports the referencing service's EndpointSlice.
func (c *ServiceExportController) reportEndpointSliceWithServiceExportCreate(ctx context.Context, serviceExportKey keys.FederatedKey) error {
	var (
		endpointSliceObjects []runtime.Object
		err                  error
		errs                 []error
	)

	singleClusterManager := c.InformerManager.GetSingleClusterManager(serviceExportKey.Cluster)
	if singleClusterManager == nil {
		return nil
	}

	// Before retrieving EndpointSlice objects from the informer, ensure the informer cache is synced.
	// This is necessary because the informer for EndpointSlice is created dynamically in the Reconcile() routine
	// when a Work resource containing an ServiceExport is detected for the cluster. If the informer is not yet synced,
	// return an error and wait a retry at the next time.
	if !singleClusterManager.IsInformerSynced(endpointSliceGVR) {
		return fmt.Errorf("the informer for cluster %s has not been synced, wait a retry at the next time", serviceExportKey.Cluster)
	}

	endpointSliceLister := singleClusterManager.Lister(endpointSliceGVR)
	if endpointSliceObjects, err = endpointSliceLister.ByNamespace(serviceExportKey.Namespace).List(labels.SelectorFromSet(labels.Set{
		discoveryv1.LabelServiceName: serviceExportKey.Name,
	})); err != nil {
		return err
	}

	err = c.removeOrphanWork(ctx, endpointSliceObjects, serviceExportKey)
	if err != nil {
		return err
	}

	for index := range endpointSliceObjects {
		if err = reportEndpointSlice(ctx, c.Client, endpointSliceObjects[index].(*unstructured.Unstructured), serviceExportKey.Cluster); err != nil {
			errs = append(errs, err)
		}
	}
	return utilerrors.NewAggregate(errs)
}

func (c *ServiceExportController) removeOrphanWork(ctx context.Context, endpointSliceObjects []runtime.Object, serviceExportKey keys.FederatedKey) error {
	willReportWorks := sets.NewString()
	for index := range endpointSliceObjects {
		endpointSlice := endpointSliceObjects[index].(*unstructured.Unstructured)
		workName := names.GenerateWorkName(endpointSlice.GetKind(), endpointSlice.GetName(), endpointSlice.GetNamespace())
		willReportWorks.Insert(workName)
	}

	collectedEpsWorkList := &workv1alpha1.WorkList{}
	if err := c.List(ctx, collectedEpsWorkList, &client.ListOptions{
		Namespace: names.GenerateExecutionSpaceName(serviceExportKey.Cluster),
		LabelSelector: labels.SelectorFromSet(labels.Set{
			util.ServiceNamespaceLabel: serviceExportKey.Namespace,
			util.ServiceNameLabel:      serviceExportKey.Name,
		}),
		FieldSelector: fields.OneTermEqualSelector(indexregistry.WorkIndexByFieldSuspendDispatching, "true"),
	}); err != nil {
		klog.ErrorS(err, "Failed to list workList reported by ServiceExport in executionSpace",
			"namespace", serviceExportKey.Namespace, "name", serviceExportKey.Name, "executionSpace",
			names.GenerateExecutionSpaceName(serviceExportKey.Cluster))
		return err
	}

	var errs []error
	for index := range collectedEpsWorkList.Items {
		work := collectedEpsWorkList.Items[index]
		if !util.IsWorkContains(work.Spec.Workload.Manifests, endpointSliceGVK) {
			continue
		}

		managedByStr := work.Labels[util.EndpointSliceWorkManagedByLabel]
		if !strings.Contains(managedByStr, serviceExportGVK.Kind) {
			continue
		}

		if willReportWorks.Has(work.Name) {
			continue
		}

		err := cleanEndpointSliceWork(ctx, c.Client, &work)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return utilerrors.NewAggregate(errs)
}

// reportEndpointSliceWithEndpointSliceCreateOrUpdate reports the EndpointSlice when referencing service has been exported.
func (c *ServiceExportController) reportEndpointSliceWithEndpointSliceCreateOrUpdate(ctx context.Context, clusterName string, endpointSlice *unstructured.Unstructured) error {
	relatedServiceName := endpointSlice.GetLabels()[discoveryv1.LabelServiceName]

	singleClusterManager := c.InformerManager.GetSingleClusterManager(clusterName)
	if singleClusterManager == nil {
		return nil
	}

	// Before retrieving ServiceExport objects from the informer, ensure the informer cache is synced.
	// This is necessary because the informer for ServiceExport is created dynamically in the Reconcile() routine
	// when a Work resource containing an ServiceExport is detected for the cluster. If the informer is not yet synced,
	// return an error and wait a retry at the next time.
	if !singleClusterManager.IsInformerSynced(serviceExportGVR) {
		return fmt.Errorf("the informer for cluster %s has not been synced, wait a retry at the next time", clusterName)
	}

	serviceExportLister := singleClusterManager.Lister(serviceExportGVR)
	_, err := serviceExportLister.ByNamespace(endpointSlice.GetNamespace()).Get(relatedServiceName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		klog.ErrorS(err, "Failed to get ServiceExport object", "namespace", endpointSlice.GetNamespace(), "name", relatedServiceName)
		return err
	}

	return reportEndpointSlice(ctx, c.Client, endpointSlice, clusterName)
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
		klog.ErrorS(err, "Failed to get EndpointSlice work", "namespace", ns, "name", workName)
		return metav1.ObjectMeta{}, err
	}

	existFinalizers := existWork.GetFinalizers()
	finalizersToAdd := []string{util.EndpointSliceControllerFinalizer}
	newFinalizers := util.MergeFinalizers(existFinalizers, finalizersToAdd)

	workMeta := metav1.ObjectMeta{
		Name:       workName,
		Namespace:  ns,
		Finalizers: newFinalizers,
		Labels: map[string]string{
			util.ServiceNamespaceLabel:           endpointSlice.GetNamespace(),
			util.ServiceNameLabel:                endpointSlice.GetLabels()[discoveryv1.LabelServiceName],
			util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
		},
	}

	if existWork.Labels == nil || (err != nil && apierrors.IsNotFound(err)) {
		return workMeta, nil
	}

	workMeta.Labels = util.DedupeAndMergeLabels(workMeta.Labels, existWork.Labels)
	if value, ok := existWork.Labels[util.EndpointSliceWorkManagedByLabel]; ok {
		controllerSet := sets.New[string]()
		controllerSet.Insert(strings.Split(value, ".")...)
		controllerSet.Insert(util.ServiceExportKind)
		workMeta.Labels[util.EndpointSliceWorkManagedByLabel] = strings.Join(controllerSet.UnsortedList(), ".")
	}
	return workMeta, nil
}

func cleanupWorkWithServiceExportDelete(ctx context.Context, c client.Client, serviceExportKey keys.FederatedKey) error {
	executionSpace := names.GenerateExecutionSpaceName(serviceExportKey.Cluster)

	workList := &workv1alpha1.WorkList{}
	if err := c.List(ctx, workList, &client.ListOptions{
		Namespace: executionSpace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			util.ServiceNamespaceLabel: serviceExportKey.Namespace,
			util.ServiceNameLabel:      serviceExportKey.Name,
		}),
	}); err != nil {
		klog.ErrorS(err, "Failed to list workList reported by ServiceExport in executionSpace",
			"namespace", serviceExportKey.Namespace, "name", serviceExportKey.Name, "executionSpace", executionSpace)
		return err
	}

	var errs []error
	for _, work := range workList.Items {
		if err := cleanEndpointSliceWork(ctx, c, work.DeepCopy()); err != nil {
			errs = append(errs, err)
		}
	}
	return utilerrors.NewAggregate(errs)
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

		klog.ErrorS(err, "Failed to get work in executionSpace", "work", workNamespaceKey.String(), "executionSpace", executionSpace)
		return err
	}

	return cleanEndpointSliceWork(ctx, c, work.DeepCopy())
}

// TBD: Currently, the EndpointSlice work can be handled by both service-export-controller and endpointslice-collect-controller. To indicate this, we've introduced the label endpointslice.karmada.io/managed-by. Therefore,
// if managed by both controllers, simply remove each controller's respective labels.
// If managed solely by its own controller, delete the work accordingly.
// This logic should be deleted after the conflict is fixed.
func cleanEndpointSliceWork(ctx context.Context, c client.Client, work *workv1alpha1.Work) error {
	controllers := util.GetLabelValue(work.Labels, util.EndpointSliceWorkManagedByLabel)
	controllerSet := sets.New[string]()
	controllerSet.Insert(strings.Split(controllers, ".")...)
	controllerSet.Delete(util.ServiceExportKind)
	if controllerSet.Len() > 0 {
		delete(work.Labels, util.ServiceNameLabel)
		delete(work.Labels, util.ServiceNamespaceLabel)
		work.Labels[util.EndpointSliceWorkManagedByLabel] = strings.Join(controllerSet.UnsortedList(), ".")

		if err := c.Update(ctx, work); err != nil {
			klog.ErrorS(err, "Failed to update work", "namespace", work.Namespace, "workName", work.Name)
			return err
		}
		klog.Infof("Successfully updated work(%s/%s)", work.Namespace, work.Name)
		return nil
	}

	if err := c.Delete(ctx, work); err != nil {
		klog.ErrorS(err, "Failed to delete work", "namespace", work.Namespace, "workName", work.Name)
		return err
	}
	klog.Infof("Successfully deleted work(%s/%s)", work.Namespace, work.Name)

	return nil
}
