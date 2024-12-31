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
	"math"

	"k8s.io/utils/ptr"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

// GroupClustersInfo indicate the cluster global view
type GroupClustersInfo struct {
	Providers map[string]ProviderInfo
	Regions   map[string]RegionInfo
	Zones     map[string]ZoneInfo

	// Clusters from global view, sorted by cluster.Score descending.
	Clusters []ClusterDetailInfo

	calAvailableReplicasFunc func(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster
}

// ProviderInfo indicate the provider information
type ProviderInfo struct {
	Name              string
	Score             int64 // the comprehensive score in all clusters of the provider
	AvailableReplicas int64

	// Regions under this provider
	Regions map[string]struct{}
	// Zones under this provider
	Zones map[string]struct{}
	// Clusters under this provider, sorted by cluster.Score descending.
	Clusters []ClusterDetailInfo
}

// RegionInfo indicate the region information
type RegionInfo struct {
	Name              string
	Score             int64 // the comprehensive score in all clusters of the region
	AvailableReplicas int64

	// Zones under this provider
	Zones map[string]struct{}
	// Clusters under this region, sorted by cluster.Score descending.
	Clusters []ClusterDetailInfo
}

// ZoneInfo indicate the zone information
type ZoneInfo struct {
	Name              string
	Score             int64 // the comprehensive score in all clusters of the zone
	AvailableReplicas int64

	// Clusters under this zone, sorted by cluster.Score descending.
	Clusters []ClusterDetailInfo
}

// ClusterDetailInfo indicate the cluster information
type ClusterDetailInfo struct {
	Name              string
	Score             int64
	AvailableReplicas int64

	Cluster *clusterv1alpha1.Cluster
}

// GroupClustersWithScore groups cluster base provider/region/zone/cluster
func GroupClustersWithScore(
	clustersScore framework.ClusterScoreList,
	placement *policyv1alpha1.Placement,
	spec *workv1alpha2.ResourceBindingSpec,
	calAvailableReplicasFunc func(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster,
) *GroupClustersInfo {
	if isTopologyIgnored(placement) {
		return groupClustersIgnoringTopology(clustersScore, spec, calAvailableReplicasFunc)
	}

	return groupClustersBasedTopology(clustersScore, spec, placement.SpreadConstraints, calAvailableReplicasFunc)
}

func groupClustersBasedTopology(
	clustersScore framework.ClusterScoreList,
	rbSpec *workv1alpha2.ResourceBindingSpec,
	spreadConstraints []policyv1alpha1.SpreadConstraint,
	calAvailableReplicasFunc func(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster,
) *GroupClustersInfo {
	groupClustersInfo := &GroupClustersInfo{
		Providers: make(map[string]ProviderInfo),
		Regions:   make(map[string]RegionInfo),
		Zones:     make(map[string]ZoneInfo),
	}
	groupClustersInfo.calAvailableReplicasFunc = calAvailableReplicasFunc
	groupClustersInfo.generateClustersInfo(clustersScore, rbSpec)
	groupClustersInfo.generateZoneInfo(spreadConstraints, rbSpec)
	groupClustersInfo.generateRegionInfo(spreadConstraints, rbSpec)
	groupClustersInfo.generateProviderInfo(spreadConstraints, rbSpec)

	return groupClustersInfo
}

func groupClustersIgnoringTopology(
	clustersScore framework.ClusterScoreList,
	rbSpec *workv1alpha2.ResourceBindingSpec,
	calAvailableReplicasFunc func(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster,
) *GroupClustersInfo {
	groupClustersInfo := &GroupClustersInfo{}
	groupClustersInfo.calAvailableReplicasFunc = calAvailableReplicasFunc
	groupClustersInfo.generateClustersInfo(clustersScore, rbSpec)

	return groupClustersInfo
}

// weightUnit is used to minimize the impact of avg(cluster.Score).
// By multiply the weightUnit, the unit of targetReplica will be 1000, and the avg(cluster.Score) will in [0, 100].
// Thus, when sorting by Group Score, targetReplica will be considered first, and if the Weights are the same, then Score will be considered.
const weightUnit int64 = 1000

func (info *GroupClustersInfo) calcGroupScoreForDuplicate(
	clusters []ClusterDetailInfo,
	rbSpec *workv1alpha2.ResourceBindingSpec) int64 {
	targetReplica := int64(rbSpec.Replicas)
	var validClusters int64
	// validClusters is the number of clusters that have available replicas.
	var sumValidScore int64
	for _, cluster := range clusters {
		if cluster.AvailableReplicas >= targetReplica {
			validClusters++
			sumValidScore += cluster.Score
		}
	}

	// Here is an example, the rbSpec.Replicas == 50.

	// There is the Group 1, it has five clusters as follows.
	// ----------------------------------------------------------------------
	// | clusterName      | member1 | member2 | member3 | member4 | member5 |
	// |---------------------------------------------------------------------
	// | score            |   100   |   100   |   100   |   100   |   100   |
	// |------------------------------------------------|---------|---------|
	// |AvailableReplicas |   60    |    70   |    40   |    30   |    10   |
	// |------------------------------------------------|---------|---------|

	// There is the Group 2, it has four clusters as follows.
	// ------------------------------------------------------------
	// | clusterName      | member1 | member2 | member3 | member4 |
	// |-----------------------------------------------------------
	// | score            |    0    |    0    |    0    |    0    |
	// |------------------------------------------------|---------|
	// |AvailableReplicas |   60    |    60   |    60   |    60   |
	// |------------------------------------------------|---------|

	// According to our expectations, Group 2 is a more ideal choice than Group 1,
	// as the number of clusters in Group 2 that meet the Replica requirements
	// for available copies is greater. Although the average Cluster.Score in Group 1 is higher,
	// under the Duplicate replica allocation strategy,
	// we prioritize whether the number of available replicas in each Cluster
	// meets the Replica requirements. Based on our algorithm, the score for Group 2
	// is also higher than that of Group 1.

	// Group1's Score = 2 * 1000 + 100 = 2100
	// Group2's Score = 4 * 1000 + 0 = 4000

	// There is another example, the rbSpec.Replicas == 50.

	// There is the Group 1, it has five clusters as follows.
	// ----------------------------------------------------------------------
	// | clusterName      | member1 | member2 | member3 | member4 | member5 |
	// |---------------------------------------------------------------------
	// | score            |   100   |   100   |   100   |   100   |   100   |
	// |------------------------------------------------|---------|---------|
	// |AvailableReplicas |   60    |    70   |    10   |    10   |    5    |
	// |------------------------------------------------|---------|---------|

	// There is the Group 2, it has four clusters as follows.
	// ------------------------------------------------------------
	// | clusterName      | member1 | member2 | member3 | member4 |
	// |-----------------------------------------------------------
	// | score            |    0    |    0    |    0    |    0    |
	// |------------------------------------------------|---------|
	// |AvailableReplicas |   100   |    100  |    10   |    10   |
	// |------------------------------------------------|---------|

	// According to our expectations, Group 1 is a more ideal choice than Group 2.
	// Although the number of clusters meeting the Replica requirements for available
	// copies is the same in both Group 1 and Group 2, the average Cluster.Score in Group 1 is higher.
	// Therefore, Group 1 is the better choice. Based on our algorithm,
	// the score for Group 1 is also higher than that of Group 2.

	// Group1's Score = 2 * 1000 + 100 = 2100
	// Group2's Score = 2 * 1000 + 0 = 2000

	// the priority of validClusters is higher than sumValidScore.
	weightedValidClusters := validClusters * weightUnit
	return weightedValidClusters + sumValidScore/validClusters
}

func (info *GroupClustersInfo) calcGroupScore(
	clusters []ClusterDetailInfo,
	rbSpec *workv1alpha2.ResourceBindingSpec,
	minGroups int) int64 {
	if rbSpec.Placement == nil || rbSpec.Placement.ReplicaSchedulingType() == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		// if the replica scheduling type is duplicated, the score is calculated by calcGroupScoreForDuplicate.
		return info.calcGroupScoreForDuplicate(clusters, rbSpec)
	}

	// if the replica scheduling type is divided, the score is calculated by followed.
	float64MinGroups := float64(minGroups)
	targetReplica := int64(math.Ceil(float64(rbSpec.Replicas) / float64MinGroups))

	// get the minGroups of Cluster.
	var clusterMinGroups int
	if rbSpec.Placement.SpreadConstraints != nil {
		for _, sc := range rbSpec.Placement.SpreadConstraints {
			if sc.SpreadByField == policyv1alpha1.SpreadByFieldCluster {
				clusterMinGroups = sc.MinGroups
			}
		}
	}

	// if the minGroups of Cluster is less than the minGroups of Group, set the minGroups of Cluster to the minGroups of Group.
	if clusterMinGroups < minGroups {
		clusterMinGroups = minGroups
	}
	int64ClusterMinGroups := int64(clusterMinGroups)

	// clusters have been sorted by cluster.Score descending,
	// and if the cluster.Score is the same, the cluster.availableReplica is ascending.
	var sumAvailableReplica int64
	var sumScore int64
	var validClusters int64
	for _, cluster := range clusters {
		sumAvailableReplica += cluster.AvailableReplicas
		sumScore += cluster.Score
		validClusters++
		if validClusters >= int64ClusterMinGroups && sumAvailableReplica >= targetReplica {
			break
		}
	}

	// cluster.Score is 0 or 100. To minimize the impact of Score,
	// set the atomic value of targetReplica to 1000. This way,
	// when sorting by Group Score, targetReplica will be considered first,
	// and if the Weights are the same, then Score will be considered.

	// Here is an example, the rbSpec.Replicas == 100 and the Group.minGroups == 2, Cluster.minGroups == 1.
	// Thus, the targetReplica is 50, and the int64ClusterMinGroups == 2, because int64ClusterMinGroups == max(Group.minGroups, Cluster.minGroups).

	// There is the Group 1, it has five clusters as follows.
	// ----------------------------------------------------------------------
	// | clusterName      | member1 | member2 | member3 | member4 | member5 |
	// |---------------------------------------------------------------------
	// | score            |   100   |   100   |   100   |   100   |   100   |
	// |------------------------------------------------|---------|---------|
	// |AvailableReplicas |   10    |    10   |    10   |    10   |    5    |
	// |------------------------------------------------|---------|---------|

	// There is the Group 2, it has four clusters as follows.
	// ------------------------------------------------------------
	// | clusterName      | member1 | member2 | member3 | member4 |
	// |-----------------------------------------------------------
	// | score            |    0    |    0    |    0    |    0    |
	// |------------------------------------------------|---------|
	// |AvailableReplicas |   40    |    30   |    10   |    10   |
	// |------------------------------------------------|---------|

	// According to our expectations, Group 2 is a more ideal choice
	// than Group 1 because Group 2 has more available replica capacity,
	// which meets the needs of replica allocation, even though Group 1 has a higher Cluster balance.
	// Based on our algorithm, Group 2’s Score is also higher than that of Group 1.

	// Group1's Score = 45 * 1000 + 100 = 45100
	// Group2's Score = 50 * 1000 + 0 = 50000

	// There is another example, the targetReplica is 50, and the int64ClusterMinGroups == 2.
	// The difference now is the situation of the Groups; both Groups now meet the requirements for available replica capacity.

	// There is the Group 1, it has five clusters as follows.
	// ----------------------------------------------------------------------
	// | clusterName      | member1 | member2 | member3 | member4 | member5 |
	// |---------------------------------------------------------------------
	// | score            |   100   |   100   |   100   |   100   |   100   |
	// |------------------------------------------------|---------|---------|
	// |AvailableReplicas |   40    |    40   |    10   |    10   |    5    |
	// |------------------------------------------------|---------|---------|

	// There is the Group 2, it has four clusters as follows.
	// ------------------------------------------------------------
	// | clusterName      | member1 | member2 | member3 | member4 |
	// |-----------------------------------------------------------
	// | score            |    0    |    0    |    0    |    0    |
	// |------------------------------------------------|---------|
	// |AvailableReplicas |   100   |    100  |    10   |    10   |
	// |------------------------------------------------|---------|

	// According to our expectations, Group 1 is a more ideal choice than Group 2,
	// as both Group 2 and Group 1 can now meet the replica allocation requirements.
	// However, Group 1 has a higher Cluster balance (even though Group 2 has more available replicas).
	// Based on our algorithm, the Score for Group 1 is also higher than that of Group 2.

	// Group1's Score = 50 * 1000 + 100 = 50100
	// Group2's Score = 50 * 1000 + 0 = 50000

	if sumAvailableReplica < targetReplica {
		sumAvailableReplica = sumAvailableReplica * weightUnit
		return sumAvailableReplica + sumScore/int64(len(clusters))
	}

	targetReplica = targetReplica * weightUnit
	return targetReplica + sumScore/validClusters
}

func (info *GroupClustersInfo) generateClustersInfo(clustersScore framework.ClusterScoreList, rbSpec *workv1alpha2.ResourceBindingSpec) {
	var clusters []*clusterv1alpha1.Cluster
	for _, clusterScore := range clustersScore {
		clusterInfo := ClusterDetailInfo{}
		clusterInfo.Name = clusterScore.Cluster.Name
		clusterInfo.Score = clusterScore.Score
		clusterInfo.Cluster = clusterScore.Cluster
		info.Clusters = append(info.Clusters, clusterInfo)
		clusters = append(clusters, clusterScore.Cluster)
	}

	clustersReplicas := info.calAvailableReplicasFunc(clusters, rbSpec)
	for i, clustersReplica := range clustersReplicas {
		info.Clusters[i].AvailableReplicas = int64(clustersReplica.Replicas)
		info.Clusters[i].AvailableReplicas += int64(rbSpec.AssignedReplicasForCluster(clustersReplica.Name))
	}

	sortClusters(info.Clusters, func(i *ClusterDetailInfo, j *ClusterDetailInfo) *bool {
		if i.AvailableReplicas != j.AvailableReplicas {
			return ptr.To(i.AvailableReplicas > j.AvailableReplicas)
		}
		return nil
	})
}

func (info *GroupClustersInfo) generateZoneInfo(spreadConstraints []policyv1alpha1.SpreadConstraint, rbSpec *workv1alpha2.ResourceBindingSpec) {
	if !IsSpreadConstraintExisted(spreadConstraints, policyv1alpha1.SpreadByFieldZone) {
		return
	}

	for _, clusterInfo := range info.Clusters {
		zones := clusterInfo.Cluster.Spec.Zones
		if len(zones) == 0 {
			continue
		}

		for _, zone := range zones {
			zoneInfo, ok := info.Zones[zone]
			if !ok {
				zoneInfo = ZoneInfo{
					Name:     zone,
					Clusters: make([]ClusterDetailInfo, 0),
				}
			}
			zoneInfo.Clusters = append(zoneInfo.Clusters, clusterInfo)
			zoneInfo.AvailableReplicas += clusterInfo.AvailableReplicas
			info.Zones[zone] = zoneInfo
		}
	}

	var minGroups int
	for _, sc := range spreadConstraints {
		if sc.SpreadByField == policyv1alpha1.SpreadByFieldZone {
			minGroups = sc.MinGroups
		}
	}

	for zone, zoneInfo := range info.Zones {
		zoneInfo.Score = info.calcGroupScore(zoneInfo.Clusters, rbSpec, minGroups)
		info.Zones[zone] = zoneInfo
	}
}

func (info *GroupClustersInfo) generateRegionInfo(spreadConstraints []policyv1alpha1.SpreadConstraint, rbSpec *workv1alpha2.ResourceBindingSpec) {
	if !IsSpreadConstraintExisted(spreadConstraints, policyv1alpha1.SpreadByFieldRegion) {
		return
	}

	for _, clusterInfo := range info.Clusters {
		region := clusterInfo.Cluster.Spec.Region
		if region == "" {
			continue
		}

		regionInfo, ok := info.Regions[region]
		if !ok {
			regionInfo = RegionInfo{
				Name:     region,
				Zones:    make(map[string]struct{}),
				Clusters: make([]ClusterDetailInfo, 0),
			}
		}

		if clusterInfo.Cluster.Spec.Zone != "" {
			regionInfo.Zones[clusterInfo.Cluster.Spec.Zone] = struct{}{}
		}
		regionInfo.Clusters = append(regionInfo.Clusters, clusterInfo)
		regionInfo.AvailableReplicas += clusterInfo.AvailableReplicas
		info.Regions[region] = regionInfo
	}

	var minGroups int
	for _, sc := range spreadConstraints {
		if sc.SpreadByField == policyv1alpha1.SpreadByFieldRegion {
			minGroups = sc.MinGroups
		}
	}

	for region, regionInfo := range info.Regions {
		regionInfo.Score = info.calcGroupScore(regionInfo.Clusters, rbSpec, minGroups)
		info.Regions[region] = regionInfo
	}
}

func (info *GroupClustersInfo) generateProviderInfo(spreadConstraints []policyv1alpha1.SpreadConstraint, rbSpec *workv1alpha2.ResourceBindingSpec) {
	if !IsSpreadConstraintExisted(spreadConstraints, policyv1alpha1.SpreadByFieldProvider) {
		return
	}

	for _, clusterInfo := range info.Clusters {
		provider := clusterInfo.Cluster.Spec.Provider
		if provider == "" {
			continue
		}

		providerInfo, ok := info.Providers[provider]
		if !ok {
			providerInfo = ProviderInfo{
				Name:     provider,
				Regions:  make(map[string]struct{}),
				Zones:    make(map[string]struct{}),
				Clusters: make([]ClusterDetailInfo, 0),
			}
		}

		if clusterInfo.Cluster.Spec.Zone != "" {
			providerInfo.Zones[clusterInfo.Cluster.Spec.Zone] = struct{}{}
		}

		if clusterInfo.Cluster.Spec.Region != "" {
			providerInfo.Regions[clusterInfo.Cluster.Spec.Region] = struct{}{}
		}

		providerInfo.Clusters = append(providerInfo.Clusters, clusterInfo)
		providerInfo.AvailableReplicas += clusterInfo.AvailableReplicas
		info.Providers[provider] = providerInfo
	}

	var minGroups int
	for _, sc := range spreadConstraints {
		if sc.SpreadByField == policyv1alpha1.SpreadByFieldProvider {
			minGroups = sc.MinGroups
		}
	}

	for provider, providerInfo := range info.Providers {
		providerInfo.Score = info.calcGroupScore(providerInfo.Clusters, rbSpec, minGroups)
		info.Providers[provider] = providerInfo
	}
}

func isTopologyIgnored(placement *policyv1alpha1.Placement) bool {
	spreadConstraints := placement.SpreadConstraints

	if len(spreadConstraints) == 0 || (len(spreadConstraints) == 1 && spreadConstraints[0].SpreadByField == policyv1alpha1.SpreadByFieldCluster) {
		return true
	}

	return shouldIgnoreSpreadConstraint(placement)
}
