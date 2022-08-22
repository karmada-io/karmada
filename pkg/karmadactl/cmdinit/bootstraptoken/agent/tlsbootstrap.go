package agent

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

const (
	// KarmadaAgentBootstrapperClusterRoleName defines the name of the auto-bootstrapped ClusterRole for letting someone post a CSR
	KarmadaAgentBootstrapperClusterRoleName = "system:node-bootstrapper"
	// KarmadaAgentBootstrap defines the name of the ClusterRoleBinding that lets Karmada Agent post CSRs
	KarmadaAgentBootstrap = "karmada:agent-bootstrap"
	// KarmadaAgentGroup defines the group of Karmada Agent
	KarmadaAgentGroup = "system:nodes"
	// KarmadaAgentAutoApproveBootstrapClusterRoleBinding defines the name of the ClusterRoleBinding that makes the csrapprover approve agent CSRs
	KarmadaAgentAutoApproveBootstrapClusterRoleBinding = "karmada:agent-autoapprove-bootstrap"
	// KarmadaAgentAutoApproveCertificateRotationClusterRoleBinding defines name of the ClusterRoleBinding that makes the csrapprover approve agent auto rotated CSRs
	KarmadaAgentAutoApproveCertificateRotationClusterRoleBinding = "karmada:agent-autoapprove-certificate-rotation"
	// CSRAutoApprovalClusterRoleName defines the name of the auto-bootstrapped ClusterRole for making the csrapprover controller auto-approve the CSR
	CSRAutoApprovalClusterRoleName = "system:certificates.k8s.io:certificatesigningrequests:nodeclient"
	// KarmadaAgentSelfCSRAutoApprovalClusterRoleName is a role for automatic CSR approvals for automatically rotated agent certificates
	KarmadaAgentSelfCSRAutoApprovalClusterRoleName = "system:certificates.k8s.io:certificatesigningrequests:selfnodeclient"
	// KarmadaAgentBootstrapTokenAuthGroup specifies which group a Karmada Agent Bootstrap Token should be authenticated in
	KarmadaAgentBootstrapTokenAuthGroup = "system:bootstrappers:karmada:default-cluster-token"
)

// AllowBootstrapTokensToPostCSRs creates RBAC rules in a way the makes Karmada Agent Bootstrap Tokens able to post CSRs
func AllowBootstrapTokensToPostCSRs(clientSet *kubernetes.Clientset) error {
	klog.Infoln("[bootstrap-token] configured RBAC rules to allow Karmada Agent Bootstrap tokens to post CSRs in order for agent to get long term certificate credentials")

	clusterRoleBinding := utils.ClusterRoleBindingFromSubjects(KarmadaAgentBootstrap, KarmadaAgentBootstrapperClusterRoleName,
		[]rbacv1.Subject{
			{
				Kind: rbacv1.GroupKind,
				Name: KarmadaAgentBootstrapTokenAuthGroup,
			},
		}, nil)
	return utils.CreateIfNotExistClusterRoleBinding(clientSet, clusterRoleBinding)
}

// AutoApproveKarmadaAgentBootstrapTokens creates RBAC rules in a way that makes Karmada Agent Bootstrap Tokens' CSR auto-approved by the csrapprover controller
func AutoApproveKarmadaAgentBootstrapTokens(clientSet *kubernetes.Clientset) error {
	klog.Infoln("[bootstrap-token] configured RBAC rules to allow the csrapprover controller automatically approve CSRs from a Karmada Agent Bootstrap Token")

	clusterRoleBinding := utils.ClusterRoleBindingFromSubjects(KarmadaAgentAutoApproveBootstrapClusterRoleBinding, CSRAutoApprovalClusterRoleName,
		[]rbacv1.Subject{
			{
				Kind: rbacv1.GroupKind,
				Name: KarmadaAgentBootstrapTokenAuthGroup,
			},
		}, nil)
	return utils.CreateIfNotExistClusterRoleBinding(clientSet, clusterRoleBinding)
}

// AutoApproveAgentCertificateRotation creates RBAC rules in a way that makes Agent certificate rotation CSR auto-approved by the csrapprover controller
func AutoApproveAgentCertificateRotation(clientSet *kubernetes.Clientset) error {
	klog.Infoln("[bootstrap-token] configured RBAC rules to allow certificate rotation for all agent client certificates in the member cluster")

	clusterRoleBinding := utils.ClusterRoleBindingFromSubjects(KarmadaAgentAutoApproveCertificateRotationClusterRoleBinding, KarmadaAgentSelfCSRAutoApprovalClusterRoleName,
		[]rbacv1.Subject{
			{
				Kind: rbacv1.GroupKind,
				Name: KarmadaAgentGroup,
			},
		}, nil)
	return utils.CreateIfNotExistClusterRoleBinding(clientSet, clusterRoleBinding)
}
