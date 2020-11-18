package membercluster

import (
	"context"
	"strings"

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
	clusterVersion, err := clusterClient.KubeClient.Discovery().ServerVersion()
	if err != nil {
		klog.Warningf("Failed to get server version of the member cluster: %v, err is : %v", cluster.Name, err)
	}

	currentClusterStatus.KubernetesVersion = clusterVersion.GitVersion
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
		klog.Warningf("Failed to do cluster health check for cluster %v, err is : %v ", clusterClient.ClusterName, err)
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
