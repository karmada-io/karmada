package status

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/component-helpers/apimachinery/lease"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/informermanager/keys"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName            = "cluster-status-controller"
	clusterReady              = "ClusterReady"
	clusterHealthy            = "cluster is healthy and ready to accept workloads"
	clusterNotReady           = "ClusterNotReady"
	clusterUnhealthy          = "cluster is reachable but health endpoint responded without ok"
	clusterNotReachableReason = "ClusterNotReachable"
	clusterNotReachableMsg    = "cluster is not reachable"
	statusCollectionFailed    = "StatusCollectionFailed"
)

var (
	nodeGVR = corev1.SchemeGroupVersion.WithResource("nodes")
	podGVR  = corev1.SchemeGroupVersion.WithResource("pods")
)

// ClusterStatusController is to sync status of Cluster.
type ClusterStatusController struct {
	client.Client               // used to operate Cluster resources.
	KubeClient                  clientset.Interface
	EventRecorder               record.EventRecorder
	PredicateFunc               predicate.Predicate
	InformerManager             informermanager.MultiClusterInformerManager
	StopChan                    <-chan struct{}
	ClusterClientSetFunc        func(string, client.Client, *util.ClientOption) (*util.ClusterClient, error)
	ClusterDynamicClientSetFunc func(clusterName string, client client.Client) (*util.DynamicClusterClient, error)
	// ClusterClientOption holds the attributes that should be injected to a Kubernetes client.
	ClusterClientOption *util.ClientOption

	// ClusterStatusUpdateFrequency is the frequency that controller computes and report cluster status.
	ClusterStatusUpdateFrequency metav1.Duration
	// ClusterLeaseDuration is a duration that candidates for a lease need to wait to force acquire it.
	// This is measure against time of last observed lease RenewTime.
	ClusterLeaseDuration metav1.Duration
	// ClusterLeaseRenewIntervalFraction is a fraction coordinated with ClusterLeaseDuration that
	// how long the current holder of a lease has last updated the lease.
	ClusterLeaseRenewIntervalFraction float64
	// ClusterLeaseControllers store clusters and their corresponding lease controllers.
	ClusterLeaseControllers sync.Map
	// ClusterSuccessThreshold is the duration of successes for the cluster to be considered healthy after recovery.
	ClusterSuccessThreshold metav1.Duration
	// ClusterFailureThreshold is the duration of failure for the cluster to be considered unhealthy.
	ClusterFailureThreshold metav1.Duration
	// clusterConditionCache stores the condition status of each cluster.
	clusterConditionCache clusterConditionStore

	ClusterCacheSyncTimeout metav1.Duration
	RateLimiterOptions      ratelimiterflag.Options

	WorkerNumber int // WorkerNumber is the number of worker goroutines
	// eventHandlers holds the handlers which used to handle events reported from member clusters.
	// Each handler takes the cluster name as key and takes the handler function as the value, e.g.
	// "member1": instance of ResourceEventHandler
	eventHandlers sync.Map
	// worker maintains a rate limited queue which used to store the key of node/pod and
	// a reconcile function to consume the items in queue.
	worker              util.AsyncWorker
	RESTMapper          meta.RESTMapper
	ClusterSummaryCache ClusterSummaryCache
}

// Reconcile syncs status of the given member cluster.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will requeue the reconcile key after the duration.
func (c *ClusterStatusController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Syncing cluster status: %s", req.NamespacedName.Name)

	cluster := &clusterv1alpha1.Cluster{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, cluster); err != nil {
		// The resource may no longer exist, in which case we stop the informer.
		if apierrors.IsNotFound(err) {
			c.InformerManager.Stop(req.NamespacedName.Name)
			c.clusterConditionCache.delete(req.Name)
			c.ClusterSummaryCache.deleteCluster(req.Name)
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	// start syncing status only when the finalizer is present on the given Cluster to
	// avoid conflict with cluster controller.
	if !controllerutil.ContainsFinalizer(cluster, util.ClusterControllerFinalizer) {
		klog.V(2).Infof("waiting finalizer present for member cluster: %s", cluster.Name)
		return controllerruntime.Result{Requeue: true}, nil
	}

	return c.syncClusterStatus(cluster)
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ClusterStatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	c.clusterConditionCache = clusterConditionStore{
		successThreshold: c.ClusterSuccessThreshold.Duration,
		failureThreshold: c.ClusterFailureThreshold.Duration,
	}
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&clusterv1alpha1.Cluster{}, builder.WithPredicates(c.PredicateFunc)).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions),
		}).Complete(c)
}

