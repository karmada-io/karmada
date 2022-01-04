package v1alpha1

// GetName returns the name of OverridePolicy
func (p *OverridePolicy) GetName() string {
	return p.Name
}

// GetNamespace returns the namespace of OverridePolicy
func (p *OverridePolicy) GetNamespace() string {
	return p.Namespace
}

// GetOverrideSpec returns the OverrideSpec of OverridePolicy
func (p *OverridePolicy) GetOverrideSpec() OverrideSpec {
	return p.Spec
}

// GetName returns the name of ClusterOverridePolicy
func (p *ClusterOverridePolicy) GetName() string {
	return p.Name
}

// GetNamespace returns the namespace of ClusterOverridePolicy
func (p *ClusterOverridePolicy) GetNamespace() string {
	return p.Namespace
}

// GetOverrideSpec returns the OverrideSpec of ClusterOverridePolicy
func (p *ClusterOverridePolicy) GetOverrideSpec() OverrideSpec {
	return p.Spec
}
