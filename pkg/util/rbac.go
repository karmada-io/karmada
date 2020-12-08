package util

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
)

// IsClusterRoleExist tells if specific ClusterRole already exists.
func IsClusterRoleExist(client kubeclient.Interface, name string) (bool, error) {
	_, err := client.RbacV1().ClusterRoles().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// CreateClusterRole just try to create the ClusterRole.
func CreateClusterRole(client kubeclient.Interface, clusterRoleObj *rbacv1.ClusterRole) (*rbacv1.ClusterRole, error) {
	createdObj, err := client.RbacV1().ClusterRoles().Create(context.TODO(), clusterRoleObj, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return clusterRoleObj, nil
		}

		return nil, err
	}

	return createdObj, nil
}

// DeleteClusterRole just try to delete the ClusterRole.
func DeleteClusterRole(client kubeclient.Interface, name string) error {
	err := client.RbacV1().ClusterRoles().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// IsClusterRoleBindingExist tells if specific ClusterRole already exists.
func IsClusterRoleBindingExist(client kubeclient.Interface, name string) (bool, error) {
	_, err := client.RbacV1().ClusterRoleBindings().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// CreateClusterRoleBinding just try to create the ClusterRoleBinding.
func CreateClusterRoleBinding(client kubeclient.Interface, clusterRoleBindingObj *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
	createdObj, err := client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBindingObj, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return clusterRoleBindingObj, nil
		}

		return nil, err
	}

	return createdObj, nil
}

// DeleteClusterRoleBinding just try to delete the ClusterRoleBinding.
func DeleteClusterRoleBinding(client kubeclient.Interface, name string) error {
	err := client.RbacV1().ClusterRoleBindings().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
