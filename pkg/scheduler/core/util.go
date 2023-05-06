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

type calculator func([]*clusterv1alpha1.Cluster, *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster

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

// attachZeroReplicasCluster  attach cluster in clusters into targetCluster
// The purpose is to avoid workload not appeared in rb's spec.clusters field
func attachZeroReplicasCluster(clusters []*clusterv1alpha1.Cluster, targetClusters []workv1alpha2.TargetCluster) []workv1alpha2.TargetCluster {
	targetClusterSet := sets.NewString()
	for i := range targetClusters {
		targetClusterSet.Insert(targetClusters[i].Name)
	}
	for i := range clusters {
		if !targetClusterSet.Has(clusters[i].Name) {
			targetClusters = append(targetClusters, workv1alpha2.TargetCluster{Name: clusters[i].Name, Replicas: 0})
		}
	}
	return targetClusters
}

// removeZeroReplicasCLuster remove the cluster with 0 replicas in assignResults
func removeZeroReplicasCluster(assignResults []workv1alpha2.TargetCluster) []workv1alpha2.TargetCluster {
	targetClusters := make([]workv1alpha2.TargetCluster, 0, len(assignResults))
	for _, cluster := range assignResults {
		if cluster.Replicas > 0 {
			targetClusters = append(targetClusters, workv1alpha2.TargetCluster{Name: cluster.Name, Replicas: cluster.Replicas})
		}
	}
	return targetClusters
}
