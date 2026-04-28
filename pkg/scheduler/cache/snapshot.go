/*
Copyright 2021 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
func NewEmptySnapshot() Snapshot {
	return Snapshot{}
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
		if util.IsClusterReady(&c.Cluster().Status) {
			readyClusterInfoList = append(readyClusterInfoList, c)
		}
	}

	return readyClusterInfoList
}

// GetReadyClusterNames returns the clusterNames in ready status.
func (s *Snapshot) GetReadyClusterNames() sets.Set[string] {
	readyClusterNames := sets.New[string]()
	for _, c := range s.clusterInfoList {
		if util.IsClusterReady(&c.Cluster().Status) {
			readyClusterNames.Insert(c.Cluster().Name)
		}
	}

	return readyClusterNames
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
