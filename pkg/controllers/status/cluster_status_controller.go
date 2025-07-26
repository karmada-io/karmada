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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientset "k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
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
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/modeling"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/typedmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

const (
	// ControllerName is the controller name that will be used when reporting events and metrics.
	ControllerName            = "cluster-status-controller"
	clusterReady              = "ClusterReady"
	clusterHealthy            = "cluster is healthy and ready to accept workloads"
	clusterNotReady           = "ClusterNotReady"
	clusterUnhealthy          = "cluster is reachable but health endpoint responded without ok"
	clusterNotReachableReason = "ClusterNotReachable"
	clusterNotReachableMsg    = "cluster is not reachable"
	statusCollectionFailed    = "StatusCollectionFailed"

	apiEnablementsComplete             = "Complete"
	apiEnablementPartialAPIEnablements = "Partial"
	apiEnablementEmptyAPIEnablements   = "Empty"
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
	TypedInformerManager        typedmanager.MultiClusterInformerManager
	GenericInformerManager      genericmanager.MultiClusterInformerManager
	ClusterClientSetFunc        util.NewClusterClientSetFunc
	ClusterDynamicClientSetFunc util.NewClusterDynamicClientSetFunc
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
	// ClusterLeaseControllers stores context canceler function for each lease controller.
	// Each lease controller is started with a separated context.
	// key: cluster name of the lease controller servers for.
	// value: context canceler function to stop the controller after cluster is un-registered.
	ClusterLeaseControllers sync.Map
	// ClusterSuccessThreshold is the duration of successes for the cluster to be considered healthy after recovery.
	ClusterSuccessThreshold metav1.Duration
	// ClusterFailureThreshold is the duration of failure for the cluster to be considered unhealthy.
	ClusterFailureThreshold metav1.Duration
	// clusterConditionCache stores the condition status of each cluster.
	clusterConditionCache clusterConditionStore

	ClusterCacheSyncTimeout metav1.Duration
	RateLimiterOptions      ratelimiterflag.Options

	// EnableClusterResourceModeling indicates if enable cluster resource modeling.
	// The resource modeling might be used by the scheduler to make scheduling decisions
	// in scenario of dynamic replica assignment based on cluster free resources.
	// Disable if it does not fit your cases for better performance.
	EnableClusterResourceModeling bool
}

// Reconcile syncs status of the given member cluster.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will requeue the reconcile key after the duration.
func (c *ClusterStatusController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Syncing cluster status", "cluster", req.NamespacedName.Name)

	cluster := &clusterv1alpha1.Cluster{}
	if err := c.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		// The resource may no longer exist, in which case we stop the informer.
		if apierrors.IsNotFound(err) {
			c.GenericInformerManager.Stop(req.NamespacedName.Name)
			c.TypedInformerManager.Stop(req.NamespacedName.Name)
			c.clusterConditionCache.delete(req.NamespacedName.Name)
			metrics.CleanupMetricsForCluster(req.NamespacedName.Name)

			// stop lease controller after the cluster is gone.
			// only used for clusters in Pull mode because no need to set up lease syncing for Push clusters.
			canceller, exists := c.ClusterLeaseControllers.LoadAndDelete(req.NamespacedName.Name)
			if exists {
				if cf, ok := canceller.(context.CancelFunc); ok {
					cf()
				}
			}
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{}, err
	}

	// start syncing status only when the finalizer is present on the given Cluster to
	// avoid conflict with the cluster controller.
	if !controllerutil.ContainsFinalizer(cluster, util.ClusterControllerFinalizer) {
		klog.V(2).InfoS("Waiting finalizer present for member cluster", "cluster", cluster.Name)
		return controllerruntime.Result{Requeue: true}, nil
	}

	err := c.syncClusterStatus(ctx, cluster)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	return controllerruntime.Result{RequeueAfter: c.ClusterStatusUpdateFrequency.Duration}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ClusterStatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	c.clusterConditionCache = clusterConditionStore{
		successThreshold: c.ClusterSuccessThreshold.Duration,
		failureThreshold: c.ClusterFailureThreshold.Duration,
	}
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&clusterv1alpha1.Cluster{}, builder.WithPredicates(c.PredicateFunc, predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions),
		}).Complete(c)
}

