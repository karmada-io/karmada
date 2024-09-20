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
	"sort"

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

	// The average score of all cluster.
	averageScore int64

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

func checkIfDuplicate(rbSpec *workv1alpha2.ResourceBindingSpec) bool {
	return rbSpec.Placement == nil ||
		rbSpec.Placement.ReplicaScheduling == nil ||
		rbSpec.Placement.ReplicaScheduling.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated
}

func checkIfStaticWeight(rbSpec *workv1alpha2.ResourceBindingSpec) bool {
	return rbSpec.Placement != nil && rbSpec.Placement.ReplicaScheduling != nil &&
		rbSpec.Placement.ReplicaScheduling.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided &&
		rbSpec.Placement.ReplicaScheduling.ReplicaDivisionPreference == policyv1alpha1.ReplicaDivisionPreferenceWeighted &&
		(rbSpec.Placement.ReplicaScheduling.WeightPreference == nil ||
			len(rbSpec.Placement.ReplicaScheduling.WeightPreference.StaticWeightList) != 0 && rbSpec.Placement.ReplicaScheduling.WeightPreference.DynamicWeight == "")
}

func (info *GroupClustersInfo) calcGroupScore(clusters []ClusterDetailInfo, rbSpec *workv1alpha2.ResourceBindingSpec) int64 {
	// sort clusters by Score, from high score to low score.
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].Score != clusters[j].Score {
			return clusters[i].Score > clusters[j].Score
		}
		// if Score same，sort by Name.
		return clusters[i].Name < clusters[j].Name
	})
	var highScoreSum, selectedNum int64
	midIndex := (len(clusters) - 1) / 2
	// The same applies to both odd and even numbers.
	for i, cluster := range clusters {
		if i > midIndex {
			break
		}
		highScoreSum += cluster.Score
		selectedNum++
	}
	var baseScore, bonusScore int64
	baseScore = highScoreSum/selectedNum - (clusters[0].Score - clusters[len(clusters)-1].Score)

	// if duplicate =>
	if checkIfDuplicate(rbSpec) {
		var goodNum int
		for _, cluster := range clusters {
			if cluster.AvailableReplicas >= int64(rbSpec.Replicas) {
				goodNum++
			}
		}
		bonusRatio := float64(goodNum) / float64(len(clusters))
		bonusScore = int64(float64(baseScore) * bonusRatio)
		return baseScore + bonusScore
	}

	// if static weight =>
	if checkIfStaticWeight(rbSpec) {
		return baseScore
	}

	// if Aggregated or dynamic weight =>
	// Record the score ranking.
	scoreSortedMap := make(map[string]int)
	for i, cluster := range clusters {
		scoreSortedMap[cluster.Name] = i
	}

	// sort clusters by AvailableReplicas, from high score to low score.
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].AvailableReplicas != clusters[j].AvailableReplicas {
			return clusters[i].AvailableReplicas > clusters[j].AvailableReplicas
		}
		// if AvailableReplicas same，sort by Name.
		return clusters[i].Name < clusters[j].Name
	})
	var sumDistance int64
	// the index of high score should be closed to the index of high AvailableReplicas.
	// then the sumDistance will be lowed.
	for i, cluster := range clusters {
		sortedIndex := scoreSortedMap[cluster.Name]
		sumDistance += int64(math.Abs(float64(sortedIndex - i)))
	}

	avgDistance := sumDistance / int64(len(clusters))
	bonusScore = -avgDistance * info.averageScore

	return baseScore + bonusScore
}

func (info *GroupClustersInfo) generateClustersInfo(clustersScore framework.ClusterScoreList, rbSpec *workv1alpha2.ResourceBindingSpec) {
	var clusters []*clusterv1alpha1.Cluster
	var totalScore int64
	for _, clusterScore := range clustersScore {
		totalScore += clusterScore.Score
		clusterInfo := ClusterDetailInfo{}
		clusterInfo.Name = clusterScore.Cluster.Name
		clusterInfo.Score = clusterScore.Score
		clusterInfo.Cluster = clusterScore.Cluster
		info.Clusters = append(info.Clusters, clusterInfo)
		clusters = append(clusters, clusterScore.Cluster)
	}
	info.averageScore = totalScore / int64(len(clusters))

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

	for zone, zoneInfo := range info.Zones {
		zoneInfo.Score = info.calcGroupScore(zoneInfo.Clusters, rbSpec)
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

	for region, regionInfo := range info.Regions {
		regionInfo.Score = info.calcGroupScore(regionInfo.Clusters, rbSpec)
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

	for provider, providerInfo := range info.Providers {
		providerInfo.Score = info.calcGroupScore(providerInfo.Clusters, rbSpec)
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
