package core

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

var (
	assignFuncMap = map[string]func(*assignState) ([]workv1alpha2.TargetCluster, error){
		DuplicatedStrategy:    assignByDuplicatedStrategy,
		AggregatedStrategy:    assignByAggregatedStrategy,
		StaticWeightStrategy:  assignByStaticWeightStrategy,
		DynamicWeightStrategy: assignByDynamicWeightStrategy,
	}
)

// assignState is a wrapper of the input for assigning function.
type assignState struct {
	candidates []*clusterv1alpha1.Cluster
	strategy   *policyv1alpha1.ReplicaSchedulingStrategy
	object     *workv1alpha2.ResourceBindingSpec
}

const (
	// DuplicatedStrategy indicates each candidate member cluster will directly apply the original replicas.
	DuplicatedStrategy = "Duplicated"
	// AggregatedStrategy indicates dividing replicas among clusters as few as possible and
	// taking clusters' available replicas into consideration as well.
	AggregatedStrategy = "Aggregated"
	// StaticWeightStrategy indicates dividing replicas by static weight according to WeightPreference.
	StaticWeightStrategy = "StaticWeight"
	// DynamicWeightStrategy indicates dividing replicas by dynamic weight according to WeightPreference.
	DynamicWeightStrategy = "DynamicWeight"
)

// assignByDuplicatedStrategy assigns replicas by DuplicatedStrategy.
func assignByDuplicatedStrategy(state *assignState) ([]workv1alpha2.TargetCluster, error) {
	targetClusters := make([]workv1alpha2.TargetCluster, len(state.candidates))
	for i, cluster := range state.candidates {
		targetClusters[i] = workv1alpha2.TargetCluster{Name: cluster.Name, Replicas: state.object.Replicas}
	}
	return targetClusters, nil
}

// assignByAggregatedStrategy assigns replicas by AggregatedStrategy.
func assignByAggregatedStrategy(state *assignState) ([]workv1alpha2.TargetCluster, error) {
	return divideReplicasByResource(state.candidates, state.object, policyv1alpha1.ReplicaDivisionPreferenceAggregated)
}

/*
* assignByStaticWeightStrategy assigns a total number of replicas to the selected clusters by the weight list.
* For example, we want to assign replicas to two clusters named A and B.
* | Total | Weight(A:B) | Assignment(A:B) |
* |   9   |   1:2       |     3:6         |
* |   9   |   1:3       |     2:7         | Approximate assignment
* Note:
* 1. If any selected cluster which not present on the weight list will be ignored(different with '0' replica).
* 2. In case of not enough replica for specific cluster which will get '0' replica.
 */
func assignByStaticWeightStrategy(state *assignState) ([]workv1alpha2.TargetCluster, error) {
	// If ReplicaDivisionPreference is set to "Weighted" and WeightPreference is not set,
	// scheduler will weight all clusters averagely.
	if state.strategy.WeightPreference == nil {
		state.strategy.WeightPreference = getDefaultWeightPreference(state.candidates)
	}
	weightList := getStaticWeightInfoList(state.candidates, state.strategy.WeightPreference.StaticWeightList)

	acc := newDispenser(state.object.Replicas, nil)
	acc.takeByWeight(weightList)

	return acc.result, nil
}

// assignByDynamicWeightStrategy assigns replicas by assignByDynamicWeightStrategy.
func assignByDynamicWeightStrategy(state *assignState) ([]workv1alpha2.TargetCluster, error) {
	return divideReplicasByDynamicWeight(state.candidates, state.strategy.WeightPreference.DynamicWeight, state.object)
}
