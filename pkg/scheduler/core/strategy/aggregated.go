package strategy

import (
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
	scheduledClusterNames := sets.NewString()
	if assignedReplicas < spec.Replicas {
		// Create a new spec
		if assignedReplicas > 0 {
			newSpec = spec.DeepCopy()
			newSpec.Replicas = spec.Replicas - assignedReplicas
		}
		// Calculate available replicas of all candidates
		newCluster = calAvailableReplicas(clusters, newSpec)
		// Get the names of ready clusters
		scheduledClusterNames = util.ConvertToClusterNames(scheduledClusters)

		clusterAvailableReplicas, clustersMaxReplicas := calculateReplicas(newCluster, newSpec.Replicas, scheduledClusterNames)
		return scaleUpSchedule(clusterAvailableReplicas, scheduledClusters, newSpec.Replicas, clustersMaxReplicas)

	} else if assignedReplicas > spec.Replicas {
		clusterAvailableReplicas, clustersMaxReplicas := calculateReplicas(newCluster, newSpec.Replicas, scheduledClusterNames)
		return scaleDownSchedule(clusterAvailableReplicas, newSpec.Replicas, clustersMaxReplicas)
	}

	// Return default scheduled clusters
	return scheduledClusters, nil
}

func calculateReplicas(
	clusterAvailableReplicas []workv1alpha2.TargetCluster,
	replicas int32,
	scheduledClusterNames sets.String,
) ([]workv1alpha2.TargetCluster, int32) {
	clusterAvailableReplicas = resortClusterList(clusterAvailableReplicas, scheduledClusterNames)
	clustersNum, clustersMaxReplicas := 0, int32(0)

	for _, clusterInfo := range clusterAvailableReplicas {
		clustersNum++
		clustersMaxReplicas += clusterInfo.Replicas
		if clustersMaxReplicas >= replicas {
			break
		}
	}

	//return divideReplicasByAvailableReplica(clusterAvailableReplicas[0:clustersNum], replicas, clustersMaxReplicas)
	return clusterAvailableReplicas[0:clustersNum], clustersMaxReplicas
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
