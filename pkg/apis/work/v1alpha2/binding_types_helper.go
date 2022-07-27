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

// RemoveCluster removes specific cluster from the target list.
// This function no-opts if cluster not exist.
func (s *ResourceBindingSpec) RemoveCluster(name string) {
	var i int

	for i = 0; i < len(s.Clusters); i++ {
		if s.Clusters[i].Name == name {
			break
		}
	}

	// not found, do nothing
	if i >= len(s.Clusters) {
		return
	}

	s.Clusters = append(s.Clusters[:i], s.Clusters[i+1:]...)
}
