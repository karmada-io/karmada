package util

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeclientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

// IsClusterReady tells whether the cluster status in 'Ready' condition.
func IsClusterReady(clusterStatus *v1alpha1.ClusterStatus) bool {
	for _, condition := range clusterStatus.Conditions {
		if condition.Type == v1alpha1.ClusterConditionReady {
			if condition.Status == metav1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

// GetCluster returns the given Cluster resource
func GetCluster(hostClient client.Client, clusterName string) (*v1alpha1.Cluster, error) {
	cluster := &v1alpha1.Cluster{}
	if err := hostClient.Get(context.TODO(), types.NamespacedName{Name: clusterName}, cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}

// BuildDynamicClusterClient builds dynamic client for informerFactory by clusterName,
// it will build kubeconfig from cluster resource and construct dynamic client.
func BuildDynamicClusterClient(hostClient client.Client, kubeClient kubeclientset.Interface, clusterName string) (*DynamicClusterClient, error) {
	cluster, err := GetCluster(hostClient, clusterName)
	if err != nil {
		return nil, err
	}

	dynamicClusterClient, err := NewClusterDynamicClientSet(cluster, kubeClient)
	if err != nil {
		return nil, err
	}
	return dynamicClusterClient, nil
}
