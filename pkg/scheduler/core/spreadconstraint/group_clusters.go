package spreadconstraint

import (
	"sort"

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

	// Clusters from globally view, sorted by cluster.Score descending.
	Clusters []ClusterDetailInfo
}

// ProviderInfo indicate the provider information
type ProviderInfo struct {
	Name              string
	Score             int64
	AvailableReplicas int64

	Regions map[string]struct{}
	Zones   map[string]struct{}
	// Clusters under this provider, sorted by cluster.Score descending.
	Clusters []ClusterDetailInfo
}

// RegionInfo indicate the region information
type RegionInfo struct {
	Name              string
	Score             int64
	AvailableReplicas int64

	Zones map[string]struct{}
	// Clusters under this region, sorted by cluster.Score descending.
	Clusters []ClusterDetailInfo
}

// ZoneInfo indicate the zone information
type ZoneInfo struct {
	Name              string
	Score             int64
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
) *GroupClustersInfo {
	if isTopologyIgnored(placement) {
		return groupClustersIngoreTopology(clustersScore, spec)
	}

	return groupClustersBasedTopology(clustersScore, spec, placement.SpreadConstraints)
}

func groupClustersBasedTopology(
	clustersScore framework.ClusterScoreList,
	rbSpec *workv1alpha2.ResourceBindingSpec,
	spreadConstraints []policyv1alpha1.SpreadConstraint,
) *GroupClustersInfo {
	groupClustersInfo := &GroupClustersInfo{
		Providers: make(map[string]ProviderInfo),
		Regions:   make(map[string]RegionInfo),
		Zones:     make(map[string]ZoneInfo),
	}

	groupClustersInfo.generateClustersInfo(clustersScore, rbSpec)
	groupClustersInfo.generateZoneInfo(spreadConstraints)
	groupClustersInfo.generateRegionInfo(spreadConstraints)
	groupClustersInfo.generateProviderInfo(spreadConstraints)

	return groupClustersInfo
}

func groupClustersIngoreTopology(
	clustersScore framework.ClusterScoreList,
	rbSpec *workv1alpha2.ResourceBindingSpec,
) *GroupClustersInfo {
	groupClustersInfo := &GroupClustersInfo{}
	groupClustersInfo.generateClustersInfo(clustersScore, rbSpec)

	return groupClustersInfo
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

	clustersReplicas := calAvailableReplicas(clusters, rbSpec)
	for i, clustersReplica := range clustersReplicas {
		info.Clusters[i].AvailableReplicas = int64(clustersReplica.Replicas)
	}

	sortClusters(info.Clusters)
}

func (info *GroupClustersInfo) generateZoneInfo(spreadConstraints []policyv1alpha1.SpreadConstraint) {
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
		zoneInfo.Score += clusterInfo.Score
		zoneInfo.AvailableReplicas += clusterInfo.AvailableReplicas
		info.Zones[zone] = zoneInfo
	}
}

func (info *GroupClustersInfo) generateRegionInfo(spreadConstraints []policyv1alpha1.SpreadConstraint) {
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
		regionInfo.Score += clusterInfo.Score
		regionInfo.AvailableReplicas += clusterInfo.AvailableReplicas
		info.Regions[region] = regionInfo
	}
}

func (info *GroupClustersInfo) generateProviderInfo(spreadConstraints []policyv1alpha1.SpreadConstraint) {
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
		providerInfo.Score += clusterInfo.Score
		providerInfo.AvailableReplicas += clusterInfo.AvailableReplicas
		info.Providers[provider] = providerInfo
	}
}

func isTopologyIgnored(placement *policyv1alpha1.Placement) bool {
	strategy := placement.ReplicaScheduling
	spreadConstraints := placement.SpreadConstraints

	if len(spreadConstraints) == 0 || (len(spreadConstraints) == 1 && spreadConstraints[0].SpreadByField == policyv1alpha1.SpreadByFieldCluster) {
		return true
	}

	// If the replica division preference is 'static weighted', ignore the declaration specified by spread constraints.
	if strategy != nil && strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided &&
		strategy.ReplicaDivisionPreference == policyv1alpha1.ReplicaDivisionPreferenceWeighted &&
		(strategy.WeightPreference == nil || strategy.WeightPreference.DynamicWeight == "") {
		return true
	}

	return false
}

func sortClusters(infos []ClusterDetailInfo) {
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Score != infos[j].Score {
			return infos[i].Score > infos[j].Score
		}

		return infos[i].Name < infos[j].Name
	})
}
