package helper

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

// NewExplicitPriorityPropagationPolicy will build a PropagationPolicy object with explicit priority.
func NewExplicitPriorityPropagationPolicy(ns, name string, rsSelectors []policyv1alpha1.ResourceSelector,
	placement policyv1alpha1.Placement, priority int32) *policyv1alpha1.PropagationPolicy {
	return &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: rsSelectors,
			Priority:          &priority,
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

// NewExplicitPriorityClusterPropagationPolicy will build a ClusterPropagationPolicy object with explicit priority.
func NewExplicitPriorityClusterPropagationPolicy(policyName string, rsSelectors []policyv1alpha1.ResourceSelector,
	placement policyv1alpha1.Placement, priority int32) *policyv1alpha1.ClusterPropagationPolicy {
	return &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: rsSelectors,
			Priority:          &priority,
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

// NewFederatedResourceQuota will build a demo FederatedResourceQuota object.
func NewFederatedResourceQuota(ns, name string) *policyv1alpha1.FederatedResourceQuota {
	return &policyv1alpha1.FederatedResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: policyv1alpha1.FederatedResourceQuotaSpec{
			Overall: map[corev1.ResourceName]resource.Quantity{
				"cpu":    resource.MustParse("8"),
				"memory": resource.MustParse("16Gi"),
			},
			StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
				{
					ClusterName: "member1",
					Hard: map[corev1.ResourceName]resource.Quantity{
						"cpu":    resource.MustParse("1"),
						"memory": resource.MustParse("2Gi"),
					},
				},
				{
					ClusterName: "member2",
					Hard: map[corev1.ResourceName]resource.Quantity{
						"cpu":    resource.MustParse("1"),
						"memory": resource.MustParse("2Gi"),
					},
				},
			},
		},
	}
}
