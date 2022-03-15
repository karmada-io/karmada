package helper

import (
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// IsScheduleResultEqual will check whether two schedule results are equal.
func IsScheduleResultEqual(tc1, tc2 []workv1alpha2.TargetCluster) bool {
	if len(tc1) != len(tc2) {
		return false
	}
	for _, c1 := range tc1 {
		found := false
		for _, c2 := range tc2 {
			if c1 == c2 {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// IsExclude indicate if the target clusters exclude the srcCluster
func IsExclude(srcCluster string, targetClusters []string) bool {
	for _, cluster := range targetClusters {
		if cluster == srcCluster {
			return false
		}
	}
	return true
}
