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

type Candidate struct {
	Name     string         // The name of the group.
	MaxScore int64          // The highest cluster score in this group.
	Replicas int32          // Number of available replicas in this group.
	Clusters []*ClusterDesc // Clusters in this group, sorted by cluster.MaxScore descending.
}

// Elect method to select clusters based on the required replicas
func (root *GroupRoot) Elect() ([]*clusterV1alpha1.Cluster, error) {
	election, err := root.selectCluster(root.Replicas)
	if err != nil {
		return nil, err
	} else {
		clusters := election.Clusters
		sort.Slice(clusters, func(i, j int) bool {
			return clusters[i].Score > clusters[j].Score
		})

		result := make([]*clusterV1alpha1.Cluster, len(clusters))
		for i, cluster := range clusters {
			result[i] = cluster.Cluster
		}
		return result, nil
	}
}

// selectCluster method to select clusters based on the required replicas
func (node *GroupNode) selectCluster(replicas int32) (*Candidate, error) {
	if node.Leaf {
		return node.selectByClusters(replicas)
	} else {
		return node.selectByGroups(replicas)
	}
}

// selectByClusters selects clusters from the current group's Clusters list
func (node *GroupNode) selectByClusters(replicas int32) (*Candidate, error) {
	groupElection := &Candidate{
		Name: node.Name,
	}
	availableReplicas := int32(0)
	maxScore := int64(0)
	for _, cluster := range node.Clusters {
		if replicas == InvalidReplicas || cluster.AvailableReplicas > 0 {
			if cluster.Score > maxScore {
				maxScore = cluster.Score
			}
			availableReplicas += cluster.AvailableReplicas
			groupElection.Clusters = append(groupElection.Clusters, cluster)
		}
	}
	groupElection.MaxScore = maxScore
	groupElection.Replicas = availableReplicas
	return groupElection, nil
}

// selectByGroups selects clusters from the sub-groups
func (node *GroupNode) selectByGroups(replicas int32) (*Candidate, error) {
	if !node.Valid {
		return nil, errors.New("the number of feasible clusters is less than spreadConstraint.MinGroups")
	} else {
		// all matched group order by score desc.
		var candidates []*Candidate
		// TODO candidate maybe is not best.
		for _, group := range node.Groups {
			candidate, err := group.selectCluster(InvalidReplicas)
			if err == nil {
				candidates = append(candidates, candidate)
			}
		}

		candidates, err := node.chooseBest(candidates, replicas)
		if err != nil {
			return nil, err
		}
		return node.merge(candidates), nil
	}
}

// chooseBest selects the best candidates to meet the required replicas.
// It returns a slice of selected candidates or an error if the requirements cannot be met.
func (node *GroupNode) chooseBest(candidates []*Candidate, replicas int32) ([]*Candidate, error) {
	size := len(candidates)
	// Does not meet the minimum group size
	if node.MinGroups > 0 && size <= node.MinGroups {
		return nil, errors.New("the number of feasible clusters is less than spreadConstraint.MinGroups")
	} else if size == 0 {
		return nil, errors.New("the number of feasible clusters is zero")
	} else {
		var selects []*Candidate
		if node.MaxGroups > 0 && node.MaxGroups < size {
			selects = candidates[:node.MaxGroups]
		} else if node.MaxGroups > 0 && node.MaxGroups == size {
			selects = candidates
		} else if node.MinGroups > 0 && node.MinGroups < size {
			selects = candidates[:node.MinGroups]
		} else {
			selects = candidates
		}
		if replicas == InvalidReplicas {
			return selects, nil
		}
		if node.match(selects, replicas) {
			return selects, nil
		} else if len(selects) == len(candidates) {
			return nil, fmt.Errorf("no enough resource when selecting %d %ss", node.MaxGroups, node.Constraint)
		} else {
			if node.MaxGroups > 0 && len(selects) < node.MaxGroups {
				for i := len(selects); i < node.MaxGroups; i++ {
					selects = candidates[:i]
					if node.match(selects, replicas) {
						return selects, nil
					}
				}
			}
			for i := len(selects) - 1; i > 0; i-- {
				maxPos := -1
				for j := len(selects); j < len(candidates); j++ {
					if maxPos == -1 || candidates[j].Replicas > candidates[maxPos].Replicas {
						maxPos = j
					}
				}
				candidates[i], candidates[maxPos] = candidates[maxPos], candidates[i]
				selects = candidates[:len(selects)]
				if node.match(selects, replicas) {
					return selects, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("no enough resource when selecting %d %ss", node.MaxGroups, node.Constraint)
}

// match checks if the candidates meet the required replicas.
func (node *GroupNode) match(candidates []*Candidate, replicas int32) bool {
	candidate := node.merge(candidates)
	if candidate.Replicas >= replicas {
		return true
	}
	return false
}

// merge combines a list of Candidate objects into a single Candidate.
// It merges the clusters, updates the maximum score, and sums the replicas.
func (node *GroupNode) merge(candidates []*Candidate) *Candidate {
	size := len(candidates)
	maxScore := int64(0)
	replicas := int32(0)
	var clusters []*ClusterDesc
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
	return &Candidate{
		Name:     node.Name,
		MaxScore: maxScore,
		Replicas: replicas,
		Clusters: clusters,
	}
}
