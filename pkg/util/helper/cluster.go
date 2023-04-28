package helper

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// IsAPIEnabled checks if target API (or CRD) referencing by groupVersion and kind has been installed.
func IsAPIEnabled(APIEnablements []clusterv1alpha1.APIEnablement, groupVersion string, kind string) bool {
	for _, APIEnablement := range APIEnablements {
		if APIEnablement.GroupVersion != groupVersion {
			continue
		}

		for _, resource := range APIEnablement.Resources {
			if resource.Kind != kind {
				continue
			}
			return true
		}
	}

	return false
}

// ClusterInGracefulEvictionTasks checks if the target cluster is in the GracefulEvictionTasks which means it is in the process of eviction.
func ClusterInGracefulEvictionTasks(gracefulEvictionTasks []workv1alpha2.GracefulEvictionTask, clusterName string) bool {
	for _, task := range gracefulEvictionTasks {
		if task.FromCluster == clusterName {
			return true
		}
	}

	return false
}
