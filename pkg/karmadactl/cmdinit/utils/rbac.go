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

// CreateOrUpdateRole creates a Role if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateRole(clientSet kubernetes.Interface, role *rbacv1.Role) error {
	if _, err := clientSet.RbacV1().Roles(role.ObjectMeta.Namespace).Create(context.TODO(), role, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create RBAC role: %v", err)
		}

		if _, err := clientSet.RbacV1().Roles(role.ObjectMeta.Namespace).Update(context.TODO(), role, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update RBAC role: %v", err)
		}
	}
	klog.Infof("Role %s%s has been created or updated.", role.ObjectMeta.Namespace, role.ObjectMeta.Name)

	return nil
}

// CreateOrUpdateRoleBinding creates a RoleBinding if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateRoleBinding(clientSet kubernetes.Interface, roleBinding *rbacv1.RoleBinding) error {
	if _, err := clientSet.RbacV1().RoleBindings(roleBinding.ObjectMeta.Namespace).Create(context.TODO(), roleBinding, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create RBAC rolebinding: %v", err)
		}

		if _, err := clientSet.RbacV1().RoleBindings(roleBinding.ObjectMeta.Namespace).Update(context.TODO(), roleBinding, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update RBAC rolebinding: %v", err)
		}
	}
	klog.Infof("RoleBinding %s/%s has been created or updated.", roleBinding.ObjectMeta.Namespace, roleBinding.ObjectMeta.Name)

	return nil
}
