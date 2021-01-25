package helper

import (
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	propagationapi "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// NewPolicyWithSingleDeployment will build a PropagationPolicy object.
func NewPolicyWithSingleDeployment(namespace string, name string, deployment *appsv1.Deployment, clusters []string) *propagationapi.PropagationPolicy {
	return &propagationapi.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: propagationapi.PropagationSpec{
			ResourceSelectors: []propagationapi.ResourceSelector{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
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
