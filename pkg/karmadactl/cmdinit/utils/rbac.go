package utils

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterRoleFromRules ClusterRole Rules
func ClusterRoleFromRules(name string, rules []rbacv1.PolicyRule, annotations map[string]string, labels map[string]string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
			Labels:      labels,
		},
		Rules: rules,
	}
}

// ClusterRoleBindingFromSubjects ClusterRoleBinding Subjects
func ClusterRoleBindingFromSubjects(clusterRoleBindingName, clusterRoleName string, sub []rbacv1.Subject, labels map[string]string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleBindingName,
			Labels: labels,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: sub,
	}
}
