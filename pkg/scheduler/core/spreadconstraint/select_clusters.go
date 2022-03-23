package spreadconstraint

import (
	"fmt"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// SelectBestClusters selects the cluster set based the GroupClustersInfo and placement
func SelectBestClusters(placement *policyv1alpha1.Placement, groupClustersInfo *GroupClustersInfo) ([]*clusterv1alpha1.Cluster, error) {
	if len(placement.SpreadConstraints) != 0 {
		return selectBestClustersBySpreadConstraints(placement.SpreadConstraints, groupClustersInfo)
	}

	var clusters []*clusterv1alpha1.Cluster
	for _, cluster := range groupClustersInfo.Clusters {
		clusters = append(clusters, cluster.Cluster)
	}

	return clusters, nil
}

func selectBestClustersBySpreadConstraints(spreadConstraints []policyv1alpha1.SpreadConstraint,
	groupClustersInfo *GroupClustersInfo) ([]*clusterv1alpha1.Cluster, error) {
	if len(spreadConstraints) > 1 {
		return nil, fmt.Errorf("just support single spread constraint")
	}

	spreadConstraint := spreadConstraints[0]
	if spreadConstraint.SpreadByField == policyv1alpha1.SpreadByFieldCluster {
		return selectBestClustersByCluster(spreadConstraint, groupClustersInfo)
	}

	return nil, fmt.Errorf("just support cluster spread constraint")
}

func selectBestClustersByCluster(spreadConstraint policyv1alpha1.SpreadConstraint, groupClustersInfo *GroupClustersInfo) ([]*clusterv1alpha1.Cluster, error) {
	totalClusterCnt := len(groupClustersInfo.Clusters)
	if spreadConstraint.MinGroups > totalClusterCnt {
		return nil, fmt.Errorf("the number of feasible clusters is less than spreadConstraint.MinGroups")
	}

	needCnt := spreadConstraint.MaxGroups
	if spreadConstraint.MaxGroups > totalClusterCnt {
		needCnt = totalClusterCnt
	}

	var clusters []*clusterv1alpha1.Cluster
	for i := 0; i < needCnt; i++ {
		clusters = append(clusters, groupClustersInfo.Clusters[i].Cluster)
	}

	return clusters, nil
}
