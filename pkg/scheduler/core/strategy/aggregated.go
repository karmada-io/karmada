package strategy

import (
	"fmt"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

type Aggregated struct {
}

func (d Aggregated) AssignReplica(
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
		clusterAvailableReplicas, newSpec, scaleRatio, _ := scaleReplicas(assignedReplicas, spec, clusters)

		scheduledClusterNames := sets.NewString()
		// scaleRatio > 0: scaleUp
		// scaleRatio < 0: scaleDown
		if scaleRatio > 0 {
			scheduledClusterNames = util.ConvertToClusterNames(scheduledClusters)
		}

		result, err := divideReplicasByAggregation(clusterAvailableReplicas, newSpec.Replicas, scheduledClusterNames)
		if err != nil {
			return nil, fmt.Errorf("failed to scale: %v", err)
		}

		// Step 4: merge clusters
		return util.MergeTargetClusters(scheduledClusters, result), nil
	}

	// Return default scheduled clusters
	return scheduledClusters, nil
}

func divideReplicasByAggregation(
	clusterAvailableReplicas []workv1alpha2.TargetCluster,
	replicas int32,
	scheduledClusterNames sets.String,
) ([]workv1alpha2.TargetCluster, error) {
	clusterAvailableReplicas = resortClusterList(clusterAvailableReplicas, scheduledClusterNames)
	clustersNum, clustersMaxReplicas := 0, int32(0)

	for _, clusterInfo := range clusterAvailableReplicas {
		clustersNum++
		clustersMaxReplicas += clusterInfo.Replicas
		if clustersMaxReplicas >= replicas {
			break
		}
	}

	return divideReplicasByAvailableReplica(clusterAvailableReplicas[0:clustersNum], replicas, clustersMaxReplicas)
}

// resortClusterList is used to make sure scheduledClusterNames are in front of the other clusters in the list of
// clusterAvailableReplicas so that we can assign new replicas to them preferentially when scale up.
// Note that scheduledClusterNames have none items during first scheduler
func resortClusterList(
	clusterAvailableReplicas []workv1alpha2.TargetCluster,
	scheduledClusterNames sets.String,
) []workv1alpha2.TargetCluster {
	if scheduledClusterNames.Len() == 0 {
		return clusterAvailableReplicas
	}

	var preUsedCluster []workv1alpha2.TargetCluster
	var unUsedCluster []workv1alpha2.TargetCluster
	for i := range clusterAvailableReplicas {
		if scheduledClusterNames.Has(clusterAvailableReplicas[i].Name) {
			preUsedCluster = append(preUsedCluster, clusterAvailableReplicas[i])
		} else {
			unUsedCluster = append(unUsedCluster, clusterAvailableReplicas[i])
		}
	}
	clusterAvailableReplicas = append(preUsedCluster, unUsedCluster...)
	klog.V(4).Infof("Resorted target cluster: %v", clusterAvailableReplicas)

	return clusterAvailableReplicas
}