func (c *ClusterStatusController) syncClusterStatus(cluster *clusterv1alpha1.Cluster) (controllerruntime.Result, error) {
	currentClusterStatus := *cluster.Status.DeepCopy()

	// create a ClusterClient for the given member cluster
	clusterClient, err := c.ClusterClientSetFunc(cluster.Name, c.Client, c.ClusterClientOption)
	if err != nil {
		klog.Errorf("Failed to create a ClusterClient for the given member cluster: %v, err is : %v", cluster.Name, err)
		return c.setStatusCollectionFailedCondition(cluster, currentClusterStatus, fmt.Sprintf("failed to create a ClusterClient: %v", err))
	}

	online, healthy := getClusterHealthStatus(clusterClient)
	observedReadyCondition := generateReadyCondition(online, healthy)
	readyCondition := c.clusterConditionCache.thresholdAdjustedReadyCondition(cluster, &observedReadyCondition)

	// cluster is offline after retry timeout, update cluster status immediately and return.
	if !online && readyCondition.Status != metav1.ConditionTrue {
		klog.V(2).Infof("Cluster(%s) still offline after %s, ensuring offline is set.",
			cluster.Name, c.ClusterFailureThreshold.Duration)
		c.InformerManager.Stop(cluster.Name)
		setTransitionTime(cluster.Status.Conditions, readyCondition)
		meta.SetStatusCondition(&currentClusterStatus.Conditions, *readyCondition)
		return c.updateStatusIfNeeded(cluster, currentClusterStatus)
	}

	// skip collecting cluster status if not ready
	if online && healthy && readyCondition.Status == metav1.ConditionTrue {
		// get or create informer for pods and nodes in member cluster
		clusterInformerManager, err := c.buildInformerForCluster(cluster)
		if err != nil {
			klog.Errorf("Failed to get or create informer for Cluster %s. Error: %v.", cluster.GetName(), err)
		}

		// init the lease controller for every cluster
		c.initLeaseController(clusterInformerManager.Context(), cluster)

		clusterVersion, err := getKubernetesVersion(clusterClient)
		if err != nil {
			klog.Errorf("Failed to get Kubernetes version for Cluster %s. Error: %v.", cluster.GetName(), err)
		}

		// get the list of APIs installed in the member cluster
		apiEnables, err := getAPIEnablements(clusterClient)
		if len(apiEnables) == 0 {
			klog.Errorf("Failed to get any APIs installed in Cluster %s. Error: %v.", cluster.GetName(), err)
		} else if err != nil {
			klog.Warningf("Maybe get partial(%d) APIs installed in Cluster %s. Error: %v.", len(apiEnables), cluster.GetName(), err)
		}

		nodeSummary, clusterAllocatable := c.ClusterSummaryCache.getNodeSummary(cluster.Name)
		allocatingResource, allocatedResource := c.ClusterSummaryCache.getPodSummary(cluster.Name)

		resourceSummary := &clusterv1alpha1.ResourceSummary{
			Allocatable: clusterAllocatable,
			Allocated:   allocatedResource,
			Allocating:  allocatingResource,
		}
		currentClusterStatus.KubernetesVersion = clusterVersion
		currentClusterStatus.APIEnablements = apiEnables
		currentClusterStatus.NodeSummary = nodeSummary
		currentClusterStatus.ResourceSummary = resourceSummary
	}

	setTransitionTime(currentClusterStatus.Conditions, readyCondition)
	meta.SetStatusCondition(&currentClusterStatus.Conditions, *readyCondition)

	return c.updateStatusIfNeeded(cluster, currentClusterStatus)
}

