package util

import (
	"k8s.io/apimachinery/pkg/util/sets"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// GetBindingClusterNames will get clusterName list from bind clusters field
func GetBindingClusterNames(spec *workv1alpha2.ResourceBindingSpec) []string {
	var clusterNames []string
	for _, targetCluster := range spec.Clusters {
		clusterNames = append(clusterNames, targetCluster.Name)
	}
	return clusterNames
}

// IsBindingReplicasChanged will check if the sum of replicas is different from the replicas of object
func IsBindingReplicasChanged(bindingSpec *workv1alpha2.ResourceBindingSpec, strategy *policyv1alpha1.ReplicaSchedulingStrategy) bool {
	if strategy == nil {
		return false
	}
	if strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		for _, targetCluster := range bindingSpec.Clusters {
			if targetCluster.Replicas != bindingSpec.Replicas {
				return true
			}
		}
		return false
	}
	if strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided {
		replicasSum := GetSumOfReplicas(bindingSpec.Clusters)
		return replicasSum != bindingSpec.Replicas
	}
	return false
}

// GetSumOfReplicas will get the sum of replicas in target clusters
func GetSumOfReplicas(clusters []workv1alpha2.TargetCluster) int32 {
	replicasSum := int32(0)
	for i := range clusters {
		replicasSum += clusters[i].Replicas
	}
	return replicasSum
}

// ConvertFromTargetClustersToStringSet will convert a cluster slice to clusterName's sets.String
func ConvertFromTargetClustersToStringSet(clusters []workv1alpha2.TargetCluster) sets.Set[string] {
	clusterNames := sets.New[string]()
	for _, cluster := range clusters {
		clusterNames.Insert(cluster.Name)
	}

	return clusterNames
}

// ConvertFromStringSetToTargetClusters will convert clusterName's sets.String to a cluster slice
func ConvertFromStringSetToTargetClusters(clusterSet sets.Set[string]) []workv1alpha2.TargetCluster {
	var clusters []workv1alpha2.TargetCluster
	for cluster := range clusterSet {
		clusters = append(clusters, workv1alpha2.TargetCluster{Name: cluster})
	}
	return clusters
}

// MergeTargetClusters will merge the replicas in two TargetCluster
func MergeTargetClusters(old, new []workv1alpha2.TargetCluster) []workv1alpha2.TargetCluster {
	switch {
	case len(old) == 0:
		return new
	case len(new) == 0:
		return old
	}
	// oldMap is a map of the result for the old replicas so that it can be merged with the new result easily
	oldMap := make(map[string]int32)
	for _, cluster := range old {
		oldMap[cluster.Name] = cluster.Replicas
	}
	// merge the new replicas and the data of old replicas
	for i, cluster := range new {
		value, ok := oldMap[cluster.Name]
		if ok {
			new[i].Replicas = cluster.Replicas + value
			delete(oldMap, cluster.Name)
		}
	}
	for key, value := range oldMap {
		new = append(new, workv1alpha2.TargetCluster{Name: key, Replicas: value})
	}
	return new
}

// ClustersToBePropagated will return all TargetClusters owned by ResourceBindingSpec
func ClustersToBePropagated(rbSpec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster {
	targetClusters := make([]workv1alpha2.TargetCluster, len(rbSpec.Clusters))
	copy(targetClusters, rbSpec.Clusters)

	if len(rbSpec.RequiredBy) == 0 {
		return targetClusters
	}

	scheduledClusterNames := ConvertFromTargetClustersToStringSet(rbSpec.Clusters)
	for _, requiredByBinding := range rbSpec.RequiredBy {
		for _, targetCluster := range requiredByBinding.Clusters {
			if !scheduledClusterNames.Has(targetCluster.Name) {
				scheduledClusterNames.Insert(targetCluster.Name)
				targetClusters = append(targetClusters, targetCluster)
			}
		}
	}
	return targetClusters
}

// ClusterSetToBePropagated will return all TargetClusters string set owned by ResourceBindingSpec
func ClusterSetToBePropagated(rbSpec *workv1alpha2.ResourceBindingSpec) sets.Set[string] {
	return ConvertFromTargetClustersToStringSet(ClustersToBePropagated(rbSpec))
}
