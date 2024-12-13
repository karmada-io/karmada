/*
Copyright 2020 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
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

// PolicyRuleAPIGroupMatches determines if the given policy rule is applied for requested group.
func PolicyRuleAPIGroupMatches(rule *rbacv1.PolicyRule, requestedGroup string) bool {
	for _, ruleGroup := range rule.APIGroups {
		if ruleGroup == rbacv1.APIGroupAll {
			return true
		}
		if ruleGroup == requestedGroup {
			return true
		}
	}

	return false
}

// PolicyRuleResourceMatches determines if the given policy rule is applied for requested resource.
func PolicyRuleResourceMatches(rule *rbacv1.PolicyRule, requestedResource string) bool {
	for _, ruleResource := range rule.Resources {
		if ruleResource == rbacv1.ResourceAll {
			return true
		}

		if ruleResource == requestedResource {
			return true
		}
	}
	return false
}

// PolicyRuleResourceNameMatches determines if the given policy rule is applied for named resource.
func PolicyRuleResourceNameMatches(rule *rbacv1.PolicyRule, requestedName string) bool {
	if len(rule.ResourceNames) == 0 {
		return true
	}

	for _, resourceName := range rule.ResourceNames {
		if resourceName == requestedName {
			return true
		}
	}

	return false
}

// GenerateImpersonationRules generate PolicyRules from given subjects for impersonation.
func GenerateImpersonationRules(subjects []rbacv1.Subject) []rbacv1.PolicyRule {
	if len(subjects) == 0 {
		return nil
	}

	users, serviceAccounts, groups := sets.NewString(), sets.NewString(), sets.NewString()
	for _, subject := range subjects {
		switch subject.Kind {
		case rbacv1.UserKind:
			users.Insert(subject.Name)
		case rbacv1.ServiceAccountKind:
			serviceAccounts.Insert(subject.Name)
		case rbacv1.GroupKind:
			groups.Insert(subject.Name)
		}
	}

	var rules []rbacv1.PolicyRule
	if users.Len() != 0 {
		rules = append(rules, rbacv1.PolicyRule{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{""},
			Resources:     []string{"users"},
			ResourceNames: sets.StringKeySet(users).List(),
		})
	}

	if serviceAccounts.Len() != 0 {
		rules = append(rules, rbacv1.PolicyRule{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{""},
			Resources:     []string{"serviceaccounts"},
			ResourceNames: sets.StringKeySet(serviceAccounts).List(),
		})
	}

	if groups.Len() != 0 {
		rules = append(rules, rbacv1.PolicyRule{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{""},
			Resources:     []string{"groups"},
			ResourceNames: sets.StringKeySet(groups).List(),
		})
	}

	return rules
}

// CreateRole just try to create the Role.
func CreateRole(client kubeclient.Interface, roleObj *rbacv1.Role) (*rbacv1.Role, error) {
	createdObj, err := client.RbacV1().Roles(roleObj.GetNamespace()).Create(context.TODO(), roleObj, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return roleObj, nil
		}

		return nil, err
	}

	return createdObj, nil
}

// CreateRoleBinding just try to create the RoleBinding.
func CreateRoleBinding(client kubeclient.Interface, roleBindingObj *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
	createdObj, err := client.RbacV1().RoleBindings(roleBindingObj.GetNamespace()).Create(context.TODO(), roleBindingObj, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return roleBindingObj, nil
		}

		return nil, err
	}

	return createdObj, nil
}

// DeleteRole just try to delete the Role.
func DeleteRole(client kubeclient.Interface, namespace, name string) error {
	err := client.RbacV1().Roles(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// DeleteRoleBinding just try to delete the RoleBinding.
func DeleteRoleBinding(client kubeclient.Interface, namespace, name string) error {
	err := client.RbacV1().RoleBindings(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
