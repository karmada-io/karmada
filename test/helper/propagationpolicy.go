package helper

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	propagationapi "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// NewPolicyWithSingleDeployment will build a PropagationPolicy object.
func NewPolicyWithSingleDeployment(namespace, name string, deployment *appsv1.Deployment, clusters []string) *propagationapi.PropagationPolicy {
	return newPolicy(namespace, name, deployment.APIVersion, deployment.Kind, deployment.Name, clusters)
}

// NewPolicyWithSingleService will build a PropagationPolicy object.
func NewPolicyWithSingleService(namespace, name string, service *corev1.Service, clusters []string) *propagationapi.PropagationPolicy {
	return newPolicy(namespace, name, service.APIVersion, service.Kind, service.Name, clusters)
}

// NewPolicyWithSinglePod will build a PropagationPolicy object.
func NewPolicyWithSinglePod(namespace, name string, pod *corev1.Pod, clusters []string) *propagationapi.PropagationPolicy {
	return newPolicy(namespace, name, pod.APIVersion, pod.Kind, pod.Name, clusters)
}

// newPolicy will build a PropagationPolicy object.
func newPolicy(namespace, policyName, apiVersion, kind, resourceName string, clusters []string) *propagationapi.PropagationPolicy {
	return &propagationapi.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      policyName,
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
