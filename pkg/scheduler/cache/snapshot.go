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

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

// Snapshot is a snapshot of cache clusters, as well as indexes tracking workload affinity + anti-affinity groups.
// The scheduler takes a snapshot at the beginning of each scheduling cycle and uses it for its operations in that cycle.
type Snapshot struct {
	// bindingGroups maps bindingID -> set(WorkloadGroupKey)
	bindingGroups map[string]sets.Set[WorkloadGroupKey]
	clusters      []*clusterv1alpha1.Cluster
	// clustersByBinding maps bindingID -> set(clusterName)
	clustersByBinding map[string]sets.Set[string]
	// groupMembers maps (namespace, type, group) -> set(bindingID)
	groupMembers map[WorkloadGroupKey]sets.Set[string]
}

// NewEmptySnapshot initializes a Snapshot struct and returns it.
func NewEmptySnapshot() Snapshot {
	return Snapshot{}
}

// NumOfClusters returns the number of clusters.
func (s *Snapshot) NumOfClusters() int {
	return len(s.clusters)
}

// GetClusters returns all the clusters.
func (s *Snapshot) GetClusters() []*clusterv1alpha1.Cluster {
	return s.clusters
}

// GetReadyClusters returns the clusters in ready status.
func (s *Snapshot) GetReadyClusters() []*clusterv1alpha1.Cluster {
	var ready []*clusterv1alpha1.Cluster
	for _, c := range s.clusters {
		if util.IsClusterReady(&c.Status) {
			ready = append(ready, c)
		}
	}
	return ready
}

// GetPeerBindings returns binding IDs that belong to the given workload affinity/anti-affinity group.
func (s *Snapshot) GetPeerBindings(namespace string, t GroupType, group string) sets.Set[string] {
	k := makeWorkloadGroupKey(namespace, t, group)
	return s.groupMembers[k]
}

// GetClustersForBinding returns the set of clusters where the input binding has been scheduled
func (s *Snapshot) GetClustersForBinding(bindingID string) sets.Set[string] {
	return s.clustersByBinding[bindingID]
}

// GetReadyClusterNames returns the clusterNames in ready status.
func (s *Snapshot) GetReadyClusterNames() sets.Set[string] {
	names := sets.New[string]()
	for _, c := range s.clusters {
		if util.IsClusterReady(&c.Status) {
			names.Insert(c.Name)
		}
	}
	return names
}

// GetCluster returns the given clusters.
func (s *Snapshot) GetCluster(clusterName string) *clusterv1alpha1.Cluster {
	for _, c := range s.clusters {
		if c.Name == clusterName {
			return c
		}
	}
	return nil
}
