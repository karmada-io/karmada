package strategy

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

type ReplicaAssigner interface {
	AssignReplica(
		spec *workv1alpha2.ResourceBindingSpec,
		replicaSchedulingStrategy *policyv1alpha1.ReplicaSchedulingStrategy,
		clusters []*clusterv1alpha1.Cluster,
	) ([]workv1alpha2.TargetCluster, error)
}

var strategyFactory = map[string]ReplicaAssigner{}

// RegisterAssignReplicaFunc register a replicaScheduling strategy
func RegisterAssignReplicaFunc(name string, assigner ReplicaAssigner) {
	strategyFactory[name] = assigner
}

// GetAssignReplica return a AssignReplicaInterface object based the ReplicaSchedulingStrategy configuration
func GetAssignReplicas(replicaSchedulingStrategy *policyv1alpha1.ReplicaSchedulingStrategy) (replica ReplicaAssigner, ok bool) {

	var strategyName = ""
	switch replicaSchedulingStrategy.ReplicaSchedulingType {
	case policyv1alpha1.ReplicaSchedulingTypeDuplicated:
		strategyName = string(policyv1alpha1.ReplicaSchedulingTypeDuplicated)
	case policyv1alpha1.ReplicaSchedulingTypeDivided:
		switch replicaSchedulingStrategy.ReplicaDivisionPreference {
		case policyv1alpha1.ReplicaDivisionPreferenceWeighted:
			if replicaSchedulingStrategy.WeightPreference == nil || len(replicaSchedulingStrategy.WeightPreference.DynamicWeight) == 0 {
				strategyName = "StaticWeight"
			} else {
				strategyName = "DynamicWeight"
			}
		case policyv1alpha1.ReplicaDivisionPreferenceAggregated:
			strategyName = string(policyv1alpha1.ReplicaDivisionPreferenceAggregated)
		}
	}

	if assignReplicas, ok := strategyFactory[strategyName]; ok {
		return assignReplicas, true
	}
	return nil, false
}

func init() {
	RegisterAssignReplicaFunc("Duplicated", Duplicated{})
	RegisterAssignReplicaFunc("StaticWeight", StaticWeight{})
	RegisterAssignReplicaFunc("DynamicWeight", DynamicWeight{})
	RegisterAssignReplicaFunc("Aggregated", Aggregated{})
}
