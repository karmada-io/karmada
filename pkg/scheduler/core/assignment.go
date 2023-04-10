package core

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

var (
	assignFuncMap = map[string]func(*assignState) ([]workv1alpha2.TargetCluster, error){
		DuplicatedStrategy:    assignByDuplicatedStrategy,
		AggregatedStrategy:    assignByDynamicStrategy,
		StaticWeightStrategy:  assignByStaticWeightStrategy,
		DynamicWeightStrategy: assignByDynamicStrategy,
	}
)

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

// assignState is a wrapper of the input for assigning function.
type assignState struct {
	candidates []*clusterv1alpha1.Cluster
	strategy   *policyv1alpha1.ReplicaSchedulingStrategy
	spec       *workv1alpha2.ResourceBindingSpec

	// fields below are indirect results
	strategyType string

	scheduledClusters []workv1alpha2.TargetCluster
	assignedReplicas  int32
	availableClusters []workv1alpha2.TargetCluster
	availableReplicas int32

	// targetReplicas is the replicas that we need to schedule in this round
	targetReplicas int32
}

func newAssignState(candidates []*clusterv1alpha1.Cluster, placement *policyv1alpha1.Placement, obj *workv1alpha2.ResourceBindingSpec) *assignState {
	var strategyType string

	switch placement.ReplicaSchedulingType() {
	case policyv1alpha1.ReplicaSchedulingTypeDuplicated:
		strategyType = DuplicatedStrategy
	case policyv1alpha1.ReplicaSchedulingTypeDivided:
		switch placement.ReplicaScheduling.ReplicaDivisionPreference {
		case policyv1alpha1.ReplicaDivisionPreferenceAggregated:
			strategyType = AggregatedStrategy
		case policyv1alpha1.ReplicaDivisionPreferenceWeighted:
			if placement.ReplicaScheduling.WeightPreference != nil && len(placement.ReplicaScheduling.WeightPreference.DynamicWeight) != 0 {
				strategyType = DynamicWeightStrategy
			} else {
				strategyType = StaticWeightStrategy
			}
		}
	}

	return &assignState{candidates: candidates, strategy: placement.ReplicaScheduling, spec: obj, strategyType: strategyType}
}

func (as *assignState) buildScheduledClusters() {
	as.scheduledClusters = as.spec.Clusters
	as.assignedReplicas = util.GetSumOfReplicas(as.scheduledClusters)
}

func (as *assignState) buildAvailableClusters(c calculator) {
	as.availableClusters = c(as.candidates, as.spec)
	as.availableReplicas = util.GetSumOfReplicas(as.availableClusters)
}

// resortAvailableClusters is used to make sure scheduledClusters are at the front of availableClusters
// list so that we can assign new replicas to them preferentially when scale up.
func (as *assignState) resortAvailableClusters() []workv1alpha2.TargetCluster {
	// get the previous scheduled clusters
	prior := sets.NewString()
	for _, cluster := range as.scheduledClusters {
		if cluster.Replicas > 0 {
			prior.Insert(cluster.Name)
		}
	}

	if len(prior) == 0 {
		return as.availableClusters
	}

	var (
		prev = make([]workv1alpha2.TargetCluster, 0, len(prior))
		left = make([]workv1alpha2.TargetCluster, 0, len(as.scheduledClusters)-len(prior))
	)

	for _, cluster := range as.availableClusters {
		if prior.Has(cluster.Name) {
			prev = append(prev, cluster)
		} else {
			left = append(left, cluster)
		}
	}
	as.availableClusters = append(prev, left...)
	return as.availableClusters
}

// assignByDuplicatedStrategy assigns replicas by DuplicatedStrategy.
func assignByDuplicatedStrategy(state *assignState) ([]workv1alpha2.TargetCluster, error) {
	targetClusters := make([]workv1alpha2.TargetCluster, len(state.candidates))
	for i, cluster := range state.candidates {
		targetClusters[i] = workv1alpha2.TargetCluster{Name: cluster.Name, Replicas: state.spec.Replicas}
	}
	return targetClusters, nil
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

	disp := helper.NewDispenser(state.spec.Replicas, nil)
	disp.TakeByWeight(weightList)

	return disp.Result, nil
}

func assignByDynamicStrategy(state *assignState) ([]workv1alpha2.TargetCluster, error) {
	state.buildScheduledClusters()
	if state.assignedReplicas > state.spec.Replicas {
		// We need to reduce the replicas in terms of the previous result.
		result, err := dynamicScaleDown(state)
		if err != nil {
			return nil, fmt.Errorf("failed to scale down: %v", err)
		}
		return result, nil
	} else if state.assignedReplicas < state.spec.Replicas {
		// We need to enlarge the replicas in terms of the previous result (if exists).
		// First scheduling is considered as a special kind of scaling up.
		result, err := dynamicScaleUp(state)
		if err != nil {
			return nil, fmt.Errorf("failed to scale up: %v", err)
		}
		return result, nil
	} else {
		return state.scheduledClusters, nil
	}
}
