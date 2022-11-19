package karmada

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
)

const (
	karmadaAgentAccessClusterRole = "system:karmada:agent"
	karmadaAgentGroup             = "system:nodes"
)

// grantProxyPermissionToAdmin grants the proxy permission to "system:admin"
func grantProxyPermissionToAdmin(clientSet kubernetes.Interface) error {
	proxyAdminClusterRole := utils.ClusterRoleFromRules(clusterProxyAdminRole, []rbacv1.PolicyRule{
		{
			APIGroups: []string{"cluster.karmada.io"},
			Resources: []string{"clusters/proxy"},
			Verbs:     []string{"*"},
		},
	}, nil)
	err := cmdutil.CreateOrUpdateClusterRole(clientSet, proxyAdminClusterRole)
	if err != nil {
		return err
	}

	proxyAdminClusterRoleBinding := utils.ClusterRoleBindingFromSubjects(clusterProxyAdminRole, clusterProxyAdminRole,
		[]rbacv1.Subject{
			{
				Kind: "User",
				Name: clusterProxyAdminUser,
			}}, nil)

	klog.V(1).Info("grant cluster proxy permission to 'system:admin'")
	err = cmdutil.CreateOrUpdateClusterRoleBinding(clientSet, proxyAdminClusterRoleBinding)
	if err != nil {
		return err
	}

	return nil
}

// grantAccessPermissionToAgent grants the limited access permission to 'karmada-agent'
func grantAccessPermissionToAgent(clientSet kubernetes.Interface) error {
	clusterRole := utils.ClusterRoleFromRules(karmadaAgentAccessClusterRole, []rbacv1.PolicyRule{
		{
			APIGroups: []string{"authentication.k8s.io"},
			Resources: []string{"tokenreviews"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{"cluster.karmada.io"},
			Resources: []string{"clusters"},
			Verbs:     []string{"create", "get", "list", "watch", "patch", "update"},
		},
		{
			APIGroups: []string{"cluster.karmada.io"},
			Resources: []string{"clusters/status"},
			Verbs:     []string{"patch", "update"},
		},
		{
			APIGroups: []string{"work.karmada.io"},
			Resources: []string{"works"},
			Verbs:     []string{"create", "get", "list", "watch", "update"},
		},
		{
			APIGroups: []string{"work.karmada.io"},
			Resources: []string{"works/status"},
			Verbs:     []string{"patch", "update"},
		},
		{
			APIGroups: []string{"config.karmada.io"},
			Resources: []string{"resourceinterpreterwebhookconfigurations"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"config.karmada.io"},
			Resources: []string{"resourceinterpreterwebhookconfigurations"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "list", "watch", "create"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"get", "list", "watch", "create", "patch"},
		},
		{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
			Verbs:     []string{"create", "delete", "get", "patch", "update"},
		},
		{
			APIGroups: []string{"certificates.k8s.io"},
			Resources: []string{"certificatesigningrequests"},
			Verbs:     []string{"create", "get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"create", "patch", "update"},
		},
	}, nil)
	err := cmdutil.CreateOrUpdateClusterRole(clientSet, clusterRole)
	if err != nil {
		return err
	}

	clusterRoleBinding := utils.ClusterRoleBindingFromSubjects(karmadaAgentAccessClusterRole, karmadaAgentAccessClusterRole,
		[]rbacv1.Subject{
			{
				Kind: rbacv1.GroupKind,
				Name: karmadaAgentGroup,
			}}, nil)

	klog.V(1).Info("grant the limited access permission to 'karmada-agent'")
	err = cmdutil.CreateOrUpdateClusterRoleBinding(clientSet, clusterRoleBinding)
	if err != nil {
		return err
	}

	return nil
}
