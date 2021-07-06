package util

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

const (
	// NamespaceClusterLease is the namespace which cluster lease are stored.
	NamespaceClusterLease = "karmada-cluster"
)

// IsClusterReady tells whether the cluster status in 'Ready' condition.
func IsClusterReady(clusterStatus *v1alpha1.ClusterStatus) bool {
	return meta.IsStatusConditionTrue(clusterStatus.Conditions, v1alpha1.ClusterConditionReady)
}

// GetCluster returns the given Cluster resource
func GetCluster(hostClient client.Client, clusterName string) (*v1alpha1.Cluster, error) {
	cluster := &v1alpha1.Cluster{}
	if err := hostClient.Get(context.TODO(), types.NamespacedName{Name: clusterName}, cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}

// IsAPIInstallInCluster checks if the given api is installed in member cluster
func IsAPIInstallInCluster(clusterStatus v1alpha1.ClusterStatus, groupVersion, name, kind string) bool {
	for _, apiEnablement := range clusterStatus.APIEnablements {
		if apiEnablement.GroupVersion == groupVersion {
			for _, apiResource := range apiEnablement.Resources {
				if apiResource.Kind == kind && apiResource.Name == name {
					return true
				}
			}
		}
	}
	return false
}
