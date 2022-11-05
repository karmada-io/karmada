package mcs

import (
	"context"
	"fmt"
	"reflect"
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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// ServiceExportControllerName is the controller name that will be used when reporting events.
const ServiceExportControllerName = "service-export-controller"

// ServiceExportController is to sync ServiceExport and report EndpointSlices of exported service to control-plane.
type ServiceExportController struct {
	client.Client
	EventRecorder               record.EventRecorder
	RESTMapper                  meta.RESTMapper
	StopChan                    <-chan struct{}
	InformerManager             genericmanager.MultiClusterInformerManager
	WorkerNumber                int                 // WorkerNumber is the number of worker goroutines
	PredicateFunc               predicate.Predicate // PredicateFunc is the function that filters events before enqueuing the keys.
	ClusterDynamicClientSetFunc func(clusterName string, client client.Client) (*util.DynamicClusterClient, error)
	// eventHandlers holds the handlers which used to handle events reported from member clusters.
	// Each handler takes the cluster name as key and takes the handler function as the value, e.g.
	// "member1": instance of ResourceEventHandler
	eventHandlers sync.Map
	worker        util.AsyncWorker // worker process resources periodic from rateLimitingQueue.

	ClusterCacheSyncTimeout metav1.Duration
}

var (
	serviceExportGVR = mcsv1alpha1.SchemeGroupVersion.WithResource("serviceexports")
	serviceExportGVK = mcsv1alpha1.SchemeGroupVersion.WithKind("ServiceExport")
	endpointSliceGVR = discoveryv1.SchemeGroupVersion.WithResource("endpointslices")
)

// Reconcile performs a full reconciliation for the object referred to by the Request.
func (c *ServiceExportController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling Work %s", req.NamespacedName.String())

	work := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, req.NamespacedName, work); err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{Requeue: true}, err
	}

	if !work.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	if !helper.IsResourceApplied(&work.Status) {
		return controllerruntime.Result{}, nil
	}

	if !isWorkContains(work.Spec.Workload.Manifests, serviceExportGVK) {
		return controllerruntime.Result{}, nil
	}

	clusterName, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.Errorf("Failed to get member cluster name for work %s/%s", work.Namespace, work.Name)
		return controllerruntime.Result{Requeue: true}, err
	}

	cluster, err := util.GetCluster(c.Client, clusterName)
	if err != nil {
		klog.Errorf("Failed to get the given member cluster %s", clusterName)
		return controllerruntime.Result{Requeue: true}, err
	}

	if !util.IsClusterReady(&cluster.Status) {
		klog.Errorf("Stop sync work(%s/%s) for cluster(%s) as cluster not ready.", work.Namespace, work.Name, cluster.Name)
		return controllerruntime.Result{Requeue: true}, fmt.Errorf("cluster(%s) not ready", cluster.Name)
	}

	return c.buildResourceInformers(cluster)
}

// isWorkContains checks if the target resource exists in a work.spec.workload.manifests.
func isWorkContains(manifests []workv1alpha1.Manifest, targetResource schema.GroupVersionKind) bool {
	for index := range manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifests[index].Raw)
		if err != nil {
			klog.Errorf("failed to unmarshal work manifests index %d, error is: %v", index, err)
			continue
		}

		if targetResource == workload.GroupVersionKind() {
			return true
		}
	}
	return false
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ServiceExportController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&workv1alpha1.Work{}, builder.WithPredicates(c.PredicateFunc)).Complete(c)
}

// RunWorkQueue initializes worker and run it, worker will process resource asynchronously.
func (c *ServiceExportController) RunWorkQueue() {
	workerOptions := util.Options{
		Name:          "service-export",
		KeyFunc:       nil,
		ReconcileFunc: c.syncServiceExportOrEndpointSlice,
	}
	c.worker = util.NewAsyncWorker(workerOptions)
	c.worker.Run(c.WorkerNumber, c.StopChan)
}

func (c *ServiceExportController) syncServiceExportOrEndpointSlice(key util.QueueKey) error {
	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		klog.Errorf("Failed to sync serviceExport as invalid key: %v", key)
		return fmt.Errorf("invalid key")
	}

	klog.V(4).Infof("Begin to sync %s %s.", fedKey.Kind, fedKey.NamespaceKey())

	switch fedKey.Kind {
	case util.ServiceExportKind:
		if err := c.handleServiceExportEvent(fedKey); err != nil {
			klog.Errorf("Failed to handle serviceExport(%s) event, Error: %v",
				fedKey.NamespaceKey(), err)
			return err
		}
	case util.EndpointSliceKind:
		if err := c.handleEndpointSliceEvent(fedKey); err != nil {
			klog.Errorf("Failed to handle endpointSlice(%s) event, Error: %v",
				fedKey.NamespaceKey(), err)
			return err
		}
	}

	return nil
}

