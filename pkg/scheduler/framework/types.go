package framework

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

// ClusterInfo is cluster level aggregated information.
type ClusterInfo struct {
	// Overall cluster information.
	cluster *clusterv1alpha1.Cluster
}

// NewClusterInfo creates a ClusterInfo object.
func NewClusterInfo(cluster *clusterv1alpha1.Cluster) *ClusterInfo {
	return &ClusterInfo{
		cluster: cluster,
	}
}

// Cluster returns overall information about this cluster.
func (n *ClusterInfo) Cluster() *clusterv1alpha1.Cluster {
	if n == nil {
		return nil
	}
	return n.cluster
}
