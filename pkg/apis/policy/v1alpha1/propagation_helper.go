package v1alpha1

// ExplicitPriority returns the explicit priority declared
// by '.spec.Priority'.
func (p *PropagationSpec) ExplicitPriority() int32 {
	if p.Priority == nil {
		return 0
	}
	return *p.Priority
}

// ExplicitPriority returns the explicit priority of a PropagationPolicy.
func (p *PropagationPolicy) ExplicitPriority() int32 {
	return p.Spec.ExplicitPriority()
}

// ExplicitPriority returns the explicit priority of a ClusterPropagationPolicy.
func (p *ClusterPropagationPolicy) ExplicitPriority() int32 {
	return p.Spec.ExplicitPriority()
}
