package util

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeclientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

// IsMemberClusterReady tells whether the cluster status in 'Ready' condition.
func IsMemberClusterReady(clusterStatus *v1alpha1.ClusterStatus) bool {
	for _, condition := range clusterStatus.Conditions {
		// TODO(RainbowMango): Condition type should be defined in API, and after that update this hard code accordingly.
		if condition.Type == v1alpha1.ClusterConditionReady {
			if condition.Status == metav1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

// GetMemberCluster returns the given Cluster resource
func GetMemberCluster(hostClient client.Client, clusterName string) (*v1alpha1.Cluster, error) {
	memberCluster := &v1alpha1.Cluster{}
	if err := hostClient.Get(context.TODO(), types.NamespacedName{Name: clusterName}, memberCluster); err != nil {
		return nil, err
	}
	return memberCluster, nil
}

// BuildDynamicClusterClient builds dynamic client for informerFactory by clusterName,
// it will build kubeconfig from memberCluster resource and construct dynamic client.
func BuildDynamicClusterClient(hostClient client.Client, kubeClient kubeclientset.Interface, cluster string) (*DynamicClusterClient, error) {
	memberCluster, err := GetMemberCluster(hostClient, cluster)
	if err != nil {
		return nil, err
	}

	dynamicClusterClient, err := NewClusterDynamicClientSet(memberCluster, kubeClient)
	if err != nil {
		return nil, err
	}
	return dynamicClusterClient, nil
}