func (c *ClusterStatusController) syncClusterStatus(ctx context.Context, cluster *clusterv1alpha1.Cluster) error {
	start := time.Now()
	defer func() {
		metrics.RecordClusterStatus(cluster)
		metrics.RecordClusterSyncStatusDuration(cluster, start)
	}()

	currentClusterStatus := *cluster.Status.DeepCopy()

	// create a ClusterClient for the given member cluster
	clusterClient, err := c.ClusterClientSetFunc(cluster.Name, c.Client, c.ClusterClientOption)
	if err != nil {
		klog.ErrorS(err, "Failed to create a ClusterClient for the given member cluster", "cluster", cluster.Name)
		return setStatusCollectionFailedCondition(ctx, c.Client, cluster, fmt.Sprintf("failed to create a ClusterClient: %v", err))
	}

	online, healthy := getClusterHealthStatus(clusterClient)
	observedReadyCondition := generateReadyCondition(online, healthy)
	readyCondition := c.clusterConditionCache.thresholdAdjustedReadyCondition(cluster, &observedReadyCondition)

	// cluster is offline after retry timeout, update cluster status immediately and return.
	if !online && readyCondition.Status != metav1.ConditionTrue {
		klog.V(2).InfoS("Cluster still offline after ensuring offline is set",
			"cluster", cluster.Name, "duration", c.ClusterFailureThreshold.Duration)
		return updateStatusCondition(ctx, c.Client, cluster, *readyCondition)
	}

	// skip collecting cluster status if not ready
	if online && healthy && readyCondition.Status == metav1.ConditionTrue {
		if cluster.Spec.SyncMode == clusterv1alpha1.Pull {
			// init the lease controller for pull mode clusters
			c.initLeaseController(cluster)
		}

		// The generic informer manager actually used by 'execution-controller' and 'work-status-controller'.
		// TODO(@RainbowMango): We should follow who-use who takes the responsibility to initialize it.
		// 	We should move this logic to both `execution-controller` and `work-status-controller`.
		//  After that the 'initializeGenericInformerManagerForCluster' function as well as 'c.GenericInformerManager'
		//  can be safely removed from current controller.
		c.initializeGenericInformerManagerForCluster(clusterClient)

		var conditions []metav1.Condition
		conditions, err = c.setCurrentClusterStatus(clusterClient, cluster, &currentClusterStatus)
		if err != nil {
			return err
		}
		conditions = append(conditions, *readyCondition)
		return c.updateStatusIfNeeded(ctx, cluster, currentClusterStatus, conditions...)
	}

	return c.updateStatusIfNeeded(ctx, cluster, currentClusterStatus, *readyCondition)
}

