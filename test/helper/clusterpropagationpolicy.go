package helper

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// NewPolicyWithSingleCRD will build a ClusterPropagationPolicy object.
func NewPolicyWithSingleCRD(name string, crd *apiextensionsv1.CustomResourceDefinition, clusters []string) *policyv1alpha1.ClusterPropagationPolicy {
	return newClusterPolicy(name, crd.APIVersion, crd.Kind, crd.Name, clusters)
}

// NewConstraintsPolicyWithSingleCRD will build a ClusterPropagationPolicy object with specified label and group constraints.
func NewConstraintsPolicyWithSingleCRD(name string, crd *apiextensionsv1.CustomResourceDefinition, maxGroups, minGroups int, clusterLabels map[string]string) *policyv1alpha1.ClusterPropagationPolicy {
	return newConstraintsClusterPolicy(name, crd.APIVersion, crd.Kind, crd.Name, maxGroups, minGroups, clusterLabels)
}

// newClusterPolicy will build a ClusterPropagationPolicy object.
func newClusterPolicy(policyName, apiVersion, kind, resourceName string, clusters []string) *policyv1alpha1.ClusterPropagationPolicy {
	return &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: apiVersion,
					Kind:       kind,
					Name:       resourceName,
				},
			},
			Placement: policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: clusters,
				},
			},
		},
	}
}

// newConstraintsClusterPolicy will build a ClusterPropagationPolicy object with label of cluster and spreadConstraints.
func newConstraintsClusterPolicy(policyName, apiVersion, kind, resourceName string, maxGroups, minGroups int, clusterLabels map[string]string) *policyv1alpha1.ClusterPropagationPolicy {
	return &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: apiVersion,
					Kind:       kind,
					Name:       resourceName,
				},
			},
			Placement: policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: clusterLabels,
					},
				},
				SpreadConstraints: []policyv1alpha1.SpreadConstraint{
					{
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
						MaxGroups:     maxGroups,
						MinGroups:     minGroups,
					},
				},
			},
		},
	}
}
