package cache

import (
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/util"
)

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

// NumOfClusters returns the number of clusters.
func (s *Snapshot) NumOfClusters() int {
	return len(s.clusterInfoList)
}

// GetClusters returns all the clusters.
func (s *Snapshot) GetClusters() []*framework.ClusterInfo {
	return s.clusterInfoList
}

// GetReadyClusters returns the clusters in ready status.
func (s *Snapshot) GetReadyClusters() []*framework.ClusterInfo {
	var readyClusterInfoList []*framework.ClusterInfo
	for _, c := range s.clusterInfoList {
		if util.IsClusterReady(&c.Cluster().Status) && c.Cluster().DeletionTimestamp.IsZero() {
			readyClusterInfoList = append(readyClusterInfoList, c)
		}
	}

	return readyClusterInfoList
}

// InspectClusters returns ready/unready clusters' name.
func (s *Snapshot) InspectClusters() (sets.String, sets.String) {
	readyClusterNames := sets.NewString()
	unreadyClusterNames := sets.NewString()
	for _, c := range s.clusterInfoList {
		cluster := c.Cluster()
		if cluster.DeletionTimestamp.IsZero() {
			if util.IsClusterReady(&cluster.Status) {
				readyClusterNames.Insert(cluster.Name)
			} else {
				unreadyClusterNames.Insert(cluster.Name)
			}
		}
	}
	return readyClusterNames, unreadyClusterNames
}

// GetCluster returns the given clusters.
func (s *Snapshot) GetCluster(clusterName string) *framework.ClusterInfo {
	for _, c := range s.clusterInfoList {
		if c.Cluster().Name == clusterName {
			return c
		}
	}
	return nil
}
