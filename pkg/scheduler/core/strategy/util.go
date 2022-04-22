package strategy

import (
	"context"
	"fmt"
	"math"
	"sort"

	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	ClusterMember1 = "member1"
	ClusterMember2 = "member2"
	ClusterMember3 = "member3"
	ClusterMember4 = "member4"
)

// TargetClustersList is a slice of TargetCluster that implements sort.Interface to sort by Value.
type TargetClustersList []workv1alpha2.TargetCluster

func (a TargetClustersList) Len() int           { return len(a) }
func (a TargetClustersList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a TargetClustersList) Less(i, j int) bool { return a[i].Replicas > a[j].Replicas }

func divideRemainingReplicas(remainingReplicas int, desiredReplicaInfos map[string]int64, clusterNames []string) {
	if remainingReplicas <= 0 {
		return
	}

	clusterSize := len(clusterNames)
	if remainingReplicas < clusterSize {
		for i := 0; i < remainingReplicas; i++ {
			desiredReplicaInfos[clusterNames[i]]++
		}
	} else {
		avg, residue := remainingReplicas/clusterSize, remainingReplicas%clusterSize
		for i := 0; i < clusterSize; i++ {
			if i < residue {
				desiredReplicaInfos[clusterNames[i]] += int64(avg) + 1
			} else {
				desiredReplicaInfos[clusterNames[i]] += int64(avg)
			}
		}
	}
}

func divideReplicasByAvailableReplica(
	clusterAvailableReplicas []workv1alpha2.TargetCluster,
	replicas int32,
	clustersMaxReplicas int32,
) ([]workv1alpha2.TargetCluster, error) {
	desireReplicaInfos := make(map[string]int64)
	allocatedReplicas := int32(0)
	for _, clusterInfo := range clusterAvailableReplicas {
		desireReplicaInfos[clusterInfo.Name] = int64(clusterInfo.Replicas * replicas / clustersMaxReplicas)
		allocatedReplicas += int32(desireReplicaInfos[clusterInfo.Name])
	}

	var clusterNames []string
	for _, targetCluster := range clusterAvailableReplicas {
		clusterNames = append(clusterNames, targetCluster.Name)
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

func calAvailableReplicas(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster {
	availableTargetClusters := make([]workv1alpha2.TargetCluster, len(clusters))

	// Set the boundary.
	for i := range availableTargetClusters {
		availableTargetClusters[i].Name = clusters[i].Name
		availableTargetClusters[i].Replicas = math.MaxInt32
	}

	// Get the minimum value of MaxAvailableReplicas in terms of all estimators.
	estimators := estimatorclient.GetReplicaEstimators()
	ctx := context.WithValue(context.TODO(), util.ContextKeyObject,
		fmt.Sprintf("kind=%s, name=%s/%s", spec.Resource.Kind, spec.Resource.Namespace, spec.Resource.Name))
	for _, estimator := range estimators {
		res, err := estimator.MaxAvailableReplicas(ctx, clusters, spec.ReplicaRequirements)
		if err != nil {
			klog.Errorf("Max cluster available replicas error: %v", err)
			continue
		}
		for i := range res {
			if res[i].Replicas == estimatorclient.UnauthenticReplica {
				continue
			}
			if availableTargetClusters[i].Name == res[i].Name && availableTargetClusters[i].Replicas > res[i].Replicas {
				availableTargetClusters[i].Replicas = res[i].Replicas
			}
		}
	}

	// In most cases, the target cluster max available replicas should not be MaxInt32 unless the workload is best-effort
	// and the scheduler-estimator has not been enabled. So we set the replicas to spec.Replicas for avoiding overflow.
	for i := range availableTargetClusters {
		if availableTargetClusters[i].Replicas == math.MaxInt32 {
			availableTargetClusters[i].Replicas = spec.Replicas
		}
	}

	sort.Sort(TargetClustersList(availableTargetClusters))
	klog.V(4).Infof("Target cluster: %v", availableTargetClusters)
	return availableTargetClusters
}

// findOutScheduledCluster will return a slice of clusters
// which are a part of `TargetClusters` and have non-zero replicas.
func findOutScheduledCluster(tcs []workv1alpha2.TargetCluster, candidates []*clusterv1alpha1.Cluster) []workv1alpha2.TargetCluster {
	validTarget := make([]workv1alpha2.TargetCluster, 0)
	if len(tcs) == 0 {
		return validTarget
	}

	for _, targetCluster := range tcs {
		// must have non-zero replicas
		if targetCluster.Replicas <= 0 {
			continue
		}
		// must in `candidates`
		for _, cluster := range candidates {
			if targetCluster.Name == cluster.Name {
				validTarget = append(validTarget, targetCluster)
				break
			}
		}
	}

	return validTarget
}

func scaleDownSchedule(
	newCluster []workv1alpha2.TargetCluster,
	replicas int32,
	clustersMaxReplicas int32,
) ([]workv1alpha2.TargetCluster, error) {
	if clustersMaxReplicas < replicas {
		return nil, fmt.Errorf("clusters resources are not enough to schedule, max %d replicas are support", clustersMaxReplicas)
	}
	return divideReplicasByAvailableReplica(newCluster, replicas, clustersMaxReplicas)
}

func scaleUpSchedule(
	newCluster []workv1alpha2.TargetCluster,
	scheduledClusters []workv1alpha2.TargetCluster,
	replicas int32,
	clustersMaxReplicas int32,
) ([]workv1alpha2.TargetCluster, error) {
	if clustersMaxReplicas < replicas {
		return nil, fmt.Errorf("clusters resources are not enough to schedule, max %d replicas are support", clustersMaxReplicas)
	}

	result, err := divideReplicasByAvailableReplica(newCluster, replicas, clustersMaxReplicas)
	if err != nil {
		return result, err
	}

	// Merge clusters and return
	return util.MergeTargetClusters(scheduledClusters, result), nil
}
