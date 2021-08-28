package util

import (
	"k8s.io/apimachinery/pkg/util/sets"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

// IsBindingReplicasChanged will check if the sum of replicas is different from the replicas of object
func IsBindingReplicasChanged(bindingSpec *workv1alpha1.ResourceBindingSpec, strategy *policyv1alpha1.ReplicaSchedulingStrategy) bool {
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
func GetSumOfReplicas(clusters []workv1alpha1.TargetCluster) int32 {
	replicasSum := int32(0)
	for i := range clusters {
		replicasSum += clusters[i].Replicas
	}
	return replicasSum
}

// ConvertToClusterNames will convert a cluster slice to clusterName's sets.String
func ConvertToClusterNames(clusters []workv1alpha1.TargetCluster) sets.String {
	clusterNames := sets.NewString()
	for _, cluster := range clusters {
		clusterNames.Insert(cluster.Name)
	}

	return clusterNames
}
