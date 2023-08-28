package spreadconstraint

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/scheduler/core/spreadconstraint/algorithm"
)

const (
	ReplicaKind  = "replica"
	ClusterKind  = "cluster"
	ZoneKind     = "zone"
	RegionKind   = "region"
	ProviderKind = "provider"
	LabelKind    = "label"
)

type Group interface {
	algorithm.Group
	GetClusters() []*clusterv1alpha1.Cluster
}

// ClusterInfo indicate the cluster information
type ClusterInfo struct {
	*clusterv1alpha1.Cluster
	Score             int64
	AvailableReplicas int64
}

func (c *ClusterInfo) GetName() string {
	return c.Name
}

func (c *ClusterInfo) GetScore() int64 {
	return c.Score
}

func (c *ClusterInfo) GetNumber(_ string) int64 {
	return c.AvailableReplicas
}

func (c *ClusterInfo) GetClusters() []*clusterv1alpha1.Cluster {
	return []*clusterv1alpha1.Cluster{c.Cluster}
}

type ClusterGroupInfo struct {
	Name string
	// the comprehensive score in all clusters of the provider
	Score int64
	// Clusters under this group, sorted by cluster.Score descending.
	Clusters []*ClusterInfo
}

func (cg *ClusterGroupInfo) GetName() string {
	return cg.Name
}

func (cg *ClusterGroupInfo) GetScore() int64 {
	return cg.Score
}

func (cg *ClusterGroupInfo) GetNumber(kind string) int64 {
	switch kind {
	case ClusterKind:
		return int64(len(cg.Clusters))
	default:
		return 0
	}
}

func (cg *ClusterGroupInfo) GetClusters() []*clusterv1alpha1.Cluster {
	clusters := make([]*clusterv1alpha1.Cluster, 0, len(cg.Clusters))
	for _, c := range cg.Clusters {
		clusters = append(clusters, c.Cluster)
	}
	return clusters
}

// ZoneInfo indicate the zone information
type ZoneInfo struct {
	*ClusterGroupInfo
}

// RegionInfo indicate the region information
type RegionInfo struct {
	*ClusterGroupInfo
	// Zones under this provider
	Zones []*ZoneInfo
}

func (r *RegionInfo) GetNumber(kind string) int64 {
	switch kind {
	case ZoneKind:
		return int64(len(r.Zones))
	case ClusterKind:
		return int64(len(r.Clusters))
	default:
		return 0
	}
}

// ProviderInfo indicate the provider information
type ProviderInfo struct {
	*ClusterGroupInfo
	// Regions under this provider
	Regions []*RegionInfo
	// Zones under this provider
	Zones []*ZoneInfo
}

func (p *ProviderInfo) GetNumber(kind string) int64 {
	switch kind {
	case RegionKind:
		return int64(len(p.Regions))
	case ZoneKind:
		return int64(len(p.Zones))
	case ClusterKind:
		return int64(len(p.Clusters))
	default:
		return 0
	}
}