func (c *ClusterStatusController) setCurrentClusterStatus(clusterClient *util.ClusterClient, cluster *clusterv1alpha1.Cluster, currentClusterStatus *clusterv1alpha1.ClusterStatus) ([]metav1.Condition, error) {
	var conditions []metav1.Condition
	clusterVersion, err := getKubernetesVersion(clusterClient)
	if err != nil {
		klog.ErrorS(err, "Failed to get Kubernetes version for Cluster", "cluster", cluster.GetName())
	}
	currentClusterStatus.KubernetesVersion = clusterVersion

	var apiEnablementCondition metav1.Condition
	// get the list of APIs installed in the member cluster
	apiEnables, err := getAPIEnablements(clusterClient)
	if len(apiEnables) == 0 {
		apiEnablementCondition = util.NewCondition(clusterv1alpha1.ClusterConditionCompleteAPIEnablements,
			apiEnablementEmptyAPIEnablements, "collected empty APIEnablements from the cluster", metav1.ConditionFalse)
		klog.ErrorS(err, "Failed to get any APIs installed in Cluster", "cluster", cluster.GetName())
	} else if err != nil {
		apiEnablementCondition = util.NewCondition(clusterv1alpha1.ClusterConditionCompleteAPIEnablements,
			apiEnablementPartialAPIEnablements, fmt.Sprintf("might collect partial APIEnablements(%d) from the cluster", len(apiEnables)), metav1.ConditionFalse)
		klog.ErrorS(err, "Collected partial number of APIs installed in Cluster", "numApiEnablements", len(apiEnables), "cluster", cluster.GetName())
	} else {
		apiEnablementCondition = util.NewCondition(clusterv1alpha1.ClusterConditionCompleteAPIEnablements,
			apiEnablementsComplete, "collected complete APIEnablements from the cluster", metav1.ConditionTrue)
	}
	conditions = append(conditions, apiEnablementCondition)
	currentClusterStatus.APIEnablements = apiEnables

	if c.EnableClusterResourceModeling {
		// get or create informer for pods and nodes in member cluster
		clusterInformerManager, err := c.buildInformerForCluster(clusterClient)
		if err != nil {
			klog.ErrorS(err, "Failed to get or create informer for Cluster", "cluster", cluster.GetName())
			// in large-scale clusters, the timeout may occur.
			// if clusterInformerManager fails to be built, should be returned, otherwise, it may cause a nil pointer
			return nil, err
		}
		nodes, err := listNodes(clusterInformerManager)
		if err != nil {
			klog.ErrorS(err, "Failed to list nodes for Cluster", "cluster", cluster.GetName())
		}

		pods, err := listPods(clusterInformerManager)
		if err != nil {
			klog.ErrorS(err, "Failed to list pods for Cluster", "cluster", cluster.GetName())
		}
		currentClusterStatus.NodeSummary = getNodeSummary(nodes)
		currentClusterStatus.ResourceSummary = getResourceSummary(nodes, pods)

		if features.FeatureGate.Enabled(features.CustomizedClusterResourceModeling) {
			currentClusterStatus.ResourceSummary.AllocatableModelings = getAllocatableModelings(cluster, nodes, pods)
		}
	}
	return conditions, nil
}

func setStatusCollectionFailedCondition(ctx context.Context, c client.Client, cluster *clusterv1alpha1.Cluster, message string) error {
	readyCondition := util.NewCondition(clusterv1alpha1.ClusterConditionReady, statusCollectionFailed, message, metav1.ConditionFalse)
	return updateStatusCondition(ctx, c, cluster, readyCondition)
}

// updateStatusIfNeeded calls updateStatus only if the status of the member cluster is not the same as the old status
func (c *ClusterStatusController) updateStatusIfNeeded(ctx context.Context, cluster *clusterv1alpha1.Cluster, currentClusterStatus clusterv1alpha1.ClusterStatus, conditions ...metav1.Condition) error {
	for _, condition := range conditions {
		meta.SetStatusCondition(&currentClusterStatus.Conditions, condition)
	}
	if !equality.Semantic.DeepEqual(cluster.Status, currentClusterStatus) {
		klog.V(4).InfoS("Start to update cluster status", "cluster", cluster.Name)
		err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
			_, err = helper.UpdateStatus(ctx, c.Client, cluster, func() error {
				cluster.Status.KubernetesVersion = currentClusterStatus.KubernetesVersion
				cluster.Status.APIEnablements = currentClusterStatus.APIEnablements
				cluster.Status.NodeSummary = currentClusterStatus.NodeSummary
				cluster.Status.ResourceSummary = currentClusterStatus.ResourceSummary
				for _, condition := range conditions {
					meta.SetStatusCondition(&cluster.Status.Conditions, condition)
				}
				return nil
			})
			return err
		})
		if err != nil {
			klog.ErrorS(err, "Failed to update health status of the member cluster", "cluster", cluster.Name)
			return err
		}
	}

	return nil
}