func (c *ClusterStatusController) setStatusCollectionFailedCondition(cluster *clusterv1alpha1.Cluster, currentClusterStatus clusterv1alpha1.ClusterStatus, message string) (controllerruntime.Result, error) {
	readyCondition := util.NewCondition(clusterv1alpha1.ClusterConditionReady, statusCollectionFailed, message, metav1.ConditionFalse)
	setTransitionTime(cluster.Status.Conditions, &readyCondition)
	meta.SetStatusCondition(&currentClusterStatus.Conditions, readyCondition)
	return c.updateStatusIfNeeded(cluster, currentClusterStatus)
}

// updateStatusIfNeeded calls updateStatus only if the status of the member cluster is not the same as the old status
func (c *ClusterStatusController) updateStatusIfNeeded(cluster *clusterv1alpha1.Cluster, currentClusterStatus clusterv1alpha1.ClusterStatus) (controllerruntime.Result, error) {
	if !equality.Semantic.DeepEqual(cluster.Status, currentClusterStatus) {
		klog.V(4).Infof("Start to update cluster status: %s", cluster.Name)
		err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
			cluster.Status = currentClusterStatus
			updateErr := c.Status().Update(context.TODO(), cluster)
			if updateErr == nil {
				return nil
			}

			updated := &clusterv1alpha1.Cluster{}
			if err = c.Get(context.TODO(), client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name}, updated); err == nil {
				// make a copy, so we don't mutate the shared cache
				cluster = updated.DeepCopy()
			} else {
				klog.Errorf("failed to get updated cluster %s: %v", cluster.Name, err)
			}
			return updateErr
		})
		if err != nil {
			klog.Errorf("Failed to update health status of the member cluster: %v, err is : %v", cluster.Name, err)
			return controllerruntime.Result{Requeue: true}, err
		}
	}

	return controllerruntime.Result{RequeueAfter: c.ClusterStatusUpdateFrequency.Duration}, nil
}

// buildInformerForCluster builds informer manager for cluster if it doesn't exist, then constructs informers for node
// and pod and start it. If the informer manager exist, return it.
func (c *ClusterStatusController) buildInformerForCluster(cluster *clusterv1alpha1.Cluster) (informermanager.SingleClusterInformerManager, error) {
	singleClusterInformerManager := c.InformerManager.GetSingleClusterManager(cluster.Name)
	if singleClusterInformerManager == nil {
		clusterClient, err := c.ClusterDynamicClientSetFunc(cluster.Name, c.Client)
		if err != nil {
			klog.Errorf("Failed to build dynamic cluster client for cluster %s.", cluster.Name)
			return nil, err
		}
		singleClusterInformerManager = c.InformerManager.ForCluster(clusterClient.ClusterName, clusterClient.DynamicClientSet, 0)
	}

	gvrs := []schema.GroupVersionResource{nodeGVR, podGVR}

	// create the informer for pods and nodes
	allSynced := true
	for _, gvr := range gvrs {
		if !singleClusterInformerManager.IsInformerSynced(gvr) || !singleClusterInformerManager.IsHandlerExist(gvr, c.getEventHandler(cluster.Name)) {
			allSynced = false
			singleClusterInformerManager.ForResource(gvr, c.getEventHandler(cluster.Name))
		}
	}
	if allSynced {
		return singleClusterInformerManager, nil
	}

	c.InformerManager.Start(cluster.Name)

	if err := func() error {
		synced := c.InformerManager.WaitForCacheSyncWithTimeout(cluster.Name, c.ClusterCacheSyncTimeout.Duration)
		if synced == nil {
			return fmt.Errorf("no informerFactory for cluster %s exist", cluster.Name)
		}
		for _, gvr := range gvrs {
			if !synced[gvr] {
				return fmt.Errorf("informer for %s hasn't synced", gvr)
			}
		}
		return nil
	}(); err != nil {
		klog.Errorf("Failed to sync cache for cluster: %s, error: %v", cluster.Name, err)
		c.InformerManager.Stop(cluster.Name)
		return nil, err
	}

	return singleClusterInformerManager, nil
}