func (c *ServiceExportController) buildResourceInformers(cluster *clusterv1alpha1.Cluster) (controllerruntime.Result, error) {
	err := c.registerInformersAndStart(cluster)
	if err != nil {
		klog.Errorf("Failed to register informer for Cluster %s. Error: %v.", cluster.Name, err)
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// registerInformersAndStart builds informer manager for cluster if it doesn't exist, then constructs informers for gvr
// and start it.
func (c *ServiceExportController) registerInformersAndStart(cluster *clusterv1alpha1.Cluster) error {
	singleClusterInformerManager := c.InformerManager.GetSingleClusterManager(cluster.Name)
	if singleClusterInformerManager == nil {
		dynamicClusterClient, err := c.ClusterDynamicClientSetFunc(cluster.Name, c.Client)
		if err != nil {
			klog.Errorf("Failed to build dynamic cluster client for cluster %s.", cluster.Name)
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
		klog.Errorf("Failed to sync cache for cluster: %s, error: %v", cluster.Name, err)
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
			klog.Warningf("Failed to generate key for obj: %s", curObj.GetObjectKind().GroupVersionKind())
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
				klog.Warningf("Failed to generate key for obj: %s", curObj.GetObjectKind().GroupVersionKind())
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
			klog.Warningf("Failed to generate key for obj: %s", oldObj.GetObjectKind().GroupVersionKind())
			return
		}
		c.worker.Add(key)
	}
}

// handleServiceExportEvent syncs EndPointSlice objects to control-plane according to ServiceExport event.
// For ServiceExport create or update event, reports the referencing service's EndpointSlice.
// For ServiceExport delete event, cleanup the previously reported EndpointSlice.
func (c *ServiceExportController) handleServiceExportEvent(serviceExportKey keys.FederatedKey) error {
	_, err := helper.GetObjectFromCache(c.RESTMapper, c.InformerManager, serviceExportKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return cleanupWorkWithServiceExportDelete(c.Client, serviceExportKey)
		}
		return err
	}

	// Even though the EndpointSlice will be synced when dealing with EndpointSlice events, thus the 'report' here may
	// be redundant, but it helps to avoid a corner case:
	// If skip report here, after ServiceExport deletion and re-creation, if no EndpointSlice changes, we didn't get a
	// change to sync.
	if err = c.reportEndpointSliceWithServiceExportCreate(serviceExportKey); err != nil {
		klog.Errorf("Failed to handle ServiceExport(%s) event, Error: %v",
			serviceExportKey.NamespaceKey(), err)
		return err
	}

	return nil
}

// handleEndpointSliceEvent syncs EndPointSlice objects to control-plane according to EndpointSlice event.
// For EndpointSlice create or update event, reports the EndpointSlice when referencing service has been exported.
// For EndpointSlice delete event, cleanup the previously reported EndpointSlice.
func (c *ServiceExportController) handleEndpointSliceEvent(endpointSliceKey keys.FederatedKey) error {
	endpointSliceObj, err := helper.GetObjectFromCache(c.RESTMapper, c.InformerManager, endpointSliceKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return cleanupWorkWithEndpointSliceDelete(c.Client, endpointSliceKey)
		}
		return err
	}

	if err = c.reportEndpointSliceWithEndpointSliceCreateOrUpdate(endpointSliceKey.Cluster, endpointSliceObj); err != nil {
		klog.Errorf("Failed to handle endpointSlice(%s) event, Error: %v",
			endpointSliceKey.NamespaceKey(), err)
		return err
	}

	return nil
}

// reportEndpointSliceWithServiceExportCreate reports the referencing service's EndpointSlice.
func (c *ServiceExportController) reportEndpointSliceWithServiceExportCreate(serviceExportKey keys.FederatedKey) error {
	var (
		endpointSliceObjects []runtime.Object
		err                  error
		errs                 []error
	)

	singleClusterManager := c.InformerManager.GetSingleClusterManager(serviceExportKey.Cluster)
	if singleClusterManager == nil {
		return nil
	}

	endpointSliceLister := singleClusterManager.Lister(endpointSliceGVR)
	if endpointSliceObjects, err = endpointSliceLister.ByNamespace(serviceExportKey.Namespace).List(labels.SelectorFromSet(labels.Set{
		discoveryv1.LabelServiceName: serviceExportKey.Name,
	})); err != nil {
		return err
	}

	for index := range endpointSliceObjects {
		if err = reportEndpointSlice(c.Client, endpointSliceObjects[index].(*unstructured.Unstructured), serviceExportKey.Cluster); err != nil {
			errs = append(errs, err)
		}
	}
	return utilerrors.NewAggregate(errs)
}

