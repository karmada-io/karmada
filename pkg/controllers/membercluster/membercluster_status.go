package membercluster

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/huawei-cloudnative/karmada/pkg/apis/membercluster/v1alpha1"
	"github.com/huawei-cloudnative/karmada/pkg/controllers/util"
	clientset "github.com/huawei-cloudnative/karmada/pkg/generated/clientset/versioned"
)

const (
	clusterReady              = "ClusterReady"
	healthzOk                 = "/healthz responded with ok"
	clusterNotReady           = "ClusterNotReady"
	healthzNotOk              = "/healthz responded without ok"
	clusterNotReachableReason = "ClusterNotReachable"
	clusterNotReachableMsg    = "cluster is not reachable"
	clusterReachableReason    = "ClusterReachable"
	clusterReachableMsg       = "cluster is reachable"
	clusterOffline            = "Offline"
)

func updateIndividualClusterStatus(cluster *v1alpha1.MemberCluster, hostClient clientset.Interface, clusterClient *util.ClusterClient) {
	// update the health status of member cluster
	currentClusterStatus, err := getMemberClusterHealthStatus(clusterClient)
	if err != nil {
		klog.Warningf("Failed to get health status of the member cluster: %v, err is : %v", cluster.Name, err)
		cluster.Status = *currentClusterStatus
		_, err = hostClient.MemberclusterV1alpha1().MemberClusters("karmada-cluster").Update(context.TODO(), cluster, v1.UpdateOptions{})
		if err != nil {
			klog.Warningf("Failed to update health status of the member cluster: %v, err is : %v", cluster.Name, err)
			return
		}
		return
	}

	// update the cluster version of member cluster
	currentClusterStatus, err = getKubernetesVersion(currentClusterStatus, clusterClient)
	if err != nil {
		klog.Warningf("Failed to get server version of the member cluster: %v, err is : %v", cluster.Name, err)
	}

	// update the list of APIs installed in the member cluster
	currentClusterStatus, err = getAPIEnablements(currentClusterStatus, clusterClient)
	if err != nil {
		klog.Warningf("Failed to get APIs installed in the member cluster: %v, err is : %v", cluster.Name, err)
	}

	// update the summary of nodes status in the member cluster
	currentClusterStatus, err = getNodeSummary(currentClusterStatus, clusterClient)
	if err != nil {
		klog.Warningf("Failed to get summary of nodes status in the member cluster: %v, err is : %v", cluster.Name, err)
	}

	cluster.Status = *currentClusterStatus
	_, err = hostClient.MemberclusterV1alpha1().MemberClusters("karmada-cluster").Update(context.TODO(), cluster, v1.UpdateOptions{})
	if err != nil {
		klog.Warningf("Failed to update health status of the member cluster: %v, err is : %v", cluster.Name, err)
		return
	}
}

func getMemberClusterHealthStatus(clusterClient *util.ClusterClient) (*v1alpha1.MemberClusterStatus, error) {
	clusterStatus := v1alpha1.MemberClusterStatus{}
	currentTime := v1.Now()
	clusterReady := clusterReady
	healthzOk := healthzOk
	newClusterReadyCondition := v1.Condition{
		Type:               clusterReady,
		Status:             v1.ConditionTrue,
		Reason:             clusterReady,
		Message:            healthzOk,
		LastTransitionTime: currentTime,
	}

	clusterNotReady := clusterNotReady
	healthzNotOk := healthzNotOk
	newClusterNotReadyCondition := v1.Condition{
		Type:               clusterReady,
		Status:             v1.ConditionFalse,
		Reason:             clusterNotReady,
		Message:            healthzNotOk,
		LastTransitionTime: currentTime,
	}

	clusterNotReachableReason := clusterNotReachableReason
	clusterNotReachableMsg := clusterNotReachableMsg
	newClusterOfflineCondition := v1.Condition{
		Type:               clusterOffline,
		Status:             v1.ConditionTrue,
		Reason:             clusterNotReachableReason,
		Message:            clusterNotReachableMsg,
		LastTransitionTime: currentTime,
	}
	clusterReachableReason := clusterReachableReason
	clusterReachableMsg := clusterReachableMsg
	newClusterNotOfflineCondition := v1.Condition{
		Type:               clusterOffline,
		Status:             v1.ConditionFalse,
		Reason:             clusterReachableReason,
		Message:            clusterReachableMsg,
		LastTransitionTime: currentTime,
	}

	body, err := clusterClient.KubeClient.DiscoveryClient.RESTClient().Get().AbsPath("/healthz").Do(context.TODO()).Raw()
	if err != nil {
		klog.Warningf("Failed to do cluster health check for member cluster %v, err is : %v ", clusterClient.ClusterName, err)
		clusterStatus.Conditions = append(clusterStatus.Conditions, newClusterOfflineCondition)
	} else {
		if !strings.EqualFold(string(body), "ok") {
			clusterStatus.Conditions = append(clusterStatus.Conditions, newClusterNotReadyCondition, newClusterNotOfflineCondition)
		} else {
			clusterStatus.Conditions = append(clusterStatus.Conditions, newClusterReadyCondition)
		}
	}

	return &clusterStatus, err
}

