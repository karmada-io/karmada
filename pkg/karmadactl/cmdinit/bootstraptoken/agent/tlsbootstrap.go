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

package agent

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
)

const (
	// KarmadaAgentBootstrapperClusterRoleName defines the name of the auto-bootstrapped ClusterRole for letting someone post a CSR
	KarmadaAgentBootstrapperClusterRoleName = "system:node-bootstrapper"
	// KarmadaAgentBootstrap defines the name of the ClusterRoleBinding that lets Karmada Agent post CSRs
	KarmadaAgentBootstrap = "system:karmada:agent-bootstrap"
	// KarmadaAgentGroup defines the group of Karmada Agent
	KarmadaAgentGroup = "system:karmada:agents"
	// KarmadaAgentAutoApproveBootstrapClusterRoleBinding defines the name of the ClusterRoleBinding that makes the csrapprover approve agent CSRs
	KarmadaAgentAutoApproveBootstrapClusterRoleBinding = "system:karmada:agent-autoapprove-bootstrap"
	// KarmadaAgentAutoApproveCertificateRotationClusterRoleBinding defines name of the ClusterRoleBinding that makes the csrapprover approve agent auto rotated CSRs
	KarmadaAgentAutoApproveCertificateRotationClusterRoleBinding = "system:karmada:agent-autoapprove-certificate-rotation"
	// CSRAutoApprovalClusterRoleName defines the name of the auto-bootstrapped ClusterRole for making the csrapprover controller auto-approve the CSR
	CSRAutoApprovalClusterRoleName = "system:karmada:certificatesigningrequest:autoapprover"
	// KarmadaAgentSelfCSRAutoApprovalClusterRoleName is a role for automatic CSR approvals for automatically rotated agent certificates
	KarmadaAgentSelfCSRAutoApprovalClusterRoleName = "system:karmada:certificatesigningrequest:selfautoapprover"
	// KarmadaAgentBootstrapTokenAuthGroup specifies which group a Karmada Agent Bootstrap Token should be authenticated in
	KarmadaAgentBootstrapTokenAuthGroup = "system:bootstrappers:karmada:default-cluster-token"
)

// AllowBootstrapTokensToPostCSRs creates RBAC rules in a way the makes Karmada Agent Bootstrap Tokens able to post CSRs
func AllowBootstrapTokensToPostCSRs(clientSet kubernetes.Interface) error {
	klog.Infoln("[bootstrap-token] configured RBAC rules to allow Karmada Agent Bootstrap tokens to post CSRs in order for agent to get long term certificate credentials")

	clusterRoleBinding := utils.ClusterRoleBindingFromSubjects(KarmadaAgentBootstrap, KarmadaAgentBootstrapperClusterRoleName,
		[]rbacv1.Subject{
			{
				Kind: rbacv1.GroupKind,
				Name: KarmadaAgentBootstrapTokenAuthGroup,
			},
		}, nil)
	return cmdutil.CreateOrUpdateClusterRoleBinding(clientSet, clusterRoleBinding)
}

// AutoApproveKarmadaAgentBootstrapTokens creates RBAC rules in a way that makes Karmada Agent Bootstrap Tokens' CSR auto-approved by the csrapprover controller
func AutoApproveKarmadaAgentBootstrapTokens(clientSet kubernetes.Interface) error {
	klog.Infoln("[bootstrap-token] configured RBAC rules to allow the agentcsrapproving controller automatically approve CSRs from a Karmada Agent Bootstrap Token")
	csrAutoApprovalClusterRole := utils.ClusterRoleFromRules(CSRAutoApprovalClusterRoleName, []rbacv1.PolicyRule{
		{
			APIGroups: []string{"certificates.k8s.io"},
			Resources: []string{"certificatesigningrequests/clusteragent"},
			Verbs:     []string{"create"},
		},
	}, nil, nil)
	err := cmdutil.CreateOrUpdateClusterRole(clientSet, csrAutoApprovalClusterRole)
	if err != nil {
		return err
	}

	clusterRoleBinding := utils.ClusterRoleBindingFromSubjects(KarmadaAgentAutoApproveBootstrapClusterRoleBinding, CSRAutoApprovalClusterRoleName,
		[]rbacv1.Subject{
			{
				Kind: rbacv1.GroupKind,
				Name: KarmadaAgentBootstrapTokenAuthGroup,
			},
		}, nil)
	return cmdutil.CreateOrUpdateClusterRoleBinding(clientSet, clusterRoleBinding)
}

// AutoApproveAgentCertificateRotation creates RBAC rules in a way that makes Agent certificate rotation CSR auto-approved by the csrapprover controller
func AutoApproveAgentCertificateRotation(clientSet kubernetes.Interface) error {
	klog.Infoln("[bootstrap-token] configured RBAC rules to allow certificate rotation for all agent client certificates in the member cluster")
	karmadaAgentSelfCSRAutoApprovalClusterRole := utils.ClusterRoleFromRules(KarmadaAgentSelfCSRAutoApprovalClusterRoleName, []rbacv1.PolicyRule{
		{
			APIGroups: []string{"certificates.k8s.io"},
			Resources: []string{"certificatesigningrequests/selfclusteragent"},
			Verbs:     []string{"create"},
		},
	}, nil, nil)
	err := cmdutil.CreateOrUpdateClusterRole(clientSet, karmadaAgentSelfCSRAutoApprovalClusterRole)
	if err != nil {
		return err
	}

	clusterRoleBinding := utils.ClusterRoleBindingFromSubjects(KarmadaAgentAutoApproveCertificateRotationClusterRoleBinding, KarmadaAgentSelfCSRAutoApprovalClusterRoleName,
		[]rbacv1.Subject{
			{
				Kind: rbacv1.GroupKind,
				Name: KarmadaAgentGroup,
			},
		}, nil)
	return cmdutil.CreateOrUpdateClusterRoleBinding(clientSet, clusterRoleBinding)
}
