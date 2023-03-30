package spreadconstraint

import (
	"fmt"

	"k8s.io/utils/pointer"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func selectBestClustersByRegion(spreadConstraintMap map[policyv1alpha1.SpreadFieldValue]policyv1alpha1.SpreadConstraint,
	groupClustersInfo *GroupClustersInfo) ([]*clusterv1alpha1.Cluster, error) {
	var clusters []*clusterv1alpha1.Cluster
	var candidateClusters []ClusterDetailInfo

	if len(groupClustersInfo.Regions) < spreadConstraintMap[policyv1alpha1.SpreadByFieldRegion].MinGroups {
		return nil, fmt.Errorf("the number of feasible region is less than spreadConstraint.MinGroups")
	}

	// firstly, select regions which have enough clusters to satisfy the cluster and region propagation constraints
	regions := selectRegions(groupClustersInfo.Regions, spreadConstraintMap[policyv1alpha1.SpreadByFieldRegion],
		spreadConstraintMap[policyv1alpha1.SpreadByFieldCluster])
	if len(regions) == 0 {
		return nil, fmt.Errorf("the number of clusters is less than the cluster spreadConstraint.MinGroups")
	}

	// secondly, select the clusters with the highest score in per region,
	for i := range regions {
		clusters = append(clusters, regions[i].Clusters[0].Cluster)
		candidateClusters = append(candidateClusters, regions[i].Clusters[1:]...)
	}

	needCnt := len(candidateClusters) + len(clusters)
	if needCnt > spreadConstraintMap[policyv1alpha1.SpreadByFieldCluster].MaxGroups {
		needCnt = spreadConstraintMap[policyv1alpha1.SpreadByFieldCluster].MaxGroups
	}

	// thirdly, select the remaining Clusters based cluster.Score
	restCnt := needCnt - len(clusters)
	if restCnt > 0 {
		sortClusters(candidateClusters, func(i *ClusterDetailInfo, j *ClusterDetailInfo) *bool {
			if i.AvailableReplicas != j.AvailableReplicas {
				return pointer.Bool(i.AvailableReplicas > j.AvailableReplicas)
			}
			return nil
		})
		for i := 0; i < restCnt; i++ {
			clusters = append(clusters, candidateClusters[i].Cluster)
		}
	}

	return clusters, nil
}

// selectRegions is an implementation of the region selection algorithm, the purpose of
// the region selection algorithm is to try to find those regions that satisfy all spread constraints.
//
// First, it needs to be clear how many regions we need. of course, it needs to meet the
// region spread constraints([minGroups, maxGroups]), and starts from region's minGroups.
//
// Second, we need to find the combination with the highest score when there are multiple
// sets of choices that can satisfy the cluster spread constraint.
// For example:
//
//	R1 -> R2 -> R3 -> R4
//
// We assume that those regions have already done the above descending order according to
// the score. Now those combinations([R1,R3] and [R1,R4]) can all satisfy all spread constraints.
// Who is the best choice?
// Obviously [R1,R3].
//
// Finally, the number of regions that we need should be auto-incremented when there is no
// combination to meet cluster spread constraint. Of course, it can never exceed region's
// maxGroups or the total number of regions. Let's use the example from the second statement above:
//
//	R1 -> R2 -> R3 -> R4
//
// When any combination of two regions cannot satisfy cluster spread constraint, we should try
// to increase the number of regions. It is very likely that we will find the best choice by trying
// to choose a combination of three regions.
func selectRegions(regionMap map[string]RegionInfo, regionConstraint, clusterConstraint policyv1alpha1.SpreadConstraint) []RegionInfo {
	groups := make([]*GroupInfo, 0, len(regionMap))
	for _, region := range regionMap {
		group := &GroupInfo{
			name:   region.Name,
			value:  len(region.Clusters),
			weight: region.Score,
		}
		groups = append(groups, group)
	}

	groups = selectGroups(groups, regionConstraint.MinGroups, regionConstraint.MaxGroups, clusterConstraint.MinGroups)
	if len(groups) == 0 {
		return nil
	}

	result := make([]RegionInfo, 0, len(groups))
	for _, group := range groups {
		result = append(result, regionMap[group.name])
	}
	return result
}
