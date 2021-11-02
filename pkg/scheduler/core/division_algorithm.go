package core

import (
	"fmt"
	"sort"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// TargetClustersList is a slice of TargetCluster that implements sort.Interface to sort by Value.
type TargetClustersList []workv1alpha2.TargetCluster

func (a TargetClustersList) Len() int           { return len(a) }
func (a TargetClustersList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a TargetClustersList) Less(i, j int) bool { return a[i].Replicas > a[j].Replicas }

// divideReplicasByDynamicWeight assigns a total number of replicas to the selected clusters by the dynamic weight list.
func divideReplicasByDynamicWeight(clusters []*clusterv1alpha1.Cluster, dynamicWeight policyv1alpha1.DynamicWeightFactor, spec *workv1alpha2.ResourceBindingSpec) ([]workv1alpha2.TargetCluster, error) {
	switch dynamicWeight {
	case policyv1alpha1.DynamicWeightByAvailableReplicas:
		return divideReplicasByResource(clusters, spec, policyv1alpha1.ReplicaDivisionPreferenceWeighted)
	default:
		return nil, fmt.Errorf("undefined replica dynamic weight factor: %s", dynamicWeight)
	}
}

func divideReplicasByResource(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec,
	preference policyv1alpha1.ReplicaDivisionPreference, preUsedClustersName ...string) ([]workv1alpha2.TargetCluster, error) {
	clusterAvailableReplicas := calAvailableReplicas(clusters, spec)
	sort.Sort(TargetClustersList(clusterAvailableReplicas))
	return divideReplicasByPreference(clusterAvailableReplicas, spec.Replicas, preference, preUsedClustersName...)
}

// divideReplicasByStaticWeight assigns a total number of replicas to the selected clusters by the weight list.
func divideReplicasByStaticWeight(clusters []*clusterv1alpha1.Cluster, weightList []policyv1alpha1.StaticClusterWeight,
	replicas int32) ([]workv1alpha2.TargetCluster, error) {
	weightSum := int64(0)
	matchClusters := make(map[string]int64)
	desireReplicaInfos := make(map[string]int64)

	for _, cluster := range clusters {
		for _, staticWeightRule := range weightList {
			if util.ClusterMatches(cluster, staticWeightRule.TargetCluster) {
				weightSum += staticWeightRule.Weight
				matchClusters[cluster.Name] = staticWeightRule.Weight
				break
			}
		}
	}

	if weightSum == 0 {
		for _, cluster := range clusters {
			weightSum++
			matchClusters[cluster.Name] = 1
		}
	}

	allocatedReplicas := int32(0)
	for clusterName, weight := range matchClusters {
		desireReplicaInfos[clusterName] = weight * int64(replicas) / weightSum
		allocatedReplicas += int32(desireReplicaInfos[clusterName])
	}

	if remainReplicas := replicas - allocatedReplicas; remainReplicas > 0 {
		sortedClusters := helper.SortClusterByWeight(matchClusters)
		for i := 0; remainReplicas > 0; i++ {
			desireReplicaInfos[sortedClusters[i].ClusterName]++
			remainReplicas--
			if i == len(desireReplicaInfos) {
				i = 0
			}
		}
	}

	for _, cluster := range clusters {
		if _, exist := matchClusters[cluster.Name]; !exist {
			desireReplicaInfos[cluster.Name] = 0
		}
	}

	targetClusters := make([]workv1alpha2.TargetCluster, len(desireReplicaInfos))
	i := 0
	for key, value := range desireReplicaInfos {
		targetClusters[i] = workv1alpha2.TargetCluster{Name: key, Replicas: int32(value)}
		i++
	}
	return targetClusters, nil
}

// divideReplicasByPreference assigns a total number of replicas to the selected clusters by preference according to the resource.
func divideReplicasByPreference(clusterAvailableReplicas []workv1alpha2.TargetCluster, replicas int32,
	preference policyv1alpha1.ReplicaDivisionPreference, preUsedClustersName ...string) ([]workv1alpha2.TargetCluster, error) {
	clustersMaxReplicas := util.GetSumOfReplicas(clusterAvailableReplicas)
	if clustersMaxReplicas < replicas {
		return nil, fmt.Errorf("clusters resources are not enough to schedule, max %d replicas are support", clustersMaxReplicas)
	}

	switch preference {
	case policyv1alpha1.ReplicaDivisionPreferenceAggregated:
		return divideReplicasByAggregation(clusterAvailableReplicas, replicas, preUsedClustersName...), nil
	case policyv1alpha1.ReplicaDivisionPreferenceWeighted:
		return divideReplicasByAvailableReplica(clusterAvailableReplicas, replicas, clustersMaxReplicas), nil
	default:
		return nil, fmt.Errorf("undefined replicaSchedulingType： %v", preference)
	}
}

func divideReplicasByAggregation(clusterAvailableReplicas []workv1alpha2.TargetCluster,
	replicas int32, preUsedClustersName ...string) []workv1alpha2.TargetCluster {
	clusterAvailableReplicas = presortClusterList(clusterAvailableReplicas, preUsedClustersName...)
	clustersNum, clustersMaxReplicas := 0, int32(0)
	for _, clusterInfo := range clusterAvailableReplicas {
		clustersNum++
		clustersMaxReplicas += clusterInfo.Replicas
		if clustersMaxReplicas >= replicas {
			break
		}
	}
	var unusedClusters []string
	for i := clustersNum; i < len(clusterAvailableReplicas); i++ {
		unusedClusters = append(unusedClusters, clusterAvailableReplicas[i].Name)
	}
	return divideReplicasByAvailableReplica(clusterAvailableReplicas[0:clustersNum], replicas, clustersMaxReplicas, unusedClusters...)
}

func divideReplicasByAvailableReplica(clusterAvailableReplicas []workv1alpha2.TargetCluster, replicas int32,
	clustersMaxReplicas int32, unusedClusters ...string) []workv1alpha2.TargetCluster {
	desireReplicaInfos := make(map[string]int32)
	allocatedReplicas := int32(0)
	for _, clusterInfo := range clusterAvailableReplicas {
		desireReplicaInfos[clusterInfo.Name] = clusterInfo.Replicas * replicas / clustersMaxReplicas
		allocatedReplicas += desireReplicaInfos[clusterInfo.Name]
	}

	if remainReplicas := replicas - allocatedReplicas; remainReplicas > 0 {
		for i := 0; remainReplicas > 0; i++ {
			desireReplicaInfos[clusterAvailableReplicas[i].Name]++
			remainReplicas--
			if i == len(desireReplicaInfos) {
				i = 0
			}
		}
	}

	// For scaling up
	for _, cluster := range unusedClusters {
		if _, exist := desireReplicaInfos[cluster]; !exist {
			desireReplicaInfos[cluster] = 0
		}
	}

	targetClusters := make([]workv1alpha2.TargetCluster, len(desireReplicaInfos))
	i := 0
	for key, value := range desireReplicaInfos {
		targetClusters[i] = workv1alpha2.TargetCluster{Name: key, Replicas: value}
		i++
	}
	return targetClusters
}

func scaleScheduleByReplicaDivisionPreference(spec *workv1alpha2.ResourceBindingSpec, preference policyv1alpha1.ReplicaDivisionPreference,
	preSelectedClusters []*clusterv1alpha1.Cluster) ([]workv1alpha2.TargetCluster, error) {
	assignedReplicas := util.GetSumOfReplicas(spec.Clusters)
	if assignedReplicas > spec.Replicas {
		newTargetClusters, err := scaleDownScheduleByReplicaDivisionPreference(spec, preference)
		if err != nil {
			return nil, fmt.Errorf("failed to scaleDown: %v", err)
		}
		return newTargetClusters, nil
	} else if assignedReplicas < spec.Replicas {
		newTargetClusters, err := scaleUpScheduleByReplicaDivisionPreference(spec, preSelectedClusters, preference, assignedReplicas)
		if err != nil {
			return nil, fmt.Errorf("failed to scaleUp: %v", err)
		}
		return newTargetClusters, nil
	} else {
		return spec.Clusters, nil
	}
}

func scaleDownScheduleByReplicaDivisionPreference(spec *workv1alpha2.ResourceBindingSpec,
	preference policyv1alpha1.ReplicaDivisionPreference) ([]workv1alpha2.TargetCluster, error) {
	return divideReplicasByPreference(spec.Clusters, spec.Replicas, preference)
}

func scaleUpScheduleByReplicaDivisionPreference(spec *workv1alpha2.ResourceBindingSpec, preSelectedClusters []*clusterv1alpha1.Cluster,
	preference policyv1alpha1.ReplicaDivisionPreference, assignedReplicas int32) ([]workv1alpha2.TargetCluster, error) {
	// Find the clusters that have old replicas, so we can prefer to assign new replicas to them.
	usedTargetClusters := helper.GetUsedBindingClusterNames(spec.Clusters)
	// only the new replicas are considered during this scheduler, the old replicas will not be moved.
	// if not the old replicas may be recreated which is not expected during scaling up
	// use usedTargetClusters to make sure that we assign new replicas to them preferentially
	// so that all the replicas are aggregated
	newObject := spec.DeepCopy()
	newObject.Replicas = spec.Replicas - assignedReplicas
	result, err := divideReplicasByResource(preSelectedClusters, newObject, preference, usedTargetClusters...)
	if err != nil {
		return result, err
	}
	// merge the result of this scheduler for new replicas and the data of old replicas
	return util.MergeTargetClusters(spec.Clusters, result), nil
}
