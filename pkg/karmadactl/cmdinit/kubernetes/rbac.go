package kubernetes

import (
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

// CreateControllerManagerRBAC karmada-controller-manager ClusterRole and ClusterRoleBinding
func (i *CommandInitOption) CreateControllerManagerRBAC() error {
	labels := map[string]string{karmadaBootstrappingLabelKey: karmadaBootstrappingLabelValue}
	// ClusterRole
	clusterRole := utils.ClusterRoleFromRules(controllerManagerDeploymentAndServiceName, []rbacv1.PolicyRule{
		{
			APIGroups: []string{"*"},
			Resources: []string{"*"},
			Verbs:     []string{"get", "watch", "list", "create", "update", "delete"},
		},
		{
			NonResourceURLs: []string{"*"},
			Verbs:           []string{"get"},
		},
	}, labels)
	err := utils.CreateIfNotExistClusterRole(i.KubeClientSet, clusterRole)
	if err != nil {
		return err
	}

	// ClusterRoleBinding
	clusterRoleBinding := utils.ClusterRoleBindingFromSubjects(controllerManagerDeploymentAndServiceName, controllerManagerDeploymentAndServiceName,
		[]rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      controllerManagerDeploymentAndServiceName,
				Namespace: i.Namespace,
			},
		}, labels)
	err = utils.CreateIfNotExistClusterRoleBinding(i.KubeClientSet, clusterRoleBinding)
	if err != nil {
		return err
	}

	return nil
}
