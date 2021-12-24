package kubernetes

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

//ClusterRoleFromSpec ClusterRole spec
func (i *CommandInitOption) ClusterRoleFromSpec(name string, rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: i.Namespace,
			Labels:    map[string]string{"karmada.io/bootstrapping": "rbac-defaults"},
		},
		Rules: rules,
	}
}

//ClusterRoleBindingFromSpec ClusterRoleBinding spec
func (i *CommandInitOption) ClusterRoleBindingFromSpec(clusterRoleBindingName, clusterRoleName, saName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRoleBindingName,
			Namespace: i.Namespace,
			Labels:    map[string]string{"karmada.io/bootstrapping": "rbac-defaults"},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: i.Namespace,
			},
		},
	}
}

//CreateClusterRole receive ClusterRoleFromSpec ClusterRole
func (i *CommandInitOption) CreateClusterRole() error {
	clusterRole := i.ClusterRoleFromSpec(kubeControllerManagerClusterRoleAndDeploymentAndServiceName, []rbacv1.PolicyRule{
		{
			APIGroups: []string{"*"},
			Resources: []string{"*"},
			Verbs:     []string{"get", "watch", "list", "create", "update", "delete"},
		},
		{
			NonResourceURLs: []string{"*"},
			Verbs:           []string{"get"},
		},
	})

	clusterRoleClient := i.KubeClientSet.RbacV1().ClusterRoles()

	clusterRoleList, err := clusterRoleClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, v := range clusterRoleList.Items {
		if clusterRole.Name == v.Name {
			klog.Warningf("ClusterRole %s already exists.", clusterRole.Name)
			return nil
		}
	}

	_, err = clusterRoleClient.Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create ClusterRole %s failed: %v", clusterRole.Name, err)
	}
	return nil
}

//CreateClusterRoleBinding receive ClusterRoleBindingFromSpec ClusterRoleBinding
func (i *CommandInitOption) CreateClusterRoleBinding(clusterRole *rbacv1.ClusterRoleBinding) error {
	crbClient := i.KubeClientSet.RbacV1().ClusterRoleBindings()

	crbList, err := crbClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, v := range crbList.Items {
		if clusterRole.Name == v.Name {
			klog.Infof("CreateClusterRoleBinding %s already exists.", clusterRole.Name)
			return nil
		}
	}

	_, err = crbClient.Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}
