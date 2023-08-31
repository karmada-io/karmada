package v1alpha1

// ContainsCrossClusterExposureType determines whether ExposureTypeCrossCluster exists.
func (spec *MultiClusterServiceSpec) ContainsCrossClusterExposureType() bool {
	for _, t := range spec.Types {
		if t == ExposureTypeCrossCluster {
			return true
		}
	}
	return false
}

// ContainsLoadBalancerExposureType determines whether ExposureTypeLoadBalancer exists.
func (spec *MultiClusterServiceSpec) ContainsLoadBalancerExposureType() bool {
	for _, t := range spec.Types {
		if t == ExposureTypeLoadBalancer {
			return true
		}
	}
	return false
}
