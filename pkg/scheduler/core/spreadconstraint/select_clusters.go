package spreadconstraint

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/core/spreadconstraint/algorithm"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

// calcAvailableReplicasFunc defines the function that calculates available replicas of clusters
type calcAvailableReplicasFunc func(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster

// pair is the combination of priority and weight.
type pair struct {
	priority int8
	weight   int8
}

// priorityAndWeight indicates the combination of weight and priority.
//
// When there are multiple kinds, the kind with the highest priority is used first.
// For example: there are kinds: provider, region, cluster, we will select clusters by provider.
//
// Weight is used to measure the importance of number of different kinds.
var priorityAndWeight = map[string]pair{
	LabelKind:    {priority: math.MaxInt8},
	ProviderKind: {priority: 9},
	RegionKind:   {priority: 7},
	ZoneKind:     {priority: 5, weight: 20},
	ClusterKind:  {priority: 3, weight: 41},
}

type selector struct {
	groups          []Group
	constraints     []algorithm.Constraint
	selectByKind    string
	kindsWithNumber map[string]int64
}

func kindKey(kind string) string {
	if strings.HasPrefix(kind, LabelKind) {
		return LabelKind
	}
	return kind
}

// createSelector creates the cluster selector.
func createSelector(clusterScoreList framework.ClusterScoreList, rbSpec *workv1alpha2.ResourceBindingSpec, calcFunc calcAvailableReplicasFunc) *selector {
	s := &selector{kindsWithNumber: make(map[string]int64)}
	var maxPriority int8
	for _, constraint := range rbSpec.Placement.SpreadConstraints {
		kind := string(constraint.SpreadByField)
		if len(kind) == 0 {
			kind = "label:" + constraint.SpreadByLabel
		}

		pw := priorityAndWeight[kindKey(kind)]
		if pw.priority > maxPriority {
			maxPriority = pw.priority
			s.selectByKind = kind
		}
		s.constraints = append(s.constraints, algorithm.Constraint{
			Kind:      kind,
			Weight:    pw.weight,
			MaxGroups: int64(constraint.MaxGroups),
			MinGroups: int64(constraint.MinGroups),
		})
	}

	// Generate topology information by kind.
	// TODO(whitewindmills): add more cases, such as label.
	switch s.selectByKind {
	case ProviderKind:
		s.buildProviders(s.gatherClusters(clusterScoreList))
	case RegionKind:
		s.buildRegions(s.gatherClusters(clusterScoreList))
	case ZoneKind:
		s.buildZones(s.gatherClusters(clusterScoreList))
	case ClusterKind:
		s.buildClusters(clusterScoreList, rbSpec, calcFunc)
	}
	s.kindsWithNumber[s.selectByKind] = int64(len(s.groups))
	return s
}

func (s *selector) gatherClusters(clusterScoreList framework.ClusterScoreList) []*ClusterInfo {
	clusters := make([]*ClusterInfo, 0, len(clusterScoreList))
	for _, clusterScore := range clusterScoreList {
		clusters = append(clusters, &ClusterInfo{
			Cluster: clusterScore.Cluster,
			Score:   clusterScore.Score,
		})
	}
	s.kindsWithNumber[ClusterKind] = int64(len(clusters))

	return clusters
}

func (s *selector) buildClusters(clusterScoreList framework.ClusterScoreList, rbSpec *workv1alpha2.ResourceBindingSpec, calcFunc calcAvailableReplicasFunc) {
	clusters := make([]*clusterv1alpha1.Cluster, 0, len(clusterScoreList))
	s.groups = make([]Group, 0, len(clusterScoreList))
	for _, clusterScore := range clusterScoreList {
		clusters = append(clusters, clusterScore.Cluster)
		s.groups = append(s.groups, &ClusterInfo{
			Cluster: clusterScore.Cluster,
			Score:   clusterScore.Score,
		})
	}

	var replicasSum int64
	targetClusters := calcFunc(clusters, rbSpec)
	for i, tc := range targetClusters {
		clusterInfo := s.groups[i].(*ClusterInfo)
		clusterInfo.AvailableReplicas = int64(tc.Replicas)
		clusterInfo.AvailableReplicas += int64(rbSpec.AssignedReplicasForCluster(tc.Name))
		replicasSum += clusterInfo.AvailableReplicas
	}
	// Replica constraint is added by default when selecting by cluster without duplicated strategy.
	if rbSpec.Placement.ReplicaSchedulingType() != policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		s.constraints = append(s.constraints, algorithm.Constraint{
			Kind:      ReplicaKind,
			MinGroups: int64(rbSpec.Replicas),
			MaxGroups: math.MaxInt64,
		})
		s.kindsWithNumber[ReplicaKind] = replicasSum
	}
}

func (s *selector) buildZones(clusters []*ClusterInfo) []*ZoneInfo {
	var zones []*ZoneInfo
	for _, cluster := range clusters {
		var zone *ZoneInfo
		for _, z := range zones {
			if z.Name == cluster.Spec.Zone {
				zone = z
				break
			}
		}
		if zone == nil {
			zone = &ZoneInfo{
				ClusterGroupInfo: &ClusterGroupInfo{
					Name: cluster.Spec.Zone,
				},
			}
			zones = append(zones, zone)
			if s.selectByKind == ZoneKind {
				s.groups = append(s.groups, zone)
			}
		}
		zone.Clusters = append(zone.Clusters, cluster)
	}

	if s.selectByKind == ZoneKind {
		for _, zone := range zones {
			zone.Score = algorithm.CalculateComprehensiveScore(zone.Clusters)
		}
	}
	return zones
}