func (c *ClusterStatusController) initLeaseController(ctx context.Context, cluster *clusterv1alpha1.Cluster) {
	// If lease controller has been registered, we skip this function.
	if _, exists := c.ClusterLeaseControllers.Load(cluster.Name); exists {
		return
	}

	// renewInterval is how often the lease renew time is updated.
	renewInterval := time.Duration(float64(c.ClusterLeaseDuration.Nanoseconds()) * c.ClusterLeaseRenewIntervalFraction)

	clusterLeaseController := lease.NewController(
		clock.RealClock{},
		c.KubeClient,
		cluster.Name,
		int32(c.ClusterLeaseDuration.Seconds()),
		nil,
		renewInterval,
		util.NamespaceClusterLease,
		util.SetLeaseOwnerFunc(c.Client, cluster.Name))

	c.ClusterLeaseControllers.Store(cluster.Name, clusterLeaseController)

	// start syncing lease
	go func() {
		clusterLeaseController.Run(ctx.Done())
		<-ctx.Done()
		c.ClusterLeaseControllers.Delete(cluster.Name)
	}()
}

func getClusterHealthStatus(clusterClient *util.ClusterClient) (online, healthy bool) {
	healthStatus, err := healthEndpointCheck(clusterClient.KubeClient, "/readyz")
	if err != nil && healthStatus == http.StatusNotFound {
		// do health check with healthz endpoint if the readyz endpoint is not installed in member cluster
		healthStatus, err = healthEndpointCheck(clusterClient.KubeClient, "/healthz")
	}

	if err != nil {
		klog.Errorf("Failed to do cluster health check for cluster %v, err is : %v ", clusterClient.ClusterName, err)
		return false, false
	}

	if healthStatus != http.StatusOK {
		klog.Infof("Member cluster %v isn't healthy", clusterClient.ClusterName)
		return true, false
	}

	return true, true
}

func healthEndpointCheck(client *clientset.Clientset, path string) (int, error) {
	var healthStatus int
	resp := client.DiscoveryClient.RESTClient().Get().AbsPath(path).Do(context.TODO()).StatusCode(&healthStatus)
	return healthStatus, resp.Error()
}

func generateReadyCondition(online, healthy bool) metav1.Condition {
	if !online {
		return util.NewCondition(clusterv1alpha1.ClusterConditionReady, clusterNotReachableReason, clusterNotReachableMsg, metav1.ConditionFalse)
	}
	if !healthy {
		return util.NewCondition(clusterv1alpha1.ClusterConditionReady, clusterNotReady, clusterUnhealthy, metav1.ConditionFalse)
	}

	return util.NewCondition(clusterv1alpha1.ClusterConditionReady, clusterReady, clusterHealthy, metav1.ConditionTrue)
}

func setTransitionTime(existingConditions []metav1.Condition, newCondition *metav1.Condition) {
	// preserve the last transition time if the status of given condition not changed
	if existingCondition := meta.FindStatusCondition(existingConditions, newCondition.Type); existingCondition != nil {
		if existingCondition.Status == newCondition.Status {
			newCondition.LastTransitionTime = existingCondition.LastTransitionTime
		}
	}
}

func getKubernetesVersion(clusterClient *util.ClusterClient) (string, error) {
	clusterVersion, err := clusterClient.KubeClient.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}

	return clusterVersion.GitVersion, nil
}

