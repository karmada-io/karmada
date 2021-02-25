package status

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// WorkStatusControllerName is the controller name that will be used when reporting events.
const WorkStatusControllerName = "work-status-controller"

// WorkStatusController is to sync status of Work.
type WorkStatusController struct {
	client.Client                     // used to operate Work resources.
	DynamicClient   dynamic.Interface // used to fetch arbitrary resources.
	EventRecorder   record.EventRecorder
	RESTMapper      meta.RESTMapper
	KubeClientSet   kubernetes.Interface // used to get kubernetes resources.
	InformerManager informermanager.MultiClusterInformerManager
	eventHandler    cache.ResourceEventHandler // eventHandler knows how to handle events from the member cluster.
	StopChan        <-chan struct{}
	WorkerNumber    int              // WorkerNumber is the number of worker goroutines
	worker          util.AsyncWorker // worker process resources periodic from rateLimitingQueue.
	ObjectWatcher   objectwatcher.ObjectWatcher
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *WorkStatusController) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling status of Work %s.", req.NamespacedName.String())

	work := &v1alpha1.Work{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, work); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !work.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	return c.buildResourceInformers(work)
}

// buildResourceInformers builds informer dynamically for managed resources in member cluster.
// The created informer watches resource change and then sync to the relevant Work object.
func (c *WorkStatusController) buildResourceInformers(work *v1alpha1.Work) (controllerruntime.Result, error) {
	err := c.registerInformersAndStart(work)
	if err != nil {
		klog.Errorf("Failed to register informer for Work %s/%s. Error: %v.", work.GetNamespace(), work.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// getEventHandler return callback function that knows how to handle events from the member cluster.
func (c *WorkStatusController) getEventHandler() cache.ResourceEventHandler {
	if c.eventHandler == nil {
		c.eventHandler = informermanager.NewHandlerOnAllEvents(c.worker.EnqueueRateLimited)
	}
	return c.eventHandler
}

// RunWorkQueue initializes worker and run it, worker will process resource asynchronously.
func (c *WorkStatusController) RunWorkQueue() {
	c.worker = util.NewAsyncWorker(c.syncWorkStatus, "work-status", time.Second)
	c.worker.Run(c.WorkerNumber, c.StopChan)
}

// syncWorkStatus will find work by label in workload, then update resource status to work status.
// label example: "karmada.io/created-by: karmada-es-member-cluster-1.default-deployment-nginx"
func (c *WorkStatusController) syncWorkStatus(key string) error {
	obj, err := c.getObjectFromCache(key)
	if err != nil {
		if errors.IsNotFound(err) {
			return c.handleDeleteEvent(key)
		}
		return err
	}

	if errors.IsNotFound(err) {
		return c.handleDeleteEvent(key)
	}
	if obj == nil {
		// Ignore the object which not managed by current karmada.
		klog.V(2).Infof("Ignore the event key %s which not managed by karmada.", key)
		return nil
	}
	owner := util.GetLabelValue(obj.GetLabels(), util.OwnerLabel)
	if len(owner) == 0 {
		// Ignore the object which not managed by karmada.
		// TODO(RainbowMango): Consider to add event filter to informer event handler to skip event from enqueue.
		klog.V(2).Infof("Ignore the event of %s(%s/%s) which not managed by karmada.", obj.GetKind(), obj.GetNamespace(), obj.GetName())
		return nil
	}

	ownerNamespace, ownerName, err := names.GetNamespaceAndName(owner)
	if err != nil {
		klog.Errorf("Failed to parse object(%s/%s) owner by label: %s", obj.GetNamespace(), obj.GetName(), owner)
		return err
	}

	workObject := &v1alpha1.Work{}
	if err := c.Client.Get(context.TODO(), client.ObjectKey{Namespace: ownerNamespace, Name: ownerName}, workObject); err != nil {
		// Stop processing if resource no longer exist.
		if errors.IsNotFound(err) {
			return nil
		}

		klog.Errorf("Failed to get Work(%s/%s) from cache: %v", ownerNamespace, ownerName, err)
		return err
	}

	// consult with version manager if current status needs update.
	desireObj, err := c.getRawManifest(workObject.Spec.Workload.Manifests, obj)
	if err != nil {
		return err
	}

	util.MergeLabel(desireObj, util.OwnerLabel, names.GenerateOwnerLabelValue(workObject.GetNamespace(), workObject.GetName()))

	clusterName, err := names.GetClusterName(ownerNamespace)
	if err != nil {
		klog.Errorf("Failed to get member cluster name: %v", err)
		return err
	}

	// compare version to determine if need to update resource
	needUpdate, err := c.ObjectWatcher.NeedsUpdate(clusterName, desireObj, obj)
	if err != nil {
		return err
	}

	if needUpdate {
		return c.ObjectWatcher.Update(clusterName, desireObj, obj)
	}

	klog.Infof("reflecting %s(%s/%s) status of to Work(%s/%s)", obj.GetKind(), obj.GetNamespace(), obj.GetName(), ownerNamespace, ownerName)
	return c.reflectStatus(workObject, obj)
}

func (c *WorkStatusController) handleDeleteEvent(key string) error {
	clusterWorkload, err := util.SplitMetaKey(key)
	if err != nil {
		klog.Errorf("Couldn't get key for %s. Error: %v.", key, err)
		return err
	}

	executionSpace, err := names.GenerateExecutionSpaceName(clusterWorkload.Cluster)
	if err != nil {
		return err
	}

	workName := names.GenerateBindingName(clusterWorkload.Namespace, clusterWorkload.GVK.Kind, clusterWorkload.Name)
	work := &v1alpha1.Work{}
	if err := c.Client.Get(context.TODO(), client.ObjectKey{Namespace: executionSpace, Name: workName}, work); err != nil {
		// Stop processing if resource no longer exist.
		if errors.IsNotFound(err) {
			klog.Infof("workload %v/%v not found", executionSpace, workName)
			return nil
		}

		klog.Errorf("Failed to get Work from cache: %v", err)
		return err
	}

	if !work.DeletionTimestamp.IsZero() {
		klog.Infof("resource %v/%v/%v in member cluster %v does not need to recreate", clusterWorkload.GVK.Kind, clusterWorkload.Namespace, clusterWorkload.Name, clusterWorkload.Cluster)
		return nil
	}

	return c.recreateResourceIfNeeded(work, clusterWorkload)
}

func (c *WorkStatusController) recreateResourceIfNeeded(work *v1alpha1.Work, clusterWorkload util.ClusterWorkload) error {
	for _, rawManifest := range work.Spec.Workload.Manifests {
		manifest := &unstructured.Unstructured{}
		if err := manifest.UnmarshalJSON(rawManifest.Raw); err != nil {
			return err
		}

		desiredGVK := schema.FromAPIVersionAndKind(manifest.GetAPIVersion(), manifest.GetKind())
		if reflect.DeepEqual(desiredGVK, clusterWorkload.GVK) &&
			manifest.GetNamespace() == clusterWorkload.Namespace &&
			manifest.GetName() == clusterWorkload.Name {

			util.MergeLabel(manifest, util.OwnerLabel, names.GenerateOwnerLabelValue(work.GetNamespace(), work.GetName()))

			klog.Infof("recreating %s/%s/%s in member cluster %s", clusterWorkload.GVK.Kind, clusterWorkload.Namespace, clusterWorkload.Name, clusterWorkload.Cluster)
			return c.ObjectWatcher.Create(clusterWorkload.Cluster, manifest)
		}
	}
	return nil
}

// reflectStatus grabs cluster object's running status then updates to it's owner object(Work).
func (c *WorkStatusController) reflectStatus(work *v1alpha1.Work, clusterObj *unstructured.Unstructured) error {
	// Stop processing if resource(such as ConfigMap,Secret,ClusterRole, etc.) doesn't contain 'spec.status' fields.
	statusMap, exist, err := unstructured.NestedMap(clusterObj.Object, "status")
	if err != nil {
		klog.Errorf("Failed to get status field from %s(%s/%s), error: %v", clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName(), err)
		return err
	}
	if !exist || statusMap == nil {
		klog.V(2).Infof("Ignore resources(%s) without status.", clusterObj.GetKind())
		return nil
	}

	identifier, err := c.buildStatusIdentifier(work, clusterObj)
	if err != nil {
		return err
	}

	rawExtension, err := c.buildStatusRawExtension(statusMap)
	if err != nil {
		return err
	}

	manifestStatus := v1alpha1.ManifestStatus{
		Identifier: *identifier,
		Status:     *rawExtension,
	}

	work.Status.ManifestStatuses = c.mergeStatus(work.Status.ManifestStatuses, manifestStatus)

	return c.Client.Status().Update(context.TODO(), work)
}

func (c *WorkStatusController) buildStatusIdentifier(work *v1alpha1.Work, clusterObj *unstructured.Unstructured) (*v1alpha1.ResourceIdentifier, error) {
	ordinal, err := c.getManifestIndex(work.Spec.Workload.Manifests, clusterObj)
	if err != nil {
		return nil, err
	}

	groupVersion, err := schema.ParseGroupVersion(clusterObj.GetAPIVersion())
	if err != nil {
		return nil, err
	}

	identifier := &v1alpha1.ResourceIdentifier{
		Ordinal: ordinal,
		// TODO(RainbowMango): Consider merge Group and Version to APIVersion from Work API.
		Group:   groupVersion.Group,
		Version: groupVersion.Version,
		Kind:    clusterObj.GetKind(),
		// TODO(RainbowMango): Consider remove Resource from Work API.
		Resource:  "", // we don't need this fields.
		Namespace: clusterObj.GetNamespace(),
		Name:      clusterObj.GetName(),
	}

	return identifier, nil
}

func (c *WorkStatusController) buildStatusRawExtension(status map[string]interface{}) (*runtime.RawExtension, error) {
	statusJSON, err := json.Marshal(status)
	if err != nil {
		klog.Errorf("Failed to marshal status. Error: %v.", statusJSON)
		return nil, err
	}

	return &runtime.RawExtension{
		Raw: statusJSON,
	}, nil
}

func (c *WorkStatusController) mergeStatus(statuses []v1alpha1.ManifestStatus, newStatus v1alpha1.ManifestStatus) []v1alpha1.ManifestStatus {
	// TODO(RainbowMango): update 'statuses' if 'newStatus' already exist.
	// For now, we only have at most one manifest in Work, so just override current 'statuses'.
	return []v1alpha1.ManifestStatus{newStatus}
}

func (c *WorkStatusController) getManifestIndex(manifests []v1alpha1.Manifest, clusterObj *unstructured.Unstructured) (int, error) {
	for index, rawManifest := range manifests {
		manifest := &unstructured.Unstructured{}
		if err := manifest.UnmarshalJSON(rawManifest.Raw); err != nil {
			return -1, err
		}

		if manifest.GetAPIVersion() == clusterObj.GetAPIVersion() &&
			manifest.GetKind() == clusterObj.GetKind() &&
			manifest.GetNamespace() == clusterObj.GetNamespace() &&
			manifest.GetName() == clusterObj.GetName() {
			return index, nil
		}
	}

	return -1, fmt.Errorf("no such manifest exist")
}

func (c *WorkStatusController) getRawManifest(manifests []v1alpha1.Manifest, clusterObj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
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

// getObjectFromCache gets full object information from cache by key in worker queue.
func (c *WorkStatusController) getObjectFromCache(key string) (*unstructured.Unstructured, error) {
	clusterWorkload, err := util.SplitMetaKey(key)
	if err != nil {
		klog.Errorf("Couldn't get key for %s. Error: %v.", key, err)
		return nil, err
	}
	gvr, err := restmapper.GetGroupVersionResource(c.RESTMapper, clusterWorkload.GVK)
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK %s. Error: %v", clusterWorkload.GVK, err)
		return nil, err
	}

	singleClusterManager := c.InformerManager.GetSingleClusterManager(clusterWorkload.Cluster)
	if singleClusterManager == nil {
		return nil, nil
	}
	var obj runtime.Object
	lister := singleClusterManager.Lister(gvr)
	obj, err = lister.Get(clusterWorkload.GetListerKey())
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, err
		}

		// print logs only for real error.
		klog.Errorf("Failed to get obj %s/%s/%s from cache in cluster %s. error: %v.", clusterWorkload.GVK.Kind,
			clusterWorkload.Namespace, clusterWorkload.Name, clusterWorkload.Cluster, err)

		return nil, err
	}
	return obj.(*unstructured.Unstructured), nil
}

// registerInformersAndStart builds informer manager for cluster if it doesn't exist, then constructs informers for gvr
// and start it.
func (c *WorkStatusController) registerInformersAndStart(work *v1alpha1.Work) error {
	clusterName, err := names.GetClusterName(work.GetNamespace())
	if err != nil {
		klog.Errorf("Failed to get member cluster name by %s. Error: %v.", work.GetNamespace(), err)
		return err
	}

	singleClusterInformerManager, err := c.getSingleClusterManager(clusterName)
	if err != nil {
		return err
	}

	gvrTargets, err := c.getGVRsFromWork(work)
	if err != nil {
		return err
	}

	for gvr := range gvrTargets {
		singleClusterInformerManager.ForResource(gvr, c.getEventHandler())
	}

	c.InformerManager.Start(clusterName, c.StopChan)
	synced := c.InformerManager.WaitForCacheSync(clusterName, c.StopChan)
	if synced == nil {
		klog.Errorf("No informerFactory for cluster %s exist.", clusterName)
		return fmt.Errorf("no informerFactory for cluster %s exist", clusterName)
	}
	for gvr := range gvrTargets {
		if !synced[gvr] {
			klog.Errorf("Informer for %s hasn't synced.", gvr)
			return fmt.Errorf("informer for %s hasn't synced", gvr)
		}
	}
	return nil
}

// getGVRsFromWork traverses the manifests in work to find groupVersionResource list.
func (c *WorkStatusController) getGVRsFromWork(work *v1alpha1.Work) (map[schema.GroupVersionResource]bool, error) {
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
func (c *WorkStatusController) getSingleClusterManager(clusterName string) (informermanager.SingleClusterInformerManager, error) {
	// TODO(chenxianpao): If cluster A is removed, then a new cluster that name also is A joins karmada,
	//  the cache in informer manager should be updated.
	singleClusterInformerManager := c.InformerManager.GetSingleClusterManager(clusterName)
	if singleClusterInformerManager == nil {
		dynamicClusterClient, err := util.BuildDynamicClusterClient(c.Client, c.KubeClientSet, clusterName)
		if err != nil {
			klog.Errorf("Failed to build dynamic cluster client for cluster %s.", clusterName)
			return nil, err
		}
		singleClusterInformerManager = c.InformerManager.ForCluster(dynamicClusterClient.ClusterName, dynamicClusterClient.DynamicClientSet, 0)
	}
	return singleClusterInformerManager, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *WorkStatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&v1alpha1.Work{}).Complete(c)
}
