package helper

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	propagationapi "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// NewPolicyWithSingleCRD will build a ClusterPropagationPolicy object.
func NewPolicyWithSingleCRD(name string, crd *apiextensionsv1.CustomResourceDefinition, clusters []string) *propagationapi.ClusterPropagationPolicy {
	return newClusterPolicy(name, crd.APIVersion, crd.Kind, crd.Name, clusters)
}

// newClusterPolicy will build a ClusterPropagationPolicy object.
func newClusterPolicy(policyName, apiVersion, kind, resourceName string, clusters []string) *propagationapi.ClusterPropagationPolicy {
	return &propagationapi.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: propagationapi.PropagationSpec{
			ResourceSelectors: []propagationapi.ResourceSelector{
				{
					APIVersion: apiVersion,
					Kind:       kind,
					Name:       resourceName,
				},
			},
			Placement: propagationapi.Placement{
				ClusterAffinity: &propagationapi.ClusterAffinity{
					ClusterNames: clusters,
				},
			},
		},
	}
}
