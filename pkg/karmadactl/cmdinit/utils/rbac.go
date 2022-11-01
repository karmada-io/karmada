package utils

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// ClusterRoleFromRules ClusterRole Rules
func ClusterRoleFromRules(name string, rules []rbacv1.PolicyRule, labels map[string]string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
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

// CreateIfNotExistClusterRole  create ClusterRole when it doesn't exist
func CreateIfNotExistClusterRole(clientSet kubernetes.Interface, role *rbacv1.ClusterRole) error {
	clusterRoleClient := clientSet.RbacV1().ClusterRoles()
	_, err := clusterRoleClient.Get(context.TODO(), role.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = clusterRoleClient.Create(context.TODO(), role, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("create ClusterRole %s failed: %v", role.Name, err)
			}

			klog.Infof("CreateClusterRole %s success.", role.Name)
			return nil
		}
		return fmt.Errorf("get ClusterRole %s failed: %v", role.Name, err)
	}
	klog.Infof("CreateClusterRole %s already exists.", role.Name)

	return nil
}

// CreateIfNotExistClusterRoleBinding create ClusterRoleBinding when it doesn't exist
func CreateIfNotExistClusterRoleBinding(clientSet kubernetes.Interface, binding *rbacv1.ClusterRoleBinding) error {
	crbClient := clientSet.RbacV1().ClusterRoleBindings()
	_, err := crbClient.Get(context.TODO(), binding.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = crbClient.Create(context.TODO(), binding, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("create ClusterRoleBinding %s failed: %v", binding.Name, err)
			}

			klog.Infof("CreateClusterRoleBinding %s success.", binding.Name)
			return nil
		}
		return fmt.Errorf("get ClusterRoleBinding %s failed: %v", binding.Name, err)
	}
	klog.Infof("CreateClusterRoleBinding %s already exists.", binding.Name)

	return nil
}