func updateStatusCondition(ctx context.Context, c client.Client, cluster *clusterv1alpha1.Cluster, conditions ...metav1.Condition) error {
	klog.V(4).InfoS("Start to update cluster status condition", "cluster", cluster.Name)
	err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = helper.UpdateStatus(ctx, c, cluster, func() error {
			for _, condition := range conditions {
				meta.SetStatusCondition(&cluster.Status.Conditions, condition)
			}
			return nil
		})
		return err
	})
	if err != nil {
		klog.ErrorS(err, "Failed to update status condition of the member cluster", "cluster", cluster.Name)
		return err
	}
	return nil
}

func (c *ClusterStatusController) initializeGenericInformerManagerForCluster(clusterClient *util.ClusterClient) {
	if c.GenericInformerManager.IsManagerExist(clusterClient.ClusterName) {
		return
	}

	dynamicClient, err := c.ClusterDynamicClientSetFunc(clusterClient.ClusterName, c.Client, c.ClusterClientOption)
	if err != nil {
		klog.ErrorS(err, "Failed to build dynamic cluster client", "cluster", clusterClient.ClusterName)
		return
	}
	c.GenericInformerManager.ForCluster(clusterClient.ClusterName, dynamicClient.DynamicClientSet, 0)
}

// buildInformerForCluster builds informer manager for cluster if it doesn't exist, then constructs informers for node
// and pod and start it. If the informer manager exist, return it.
func (c *ClusterStatusController) buildInformerForCluster(clusterClient *util.ClusterClient) (typedmanager.SingleClusterInformerManager, error) {
	singleClusterInformerManager := c.TypedInformerManager.GetSingleClusterManager(clusterClient.ClusterName)
	if singleClusterInformerManager == nil {
		singleClusterInformerManager = c.TypedInformerManager.ForCluster(clusterClient.ClusterName, clusterClient.KubeClient, 0)
	}

	gvrs := []schema.GroupVersionResource{nodeGVR, podGVR}

	// create the informer for pods and nodes
	allSynced := true
	for _, gvr := range gvrs {
		if !singleClusterInformerManager.IsInformerSynced(gvr) {
			allSynced = false
			if _, err := singleClusterInformerManager.Lister(gvr); err != nil {
				klog.ErrorS(err, "Failed to get the lister for gvr", "gvr", gvr.String())
			}
		}
	}
	if allSynced {
		return singleClusterInformerManager, nil
	}

	c.TypedInformerManager.Start(clusterClient.ClusterName)
	c.GenericInformerManager.Start(clusterClient.ClusterName)

	if err := func() error {
		synced := c.TypedInformerManager.WaitForCacheSyncWithTimeout(clusterClient.ClusterName, c.ClusterCacheSyncTimeout.Duration)
		if synced == nil {
			return fmt.Errorf("no informerFactory for cluster %s exist", clusterClient.ClusterName)
		}
		for _, gvr := range gvrs {
			if !synced[gvr] {
				return fmt.Errorf("informer for %s hasn't synced", gvr)
			}
		}
		return nil
	}(); err != nil {
		klog.ErrorS(err, "Failed to sync cache for cluster", "cluster", clusterClient.ClusterName)
		c.TypedInformerManager.Stop(clusterClient.ClusterName)
		return nil, err
	}

	return singleClusterInformerManager, nil
}

