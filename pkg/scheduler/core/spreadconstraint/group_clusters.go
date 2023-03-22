package spreadconstraint

import (
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
	Name  string
	Score int64 // the comprehensive score in all clusters of the provider

	// Regions under this provider
	Regions map[string]struct{}
	// Zones under this provider
	Zones map[string]struct{}
	// Clusters under this provider, sorted by cluster.Score descending.
	Clusters []ClusterDetailInfo
}

// RegionInfo indicate the region information
type RegionInfo struct {
	Name  string
	Score int64 // the comprehensive score in all clusters of the region

	// Zones under this provider
	Zones map[string]struct{}
	// Clusters under this region, sorted by cluster.Score descending.
	Clusters []ClusterDetailInfo
}

// ZoneInfo indicate the zone information
type ZoneInfo struct {
	Name  string
	Score int64 // the comprehensive score in all clusters of the zone

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
		return groupClustersIngoreTopology(clustersScore, spec, calAvailableReplicasFunc)
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

func groupClustersIngoreTopology(
	clustersScore framework.ClusterScoreList,
	rbSpec *workv1alpha2.ResourceBindingSpec,
	calAvailableReplicasFunc func(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster,
) *GroupClustersInfo {
	groupClustersInfo := &GroupClustersInfo{}
	groupClustersInfo.calAvailableReplicasFunc = calAvailableReplicasFunc
	groupClustersInfo.generateClustersInfo(clustersScore, rbSpec)

	return groupClustersInfo
}

func (info *GroupClustersInfo) generateClustersInfo(clustersScore framework.ClusterScoreList, rbSpec *workv1alpha2.ResourceBindingSpec) {
	var totalScore int64
	var clusters []*clusterv1alpha1.Cluster
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

	sortClusters(info.Clusters)
}

func (info *GroupClustersInfo) calcGroupScore(clusters []ClusterDetailInfo, rbSpec *workv1alpha2.ResourceBindingSpec) int64 {
	// First, calculate the base score.
	var highScoreSum int64
	var selectedNum int
	for _, cluster := range clusters {
		if cluster.Score > info.averageScore {
			highScoreSum += cluster.Score
			selectedNum++
		}
	}
	var baseScore int64
	if selectedNum == 0 {
		baseScore = clusters[0].Score
	} else {
		baseScore = highScoreSum / int64(selectedNum)
	}

	// Second, calculate the bonus score.
	var bonusScore int64
	if rbSpec.Placement == nil ||
		rbSpec.Placement.ReplicaScheduling == nil ||
		rbSpec.Placement.ReplicaScheduling.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		var goodNum int
		for _, cluster := range clusters {
			if cluster.AvailableReplicas >= int64(rbSpec.Replicas) {
				goodNum++
			}
		}
		bonusRatio := float64(goodNum) / float64(len(clusters))
		bonusScore = int64(float64(baseScore) * bonusRatio)
	}

	return baseScore + bonusScore
}

func (info *GroupClustersInfo) generateZoneInfo(spreadConstraints []policyv1alpha1.SpreadConstraint, rbSpec *workv1alpha2.ResourceBindingSpec) {
	if !IsSpreadConstraintExisted(spreadConstraints, policyv1alpha1.SpreadByFieldZone) {
		return
	}

	for _, clusterInfo := range info.Clusters {
		zone := clusterInfo.Cluster.Spec.Zone
		if zone == "" {
			continue
		}

		zoneInfo, ok := info.Zones[zone]
		if !ok {
			zoneInfo = ZoneInfo{
				Name:     zone,
				Clusters: make([]ClusterDetailInfo, 0),
			}
		}

		zoneInfo.Clusters = append(zoneInfo.Clusters, clusterInfo)
		info.Zones[zone] = zoneInfo
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
