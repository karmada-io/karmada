package util

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

// TargetClustersList is a slice of TargetCluster that implements sort.Interface to sort by Value.
type TargetClustersList []workv1alpha2.TargetCluster

func (a TargetClustersList) Len() int           { return len(a) }
func (a TargetClustersList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a TargetClustersList) Less(i, j int) bool { return a[i].Replicas > a[j].Replicas }

// CalAvailableReplicas get the nax available replicas by estimator or general
func CalAvailableReplicas(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster {
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
