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
	"errors"
	"fmt"
	clusterV1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"sort"
)

// Elect method to select clusters based on the required replicas
func (root *groupRoot) Elect() ([]*clusterV1alpha1.Cluster, error) {
	election, err := root.selectCluster(root.Replicas)
	if err != nil {
		return nil, err
	} else {
		return toCluster(sortClusters(election.Clusters)), nil
	}
}

// selectCluster method to select clusters based on the required replicas
func (node *groupNode) selectCluster(replicas int32) (*candidate, error) {
	if node.Leaf {
		return node.selectByClusters(replicas)
	} else {
		return node.selectByGroups(replicas)
	}
}

// selectByClusters selects clusters from the current group's Clusters list
func (node *groupNode) selectByClusters(replicas int32) (*candidate, error) {
	result := &candidate{
		Name: node.Name,
	}
	for _, cluster := range node.Clusters {
		if replicas == InvalidReplicas || cluster.AvailableReplicas > 0 {
			result.Clusters = append(result.Clusters, cluster)
		}
	}
	result.MaxScore = node.MaxScore
	result.Replicas = node.AvailableReplicas
	return result, nil
}

// selectByGroups selects clusters from the sub-groups
func (node *groupNode) selectByGroups(replicas int32) (*candidate, error) {
	if !node.Valid {
		return nil, errors.New("the number of feasible clusters is less than spreadConstraint.MinGroups")
	} else {
		// the groups of valid nod are ordered by score desc.
		var candidates []*candidate
		// use DFS to find the best path
		paths := node.findPaths(replicas)
		if len(paths) == 0 {
			return nil, fmt.Errorf("no enough resource when selecting %d %ss", node.MaxGroups, node.Constraint)
		}
		path := node.selectBest(paths)

		for _, node := range path.Nodes {
			participant, _ := node.Group.selectCluster(ternary(replicas == InvalidReplicas, InvalidReplicas, node.Replicas))
			candidates = append(candidates, participant)
		}

		return node.merge(candidates), nil
	}
}

// findPaths finds all possible paths of groups that meet the required replicas using DFS.
func (node *groupNode) findPaths(replicas int32) (paths []*dfsPath) {
	current := &dfsPath{}
	// recursively constructs paths of groups to meet the required replicas.
	var dfsFunc func(int, *dfsPath)
	dfsFunc = func(start int, current *dfsPath) {
		length := current.length()
		if replicas == InvalidReplicas && length == node.MaxGroups ||
			(replicas != InvalidReplicas && current.Replicas >= replicas && length > 0 &&
				(node.MinGroups <= 0 || length >= node.MinGroups)) {
			paths = append(paths, current.next())
		} else if length < node.MaxGroups {
			for i := start; i < len(node.Groups); i++ {
				if node.Groups[i].Valid {
					current.addLast(node.Groups[i])
					dfsFunc(i+1, current)
					current.removeLast()
				}
			}
		}
	}
	dfsFunc(0, current)
	return paths
}

// selectBest selects the best path from the given paths based on the maximum score and replicas.
func (node *groupNode) selectBest(paths []*dfsPath) *dfsPath {
	size := len(paths)
	if size == 0 {
		return nil
	} else if size > 1 {
		sort.Slice(paths, func(i, j int) bool {
			if paths[i].MaxScore != paths[j].MaxScore {
				return paths[i].MaxScore > paths[j].MaxScore
			} else if paths[i].Replicas != paths[j].Replicas {
				return paths[i].Replicas > paths[j].Replicas
			}
			return paths[i].Id < paths[j].Id
		})
	}
	return paths[0]
}

// merge combines a list of candidate objects into a single candidate.
// It merges the clusters, updates the maximum score, and sums the replicas.
func (node *groupNode) merge(candidates []*candidate) *candidate {
	size := len(candidates)
	maxScore := int64(0)
	replicas := int32(0)
	var clusters []*clusterDesc
	if size == 1 {
		maxScore = candidates[0].MaxScore
		replicas = candidates[0].Replicas
		clusters = candidates[0].Clusters
	} else {
		maps := make(map[string]bool, size)
		for _, candidate := range candidates {
			for _, cluster := range candidate.Clusters {
				if _, ok := maps[cluster.Name]; !ok {
					maps[cluster.Name] = true
					clusters = append(clusters, cluster)
					replicas += cluster.AvailableReplicas
					if cluster.Score > maxScore {
						maxScore = cluster.Score
					}
				}
			}
		}
	}
	return &candidate{
		Name:     node.Name,
		MaxScore: maxScore,
		Replicas: replicas,
		Clusters: clusters,
	}
}
