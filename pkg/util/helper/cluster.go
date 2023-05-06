package helper

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
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