func (s *selector) buildRegions(clusters []*ClusterInfo) []*RegionInfo {
	var regions []*RegionInfo
	for _, cluster := range clusters {
		var region *RegionInfo
		for _, r := range regions {
			if r.Name == cluster.Spec.Region {
				region = r
				break
			}
		}
		if region == nil {
			region = &RegionInfo{
				ClusterGroupInfo: &ClusterGroupInfo{
					Name: cluster.Spec.Region,
				},
			}
			regions = append(regions, region)
			if s.selectByKind == RegionKind {
				s.groups = append(s.groups, region)
			}
		}
		region.Clusters = append(region.Clusters, cluster)
	}

	if s.selectByKind == RegionKind && s.isKindEnabled(ZoneKind) {
		for _, region := range regions {
			region.Zones = s.buildZones(region.Clusters)
			s.kindsWithNumber[ZoneKind] += int64(len(region.Zones))
			region.Score = algorithm.CalculateComprehensiveScore(region.Clusters)
		}
	}

	return regions
}

func (s *selector) buildProviders(clusters []*ClusterInfo) []*ProviderInfo {
	var providers []*ProviderInfo
	for _, cluster := range clusters {
		var provider *ProviderInfo
		for _, p := range providers {
			if p.Name == cluster.Spec.Provider {
				provider = p
				break
			}
		}
		if provider == nil {
			provider = &ProviderInfo{
				ClusterGroupInfo: &ClusterGroupInfo{
					Name: cluster.Spec.Provider,
				},
			}
			providers = append(providers, provider)
			if s.selectByKind == ProviderKind {
				s.groups = append(s.groups, provider)
			}
		}
		provider.Clusters = append(provider.Clusters, cluster)
	}

	regionEnabled := s.isKindEnabled(RegionKind)
	zoneEnabled := s.isKindEnabled(ZoneKind)
	for _, provider := range providers {
		if regionEnabled {
			provider.Regions = s.buildRegions(provider.Clusters)
			s.kindsWithNumber[RegionKind] += int64(len(provider.Regions))
		}
		if zoneEnabled {
			provider.Zones = s.buildZones(provider.Clusters)
			s.kindsWithNumber[ZoneKind] += int64(len(provider.Zones))
		}
		provider.Score = algorithm.CalculateComprehensiveScore(provider.Clusters)
	}
	return providers
}

func (s *selector) checkMinimumConstraint() error {
	var errs []error
	for _, constraint := range s.constraints {
		if s.kindsWithNumber[constraint.Kind] < constraint.MinGroups {
			errs = append(errs, fmt.Errorf("available number %d is less than %s minimum requirement %d",
				s.kindsWithNumber[constraint.Kind], constraint.Kind, constraint.MinGroups))
		}
	}
	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}

	return nil
}

func (s *selector) isKindEnabled(kind string) bool {
	for _, constraint := range s.constraints {
		if constraint.Kind == kind {
			return true
		}
	}
	return false
}

func (s *selector) selectClusters() ([]*clusterv1alpha1.Cluster, error) {
	if s.kindsWithNumber[s.selectByKind] == 0 {
		return nil, fmt.Errorf("%s spread is not supported now", s.selectByKind)
	}
	if err := s.checkMinimumConstraint(); err != nil {
		return nil, err
	}

	groups := algorithm.SelectGroups(s.selectByKind, s.groups, &algorithm.ConstraintContext{
		Constraints: s.constraints,
	})
	var clusters []*clusterv1alpha1.Cluster
	for _, g := range groups {
		clusters = append(clusters, g.GetClusters()...)
	}
	return clusters, nil
}

func selectClustersSimply(clusterScores framework.ClusterScoreList, rbSpec *workv1alpha2.ResourceBindingSpec) []*clusterv1alpha1.Cluster {
	availableClusters := getAvailableClusters(len(clusterScores), rbSpec.Placement)
	if availableClusters == 0 {
		return nil
	}

	if availableClusters < len(clusterScores) {
		sort.Slice(clusterScores, func(i, j int) bool {
			if clusterScores[i].Score != clusterScores[j].Score {
				return clusterScores[i].Score > clusterScores[j].Score
			}
			return clusterScores[i].Cluster.Name < clusterScores[j].Cluster.Name
		})
	}
	clusters := make([]*clusterv1alpha1.Cluster, 0, availableClusters)
	for i := 0; i < availableClusters; i++ {
		clusters = append(clusters, clusterScores[i].Cluster)
	}
	klog.V(2).Infof("Simply select %d clusters", availableClusters)
	return clusters
}

func getAvailableClusters(allClusters int, placement *policyv1alpha1.Placement) int {
	if len(placement.SpreadConstraints) == 0 {
		return allClusters
	}

	strategy := placement.ReplicaScheduling
	// If the replica division preference is 'static weighted', ignore the declaration specified by spread constraints.
	if strategy != nil && strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided &&
		strategy.ReplicaDivisionPreference == policyv1alpha1.ReplicaDivisionPreferenceWeighted &&
		(strategy.WeightPreference == nil ||
			(len(strategy.WeightPreference.StaticWeightList) != 0 && strategy.WeightPreference.DynamicWeight == "")) {
		return allClusters
	}

	return 0
}

func SelectClusters(clusterScoreList framework.ClusterScoreList, rbSpec *workv1alpha2.ResourceBindingSpec, calcFunc calcAvailableReplicasFunc) ([]*clusterv1alpha1.Cluster, error) {
	if clusters := selectClustersSimply(clusterScoreList, rbSpec); len(clusters) != 0 {
		return clusters, nil
	}

	return createSelector(clusterScoreList, rbSpec, calcFunc).selectClusters()
}
