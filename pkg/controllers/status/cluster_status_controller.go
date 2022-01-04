package status

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-helpers/apimachinery/lease"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
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
	// clusterStatusRetryInterval specifies the interval between two retries.
	clusterStatusRetryInterval = 500 * time.Millisecond
	// clusterStatusRetryTimeout specifies the maximum time to wait for cluster status.
	clusterStatusRetryTimeout = 2 * time.Second
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

	ClusterCacheSyncTimeout metav1.Duration
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
	return controllerruntime.NewControllerManagedBy(mgr).For(&clusterv1alpha1.Cluster{}).WithEventFilter(c.PredicateFunc).Complete(c)
}

func (c *ClusterStatusController) syncClusterStatus(cluster *clusterv1alpha1.Cluster) (controllerruntime.Result, error) {
	var currentClusterStatus = clusterv1alpha1.ClusterStatus{}

	// create a ClusterClient for the given member cluster
	clusterClient, err := c.ClusterClientSetFunc(cluster.Name, c.Client, c.ClusterClientOption)
	if err != nil {
		klog.Errorf("Failed to create a ClusterClient for the given member cluster: %v, err is : %v", cluster.Name, err)
		return c.setStatusCollectionFailedCondition(cluster, currentClusterStatus, fmt.Sprintf("failed to create a ClusterClient: %v", err))
	}

	var online, healthy bool
	// in case of cluster offline, retry a few times to avoid network unstable problems.
	// Note: retry timeout should not be too long, otherwise will block other cluster reconcile.
	err = wait.PollImmediate(clusterStatusRetryInterval, clusterStatusRetryTimeout, func() (done bool, err error) {
		online, healthy = getClusterHealthStatus(clusterClient)
		if !online {
			klog.V(2).Infof("Cluster(%s) is offline.", cluster.Name)
			return false, nil
		}
		return true, nil
	})
	// error indicates that retry timeout, update cluster status immediately and return.
	if err != nil {
		klog.V(2).Infof("Cluster(%s) still offline after retry, ensuring offline is set.", cluster.Name)
		c.InformerManager.Stop(cluster.Name)
		readyCondition := generateReadyCondition(false, false)
		setTransitionTime(cluster.Status.Conditions, &readyCondition)
		meta.SetStatusCondition(&currentClusterStatus.Conditions, readyCondition)
		return c.updateStatusIfNeeded(cluster, currentClusterStatus)
	}

	// get or create informer for pods and nodes in member cluster
	clusterInformerManager, err := c.buildInformerForCluster(cluster)
	if err != nil {
		klog.Errorf("Failed to get or create informer for Cluster %s. Error: %v.", cluster.GetName(), err)
		return c.setStatusCollectionFailedCondition(cluster, currentClusterStatus, fmt.Sprintf("failed to get or create informer: %v", err))
	}

	// init the lease controller for every cluster
	c.initLeaseController(clusterInformerManager.Context(), cluster)

	clusterVersion, err := getKubernetesVersion(clusterClient)
	if err != nil {
		return c.setStatusCollectionFailedCondition(cluster, currentClusterStatus, fmt.Sprintf("failed to get kubernetes version: %v", err))
	}

	// get the list of APIs installed in the member cluster
	apiEnables, err := getAPIEnablements(clusterClient)
	if err != nil {
		return c.setStatusCollectionFailedCondition(cluster, currentClusterStatus, fmt.Sprintf("failed to get the list of APIs installed in the member cluster: %v", err))
	}

	nodes, err := listNodes(clusterInformerManager)
	if err != nil {
		return c.setStatusCollectionFailedCondition(cluster, currentClusterStatus, fmt.Sprintf("failed to list nodes: %v", err))
	}

	pods, err := listPods(clusterInformerManager)
	if err != nil {
		return c.setStatusCollectionFailedCondition(cluster, currentClusterStatus, fmt.Sprintf("failed to list pods: %v", err))
	}

	currentClusterStatus.KubernetesVersion = clusterVersion
	currentClusterStatus.APIEnablements = apiEnables
	currentClusterStatus.NodeSummary = getNodeSummary(nodes)
	currentClusterStatus.ResourceSummary = getResourceSummary(nodes, pods)

	readyCondition := generateReadyCondition(online, healthy)
	setTransitionTime(cluster.Status.Conditions, &readyCondition)
	meta.SetStatusCondition(&currentClusterStatus.Conditions, readyCondition)

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
		cluster.Status = currentClusterStatus
		err := c.Client.Status().Update(context.TODO(), cluster)
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
		if !singleClusterInformerManager.IsInformerSynced(gvr) {
			allSynced = false
			singleClusterInformerManager.Lister(gvr)
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

func getAPIEnablements(clusterClient *util.ClusterClient) ([]clusterv1alpha1.APIEnablement, error) {
	_, apiResourceList, err := clusterClient.KubeClient.Discovery().ServerGroupsAndResources()
	if err != nil {
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
		apiEnablements = append(apiEnablements, clusterv1alpha1.APIEnablement{GroupVersion: list.GroupVersion, Resources: apiResources})
	}

	return apiEnablements, nil
}

// listPods returns the Pod list from the informerManager cache.
func listPods(informerManager informermanager.SingleClusterInformerManager) ([]*corev1.Pod, error) {
	podLister := informerManager.Lister(podGVR)

	podList, err := podLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	pods, err := convertObjectsToPods(podList)
	if err != nil {
		return nil, err
	}

	return pods, nil
}

// listNodes returns the Node list from the informerManager cache.
func listNodes(informerManager informermanager.SingleClusterInformerManager) ([]*corev1.Node, error) {
	nodeLister := informerManager.Lister(nodeGVR)

	nodeList, err := nodeLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	nodes, err := convertObjectsToNodes(nodeList)
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

func getNodeSummary(nodes []*corev1.Node) *clusterv1alpha1.NodeSummary {
	totalNum := len(nodes)
	readyNum := 0

	for _, node := range nodes {
		if helper.NodeReady(node) {
			readyNum++
		}
	}

	var nodeSummary = &clusterv1alpha1.NodeSummary{}
	nodeSummary.TotalNum = int32(totalNum)
	nodeSummary.ReadyNum = int32(readyNum)

	return nodeSummary
}

func getResourceSummary(nodes []*corev1.Node, pods []*corev1.Pod) *clusterv1alpha1.ResourceSummary {
	allocatable := getClusterAllocatable(nodes)
	allocating := getAllocatingResource(pods)
	allocated := getAllocatedResource(pods)

	var resourceSummary = &clusterv1alpha1.ResourceSummary{}
	resourceSummary.Allocatable = allocatable
	resourceSummary.Allocating = allocating
	resourceSummary.Allocated = allocated

	return resourceSummary
}

func convertObjectsToNodes(nodeList []runtime.Object) ([]*corev1.Node, error) {
	nodes := make([]*corev1.Node, 0, len(nodeList))
	for _, obj := range nodeList {
		node, err := helper.ConvertToNode(obj.(*unstructured.Unstructured))
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured to typed object: %v", err)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func convertObjectsToPods(podList []runtime.Object) ([]*corev1.Pod, error) {
	pods := make([]*corev1.Pod, 0, len(podList))
	for _, obj := range podList {
		pod, err := helper.ConvertToPod(obj.(*unstructured.Unstructured))
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured to typed object: %v", err)
		}
		pods = append(pods, pod)
	}
	return pods, nil
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
