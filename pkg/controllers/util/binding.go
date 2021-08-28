package util

import workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"

// GetBindingClusterNames will get clusterName list from bind clusters field
func GetBindingClusterNames(binding *workv1alpha1.ResourceBinding) []string {
	var clusterNames []string
	for _, targetCluster := range binding.Spec.Clusters {
		clusterNames = append(clusterNames, targetCluster.Name)
	}
	return clusterNames
}