// reportEndpointSliceWithEndpointSliceCreateOrUpdate reports the EndpointSlice when referencing service has been exported.
func (c *ServiceExportController) reportEndpointSliceWithEndpointSliceCreateOrUpdate(clusterName string, endpointSlice *unstructured.Unstructured) error {
	relatedServiceName := endpointSlice.GetLabels()[discoveryv1.LabelServiceName]

	singleClusterManager := c.InformerManager.GetSingleClusterManager(clusterName)
	if singleClusterManager == nil {
		return nil
	}

	serviceExportLister := singleClusterManager.Lister(serviceExportGVR)
	_, err := serviceExportLister.ByNamespace(endpointSlice.GetNamespace()).Get(relatedServiceName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		klog.Errorf("Failed to get ServiceExport object %s/%s. error: %v.", relatedServiceName, endpointSlice.GetNamespace(), err)
		return err
	}

	return reportEndpointSlice(c.Client, endpointSlice, clusterName)
}

// reportEndpointSlice report EndPointSlice objects to control-plane.
func reportEndpointSlice(c client.Client, endpointSlice *unstructured.Unstructured, clusterName string) error {
	executionSpace, err := names.GenerateExecutionSpaceName(clusterName)
	if err != nil {
		return err
	}

	workMeta := metav1.ObjectMeta{
		Name:      names.GenerateWorkName(endpointSlice.GetKind(), endpointSlice.GetName(), endpointSlice.GetNamespace()),
		Namespace: executionSpace,
		Labels: map[string]string{
			util.ServiceNamespaceLabel: endpointSlice.GetNamespace(),
			util.ServiceNameLabel:      endpointSlice.GetLabels()[discoveryv1.LabelServiceName],
			// indicate the Work should be not propagated since it's collected resource.
			util.PropagationInstruction: util.PropagationInstructionSuppressed,
		},
	}

	if err = helper.CreateOrUpdateWork(c, workMeta, endpointSlice); err != nil {
		return err
	}

	return nil
}

func cleanupWorkWithServiceExportDelete(c client.Client, serviceExportKey keys.FederatedKey) error {
	executionSpace, err := names.GenerateExecutionSpaceName(serviceExportKey.Cluster)
	if err != nil {
		return err
	}

	workList := &workv1alpha1.WorkList{}
	if err = c.List(context.TODO(), workList, &client.ListOptions{
		Namespace: executionSpace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			util.ServiceNamespaceLabel: serviceExportKey.Namespace,
			util.ServiceNameLabel:      serviceExportKey.Name,
		}),
	}); err != nil {
		klog.Error("Failed to list workList reported by ServiceExport(%s) in namespace(%s), Error: %v",
			serviceExportKey.NamespaceKey(), executionSpace, err)
		return err
	}

	var errs []error
	for index, work := range workList.Items {
		if err = c.Delete(context.TODO(), &workList.Items[index]); err != nil {
			klog.Errorf("Failed to delete work(%s/%s), Error: %v", work.Namespace, work.Name, err)
			errs = append(errs, err)
		}
	}
	return utilerrors.NewAggregate(errs)
}

func cleanupWorkWithEndpointSliceDelete(c client.Client, endpointSliceKey keys.FederatedKey) error {
	executionSpace, err := names.GenerateExecutionSpaceName(endpointSliceKey.Cluster)
	if err != nil {
		return err
	}

	workNamespaceKey := types.NamespacedName{
		Namespace: executionSpace,
		Name:      names.GenerateWorkName(endpointSliceKey.Kind, endpointSliceKey.Name, endpointSliceKey.Namespace),
	}
	work := &workv1alpha1.Work{}
	if err = c.Get(context.TODO(), workNamespaceKey, work); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		klog.Error("Failed to get work(%s), Error: %v", workNamespaceKey, executionSpace, err)
		return err
	}

	if err = c.Delete(context.TODO(), work); err != nil {
		klog.Errorf("Failed to delete work(%s), Error: %v", workNamespaceKey, err)
		return err
	}

	return nil
}