// getAPIEnablements returns the list of API enablement(supported groups and resources).
// The returned lists might be non-nil with partial results even in the case of non-nil error.
func getAPIEnablements(clusterClient *util.ClusterClient) ([]clusterv1alpha1.APIEnablement, error) {
	_, apiResourceList, err := clusterClient.KubeClient.Discovery().ServerGroupsAndResources()
	if len(apiResourceList) == 0 {
		return nil, err
	}

	var apiEnablements []clusterv1alpha1.APIEnablement
	for _, list := range apiResourceList {
		var apiResources []clusterv1alpha1.APIResource
		for _, resource := range list.APIResources {
			// skip subresources such as "/status", "/scale" and etc because these are not real APIResources that we are caring about.
			if strings.Contains(resource.Name, "/") {
				continue
			}
			apiResource := clusterv1alpha1.APIResource{
				Name: resource.Name,
				Kind: resource.Kind,
			}
			apiResources = append(apiResources, apiResource)
		}
		sort.SliceStable(apiResources, func(i, j int) bool {
			return apiResources[i].Name < apiResources[j].Name
		})
		apiEnablements = append(apiEnablements, clusterv1alpha1.APIEnablement{GroupVersion: list.GroupVersion, Resources: apiResources})
	}
	sort.SliceStable(apiEnablements, func(i, j int) bool {
		return apiEnablements[i].GroupVersion < apiEnablements[j].GroupVersion
	})
	return apiEnablements, err
}

// RunWorkQueue initializes worker and run it, worker will process resource asynchronously.
func (c *ClusterStatusController) RunWorkQueue() {
	workerOptions := util.Options{
		Name:          "cluster-status",
		KeyFunc:       nil,
		ReconcileFunc: c.syncNodeOrPod,
	}
	c.worker = util.NewAsyncWorker(workerOptions)
	c.worker.Run(c.WorkerNumber, c.StopChan)
}

func (c *ClusterStatusController) syncNodeOrPod(key util.QueueKey) error {
	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		klog.Errorf("Failed to sync node or pod as invalid key: %v", key)
		return fmt.Errorf("invalid key")
	}

	klog.V(4).Infof("Begin to sync %s %s.", fedKey.Kind, fedKey.NamespaceKey())

	if err := c.handleEvent(fedKey); err != nil {
		klog.Errorf("Failed to handle %s(%s) event, Error: %v",
			fedKey.Kind, fedKey.NamespaceKey(), err)
		return err
	}

	return nil
}

func (c *ClusterStatusController) handleEvent(fedKey keys.FederatedKey) error {
	runtimeObj, err := helper.GetObjectFromCache(c.RESTMapper, c.InformerManager, fedKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			c.ClusterSummaryCache.delete(fedKey)
			return nil
		}
		return err
	}

	if err := c.ClusterSummaryCache.set(fedKey, runtimeObj); err != nil {
		klog.Errorf("Failed to sync %s(%s) into cache, err is: %v", fedKey.Kind, fedKey.NamespaceKey(), err)
		return err
	}
	return nil
}

// getEventHandler return callback function that knows how to handle events from the member cluster.
func (c *ClusterStatusController) getEventHandler(clusterName string) cache.ResourceEventHandler {
	if value, exists := c.eventHandlers.Load(clusterName); exists {
		return value.(cache.ResourceEventHandler)
	}

	eventHandler := informermanager.NewHandlerOnEvents(c.genHandlerAddFunc(clusterName), c.genHandlerUpdateFunc(clusterName),
		c.genHandlerDeleteFunc(clusterName))
	c.eventHandlers.Store(clusterName, eventHandler)
	return eventHandler
}

func (c *ClusterStatusController) genHandlerAddFunc(clusterName string) func(obj interface{}) {
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

func (c *ClusterStatusController) genHandlerUpdateFunc(clusterName string) func(oldObj, newObj interface{}) {
	return func(oldObj, newObj interface{}) {
		curObj := newObj.(runtime.Object)
		if helper.HasObjUpdated(oldObj, newObj) {
			key, err := keys.FederatedKeyFunc(clusterName, curObj)
			if err != nil {
				klog.Warningf("Failed to generate key for obj: %s", curObj.GetObjectKind().GroupVersionKind())
				return
			}
			c.worker.Add(key)
		}
	}
}

func (c *ClusterStatusController) genHandlerDeleteFunc(clusterName string) func(obj interface{}) {
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
