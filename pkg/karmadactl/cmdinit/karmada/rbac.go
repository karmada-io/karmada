/*
Copyright 2022 The Karmada Authors.

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

package karmada

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
)

const (
	karmadaViewClusterRole                      = "karmada-view"
	karmadaEditClusterRole                      = "karmada-edit"
	karmadaAgentRBACGeneratorClusterRole        = "system:karmada:agent-rbac-generator"
	karmadaAgentRBACGeneratorClusterRoleBinding = "system:karmada:agent-rbac-generator"
	agentRBACGenerator                          = "system:karmada:agent:rbac-generator"
)

// grantProxyPermissionToAdmin grants the proxy permission to "system:admin"
func grantProxyPermissionToAdmin(clientSet kubernetes.Interface) error {
	proxyAdminClusterRole := utils.ClusterRoleFromRules(clusterProxyAdminRole, []rbacv1.PolicyRule{
		{
			APIGroups: []string{"cluster.karmada.io"},
			Resources: []string{"clusters/proxy"},
			Verbs:     []string{"*"},
		},
	}, nil, nil)
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

	klog.V(1).Info("Grant cluster proxy permission to 'system:admin'")
	err = cmdutil.CreateOrUpdateClusterRoleBinding(clientSet, proxyAdminClusterRoleBinding)
	if err != nil {
		return err
	}

	return nil
}

// grantAccessPermissionToAgentRBACGenerator grants the access permission to 'karmada-agent-rbac-generator'
func grantAccessPermissionToAgentRBACGenerator(clientSet kubernetes.Interface) error {
	clusterRole := utils.ClusterRoleFromRules(karmadaAgentRBACGeneratorClusterRole, []rbacv1.PolicyRule{
		{
			APIGroups: []string{"*"},
			Resources: []string{"*"},
			Verbs:     []string{"*"},
		},
	}, nil, nil)
	err := cmdutil.CreateOrUpdateClusterRole(clientSet, clusterRole)
	if err != nil {
		return err
	}

	clusterRoleBinding := utils.ClusterRoleBindingFromSubjects(karmadaAgentRBACGeneratorClusterRoleBinding, karmadaAgentRBACGeneratorClusterRole,
		[]rbacv1.Subject{
			{
				Kind: rbacv1.UserKind,
				Name: agentRBACGenerator,
			}}, nil)

	klog.V(1).Info("Grant the access permission to 'karmada-agent-rbac-generator'")
	err = cmdutil.CreateOrUpdateClusterRoleBinding(clientSet, clusterRoleBinding)
	if err != nil {
		return err
	}

	return nil
}

// grantKarmadaPermissionToViewClusterRole grants view clusterrole with karmada resource permission
func grantKarmadaPermissionToViewClusterRole(clientSet kubernetes.Interface) error {
	annotations := map[string]string{
		// refer to https://kubernetes.io/docs/reference/access-authn-authz/rbac/#auto-reconciliation
		// and https://kubernetes.io/docs/reference/access-authn-authz/rbac/#kubectl-auth-reconcile
		"rbac.authorization.kubernetes.io/autoupdate": "true",
	}
	labels := map[string]string{
		// refer to https://kubernetes.io/docs/reference/access-authn-authz/rbac/#default-roles-and-role-bindings
		"kubernetes.io/bootstrapping": "rbac-defaults",
		// used to aggregate rules to view clusterrole
		"rbac.authorization.k8s.io/aggregate-to-view": "true",
	}
	clusterRole := utils.ClusterRoleFromRules(karmadaViewClusterRole, []rbacv1.PolicyRule{
		{
			APIGroups: []string{"autoscaling.karmada.io"},
			Resources: []string{
				"cronfederatedhpas",
				"cronfederatedhpas/status",
				"federatedhpas",
				"federatedhpas/status",
			},
			Verbs: []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"multicluster.x-k8s.io"},
			Resources: []string{
				"serviceexports",
				"serviceexports/status",
				"serviceimports",
				"serviceimports/status",
			},
			Verbs: []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"networking.karmada.io"},
			Resources: []string{
				"multiclusteringresses",
				"multiclusteringresses/status",
				"multiclusterservices",
				"multiclusterservices/status",
			},
			Verbs: []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"policy.karmada.io"},
			Resources: []string{
				"overridepolicies",
				"propagationpolicies",
			},
			Verbs: []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"work.karmada.io"},
			Resources: []string{
				"resourcebindings",
				"resourcebindings/status",
				"works",
				"works/status",
			},
			Verbs: []string{"get", "list", "watch"},
		},
	}, annotations, labels)

	err := cmdutil.CreateOrUpdateClusterRole(clientSet, clusterRole)
	if err != nil {
		return err
	}
	return nil
}

// grantKarmadaPermissionToEditClusterRole grants edit clusterrole with karmada resource permission
func grantKarmadaPermissionToEditClusterRole(clientSet kubernetes.Interface) error {
	annotations := map[string]string{
		// refer to https://kubernetes.io/docs/reference/access-authn-authz/rbac/#auto-reconciliation
		// and https://kubernetes.io/docs/reference/access-authn-authz/rbac/#kubectl-auth-reconcile
		"rbac.authorization.kubernetes.io/autoupdate": "true",
	}
	labels := map[string]string{
		// refer to https://kubernetes.io/docs/reference/access-authn-authz/rbac/#default-roles-and-role-bindings
		"kubernetes.io/bootstrapping": "rbac-defaults",
		// used to aggregate rules to edit clusterrole
		"rbac.authorization.k8s.io/aggregate-to-edit": "true",
	}
	clusterRole := utils.ClusterRoleFromRules(karmadaEditClusterRole, []rbacv1.PolicyRule{
		{
			APIGroups: []string{"autoscaling.karmada.io"},
			Resources: []string{
				"cronfederatedhpas",
				"cronfederatedhpas/status",
				"federatedhpas",
				"federatedhpas/status",
			},
			Verbs: []string{"create", "update", "patch", "delete", "deletecollection"},
		},
		{
			APIGroups: []string{"multicluster.x-k8s.io"},
			Resources: []string{
				"serviceexports",
				"serviceexports/status",
				"serviceimports",
				"serviceimports/status",
			},
			Verbs: []string{"create", "update", "patch", "delete", "deletecollection"},
		},
		{
			APIGroups: []string{"networking.karmada.io"},
			Resources: []string{
				"multiclusteringresses",
				"multiclusteringresses/status",
				"multiclusterservices",
				"multiclusterservices/status",
			},
			Verbs: []string{"create", "update", "patch", "delete", "deletecollection"},
		},
		{
			APIGroups: []string{"policy.karmada.io"},
			Resources: []string{
				"overridepolicies",
				"propagationpolicies",
			},
			Verbs: []string{"create", "update", "patch", "delete", "deletecollection"},
		},
		{
			APIGroups: []string{"work.karmada.io"},
			Resources: []string{
				"resourcebindings",
				"resourcebindings/status",
				"works",
				"works/status",
			},
			Verbs: []string{"create", "update", "patch", "delete", "deletecollection"},
		},
	}, annotations, labels)

	err := cmdutil.CreateOrUpdateClusterRole(clientSet, clusterRole)
	if err != nil {
		return err
	}
	return nil
}
