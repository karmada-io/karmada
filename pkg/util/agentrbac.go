/*
Copyright 2026 The Karmada Authors.

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
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// ClusterPermissionPrefix defines the common name prefix of the karmada-agent client certificate.
	ClusterPermissionPrefix = "system:karmada:agent:"
	// ClusterPermissionGroup defines the organization of the karmada-agent client certificate.
	ClusterPermissionGroup = "system:karmada:agents"
	// ClusterNamespaceAnnotation records the control-plane namespace used by a pull mode cluster.
	ClusterNamespaceAnnotation = "cluster.karmada.io/namespace"
)

// AgentRBACResources defines the RBAC resources required by a pull mode cluster agent on the control plane.
type AgentRBACResources struct {
	ClusterRoles        []*rbacv1.ClusterRole
	ClusterRoleBindings []*rbacv1.ClusterRoleBinding
	Roles               []*rbacv1.Role
	RoleBindings        []*rbacv1.RoleBinding
}

// GetClusterNamespace returns the control-plane namespace used by a member cluster.
func GetClusterNamespace(cluster *clusterv1alpha1.Cluster) string {
	if cluster == nil {
		return names.NamespaceKarmadaCluster
	}

	if cluster.Annotations != nil {
		if namespace := cluster.Annotations[ClusterNamespaceAnnotation]; namespace != "" {
			return namespace
		}
	}

	if cluster.Spec.SecretRef != nil && cluster.Spec.SecretRef.Namespace != "" {
		return cluster.Spec.SecretRef.Namespace
	}

	if cluster.Spec.ImpersonatorSecretRef != nil && cluster.Spec.ImpersonatorSecretRef.Namespace != "" {
		return cluster.Spec.ImpersonatorSecretRef.Namespace
	}

	return names.NamespaceKarmadaCluster
}

// GenerateAgentUserName returns the user name bound to a pull mode cluster agent.
func GenerateAgentUserName(clusterName string) string {
	return fmt.Sprintf("%s%s", ClusterPermissionPrefix, clusterName)
}

// GenerateAgentRBACResources returns the RBAC resources required by a pull mode cluster agent.
func GenerateAgentRBACResources(clusterName, clusterNamespace string) *AgentRBACResources {
	return &AgentRBACResources{
		ClusterRoles:        []*rbacv1.ClusterRole{GenerateAgentClusterRole(clusterName)},
		ClusterRoleBindings: []*rbacv1.ClusterRoleBinding{GenerateAgentClusterRoleBinding(clusterName)},
		Roles: []*rbacv1.Role{
			GenerateAgentSecretAccessRole(clusterName, clusterNamespace),
			GenerateAgentWorkAccessRole(clusterName),
		},
		RoleBindings: []*rbacv1.RoleBinding{
			GenerateAgentSecretAccessRoleBinding(clusterName, clusterNamespace),
			GenerateAgentWorkAccessRoleBinding(clusterName),
		},
	}
}

// GenerateAgentClusterRole generates the ClusterRole required by a pull mode cluster agent.
func GenerateAgentClusterRole(clusterName string) *rbacv1.ClusterRole {
	clusterRoleName := fmt.Sprintf("system:karmada:%s:agent", clusterName)
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"cluster.karmada.io"},
				Resources:     []string{"clusters"},
				ResourceNames: []string{clusterName},
				Verbs:         []string{"get", "delete"},
			},
			{
				APIGroups: []string{"cluster.karmada.io"},
				Resources: []string{"clusters"},
				Verbs:     []string{"create", "list", "watch"},
			},
			{
				APIGroups:     []string{"cluster.karmada.io"},
				Resources:     []string{"clusters/status"},
				ResourceNames: []string{clusterName},
				Verbs:         []string{"update"},
			},
			{
				APIGroups: []string{"config.karmada.io"},
				Resources: []string{"resourceinterpreterwebhookconfigurations", "resourceinterpretercustomizations"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "create", "update"},
			},
			{
				APIGroups: []string{"certificates.k8s.io"},
				Resources: []string{"certificatesigningrequests"},
				Verbs:     []string{"get", "create"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"patch", "create", "update"},
			},
		},
	}
}

// GenerateAgentClusterRoleBinding generates the ClusterRoleBinding required by a pull mode cluster agent.
func GenerateAgentClusterRoleBinding(clusterName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("system:karmada:%s:agent", clusterName),
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: rbacv1.GroupName,
				Kind:     rbacv1.UserKind,
				Name:     GenerateAgentUserName(clusterName),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     fmt.Sprintf("system:karmada:%s:agent", clusterName),
		},
	}
}

// GenerateAgentSecretAccessRole generates the Secret access Role required by a pull mode cluster agent.
func GenerateAgentSecretAccessRole(clusterName, clusterNamespace string) *rbacv1.Role {
	roleName := fmt.Sprintf("system:karmada:%s:agent-secret", clusterName)
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: clusterNamespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"get", "patch"},
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{clusterName, clusterName + "-impersonator"},
			},
			{
				Verbs:     []string{"create"},
				APIGroups: []string{""},
				Resources: []string{"secrets"},
			},
		},
	}
}

// GenerateAgentSecretAccessRoleBinding generates the Secret access RoleBinding required by a pull mode cluster agent.
func GenerateAgentSecretAccessRoleBinding(clusterName, clusterNamespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("system:karmada:%s:agent-secret", clusterName),
			Namespace: clusterNamespace,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: rbacv1.GroupName,
				Kind:     rbacv1.UserKind,
				Name:     GenerateAgentUserName(clusterName),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     fmt.Sprintf("system:karmada:%s:agent-secret", clusterName),
		},
	}
}

// GenerateAgentWorkAccessRole generates the Work access Role required by a pull mode cluster agent.
func GenerateAgentWorkAccessRole(clusterName string) *rbacv1.Role {
	roleName := fmt.Sprintf("system:karmada:%s:agent-work", clusterName)
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: names.GenerateExecutionSpaceName(clusterName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "create", "list", "watch", "update", "delete"},
				APIGroups: []string{"work.karmada.io"},
				Resources: []string{"works"},
			},
			{
				Verbs:     []string{"patch", "update"},
				APIGroups: []string{"work.karmada.io"},
				Resources: []string{"works/status"},
			},
		},
	}
}

// GenerateAgentWorkAccessRoleBinding generates the Work access RoleBinding required by a pull mode cluster agent.
func GenerateAgentWorkAccessRoleBinding(clusterName string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("system:karmada:%s:agent-work", clusterName),
			Namespace: names.GenerateExecutionSpaceName(clusterName),
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: rbacv1.GroupName,
				Kind:     rbacv1.UserKind,
				Name:     GenerateAgentUserName(clusterName),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     fmt.Sprintf("system:karmada:%s:agent-work", clusterName),
		},
	}
}
