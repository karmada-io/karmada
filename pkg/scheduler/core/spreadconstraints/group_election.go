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
	clusterV1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"sort"
)

// elect method to select clusters based on the required replicas
func (group *groupCluster) elect(replicas int32) ([]*clusterV1alpha1.Cluster, error) {
	selects := make(map[string]*clusterDesc)
	_, err := group.selectCluster(replicas, selects)
	if err != nil {
		return nil, err
	} else {
		var clusters []*clusterV1alpha1.Cluster
		for _, desc := range selects {
			clusters = append(clusters, desc.Cluster)
		}
		return clusters, nil
	}
}

// selectCluster method to select clusters based on the required replicas
func (group *groupCluster) selectCluster(replicas int32, selects map[string]*clusterDesc) (int32, error) {
	if group.Leaf {
		return group.selectByClusters(replicas, selects)
	} else {
		return group.selectByGroups(replicas, selects)
	}
}

// selectByClusters selects clusters from the current group's Clusters list
func (group *groupCluster) selectByClusters(replicas int32, selects map[string]*clusterDesc) (int32, error) {
	if replicas != InvalidReplicas && group.AvailableReplicas < replicas {
		return 0, errors.New("not enough replicas in the clusters")
	} else {
		adds := int32(0)
		for _, cluster := range group.Clusters {
			if replicas == InvalidReplicas || cluster.AvailableReplicas > 0 {
				if _, ok := selects[cluster.Name]; !ok {
					selects[cluster.Name] = cluster
					adds += cluster.AvailableReplicas
				}
			}
		}
		return adds, nil
	}
}

// selectByGroups selects clusters from the sub-groups
func (group *groupCluster) selectByGroups(replicas int32, selects map[string]*clusterDesc) (int32, error) {
	remain := replicas
	adds := int32(0)
	groups := len(group.Groups)
	if groups <= group.MinGroups {
		return 0, errors.New("not enough groups to meet the minimum group requirement")
	} else {
		maxGroups := group.MaxGroups
		if maxGroups == 0 || groups < maxGroups {
			maxGroups = groups
		}
		for i := 0; i < maxGroups; i++ {
			add, err := group.Groups[i].selectCluster(remain, selects)
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
				_, _ = group.Groups[i].selectByGroups(InvalidReplicas, left)
			}
			i, err := supplement(selects, left, remain)
			if err != nil {
				return 0, err
			}
			return adds + i, nil
		}
	}
}

func supplement(selects map[string]*clusterDesc, remains map[string]*clusterDesc, replicas int32) (int32, error) {
	adds := int32(0)
	var selectArray []*clusterDesc
	for _, cluster := range selects {
		selectArray = append(selectArray, cluster)
	}
	var remainArray []*clusterDesc
	for _, cluster := range remains {
		if _, ok := selects[cluster.Name]; !ok {
			remainArray = append(remainArray, cluster)
		}
	}
	// Sort the selected clusters by AvailableReplicas in descending order.
	sort.Slice(selectArray, func(i, j int) bool {
		return selectArray[i].AvailableReplicas > selectArray[j].AvailableReplicas
	})
	maxReplicas := int32(0)
	maxIndex := -1
	for j := len(selects) - 1; j >= 0; j-- {
		for i := 0; i < len(remains); i++ {
			if remainArray[i].AvailableReplicas > maxReplicas {
				maxReplicas = remainArray[i].AvailableReplicas
				maxIndex = i
			}
		}
		add := remainArray[maxIndex].AvailableReplicas - selectArray[j].AvailableReplicas
		replicas -= add
		adds += add
		if add <= 0 {
			return 0, errors.New("not enough replicas available in the groups")
		} else if replicas <= 0 {
			return adds, nil
		} else {
			delete(selects, selectArray[j].Name)
			selects[remainArray[maxIndex].Name] = remainArray[maxIndex]
			selectArray[j], remainArray[maxIndex] = remainArray[maxIndex], selectArray[j]
		}
	}

	return 0, errors.New("not enough replicas available in the groups")
}
