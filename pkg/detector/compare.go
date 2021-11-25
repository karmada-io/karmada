package detector

import policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"

// GetHigherPriorityPropagationPolicy compare two PropagationPolicies with some priority comparison logic
func GetHigherPriorityPropagationPolicy(a, b *policyv1alpha1.PropagationPolicy) *policyv1alpha1.PropagationPolicy {
	if a == nil {
		return b
	}
	// logic of priority comparison
	if a.Name < b.Name {
		return a
	}
	return b
}

// GetHigherPriorityClusterPropagationPolicy compare two ClusterPropagationPolicies with some priority comparison logic
func GetHigherPriorityClusterPropagationPolicy(a, b *policyv1alpha1.ClusterPropagationPolicy) *policyv1alpha1.ClusterPropagationPolicy {
	if a == nil {
		return b
	}
	// logic of priority comparison
	if a.Name < b.Name {
		return a
	}
	return b
}
