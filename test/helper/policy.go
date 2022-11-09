package helper

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// NewPropagationPolicy will build a PropagationPolicy object.
func NewPropagationPolicy(ns, name string, rsSelectors []policyv1alpha1.ResourceSelector, placement policyv1alpha1.Placement) *policyv1alpha1.PropagationPolicy {
	return &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: rsSelectors,
			Placement:         placement,
		},
	}
}

// NewClusterPropagationPolicy will build a ClusterPropagationPolicy object.
func NewClusterPropagationPolicy(policyName string, rsSelectors []policyv1alpha1.ResourceSelector, placement policyv1alpha1.Placement) *policyv1alpha1.ClusterPropagationPolicy {
	return &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: rsSelectors,
			Placement:         placement,
		},
	}
}

// NewOverridePolicy will build a OverridePolicy object.
func NewOverridePolicy(namespace, policyName string, rsSelectors []policyv1alpha1.ResourceSelector, clusterAffinity policyv1alpha1.ClusterAffinity, overriders policyv1alpha1.Overriders) *policyv1alpha1.OverridePolicy {
	return &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      policyName,
		},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: rsSelectors,
			TargetCluster:     &clusterAffinity,
			Overriders:        overriders,
		},
	}
}

// NewOverridePolicyByOverrideRules will build a OverridePolicy object by OverrideRules
func NewOverridePolicyByOverrideRules(namespace, policyName string, rsSelectors []policyv1alpha1.ResourceSelector, overrideRules []policyv1alpha1.RuleWithCluster) *policyv1alpha1.OverridePolicy {
	return &policyv1alpha1.OverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      policyName,
		},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: rsSelectors,
			OverrideRules:     overrideRules,
		},
	}
}

// NewClusterOverridePolicyByOverrideRules will build a ClusterOverridePolicy object by OverrideRules
func NewClusterOverridePolicyByOverrideRules(policyName string, rsSelectors []policyv1alpha1.ResourceSelector, overrideRules []policyv1alpha1.RuleWithCluster) *policyv1alpha1.ClusterOverridePolicy {
	return &policyv1alpha1.ClusterOverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: rsSelectors,
			OverrideRules:     overrideRules,
		},
	}
}