func (c *ClusterStatusController) initLeaseController(cluster *clusterv1alpha1.Cluster) {
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
		cluster.Name,
		util.NamespaceClusterLease,
		util.SetLeaseOwnerFunc(c.Client, cluster.Name))

	ctx, cancelFunc := context.WithCancel(context.TODO())
	c.ClusterLeaseControllers.Store(cluster.Name, cancelFunc)

	// start syncing lease
	go func() {
		klog.InfoS("Starting syncing lease for cluster", "cluster", cluster.Name)

		// lease controller will keep running until the stop channel is closed(context is canceled)
		clusterLeaseController.Run(ctx)

		klog.InfoS("Stop syncing lease for cluster", "cluster", cluster.Name)
		c.ClusterLeaseControllers.Delete(cluster.Name) // ensure the cache is clean
	}()
}

func getClusterHealthStatus(clusterClient *util.ClusterClient) (online, healthy bool) {
	healthStatus, err := healthEndpointCheck(clusterClient.KubeClient, "/readyz")
	if err != nil && healthStatus == http.StatusNotFound {
		// do health check with healthz endpoint if the readyz endpoint is not installed in member cluster
		healthStatus, err = healthEndpointCheck(clusterClient.KubeClient, "/healthz")
	}

	if err != nil {
		klog.ErrorS(err, "Failed to do cluster health check for cluster", "cluster", clusterClient.ClusterName)
		return false, false
	}

	if healthStatus != http.StatusOK {
		klog.InfoS("Member cluster isn't healthy", "cluster", clusterClient.ClusterName)
		return true, false
	}

	return true, true
}

func healthEndpointCheck(client clientset.Interface, path string) (int, error) {
	var healthStatus int
	resp := client.Discovery().RESTClient().Get().AbsPath(path).Do(context.TODO()).StatusCode(&healthStatus)
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

// listPods returns the Pod list from the informerManager cache.
func listPods(informerManager typedmanager.SingleClusterInformerManager) ([]*corev1.Pod, error) {
	podInterface, err := informerManager.Lister(podGVR)
	if err != nil {
		return nil, err
	}

	podLister, ok := podInterface.(v1.PodLister)
	if !ok {
		return nil, fmt.Errorf("failed to convert interface to PodLister")
	}

	return podLister.List(labels.Everything())
}

// listNodes returns the Node list from the informerManager cache.
func listNodes(informerManager typedmanager.SingleClusterInformerManager) ([]*corev1.Node, error) {
	nodeInterface, err := informerManager.Lister(nodeGVR)
	if err != nil {
		return nil, err
	}

	nodeLister, ok := nodeInterface.(v1.NodeLister)
	if !ok {
		return nil, fmt.Errorf("failed to convert interface to NodeLister")
	}

	return nodeLister.List(labels.Everything())
}

func getNodeSummary(nodes []*corev1.Node) *clusterv1alpha1.NodeSummary {
	totalNum := len(nodes)
	readyNum := 0

	for _, node := range nodes {
		if helper.NodeReady(node) {
			readyNum++
		}
	}

	nodeSummary := &clusterv1alpha1.NodeSummary{}
	nodeSummary.TotalNum = int32(totalNum) // #nosec G115: integer overflow conversion int -> int32
	nodeSummary.ReadyNum = int32(readyNum) // #nosec G115: integer overflow conversion int -> int32

	return nodeSummary
}

func getResourceSummary(nodes []*corev1.Node, pods []*corev1.Pod) *clusterv1alpha1.ResourceSummary {
	allocatable := getClusterAllocatable(nodes)
	allocating := getAllocatingResource(pods)
	allocated := getAllocatedResource(pods)

	resourceSummary := &clusterv1alpha1.ResourceSummary{}
	resourceSummary.Allocatable = allocatable
	resourceSummary.Allocating = allocating
	resourceSummary.Allocated = allocated

	return resourceSummary
}

func getClusterAllocatable(nodeList []*corev1.Node) (allocatable corev1.ResourceList) {
	allocatable = make(corev1.ResourceList)
	for _, node := range nodeList {
		for key, val := range node.Status.Allocatable {
			tmpCap, ok := allocatable[key]
			if ok {
				tmpCap.Add(val)
			} else {
				tmpCap = val
			}
			allocatable[key] = tmpCap
		}
	}

	return allocatable
}

func getAllocatingResource(podList []*corev1.Pod) corev1.ResourceList {
	allocating := util.EmptyResource()
	podNum := int64(0)
	for _, pod := range podList {
		if len(pod.Spec.NodeName) == 0 {
			allocating.AddPodRequest(&pod.Spec)
			podNum++
		}
	}
	allocating.AddResourcePods(podNum)
	return allocating.ResourceList()
}

func getAllocatedResource(podList []*corev1.Pod) corev1.ResourceList {
	allocated := util.EmptyResource()
	podNum := int64(0)
	for _, pod := range podList {
		// When the phase of a pod is Succeeded or Failed, kube-scheduler would not consider its resource occupation.
		if len(pod.Spec.NodeName) != 0 && pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			allocated.AddPodRequest(&pod.Spec)
			podNum++
		}
	}
	allocated.AddResourcePods(podNum)
	return allocated.ResourceList()
}

