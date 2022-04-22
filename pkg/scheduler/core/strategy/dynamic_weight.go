package strategy

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
)

type DynamicWeight struct {
}

func (d DynamicWeight) AssignReplica(
	spec *workv1alpha2.ResourceBindingSpec,
	replicaSchedulingStrategy *policyv1alpha1.ReplicaSchedulingStrategy,
	clusters []*clusterv1alpha1.Cluster,
) ([]workv1alpha2.TargetCluster, error) {
	// Step 1: Find the ready clusters that have old replicas
	scheduledClusters := findOutScheduledCluster(spec.Clusters, clusters)

	// Step 2: calculate the assigned Replicas in scheduledClusters
	assignedReplicas := util.GetSumOfReplicas(scheduledClusters)

	// Step 3: scale replicas in scheduled clusters
	// Case1: assignedReplicas < spec.Replicas -> scale up
	// Case2: assignedReplicas > spec.Replicas -> scale down
	// Case3: assignedReplicas > spec.Replicas -> No scale operation
	newSpec := spec
	newCluster := spec.Clusters

	if assignedReplicas < spec.Replicas {
		// Create a new spec
		if assignedReplicas > 0 {
			newSpec = spec.DeepCopy()
			newSpec.Replicas = spec.Replicas - assignedReplicas
		}
		// Calculate available replicas of all candidates
		newCluster = calAvailableReplicas(clusters, newSpec)
		clustersMaxReplicas := util.GetSumOfReplicas(newCluster)
		return scaleUpSchedule(newCluster, scheduledClusters, newSpec.Replicas, clustersMaxReplicas)

	} else if assignedReplicas > spec.Replicas {
		clustersMaxReplicas := util.GetSumOfReplicas(newCluster)
		return scaleDownSchedule(newCluster, newSpec.Replicas, clustersMaxReplicas)
	}

	// Return default scheduled clusters
	return scheduledClusters, nil
}
