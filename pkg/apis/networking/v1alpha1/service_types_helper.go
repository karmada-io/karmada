package v1alpha1

// HasExposureTypeCrossCluster determines whether ExposureTypeCrossCluster exists.
func (m *MultiClusterServiceSpec) HasExposureTypeCrossCluster() bool {
	for _, t := range m.Types {
		if t == ExposureTypeCrossCluster {
			return true
		}
	}
	return false
}

// HasExposureTypeLoadBalancer determines whether ExposureTypeLoadBalancer exists.
func (m *MultiClusterServiceSpec) HasExposureTypeLoadBalancer() bool {
	for _, t := range m.Types {
		if t == ExposureTypeLoadBalancer {
			return true
		}
	}
	return false
}
