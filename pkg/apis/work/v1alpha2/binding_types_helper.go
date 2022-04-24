package v1alpha2

// TargetContains checks if specific cluster present on the target list.
func (s *ResourceBindingSpec) TargetContains(name string) bool {
	for i := range s.Clusters {
		if s.Clusters[i].Name == name {
			return true
		}
	}

	return false
}

// AssignedReplicasForCluster returns assigned replicas for specific cluster.
func (s *ResourceBindingSpec) AssignedReplicasForCluster(targetCluster string) int32 {
	for i := range s.Clusters {
		if s.Clusters[i].Name == targetCluster {
			return s.Clusters[i].Replicas
		}
	}

	return 0
}
