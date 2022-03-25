package strategy

import (
	"fmt"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
)

type DynamicWeight struct {
}

func (d DynamicWeight) AssignReplica(
	spec *workv1alpha2.ResourceBindingSpec,
	clusters []*clusterv1alpha1.Cluster,
	replicaSchedulingStrategy *policyv1alpha1.ReplicaSchedulingStrategy,
) ([]workv1alpha2.TargetCluster, error) {
	// Step 1: Find the ready clusters that have old replicas
	scheduledClusters := findOutScheduledCluster(spec.Clusters, clusters)

	// Step 2: calculate the assigned Replicas in scheduledClusters
	assignedReplicas := util.GetSumOfReplicas(scheduledClusters)

	// Step 3: scale replicas in scheduledClusters
	if assignedReplicas != spec.Replicas {
		clusterAvailableReplicas, newSpec, _, _ := scaleReplicas(assignedReplicas, spec, clusters)

		// Validation check
		clustersMaxReplicas := util.GetSumOfReplicas(clusterAvailableReplicas)
		if clustersMaxReplicas < newSpec.Replicas {
			return nil, fmt.Errorf("clusters resources are not enough to schedule, max %d replicas are support", clustersMaxReplicas)
		}

		result, err := divideReplicasByAvailableReplica(clusterAvailableReplicas, newSpec.Replicas, clustersMaxReplicas)
		if err != nil {
			return nil, fmt.Errorf("failed to scale: %v", err)
		}

		// Step 4: merge clusters
		return util.MergeTargetClusters(scheduledClusters, result), nil
	}

	// Return default scheduled clusters
	return scheduledClusters, nil
}
