package status

import (
	"context"
	"encoding/json"
	"fmt"
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

	"github.com/karmada-io/karmada/pkg/apis/propagationstrategy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// WorkStatusControllerName is the controller name that will be used when reporting events.
const WorkStatusControllerName = "work-status-controller"

// PropagationWorkStatusController is to sync status of PropagationWork.
type PropagationWorkStatusController struct {
	client.Client                     // used to operate PropagationWork resources.
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
func (c *PropagationWorkStatusController) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling status of PropagationWork %s.", req.NamespacedName.String())

	work := &v1alpha1.PropagationWork{}
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
// The created informer watches resource change and then sync to the relevant PropagationWork object.
func (c *PropagationWorkStatusController) buildResourceInformers(work *v1alpha1.PropagationWork) (controllerruntime.Result, error) {
	err := c.registerInformersAndStart(work)
	if err != nil {
		klog.Errorf("Failed to register informer for propagationWork %s/%s. Error: %v.", work.GetNamespace(), work.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// getEventHandler return callback function that knows how to handle events from the member cluster.
func (c *PropagationWorkStatusController) getEventHandler() cache.ResourceEventHandler {
	if c.eventHandler == nil {
		c.eventHandler = informermanager.NewHandlerOnAllEvents(c.worker.EnqueueRateLimited)
	}
	return c.eventHandler
}

// RunWorkQueue initializes worker and run it, worker will process resource asynchronously.
func (c *PropagationWorkStatusController) RunWorkQueue() {
	c.worker = util.NewAsyncWorker(c.syncPropagationWorkStatus, "work-status", time.Second)
	c.worker.Run(c.WorkerNumber, c.StopChan)
}

// syncPropagationWorkStatus will find propagationWork by label in workload, then update resource status to propagationWork status.
// label example: "karmada.io/created-by: karmada-es-member-cluster-1.default-deployment-nginx"
// TODO(chenxianpao): sync workload status to propagationWork status.
func (c *PropagationWorkStatusController) syncPropagationWorkStatus(key string) error {
	// TODO(RainbowMango): we can't get object from cache for delete event, in that case the key will be throw into the
	// worker queue until exceed the maxRetries. We should improve this scenario after 'version manager' get on board,
	// which should knows if this remove event is expected.
	obj, err := c.getObjectFromCache(key)
	if err != nil {
		return err
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

	workObject := &v1alpha1.PropagationWork{}
	if err := c.Client.Get(context.TODO(), client.ObjectKey{Namespace: ownerNamespace, Name: ownerName}, workObject); err != nil {
		// Stop processing if resource no longer exist.
		if errors.IsNotFound(err) {
			return nil
		}

		klog.Errorf("Failed to get PropagationWork(%s/%s) from cache: %v", ownerNamespace, ownerName, err)
		return err
	}

	// consult with version manager if current status needs update.
	desireObj, err := c.getRawManifest(workObject.Spec.Workload.Manifests, obj)
	if err != nil {
		return err
	}

	util.MergeLabel(desireObj, util.OwnerLabel, names.GenerateOwnerLabelValue(workObject.GetNamespace(), workObject.GetName()))

	clusterName, err := names.GetMemberClusterName(ownerNamespace)
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

	klog.Infof("reflecting %s(%s/%s) status of to PropagationWork(%s/%s)", obj.GetKind(), obj.GetNamespace(), obj.GetName(), ownerNamespace, ownerName)
	return c.reflectStatus(workObject, obj)
}

// reflectStatus grabs cluster object's running status then updates to it's owner object(PropagationWork).
func (c *PropagationWorkStatusController) reflectStatus(work *v1alpha1.PropagationWork, clusterObj *unstructured.Unstructured) error {
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

func (c *PropagationWorkStatusController) buildStatusIdentifier(work *v1alpha1.PropagationWork, clusterObj *unstructured.Unstructured) (*v1alpha1.ResourceIdentifier, error) {
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
		// TODO(RainbowMango): Consider merge Group and Version to APIVersion from PropagationWork API.
		Group:   groupVersion.Group,
		Version: groupVersion.Version,
		Kind:    clusterObj.GetKind(),
		// TODO(RainbowMango): Consider remove Resource from PropagationWork API.
		Resource:  "", // we don't need this fields.
		Namespace: clusterObj.GetNamespace(),
		Name:      clusterObj.GetName(),
	}

	return identifier, nil
}

func (c *PropagationWorkStatusController) buildStatusRawExtension(status map[string]interface{}) (*runtime.RawExtension, error) {
	statusJSON, err := json.Marshal(status)
	if err != nil {
		klog.Errorf("Failed to marshal status. Error: %v.", statusJSON)
		return nil, err
	}

	return &runtime.RawExtension{
		Raw: statusJSON,
	}, nil
}

func (c *PropagationWorkStatusController) mergeStatus(statuses []v1alpha1.ManifestStatus, newStatus v1alpha1.ManifestStatus) []v1alpha1.ManifestStatus {
	// TODO(RainbowMango): update 'statuses' if 'newStatus' already exist.
	// For now, we only have at most one manifest in PropagationWork, so just override current 'statuses'.
	return []v1alpha1.ManifestStatus{newStatus}
}

func (c *PropagationWorkStatusController) getManifestIndex(manifests []v1alpha1.Manifest, clusterObj *unstructured.Unstructured) (int, error) {
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

func (c *PropagationWorkStatusController) getRawManifest(manifests []v1alpha1.Manifest, clusterObj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
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
func (c *PropagationWorkStatusController) getObjectFromCache(key string) (*unstructured.Unstructured, error) {
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

	lister := c.InformerManager.GetSingleClusterManager(clusterWorkload.Cluster).Lister(gvr)
	var obj runtime.Object
	obj, err = lister.Get(clusterWorkload.GetListerKey())
	if err != nil {
		klog.Errorf("Failed to get obj %s/%s/%s from cache in cluster %s. Error: %v.", clusterWorkload.GVK.Kind,
			clusterWorkload.Namespace, clusterWorkload.Name, clusterWorkload.Cluster, err)
		return nil, err
	}
	return obj.(*unstructured.Unstructured), nil
}

// registerInformersAndStart builds informer manager for cluster if it doesn't exist, then constructs informers for gvr
// and start it.
func (c *PropagationWorkStatusController) registerInformersAndStart(work *v1alpha1.PropagationWork) error {
	memberClusterName, err := names.GetMemberClusterName(work.GetNamespace())
	if err != nil {
		klog.Errorf("Failed to get member cluster name by %s. Error: %v.", work.GetNamespace(), err)
		return err
	}

	singleClusterInformerManager, err := c.getSingleClusterManager(memberClusterName)
	if err != nil {
		return err
	}

	gvrTargets, err := c.getGVRsFromPropagationWork(work)
	if err != nil {
		return err
	}

	for gvr := range gvrTargets {
		singleClusterInformerManager.ForResource(gvr, c.getEventHandler())
	}

	c.InformerManager.Start(memberClusterName, c.StopChan)
	synced := c.InformerManager.WaitForCacheSync(memberClusterName, c.StopChan)
	if synced == nil {
		klog.Errorf("No informerFactory for cluster %s exist.", memberClusterName)
		return fmt.Errorf("no informerFactory for cluster %s exist", memberClusterName)
	}
	for gvr := range gvrTargets {
		if !synced[gvr] {
			klog.Errorf("Informer for %s hasn't synced.", gvr)
			return fmt.Errorf("informer for %s hasn't synced", gvr)
		}
	}
	return nil
}

// getGVRsFromPropagationWork traverses the manifests in propagationWork to find groupVersionResource list.
func (c *PropagationWorkStatusController) getGVRsFromPropagationWork(work *v1alpha1.PropagationWork) (map[schema.GroupVersionResource]bool, error) {
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
func (c *PropagationWorkStatusController) getSingleClusterManager(memberClusterName string) (informermanager.SingleClusterInformerManager, error) {
	// TODO(chenxianpao): If cluster A is removed, then a new cluster that name also is A joins karmada,
	//  the cache in informer manager should be updated.
	singleClusterInformerManager := c.InformerManager.GetSingleClusterManager(memberClusterName)
	if singleClusterInformerManager == nil {
		dynamicClusterClient, err := util.BuildDynamicClusterClient(c.Client, c.KubeClientSet, memberClusterName)
		if err != nil {
			klog.Errorf("Failed to build dynamic cluster client for cluster %s.", memberClusterName)
			return nil, err
		}
		singleClusterInformerManager = c.InformerManager.ForCluster(dynamicClusterClient.ClusterName, dynamicClusterClient.DynamicClientSet, 0)
	}
	return singleClusterInformerManager, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *PropagationWorkStatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&v1alpha1.PropagationWork{}).Complete(c)
}
