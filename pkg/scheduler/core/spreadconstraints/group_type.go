/*
Copyright 2022 The Karmada Authors.

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

package spreadconstraint

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

const (
	// InvalidClusterID indicate a invalid cluster
	InvalidClusterID = -1
	// InvalidReplicas indicate that don't care about the available resource
	InvalidReplicas = -1
)

type AvailableReplicasFunc func(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster

// groupCluster represents a cluster group with its associated metadata.
type groupCluster struct {
	Name              string          // Name of the group.
	MaxScore          int64           // The highest cluster score in this group.
	AvailableReplicas int32           // Number of available replicas in this group.
	MinGroups         int             // Minimum number of groups
	MaxGroups         int             // Maximum number of groups
	Leaf              bool            // Indicates if it is a leaf node.
	Clusters          []*clusterDesc  // Clusters in this group, sorted by cluster.MaxScore descending.
	Groups            []*groupCluster // Sub-groups of this group.
}

// clusterDesc indicates the cluster information
type clusterDesc struct {
	Name              string                   // Name of the cluster
	Score             int64                    // Score of the cluster
	AvailableReplicas int32                    // Number of available replicas in the cluster
	Cluster           *clusterv1alpha1.Cluster // Pointer to the Cluster object
}
