package helper

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// NewPolicyWithSingleDeployment will build a PropagationPolicy object.
func NewPolicyWithSingleDeployment(namespace, name string, deployment *appsv1.Deployment, clusters []string) *policyv1alpha1.PropagationPolicy {
	return newPolicy(namespace, name, deployment.APIVersion, deployment.Kind, deployment.Name, clusters)
}

// NewPolicyWithSingleService will build a PropagationPolicy object.
func NewPolicyWithSingleService(namespace, name string, service *corev1.Service, clusters []string) *policyv1alpha1.PropagationPolicy {
	return newPolicy(namespace, name, service.APIVersion, service.Kind, service.Name, clusters)
}

// NewPolicyWithSinglePod will build a PropagationPolicy object.
func NewPolicyWithSinglePod(namespace, name string, pod *corev1.Pod, clusters []string) *policyv1alpha1.PropagationPolicy {
	return newPolicy(namespace, name, pod.APIVersion, pod.Kind, pod.Name, clusters)
}

// NewPolicyWithSingleCR will build a PropagationPolicy object.
func NewPolicyWithSingleCR(namespace, name, crAPIVersion, crKind, crName string, clusters []string) *policyv1alpha1.PropagationPolicy {
	return newPolicy(namespace, name, crAPIVersion, crKind, crName, clusters)
}

// NewPolicyWithGroupsDeployment will build a PropagationPolicy object.
func NewPolicyWithGroupsDeployment(namespace, name string, deployment *appsv1.Deployment, maxGroups, minGroups int, clusterLabels map[string]string) *policyv1alpha1.PropagationPolicy {
	return newGroupsConstraintsPolicy(namespace, name, deployment.APIVersion, deployment.Kind, deployment.Name, maxGroups, minGroups, clusterLabels)
}

// NewPolicyWithClusterToleration will build a PropagationPolicy object.
func NewPolicyWithClusterToleration(namespace, name string, deployment *appsv1.Deployment, clusters []string, clusterTolerations []corev1.Toleration) *policyv1alpha1.PropagationPolicy {
	return newPolicyWithClusterToleration(namespace, name, deployment.APIVersion, deployment.Kind, deployment.Name, clusters, clusterTolerations)
}

// NewPolicyWithFieldSelector will build a PropagationPolicy object.
func NewPolicyWithFieldSelector(namespace, name string, deployment *appsv1.Deployment, clusters []string, filedSelector *policyv1alpha1.FieldSelector) *policyv1alpha1.PropagationPolicy {
	return newPolicyWithFieldSelector(namespace, name, deployment.APIVersion, deployment.Kind, deployment.Name, clusters, filedSelector)
}

// newPolicy will build a PropagationPolicy object.
func newPolicy(namespace, policyName, apiVersion, kind, resourceName string, clusters []string) *policyv1alpha1.PropagationPolicy {
	return &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      policyName,
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

// newGroupsConstraintsPolicy will build a PropagationPolicy object with label of cluster and spreadConstraints.
func newGroupsConstraintsPolicy(namespace, policyName, apiVersion, kind, resourceName string, maxGroups, minGroups int, clusterLabels map[string]string) *policyv1alpha1.PropagationPolicy {
	return &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      policyName,
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

// newPolicyWithClusterToleration will build a PropagationPolicy object with cluster toleration.
func newPolicyWithClusterToleration(namespace, policyName, apiVersion, kind, resourceName string, clusters []string, clusterTolerations []corev1.Toleration) *policyv1alpha1.PropagationPolicy {
	return &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      policyName,
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
				ClusterTolerations: clusterTolerations,
			},
		},
	}
}

// newPolicyWithFieldSelector will build a PropagationPolicy object with fieldSelector.
func newPolicyWithFieldSelector(namespace, policyName, apiVersion, kind, resourceName string, clusters []string, filedSelector *policyv1alpha1.FieldSelector) *policyv1alpha1.PropagationPolicy {
	return &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      policyName,
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
					ClusterNames:  clusters,
					FieldSelector: filedSelector,
				},
			},
		},
	}
}
