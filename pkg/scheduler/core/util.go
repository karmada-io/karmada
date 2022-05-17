package core

import (
	"context"
	"fmt"
	"math"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/util"
)

func getDefaultWeightPreference(clusters []*clusterv1alpha1.Cluster) *policyv1alpha1.ClusterPreferences {
	staticWeightLists := make([]policyv1alpha1.StaticClusterWeight, 0)
	for _, cluster := range clusters {
		staticWeightList := policyv1alpha1.StaticClusterWeight{
			TargetCluster: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
			},
			Weight: 1,
		}
		staticWeightLists = append(staticWeightLists, staticWeightList)
	}

	return &policyv1alpha1.ClusterPreferences{
		StaticWeightList: staticWeightLists,
	}
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

// resortClusterList is used to make sure scheduledClusterNames are in front of the other clusters in the list of
// clusterAvailableReplicas so that we can assign new replicas to them preferentially when scale up.
// Note that scheduledClusterNames have none items during first scheduler
func resortClusterList(clusterAvailableReplicas []workv1alpha2.TargetCluster, scheduledClusterNames sets.String) []workv1alpha2.TargetCluster {
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
