package core

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

type StaticWeight struct {
}

func (d StaticWeight) AssignReplica(
	object *workv1alpha2.ResourceBindingSpec,
	replicaSchedulingStrategy *policyv1alpha1.ReplicaSchedulingStrategy,
	clusters []*clusterv1alpha1.Cluster,
) ([]workv1alpha2.TargetCluster, error) {

	if replicaSchedulingStrategy.WeightPreference == nil {
		replicaSchedulingStrategy.WeightPreference = getDefaultWeightPreference(clusters)
	}
	return divideReplicasByStaticWeight(clusters, replicaSchedulingStrategy.WeightPreference.StaticWeightList, object.Replicas)
}
