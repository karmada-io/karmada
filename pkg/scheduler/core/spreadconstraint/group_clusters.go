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

const weightUnit int64 = 1000

func (info *GroupClustersInfo) calcGroupScore(
	clusters []ClusterDetailInfo,
	rbSpec *workv1alpha2.ResourceBindingSpec,
	minGroups int) int64 {

	// if the replica scheduling type is divided, the score is calculated by followed.
	int32MinGroups := int32(minGroups)
	restReplica := rbSpec.Replicas % int32MinGroups
	replica := rbSpec.Replicas - restReplica
	targetReplica := int64(replica/int32MinGroups + 1)

	// clusters have been sorted by cluster.Score descending,
	// and if the cluster.Score is the same, the cluster.availableReplica is ascending.
	var sumAvailableReplica int64
	var sumScore int64
	var validClusters int64
	for _, cluster := range clusters {
		sumAvailableReplica += cluster.AvailableReplicas
		sumScore += cluster.Score
		validClusters++
		if sumAvailableReplica >= targetReplica {
			break
		}
	}

	// cluster.Score is 0 or 100. To minimize the impact of Score,
	// set the atomic value of targetReplica to 1000. This way,
	// when sorting by Group Score, targetReplica will be considered first,
	// and if the Weights are the same, then Score will be considered.

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

	isGroupByZone := false
	var minGroups int
	if rbSpec != nil && rbSpec.Placement != nil && rbSpec.Placement.SpreadConstraints != nil {
		for _, sc := range rbSpec.Placement.SpreadConstraints {
			if sc.SpreadByField == policyv1alpha1.SpreadByFieldZone {
				isGroupByZone = true
				minGroups = sc.MinGroups
			}
		}
	}

	for zone, zoneInfo := range info.Zones {
		if isGroupByZone {
			zoneInfo.Score = info.calcGroupScore(zoneInfo.Clusters, rbSpec, minGroups)
		}
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

	isGroupByRegion := false
	var minGroups int
	if rbSpec != nil && rbSpec.Placement != nil && rbSpec.Placement.SpreadConstraints != nil {
		for _, sc := range rbSpec.Placement.SpreadConstraints {
			if sc.SpreadByField == policyv1alpha1.SpreadByFieldRegion {
				isGroupByRegion = true
				minGroups = sc.MinGroups
			}
		}
	}

	for region, regionInfo := range info.Regions {
		if isGroupByRegion {
			regionInfo.Score = info.calcGroupScore(regionInfo.Clusters, rbSpec, minGroups)
		}
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

	isGroupByProvider := false
	var minGroups int

	if rbSpec != nil && rbSpec.Placement != nil && rbSpec.Placement.SpreadConstraints != nil {
		for _, sc := range rbSpec.Placement.SpreadConstraints {
			if sc.SpreadByField == policyv1alpha1.SpreadByFieldProvider {
				isGroupByProvider = true
				minGroups = sc.MinGroups
			}
		}
	}
	for provider, providerInfo := range info.Providers {
		if isGroupByProvider {
			providerInfo.Score = info.calcGroupScore(providerInfo.Clusters, rbSpec, minGroups)
		}
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
