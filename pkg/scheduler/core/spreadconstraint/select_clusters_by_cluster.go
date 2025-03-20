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
	"fmt"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func selectBestClustersByCluster(spreadConstraint policyv1alpha1.SpreadConstraint, groupClustersInfo *GroupClustersInfo,
	needReplicas int32) ([]*clusterv1alpha1.Cluster, error) {
	totalClusterCnt := len(groupClustersInfo.Clusters)
	if totalClusterCnt < spreadConstraint.MinGroups {
		return nil, fmt.Errorf("the number of feasible clusters is less than spreadConstraint.MinGroups")
	}

	needCnt := spreadConstraint.MaxGroups
	if totalClusterCnt < spreadConstraint.MaxGroups {
		needCnt = totalClusterCnt
	}

	var clusterInfos []ClusterDetailInfo

	if needReplicas == InvalidReplicas {
		clusterInfos = groupClustersInfo.Clusters[:needCnt]
	} else {
		clusterInfos = selectClustersByAvailableResource(groupClustersInfo.Clusters, int32(needCnt), needReplicas) // #nosec G115: integer overflow conversion int -> int32
		if len(clusterInfos) == 0 {
			return nil, fmt.Errorf("no enough resource when selecting %d clusters", needCnt)
		}
	}

	var clusters []*clusterv1alpha1.Cluster
	for i := range clusterInfos {
		clusters = append(clusters, clusterInfos[i].Cluster)
	}

	return clusters, nil
}

// if needClusterCount = 2, needReplicas = 80, member1 and member3 will be selected finally.
// because the total resource of member1 and member2 is less than needReplicas although their scores is highest
// --------------------------------------------------
// | clusterName      | member1 | member2 | member3 |
// |-------------------------------------------------
// | score            |   60    |    50   |    40   |
// |------------------------------------------------|
// |AvailableReplicas |   40    |    30   |    60   |
// |------------------------------------------------|
func selectClustersByAvailableResource(candidateClusters []ClusterDetailInfo, needClusterCount, needReplicas int32) []ClusterDetailInfo {
	retClusters := candidateClusters[:needClusterCount]
	restClusters := candidateClusters[needClusterCount:]

	// the retClusters is sorted by cluster.Score descending. when the total AvailableReplicas of retClusters is less than needReplicas,
	// use the cluster with the most AvailableReplicas in restClusters to instead the cluster with the lowest score,
	// from the last cluster of the slice until checkAvailableResource returns true
	var updateClusterID = len(retClusters) - 1
	for !checkAvailableResource(retClusters, needReplicas) && updateClusterID >= 0 {
		clusterID := GetClusterWithMaxAvailableResource(restClusters, retClusters[updateClusterID].AvailableReplicas)
		if clusterID == InvalidClusterID {
			updateClusterID--
			continue
		}

		retClusters[updateClusterID], restClusters[clusterID] = restClusters[clusterID], retClusters[updateClusterID]
		updateClusterID--
	}

	if !checkAvailableResource(retClusters, needReplicas) {
		return nil
	}
	return retClusters
}

func checkAvailableResource(clusters []ClusterDetailInfo, needReplicas int32) bool {
	var total int64

	for i := range clusters {
		total += clusters[i].AvailableReplicas
	}

	return total >= int64(needReplicas)
}

// GetClusterWithMaxAvailableResource returns the cluster with maxAvailableReplicas
func GetClusterWithMaxAvailableResource(candidateClusters []ClusterDetailInfo, originReplicas int64) int {
	var maxAvailableReplicas = originReplicas
	var clusterID = InvalidClusterID
	for i := range candidateClusters {
		if maxAvailableReplicas < candidateClusters[i].AvailableReplicas {
			clusterID = i
			maxAvailableReplicas = candidateClusters[i].AvailableReplicas
		}
	}

	return clusterID
}