func getKubernetesVersion(currentClusterStatus *v1alpha1.MemberClusterStatus, clusterClient *util.ClusterClient) (*v1alpha1.MemberClusterStatus, error) {
	clusterVersion, err := clusterClient.KubeClient.Discovery().ServerVersion()
	if err != nil {
		return currentClusterStatus, err
	}

	currentClusterStatus.KubernetesVersion = clusterVersion.GitVersion
	return currentClusterStatus, nil
}

func getAPIEnablements(currentClusterStatus *v1alpha1.MemberClusterStatus, clusterClient *util.ClusterClient) (*v1alpha1.MemberClusterStatus, error) {
	_, apiResourceList, err := clusterClient.KubeClient.Discovery().ServerGroupsAndResources()
	if err != nil {
		return currentClusterStatus, err
	}

	var apiEnablements []v1alpha1.APIEnablement

	for _, list := range apiResourceList {
		apiResource := []string{}
		for _, resource := range list.APIResources {
			apiResource = append(apiResource, resource.Name)
		}
		apiEnablements = append(apiEnablements, v1alpha1.APIEnablement{GroupVersion: list.GroupVersion, Resources: apiResource})
	}

	currentClusterStatus.APIEnablements = apiEnablements
	return currentClusterStatus, nil
}

func getNodeSummary(currentClusterStatus *v1alpha1.MemberClusterStatus, clusterClient *util.ClusterClient) (*v1alpha1.MemberClusterStatus, error) {
	nodeList, err := clusterClient.KubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return currentClusterStatus, err
	}

	totalNum := len(nodeList.Items)
	readyNum := 0

	for _, node := range nodeList.Items {
		if getReadyStatusForNode(node.Status) {
			readyNum++
		}
	}

	allocatable := getClusterAllocatable(nodeList)

	currentClusterStatus.NodeSummary.TotalNum = totalNum
	currentClusterStatus.NodeSummary.ReadyNum = readyNum
	currentClusterStatus.NodeSummary.Allocatable = allocatable

	podList, err := clusterClient.KubeClient.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return currentClusterStatus, err
	}

	usedResource := getUsedResource(podList)
	currentClusterStatus.NodeSummary.Used = usedResource

	return currentClusterStatus, nil
}

func getReadyStatusForNode(nodeStatus corev1.NodeStatus) bool {
	for _, condition := range nodeStatus.Conditions {
		if condition.Type == "Ready" {
			if condition.Status == corev1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

func getClusterAllocatable(nodeList *corev1.NodeList) (allocatable corev1.ResourceList) {
	allocatable = make(corev1.ResourceList)
	for _, node := range nodeList.Items {
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

func getUsedResource(podList *corev1.PodList) corev1.ResourceList {
	var requestCPU, requestMem int64
	for _, pod := range podList.Items {
		if pod.Status.Phase == "Running" {
			for _, c := range pod.Status.Conditions {
				if c.Type == "Ready" && c.Status == "True" {
					podRes := addPod(&pod)
					requestCPU += podRes.MilliCPU
					requestMem += podRes.Memory
				}
			}
		}
	}

	usedResource := corev1.ResourceList{
		corev1.ResourceCPU:    *resource.NewMilliQuantity(requestCPU, resource.DecimalSI),
		corev1.ResourceMemory: *resource.NewQuantity(requestMem, resource.BinarySI),
	}

	return usedResource
}

func addPod(pod *corev1.Pod) requestResource {
	res := calculateResource(pod)
	return res
}

func calculateResource(pod *corev1.Pod) (res requestResource) {
	resPtr := &res
	for _, c := range pod.Spec.Containers {
		resPtr.add(c.Resources.Requests)
	}
	return
}

// requestResource is a collection of compute resource.
type requestResource struct {
	MilliCPU int64
	Memory   int64
}

func (r *requestResource) add(rl corev1.ResourceList) {
	if r == nil {
		return
	}

	for rName, rQuant := range rl {
		switch rName {
		case corev1.ResourceCPU:
			r.MilliCPU += rQuant.MilliValue()
		case corev1.ResourceMemory:
			r.Memory += rQuant.Value()
		default:
			continue
		}
	}
}
