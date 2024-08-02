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
	Valid             bool           // Indicates if the number of groups is valid.
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
	Valid             bool                     // Indicates if the cluster of the group is valid.
	Cluster           *clusterv1alpha1.Cluster // Pointer to the Cluster object
}

// groupBuilder is a structure that embeds a SelectionFactory to build groups.
type groupBuilder struct {
	SelectionFactory
}

// candidate represents a group with its name, highest cluster score, number of available replicas,
// and a list of clusters sorted by their maximum score in descending order.
type candidate struct {
	Name     string         // The name of the group.
	MaxScore int64          // The highest cluster score in this group.
	Replicas int32          // Number of available replicas in this group.
	Clusters []*clusterDesc // Clusters in this group, sorted by cluster.MaxScore descending.
}

// dfsPath represents a path in the depth-first search.
type dfsPath struct {
	Id       int
	Replicas int32
	MaxScore int64
	Nodes    []*dfsNode
}

// dfsNode represents a node in the depth-first search.
type dfsNode struct {
	Replicas int32
	MaxScore int64
	Group    *groupNode
}

// addLast appends a new group to the dfsPath, updating the total replicas and maximum score of the path.
func (path *dfsPath) addLast(group *groupNode) {
	node := &dfsNode{
		Replicas: group.AvailableReplicas,
		MaxScore: group.MaxScore,
		Group:    group,
	}
	path.Nodes = append(path.Nodes, node)
	// TODO handle duplicate cluster
	path.Replicas += node.Replicas
	if node.MaxScore > path.MaxScore {
		path.MaxScore = node.MaxScore
	}
}

// removeLast removes the last node from the dfsPath and returns a new dfsPath instance with updated replicas and maximum score.
func (path *dfsPath) removeLast() {
	size := len(path.Nodes)
	last := path.Nodes[size-1]
	path.Nodes = path.Nodes[:size-1]
	path.Replicas -= last.Replicas
	// Recalculate the maximum score if the last node had the current maximum score
	if last.MaxScore == path.MaxScore {
		path.score()
	}
}

// removeLast removes the last node from the dfsPath and returns a new dfsPath instance with updated replicas and maximum score.
func (path *dfsPath) next() *dfsPath {
	result := &dfsPath{
		Id:       path.Id,
		Replicas: path.Replicas,
		MaxScore: path.MaxScore,
		Nodes:    append([]*dfsNode{}, path.Nodes...),
	}
	path.Id = path.Id + 1
	return result
}

// length returns the number of nodes in the dfsPath.
func (path *dfsPath) length() int {
	return len(path.Nodes)
}

// score calculates the maximum score among all the nodes in the dfsPath and updates the path's MaxScore.
func (path *dfsPath) score() {
	maxScore := int64(0)
	for _, node := range path.Nodes {
		if node.MaxScore > maxScore {
			maxScore = node.MaxScore
		}
	}
	path.MaxScore = maxScore
}

func ternary[T any](condition bool, trueVal, falseVal T) T {
	if condition {
		return trueVal
	}
	return falseVal
}
