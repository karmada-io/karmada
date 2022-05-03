package spreadconstraint

import (
	"fmt"
	"sort"

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
	regions := selectRegions(groupClustersInfo.Regions, spreadConstraintMap[policyv1alpha1.SpreadByFieldRegion], spreadConstraintMap[policyv1alpha1.SpreadByFieldCluster])
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
	sortClusters(candidateClusters)
	restCnt := needCnt - len(clusters)
	for i := 0; i < restCnt; i++ {
		clusters = append(clusters, candidateClusters[i].Cluster)
	}

	return clusters, nil
}

func selectRegions(RegionInfos map[string]RegionInfo, regionConstraint, clusterConstraint policyv1alpha1.SpreadConstraint) []RegionInfo {
	var regions []RegionInfo
	for i := range RegionInfos {
		regions = append(regions, RegionInfos[i])
	}

	sort.Slice(regions, func(i, j int) bool {
		if regions[i].Score != regions[j].Score {
			return regions[i].Score > regions[j].Score
		}

		return regions[i].Name < regions[j].Name
	})

	retRegions := regions[:regionConstraint.MinGroups]
	candidateRegions := regions[regionConstraint.MinGroups:]
	var replaceID = len(retRegions) - 1
	for !checkClusterTotalForRegion(retRegions, clusterConstraint) && replaceID >= 0 {
		regionID := getRegionWithMaxClusters(candidateRegions, len(retRegions[replaceID].Clusters))
		if regionID == InvalidRegionID {
			replaceID--
			continue
		}

		retRegions[replaceID], candidateRegions[regionID] = candidateRegions[regionID], retRegions[replaceID]
		replaceID--
	}

	if !checkClusterTotalForRegion(retRegions, clusterConstraint) {
		return nil
	}

	return retRegions
}

func checkClusterTotalForRegion(regions []RegionInfo, clusterConstraint policyv1alpha1.SpreadConstraint) bool {
	var sum int
	for i := range regions {
		sum += len(regions[i].Clusters)
	}

	return sum >= clusterConstraint.MinGroups
}

func getRegionWithMaxClusters(candidateRegions []RegionInfo, originClusters int) int {
	var maxClusters = originClusters
	var regionID = -1
	for i := range candidateRegions {
		if maxClusters < len(candidateRegions[i].Clusters) {
			regionID = i
			maxClusters = len(candidateRegions[i].Clusters)
		}
	}

	return regionID
}
