package util

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// EnsureClusterRoleExist makes sure that the specific cluster role exist in cluster.
// If cluster role not exit, just create it.
func EnsureClusterRoleExist(client kubeclient.Interface, clusterRole *rbacv1.ClusterRole, dryRun bool) (*rbacv1.ClusterRole, error) {
	if dryRun {
		return clusterRole, nil
	}

	exist, err := IsClusterRoleExist(client, clusterRole.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check if ClusterRole exist. ClusterRole: %s, error: %v", clusterRole.Name, err)
	}
	if exist {
		klog.V(1).Infof("ensure ClusterRole succeed as already exist. ClusterRole: %s", clusterRole.Name)
		return clusterRole, nil
	}

	createdObj, err := CreateClusterRole(client, clusterRole)
	if err != nil {
		return nil, fmt.Errorf("ensure ClusterRole failed due to create failed. ClusterRole: %s, error: %v", clusterRole.Name, err)
	}

	return createdObj, nil
}

// EnsureClusterRoleBindingExist makes sure that the specific ClusterRoleBinding exist in cluster.
// If ClusterRoleBinding not exit, just create it.
func EnsureClusterRoleBindingExist(client kubeclient.Interface, clusterRoleBinding *rbacv1.ClusterRoleBinding, dryRun bool) (*rbacv1.ClusterRoleBinding, error) {
	if dryRun {
		return clusterRoleBinding, nil
	}

	exist, err := IsClusterRoleBindingExist(client, clusterRoleBinding.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check if ClusterRole exist. ClusterRole: %s, error: %v", clusterRoleBinding.Name, err)
	}
	if exist {
		klog.V(1).Infof("ensure ClusterRole succeed as already exist. ClusterRole: %s", clusterRoleBinding.Name)
		return clusterRoleBinding, nil
	}

	createdObj, err := CreateClusterRoleBinding(client, clusterRoleBinding)
	if err != nil {
		return nil, fmt.Errorf("ensure ClusterRole failed due to create failed. ClusterRole: %s, error: %v", clusterRoleBinding.Name, err)
	}

	return createdObj, nil
}

// BuildRoleBindingSubjects will generate a subject as per service account.
// The subject used by RoleBinding or ClusterRoleBinding.
func BuildRoleBindingSubjects(serviceAccountName, serviceAccountNamespace string) []rbacv1.Subject {
	return []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName,
			Namespace: serviceAccountNamespace,
		},
	}
}

// BuildClusterRoleReference will generate a ClusterRole reference.
func BuildClusterRoleReference(roleName string) rbacv1.RoleRef {
	return rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     roleName,
	}
}
