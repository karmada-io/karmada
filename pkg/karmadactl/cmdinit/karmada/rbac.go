package karmada

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

// grantProxyPermissionToAdmin grants the proxy permission to "system:admin"
func grantProxyPermissionToAdmin(clientSet *kubernetes.Clientset) error {
	proxyAdminClusterRole := utils.ClusterRoleFromRules(clusterProxyAdminRole, []rbacv1.PolicyRule{
		{
			APIGroups: []string{"cluster.karmada.io"},
			Resources: []string{"clusters/proxy"},
			Verbs:     []string{"*"},
		},
	}, nil)
	err := utils.CreateIfNotExistClusterRole(clientSet, proxyAdminClusterRole)
	if err != nil {
		return err
	}

	proxyAdminClusterRoleBinding := utils.ClusterRoleBindingFromSubjects(clusterProxyAdminRole, clusterProxyAdminRole,
		[]rbacv1.Subject{
			{
				Kind: "User",
				Name: clusterProxyAdminUser,
			}}, nil)
	err = utils.CreateIfNotExistClusterRoleBinding(clientSet, proxyAdminClusterRoleBinding)
	if err != nil {
		return err
	}

	return nil
}
