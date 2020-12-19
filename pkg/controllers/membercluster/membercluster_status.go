package membercluster

import (
	"context"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/apis/membercluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
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

func updateIndividualClusterStatus(cluster *v1alpha1.MemberCluster, hostClient client.Client, kubeClient kubernetes.Interface) {
	// create a ClusterClient for the given member cluster
	clusterClient, err := util.NewClusterClientSet(cluster, kubeClient)
	if err != nil {
		return
	}

	// update the health status of member cluster
	currentClusterStatus, err := getMemberClusterHealthStatus(clusterClient)
	if err != nil {
		klog.Warningf("Failed to get health status of the member cluster: %v, err is : %v", cluster.Name, err)
		cluster.Status = *currentClusterStatus
		err = hostClient.Status().Update(context.TODO(), cluster)
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

	cluster.Status = *currentClusterStatus
	err = hostClient.Status().Update(context.TODO(), cluster)
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

func getKubernetesVersion(currentClusterStatus *v1alpha1.MemberClusterStatus, clusterClient *util.ClusterClient) (*v1alpha1.MemberClusterStatus, error) {
	clusterVersion, err := clusterClient.KubeClient.Discovery().ServerVersion()
	if err != nil {
		return currentClusterStatus, err
	}

	currentClusterStatus.KubernetesVersion = clusterVersion.GitVersion
	return currentClusterStatus, nil
}
