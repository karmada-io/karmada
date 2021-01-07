package framework

import (
	"github.com/karmada-io/karmada/pkg/apis/membercluster/v1alpha1"
)

// ClusterInfo is cluster level aggregated information.
type ClusterInfo struct {
	// Overall cluster information.
	cluster *v1alpha1.MemberCluster
}

// NewClusterInfo creates a ClusterInfo object.
func NewClusterInfo(cluster *v1alpha1.MemberCluster) *ClusterInfo {
	return &ClusterInfo{
		cluster: cluster,
	}
}

// Cluster returns overall information about this cluster.
func (n *ClusterInfo) Cluster() *v1alpha1.MemberCluster {
	if n == nil {
		return nil
	}
	return n.cluster
}
