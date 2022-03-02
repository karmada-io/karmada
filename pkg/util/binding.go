package util

import (
	"k8s.io/apimachinery/pkg/util/sets"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// GetBindingClusterNames will get clusterName list from bind clusters field
func GetBindingClusterNames(binding *workv1alpha2.ResourceBinding) []string {
	var clusterNames []string
	for _, targetCluster := range binding.Spec.Clusters {
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
		replicasSum := int32(0)
		for _, targetCluster := range bindingSpec.Clusters {
			replicasSum += targetCluster.Replicas
		}
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

// ConvertToClusterNames will convert a cluster slice to clusterName's sets.String
func ConvertToClusterNames(clusters []workv1alpha2.TargetCluster) sets.String {
	clusterNames := sets.NewString()
	for _, cluster := range clusters {
		clusterNames.Insert(cluster.Name)
	}

	return clusterNames
}

// DivideReplicasByTargetCluster will divide the sum number by the weight of target clusters.
func DivideReplicasByTargetCluster(clusters []workv1alpha2.TargetCluster, sum int32) []workv1alpha2.TargetCluster {
	res := make([]workv1alpha2.TargetCluster, len(clusters))
	if len(clusters) == 0 {
		return res
	}
	sumWeight := int32(0)
	allocatedReplicas := int32(0)
	for i := range clusters {
		sumWeight += clusters[i].Replicas
	}
	for i := range clusters {
		res[i].Name = clusters[i].Name
		if sumWeight > 0 {
			res[i].Replicas = clusters[i].Replicas * sum / sumWeight
		}
		allocatedReplicas += res[i].Replicas
	}
	if remainReplicas := sum - allocatedReplicas; remainReplicas > 0 {
		for i := 0; remainReplicas > 0; i++ {
			if i == len(res) {
				i = 0
			}
			res[i].Replicas++
			remainReplicas--
		}
	}
	return res
}

// MergeTargetClusters will merge the replicas in two TargetCluster
func MergeTargetClusters(old, new []workv1alpha2.TargetCluster) []workv1alpha2.TargetCluster {
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