func getNodeAvailable(allocatable corev1.ResourceList, podResources *util.Resource) corev1.ResourceList {
	if podResources == nil {
		return allocatable
	}
	allocatedResourceList := podResources.ResourceList()
	if allocatedResourceList == nil {
		return allocatable
	}
	allowedPodNumber := allocatable.Pods().Value() - allocatedResourceList.Pods().Value()
	// When too many pods have been created, scheduling will fail so that the allocating pods number may be huge.
	// If allowedPodNumber is less than or equal to 0, we don't allow more pods to be created.
	if allowedPodNumber <= 0 {
		klog.InfoS("The number of schedulable Pods on the node is less than or equal to 0, " +
			"we won't add the node to cluster resource models.")
		return nil
	}

	for allocatedName, allocatedQuantity := range allocatedResourceList {
		if allocatableQuantity, ok := allocatable[allocatedName]; ok {
			allocatableQuantity.Sub(allocatedQuantity)
			allocatable[allocatedName] = allocatableQuantity
		}
	}

	return allocatable
}

func getAllocatableModelings(cluster *clusterv1alpha1.Cluster, nodes []*corev1.Node, pods []*corev1.Pod) []clusterv1alpha1.AllocatableModeling {
	if len(cluster.Spec.ResourceModels) == 0 {
		return nil
	}
	modelingSummary, err := modeling.InitSummary(cluster.Spec.ResourceModels)
	if err != nil {
		klog.ErrorS(err, "Failed to init cluster summary from cluster resource models for Cluster", "cluster", cluster.GetName())
		return nil
	}

	nodePodResourcesMap := make(map[string]*util.Resource)
	for _, pod := range pods {
		// When the phase of a pod is Succeeded or Failed, kube-scheduler would not consider its resource occupation.
		if len(pod.Spec.NodeName) != 0 && pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			if nodePodResourcesMap[pod.Spec.NodeName] == nil {
				nodePodResourcesMap[pod.Spec.NodeName] = util.EmptyResource()
			}
			nodePodResourcesMap[pod.Spec.NodeName].AddPodRequest(&pod.Spec)
			nodePodResourcesMap[pod.Spec.NodeName].AddResourcePods(1)
		}
	}

	for _, node := range nodes {
		nodeAvailable := getNodeAvailable(node.Status.Allocatable.DeepCopy(), nodePodResourcesMap[node.Name])
		if nodeAvailable == nil {
			break
		}
		modelingSummary.AddToResourceSummary(modeling.NewClusterResourceNode(nodeAvailable))
	}

	m := make([]clusterv1alpha1.AllocatableModeling, len(modelingSummary.RMs))
	for index, resourceModel := range modelingSummary.RMs {
		m[index].Grade = cluster.Spec.ResourceModels[index].Grade
		m[index].Count = resourceModel.Quantity
	}

	return m
}
