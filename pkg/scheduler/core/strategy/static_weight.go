package strategy

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

type StaticWeight struct {
}

func (d StaticWeight) AssignReplica(
	spec *workv1alpha2.ResourceBindingSpec,
	clusters []*clusterv1alpha1.Cluster,
	replicaSchedulingStrategy *policyv1alpha1.ReplicaSchedulingStrategy,
) ([]workv1alpha2.TargetCluster, error) {

	if replicaSchedulingStrategy.WeightPreference == nil {
		replicaSchedulingStrategy.WeightPreference = getDefaultWeightPreference(clusters)
	}

	return divideReplicasByStaticWeight(clusters, replicaSchedulingStrategy.WeightPreference.StaticWeightList, spec.Replicas)
}

// divideReplicasByStaticWeight assigns a total number of replicas to the selected clusters by the weight list.
func divideReplicasByStaticWeight(
	clusters []*clusterv1alpha1.Cluster,
	weightList []policyv1alpha1.StaticClusterWeight,
	replicas int32,
) ([]workv1alpha2.TargetCluster, error) {
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

	clusterWeights := helper.SortClusterByWeight(matchClusters)

	var clusterNames []string
	for _, clusterWeightInfo := range clusterWeights {
		clusterNames = append(clusterNames, clusterWeightInfo.ClusterName)
	}

	divideRemainingReplicas(int(replicas-allocatedReplicas), desireReplicaInfos, clusterNames)

	targetClusters := make([]workv1alpha2.TargetCluster, len(desireReplicaInfos))
	i := 0
	for key, value := range desireReplicaInfos {
		targetClusters[i] = workv1alpha2.TargetCluster{Name: key, Replicas: int32(value)}
		i++
	}
	return targetClusters, nil
}

func getDefaultWeightPreference(clusters []*clusterv1alpha1.Cluster) *policyv1alpha1.ClusterPreferences {
	staticWeightLists := make([]policyv1alpha1.StaticClusterWeight, 0)

	for _, cluster := range clusters {
		staticWeightList := policyv1alpha1.StaticClusterWeight{
			TargetCluster: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
			},
			Weight: 1,
		}
		staticWeightLists = append(staticWeightLists, staticWeightList)
	}

	return &policyv1alpha1.ClusterPreferences{
		StaticWeightList: staticWeightLists,
	}
}
