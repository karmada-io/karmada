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

	var clusters []*clusterv1alpha1.Cluster
	if needReplicas == InvalidReplicas {
		clusterInfos := groupClustersInfo.Clusters[:needCnt]
		for i := range clusterInfos {
			clusters = append(clusters, clusterInfos[i].Cluster)
		}
	} else {
		clusters = selectClusters(groupClustersInfo.Clusters, spreadConstraint, needReplicas)
		if len(clusters) == 0 {
			return nil, fmt.Errorf("no enough resource when selecting %d clusters", needCnt)
		}
	}

	return clusters, nil
}

func selectClusters(clusters []ClusterDetailInfo, clusterConstraint policyv1alpha1.SpreadConstraint, needReplicas int32) []*clusterv1alpha1.Cluster {
	groups := make([]*GroupInfo, 0, len(clusters))
	clusterMap := make(map[string]*clusterv1alpha1.Cluster, len(clusters))
	for _, cluster := range clusters {
		group := &GroupInfo{
			name:   cluster.Name,
			value:  cluster.AvailableReplicas,
			weight: cluster.Score,
		}
		groups = append(groups, group)
		clusterMap[cluster.Name] = cluster.Cluster
	}

	groups = selectGroups(groups, clusterConstraint.MinGroups, clusterConstraint.MaxGroups, int64(needReplicas))
	if len(groups) == 0 {
		return nil
	}

	result := make([]*clusterv1alpha1.Cluster, 0, len(groups))
	for _, group := range groups {
		result = append(result, clusterMap[group.name])
	}
	return result
}
