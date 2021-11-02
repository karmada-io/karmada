package core

import (
	"context"
	"fmt"
	"math"
	"sort"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/scheduler/cache"
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

	sort.Sort(TargetClustersList(availableTargetClusters))
	klog.V(4).Infof("Target cluster: %v", availableTargetClusters)
	return availableTargetClusters
}

func getPreSelected(targetClusters []workv1alpha2.TargetCluster, schedulerCache cache.Cache) []*clusterv1alpha1.Cluster {
	var preSelectedClusters []*clusterv1alpha1.Cluster
	clusterInfoSnapshot := schedulerCache.Snapshot()
	for _, targetCluster := range targetClusters {
		for _, cluster := range clusterInfoSnapshot.GetClusters() {
			if targetCluster.Name == cluster.Cluster().Name {
				preSelectedClusters = append(preSelectedClusters, cluster.Cluster())
				break
			}
		}
	}
	return preSelectedClusters
}

// presortClusterList is used to make sure preUsedClusterNames are in front of the other clusters in the list of
// clusterAvailableReplicas so that we can assign new replicas to them preferentially when scale up.
// Note that preUsedClusterNames have none items during first scheduler
func presortClusterList(clusterAvailableReplicas []workv1alpha2.TargetCluster, preUsedClusterNames ...string) []workv1alpha2.TargetCluster {
	if len(preUsedClusterNames) == 0 {
		return clusterAvailableReplicas
	}
	preUsedClusterSet := sets.NewString(preUsedClusterNames...)
	var preUsedCluster []workv1alpha2.TargetCluster
	var unUsedCluster []workv1alpha2.TargetCluster
	for i := range clusterAvailableReplicas {
		if preUsedClusterSet.Has(clusterAvailableReplicas[i].Name) {
			preUsedCluster = append(preUsedCluster, clusterAvailableReplicas[i])
		} else {
			unUsedCluster = append(unUsedCluster, clusterAvailableReplicas[i])
		}
	}
	clusterAvailableReplicas = append(preUsedCluster, unUsedCluster...)
	klog.V(4).Infof("resorted target cluster: %v", clusterAvailableReplicas)
	return clusterAvailableReplicas
}

// calcReservedCluster eliminates the not-ready clusters from the 'bindClusters'.
func calcReservedCluster(bindClusters, readyClusters sets.String) sets.String {
	return bindClusters.Difference(bindClusters.Difference(readyClusters))
}

// calcAvailableCluster returns a list of ready clusters that not in 'bindClusters'.
func calcAvailableCluster(bindCluster, readyClusters sets.String) sets.String {
	return readyClusters.Difference(bindCluster)
}
