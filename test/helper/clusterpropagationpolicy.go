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
