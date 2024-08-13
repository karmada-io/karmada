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
	selects := make(map[string]*clusterDesc)
	_, err := root.selectCluster(root.Replicas, selects)
	if err != nil {
		return nil, err
	} else {
		var clusters []*clusterDesc
		for _, cluster := range selects {
			clusters = append(clusters, cluster)
		}
		sort.Slice(clusters, func(i, j int) bool {
			return clusters[i].Score > clusters[j].Score
		})

		var result []*clusterV1alpha1.Cluster
		for _, cluster := range clusters {
			result = append(result, cluster.Cluster)
		}
		return result, nil
	}
}

// selectCluster method to select clusters based on the required replicas
func (node *groupNode) selectCluster(replicas int32, selects map[string]*clusterDesc) (int32, error) {
	if node.Leaf {
		return node.selectByClusters(replicas, selects)
	} else {
		return node.selectByGroups(replicas, selects)
	}
}

// selectByClusters selects clusters from the current group's Clusters list
func (node *groupNode) selectByClusters(replicas int32, selects map[string]*clusterDesc) (int32, error) {
	adds := int32(0)
	for _, cluster := range node.Clusters {
		if replicas == InvalidReplicas || cluster.AvailableReplicas > 0 {
			if _, ok := selects[cluster.Name]; !ok {
				selects[cluster.Name] = cluster
				adds += cluster.AvailableReplicas
			}
		}
	}
	return adds, nil
}

// selectByGroups selects clusters from the sub-groups
func (node *groupNode) selectByGroups(replicas int32, selects map[string]*clusterDesc) (int32, error) {
	remain := replicas
	adds := int32(0)
	groups := len(node.Groups)
	if groups <= node.MinGroups {
		return 0, errors.New("the number of feasible clusters is less than spreadConstraint.MinGroups")
	} else {
		maxGroups := node.MaxGroups
		if maxGroups == 0 || groups < maxGroups {
			maxGroups = groups
		}
		for i := 0; i < maxGroups; i++ {
			add, err := node.Groups[i].selectCluster(remain, selects)
			adds += add
			if err == nil && replicas != InvalidReplicas {
				remain -= add
			}
		}

		if remain <= 0 {
			return adds, nil
		} else if maxGroups == groups {
			return 0, errors.New("not enough replicas in the groups")
		} else {
			left := make(map[string]*clusterDesc)
			for i := maxGroups; i < groups; i++ {
				_, _ = node.Groups[i].selectByClusters(InvalidReplicas, left)
			}
			i, err := node.supplement(selects, left, remain)
			if err != nil {
				return 0, err
			}
			return adds + i, nil
		}
	}
}

// supplement attempts to balance the cluster nodes by supplementing the selected nodes
// with the remaining nodes to meet the required replicas.
func (node *groupNode) supplement(selects map[string]*clusterDesc, remains map[string]*clusterDesc, replicas int32) (int32, error) {
	adds := int32(0)
	var selectArray []*clusterDesc
	for _, cluster := range selects {
		selectArray = append(selectArray, cluster)
	}
	sort.Slice(selectArray, func(i, j int) bool {
		return selectArray[i].Score > selectArray[j].Score
	})

	var remainArray []*clusterDesc
	for _, cluster := range remains {
		if _, ok := selects[cluster.Name]; !ok {
			remainArray = append(remainArray, cluster)
		}
	}
	maxReplicas := int32(0)
	maxIndex := -1
	for j := len(selectArray) - 1; j >= 0; j-- {
		for i := 0; i < len(remainArray); i++ {
			if remainArray[i].AvailableReplicas > maxReplicas {
				maxReplicas = remainArray[i].AvailableReplicas
				maxIndex = i
			}
		}
		add := remainArray[maxIndex].AvailableReplicas - selectArray[j].AvailableReplicas
		replicas -= add
		adds += add
		if add > 0 {
			delete(selects, selectArray[j].Name)
			selects[remainArray[maxIndex].Name] = remainArray[maxIndex]
			if replicas <= 0 {
				return adds, nil
			}
			selectArray[j], remainArray[maxIndex] = remainArray[maxIndex], selectArray[j]
		}
	}

	return 0, fmt.Errorf("no enough resource when selecting %d %ss", node.MaxGroups, node.Constraint)
}
