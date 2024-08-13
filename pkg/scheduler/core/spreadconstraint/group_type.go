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
)

// groupNode represents a group in the cluster hierarchy.
type groupNode struct {
	Name              string         // Name of the group.
	Constraint        string         // Type of constraint
	MaxScore          int64          // The highest cluster score in this group.
	AvailableReplicas int32          // Number of available replicas in this group.
	MinGroups         int            // Minimum number of groups
	MaxGroups         int            // Maximum number of groups
	Leaf              bool           // Indicates if it is a leaf node.
	Clusters          []*clusterDesc // Clusters in this group, sorted by cluster.MaxScore descending.
	Groups            []*groupNode   // Sub-groups of this group.
}

// groupRoot represents the root group node in a hierarchical structure.
type groupRoot struct {
	groupNode
	DisableConstraint bool  // Indicates if the constraint is disabled.
	Replicas          int32 // Number of replicas in the root group.
}

// clusterDesc indicates the cluster information
type clusterDesc struct {
	Name              string                   // Name of the cluster
	Score             int64                    // Score of the cluster
	AvailableReplicas int32                    // Number of available replicas in the cluster
	Cluster           *clusterv1alpha1.Cluster // Pointer to the Cluster object
}

// groupBuilder is a structure that embeds a SelectionFactory to build groups.
type groupBuilder struct {
	SelectionFactory
}
