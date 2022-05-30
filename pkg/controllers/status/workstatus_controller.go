package status

import (
	"context"
	"fmt"
	"reflect"

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
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/informermanager/keys"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// WorkStatusControllerName is the controller name that will be used when reporting events.
const WorkStatusControllerName = "work-status-controller"

// WorkStatusController is to sync status of Work.
type WorkStatusController struct {
	client.Client   // used to operate Work resources.
	EventRecorder   record.EventRecorder
	RESTMapper      meta.RESTMapper
	InformerManager informermanager.MultiClusterInformerManager
	eventHandler    cache.ResourceEventHandler // eventHandler knows how to handle events from the member cluster.
	StopChan        <-chan struct{}
	worker          util.AsyncWorker // worker process resources periodic from rateLimitingQueue.
	// ConcurrentWorkStatusSyncs is the number of Work status that are allowed to sync concurrently.
	ConcurrentWorkStatusSyncs int
	ObjectWatcher             objectwatcher.ObjectWatcher
	PredicateFunc             predicate.Predicate
	ClusterClientSetFunc      func(clusterName string, client client.Client) (*util.DynamicClusterClient, error)
	ClusterCacheSyncTimeout   metav1.Duration
	RateLimiterOptions        ratelimiterflag.Options
	ResourceInterpreter       resourceinterpreter.ResourceInterpreter
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *WorkStatusController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling status of Work %s.", req.NamespacedName.String())

	work := &workv1alpha1.Work{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, work); err != nil {
		// The resource may no longer exist, in which case we stop processing.
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

	clusterName, err := names.GetClusterName(work.GetNamespace())
	if err != nil {
		klog.Errorf("Failed to get member cluster name by %s. Error: %v.", work.GetNamespace(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	cluster, err := util.GetCluster(c.Client, clusterName)
	if err != nil {
		klog.Errorf("Failed to the get given member cluster %s", clusterName)
		return controllerruntime.Result{Requeue: true}, err
	}

	if !util.IsClusterReady(&cluster.Status) {
		klog.Errorf("Stop sync work(%s/%s) for cluster(%s) as cluster not ready.", work.Namespace, work.Name, cluster.Name)
		return controllerruntime.Result{Requeue: true}, fmt.Errorf("cluster(%s) not ready", cluster.Name)
	}

	return c.buildResourceInformers(cluster, work)
}

// buildResourceInformers builds informer dynamically for managed resources in member cluster.
// The created informer watches resource change and then sync to the relevant Work object.
func (c *WorkStatusController) buildResourceInformers(cluster *clusterv1alpha1.Cluster, work *workv1alpha1.Work) (controllerruntime.Result, error) {
	err := c.registerInformersAndStart(cluster, work)
	if err != nil {
		klog.Errorf("Failed to register informer for Work %s/%s. Error: %v.", work.GetNamespace(), work.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// getEventHandler return callback function that knows how to handle events from the member cluster.
func (c *WorkStatusController) getEventHandler() cache.ResourceEventHandler {
	if c.eventHandler == nil {
		c.eventHandler = informermanager.NewHandlerOnAllEvents(c.worker.Enqueue)
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
	cluster, err := getClusterNameFromLabel(resource)
	if err != nil {
		return nil, err
	}
	// return a nil key when the obj not managed by Karmada, which will be discarded before putting to queue.
	if cluster == "" {
		return nil, nil
	}

	return keys.FederatedKeyFunc(cluster, obj)
}

// getClusterNameFromLabel gets cluster name from ownerLabel, if label not exist, means resource is not created by karmada.
func getClusterNameFromLabel(resource *unstructured.Unstructured) (string, error) {
	workNamespace := util.GetLabelValue(resource.GetLabels(), workv1alpha1.WorkNamespaceLabel)
	if len(workNamespace) == 0 {
		klog.V(4).Infof("Ignore resource(%s/%s/%s) which not managed by karmada", resource.GetKind(), resource.GetNamespace(), resource.GetName())
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
	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		klog.Errorf("Failed to sync status as invalid key: %v", key)
		return fmt.Errorf("invalid key")
	}

	observedObj, err := helper.GetObjectFromCache(c.RESTMapper, c.InformerManager, fedKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return c.handleDeleteEvent(fedKey)
		}
		return err
	}

	workNamespace := util.GetLabelValue(observedObj.GetLabels(), workv1alpha1.WorkNamespaceLabel)
	workName := util.GetLabelValue(observedObj.GetLabels(), workv1alpha1.WorkNameLabel)
	if len(workNamespace) == 0 || len(workName) == 0 {
		klog.Infof("Ignore object(%s) which not managed by karmada.", fedKey.String())
		return nil
	}

	workObject := &workv1alpha1.Work{}
	if err := c.Client.Get(context.TODO(), client.ObjectKey{Namespace: workNamespace, Name: workName}, workObject); err != nil {
		// Stop processing if resource no longer exist.
		if apierrors.IsNotFound(err) {
			return nil
		}

		klog.Errorf("Failed to get Work(%s/%s) from cache: %v", workNamespace, workName, err)
		return err
	}

	desiredObj, err := c.getRawManifest(workObject.Spec.Workload.Manifests, observedObj)
	if err != nil {
		return err
	}

	clusterName, err := names.GetClusterName(workNamespace)
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
		if err := c.ObjectWatcher.Update(clusterName, desiredObj, observedObj); err != nil {
			klog.Errorf("Update %s failed: %v", fedKey.String(), err)
			return err
		}
		// We can't return even after a success updates, because that might lose the chance to collect status.
		// Not all updates are real, they might be no change, in that case there will be no more event for this update,
		// this usually happens with those resources not enables 'metadata.generation', like 'Service'.
		// When a Service's status changes, it's 'metadata.resourceVersion' will be increased, but 'metadata.generation'
		// not increased(defaults to 0), the ObjectWatcher can't easily tell what happened to the object, so ObjectWatcher
		// also needs to update again. The update operation will be a non-operation if the event triggered by Service's
		// status changes.
	}

	klog.Infof("reflecting %s(%s/%s) status to Work(%s/%s)", observedObj.GetKind(), observedObj.GetNamespace(), observedObj.GetName(), workNamespace, workName)
	return c.reflectStatus(workObject, observedObj)
}

func (c *WorkStatusController) handleDeleteEvent(key keys.FederatedKey) error {
	executionSpace, err := names.GenerateExecutionSpaceName(key.Cluster)
	if err != nil {
		return err
	}

	// Given the workload might has been deleted from informer cache, so that we can't get work object by it's label,
	// we have to get work by naming rule as the work's name is generated by the workload's kind, name and namespace.
	workName := names.GenerateWorkName(key.Kind, key.Name, key.Namespace)
	work := &workv1alpha1.Work{}
	if err := c.Client.Get(context.TODO(), client.ObjectKey{Namespace: executionSpace, Name: workName}, work); err != nil {
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

	return c.recreateResourceIfNeeded(work, key)
}

func (c *WorkStatusController) recreateResourceIfNeeded(work *workv1alpha1.Work, workloadKey keys.FederatedKey) error {
	for _, rawManifest := range work.Spec.Workload.Manifests {
		manifest := &unstructured.Unstructured{}
		if err := manifest.UnmarshalJSON(rawManifest.Raw); err != nil {
			return err
		}

		desiredGVK := schema.FromAPIVersionAndKind(manifest.GetAPIVersion(), manifest.GetKind())
		if reflect.DeepEqual(desiredGVK, workloadKey.GroupVersionKind()) &&
			manifest.GetNamespace() == workloadKey.Namespace &&
			manifest.GetName() == workloadKey.Name {
			klog.Infof("recreating %s", workloadKey.String())
			return c.ObjectWatcher.Create(workloadKey.Cluster, manifest)
		}
	}
	return nil
}

// reflectStatus grabs cluster object's running status then updates to its owner object(Work).
func (c *WorkStatusController) reflectStatus(work *workv1alpha1.Work, clusterObj *unstructured.Unstructured) error {
	statusRaw, err := c.ResourceInterpreter.ReflectStatus(clusterObj)
	if err != nil {
		klog.Errorf("Failed to reflect status for object(%s/%s/%s) with resourceInterpreter.",
			clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName(), err)
	}

	if statusRaw == nil {
		return nil
	}

	identifier, err := c.buildStatusIdentifier(work, clusterObj)
	if err != nil {
		return err
	}

	manifestStatus := workv1alpha1.ManifestStatus{
		Identifier: *identifier,
		Status:     statusRaw,
	}

	workCopy := work.DeepCopy()
	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		manifestStatuses := c.mergeStatus(workCopy.Status.ManifestStatuses, manifestStatus)
		if reflect.DeepEqual(workCopy.Status.ManifestStatuses, manifestStatuses) {
			return nil
		}
		workCopy.Status.ManifestStatuses = manifestStatuses
		updateErr := c.Status().Update(context.TODO(), workCopy)
		if updateErr == nil {
			return nil
		}

		updated := &workv1alpha1.Work{}
		if err = c.Get(context.TODO(), client.ObjectKey{Namespace: workCopy.Namespace, Name: workCopy.Name}, updated); err == nil {
			//make a copy, so we don't mutate the shared cache
			workCopy = updated.DeepCopy()
		} else {
			klog.Errorf("failed to get updated work %s/%s: %v", workCopy.Namespace, workCopy.Name, err)
		}
		return updateErr
	})
}

func (c *WorkStatusController) buildStatusIdentifier(work *workv1alpha1.Work, clusterObj *unstructured.Unstructured) (*workv1alpha1.ResourceIdentifier, error) {
	ordinal, err := helper.GetManifestIndex(work.Spec.Workload.Manifests, clusterObj)
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

func (c *WorkStatusController) mergeStatus(statuses []workv1alpha1.ManifestStatus, newStatus workv1alpha1.ManifestStatus) []workv1alpha1.ManifestStatus {
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
func (c *WorkStatusController) getSingleClusterManager(cluster *clusterv1alpha1.Cluster) (informermanager.SingleClusterInformerManager, error) {
	// TODO(chenxianpao): If cluster A is removed, then a new cluster that name also is A joins karmada,
	//  the cache in informer manager should be updated.
	singleClusterInformerManager := c.InformerManager.GetSingleClusterManager(cluster.Name)
	if singleClusterInformerManager == nil {
		dynamicClusterClient, err := c.ClusterClientSetFunc(cluster.Name, c.Client)
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
		For(&workv1alpha1.Work{}, builder.WithPredicates(c.PredicateFunc)).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions),
		}).Complete(c)
}
