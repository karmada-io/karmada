package util

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
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
func GenerateImpersonationRules(allSubjects []rbacv1.Subject) []rbacv1.PolicyRule {
	if len(allSubjects) == 0 {
		return nil
	}

	var users, serviceAccounts, groups []string
	for _, subject := range allSubjects {
		switch subject.Kind {
		case rbacv1.UserKind:
			users = append(users, subject.Name)
		case rbacv1.ServiceAccountKind:
			serviceAccounts = append(serviceAccounts, subject.Name)
		case rbacv1.GroupKind:
			groups = append(groups, subject.Name)
		}
	}

	var rules []rbacv1.PolicyRule
	if len(users) != 0 {
		rules = append(rules, rbacv1.PolicyRule{Verbs: []string{"impersonate"}, Resources: []string{"users"}, APIGroups: []string{""}, ResourceNames: users})
	}

	if len(serviceAccounts) != 0 {
		rules = append(rules, rbacv1.PolicyRule{Verbs: []string{"impersonate"}, Resources: []string{"serviceaccounts"}, APIGroups: []string{""}, ResourceNames: serviceAccounts})
	}

	if len(groups) != 0 {
		rules = append(rules, rbacv1.PolicyRule{Verbs: []string{"impersonate"}, Resources: []string{"groups"}, APIGroups: []string{""}, ResourceNames: groups})
	}

	return rules
}

// CreateOrUpdateClusterRole creates a ClusterRole if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateClusterRole(client kubeclient.Interface, clusterRole *rbacv1.ClusterRole) error {
	if _, err := client.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create RBAC clusterrole: %v", err)
		}

		if _, err := client.RbacV1().ClusterRoles().Update(context.TODO(), clusterRole, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update RBAC clusterrole: %v", err)
		}
	}
	klog.V(1).Infof("ClusterRole %s has been created or updated.", clusterRole.ObjectMeta.Name)

	return nil
}

// CreateOrUpdateClusterRoleBinding creates a ClusterRoleBinding if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
func CreateOrUpdateClusterRoleBinding(client kubeclient.Interface, clusterRoleBinding *rbacv1.ClusterRoleBinding) error {
	if _, err := client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("unable to create RBAC clusterrolebinding: %v", err)
		}

		if _, err := client.RbacV1().ClusterRoleBindings().Update(context.TODO(), clusterRoleBinding, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("unable to update RBAC clusterrolebinding: %v", err)
		}
	}
	klog.V(1).Infof("ClusterRoleBinding %s has been created or updated.", clusterRoleBinding.ObjectMeta.Name)

	return nil
}
