package helper

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// NewOverridePolicyWithDeployment will build a OverridePolicy object that select with Deployment resource.
func NewOverridePolicyWithDeployment(namespace, name string, deployment *appsv1.Deployment, clusters []string, overriders policyv1alpha1.Overriders) *policyv1alpha1.OverridePolicy {
	return newOverridePolicy(namespace, name, deployment.APIVersion, deployment.Kind, deployment.Name, clusters, overriders)
}

// NewOverridePolicyWithPod will build a OverridePolicy object that select with Pod resource.
func NewOverridePolicyWithPod(namespace, name string, pod *corev1.Pod, clusters []string, overriders policyv1alpha1.Overriders) *policyv1alpha1.OverridePolicy {
	return newOverridePolicy(namespace, name, pod.APIVersion, pod.Kind, pod.Name, clusters, overriders)
}

func newOverridePolicy(namespace, policyName, apiVersion, kind, resourceName string, clusters []string, overriders policyv1alpha1.Overriders) *policyv1alpha1.OverridePolicy {
	return &policyv1alpha1.OverridePolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "policy.karmada.io/v1alpha1",
			Kind:       "OverridePolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      policyName,
		},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: apiVersion,
					Kind:       kind,
					Name:       resourceName,
				},
			},
			TargetCluster: &policyv1alpha1.ClusterAffinity{
				ClusterNames: clusters,
			},
			Overriders: overriders,
		},
	}
}

// NewImageOverriderWithEmptyPredicate will build a Overriders object with empty predicate.
func NewImageOverriderWithEmptyPredicate() policyv1alpha1.Overriders {
	return policyv1alpha1.Overriders{
		ImageOverrider: []policyv1alpha1.ImageOverrider{
			{
				Component: "Registry",
				Operator:  "replace",
				Value:     "fictional.registry.us",
			},
			{
				Component: "Repository",
				Operator:  "replace",
				Value:     "busybox",
			},
			{
				Component: "Tag",
				Operator:  "replace",
				Value:     "1.0",
			},
		},
	}
}

// NewImageOverriderWithPredicate will build a Overriders object with predicate.
func NewImageOverriderWithPredicate() policyv1alpha1.Overriders {
	return policyv1alpha1.Overriders{
		ImageOverrider: []policyv1alpha1.ImageOverrider{
			{
				Predicate: &policyv1alpha1.ImagePredicate{
					Path: "/spec/template/spec/containers/0/image",
				},
				Component: "Registry",
				Operator:  "replace",
				Value:     "fictional.registry.us",
			},
		},
	}
}
