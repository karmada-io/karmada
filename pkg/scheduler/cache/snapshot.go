package cache

import "github.com/karmada-io/karmada/pkg/scheduler/framework"

// Snapshot is a snapshot of cache ClusterInfo. The scheduler takes a
// snapshot at the beginning of each scheduling cycle and uses it for its operations in that cycle.
type Snapshot struct {
	// clusterInfoList is the list of nodes as ordered in the cache's nodeTree.
	clusterInfoList []*framework.ClusterInfo
}

// NewEmptySnapshot initializes a Snapshot struct and returns it.
func NewEmptySnapshot() *Snapshot {
	return &Snapshot{}
}

// NumOfClusters returns the number of memberClusters.
func (s *Snapshot) NumOfClusters() int {
	return len(s.clusterInfoList)
}

// GetClusters returns all the clusters.
func (s *Snapshot) GetClusters() []*framework.ClusterInfo {
	return s.clusterInfoList
}
