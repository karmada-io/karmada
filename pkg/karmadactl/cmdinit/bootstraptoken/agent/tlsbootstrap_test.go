/*
Copyright 2024 The Karmada Authors.

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
	"context"
	"fmt"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

func TestAllowBootstrapTokensToPostCSRs(t *testing.T) {
	tests := []struct {
		name   string
		client clientset.Interface
		prep   func(clientset.Interface) error
		verify func(clientset.Interface) error
	}{
		{
			name:   "AllowBootstrapTokensToPostCSRs_CreateClusterRoleBinding_Created",
			client: fakeclientset.NewSimpleClientset(),
			prep:   func(clientset.Interface) error { return nil },
			verify: func(client clientset.Interface) error {
				return verifyClusterRoleBinding(client, KarmadaAgentBootstrap, KarmadaAgentBootstrapperClusterRoleName)
			},
		},
		{
			name:   "AllowBootstrapTokensToPostCSRs_ClusterRoleBindingAlreadyExists_Updated",
			client: fakeclientset.NewSimpleClientset(),
			prep: func(client clientset.Interface) error {
				return createClusterRoleBinding(client, KarmadaAgentBootstrap, KarmadaAgentBootstrapperClusterRoleName, KarmadaAgentBootstrapTokenAuthGroup)
			},
			verify: func(client clientset.Interface) error {
				return verifyClusterRoleBinding(client, KarmadaAgentBootstrap, KarmadaAgentBootstrapperClusterRoleName)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client); err != nil {
				t.Fatalf("failed to prep before allowing bootstrap tokens to post CSRs, got: %v", err)
			}
			if err := AllowBootstrapTokensToPostCSRs(test.client); err != nil {
				t.Errorf("unexpected error while allowing bootstrap tokens to post CSRs, got: %v", err)
			}
			if err := test.verify(test.client); err != nil {
				t.Errorf("failed to verify the creation of cluster role bindings, got: %v", err)
			}
		})
	}
}

func TestAutoApproveKarmadaAgentBootstrapTokens(t *testing.T) {
	tests := []struct {
		name   string
		client clientset.Interface
		prep   func(clientset.Interface) error
		verify func(clientset.Interface) error
	}{
		{
			name:   "AutoApproveKarmadaAgentBootstrapTokens_CreateClusterRoleBindings_Created",
			client: fakeclientset.NewSimpleClientset(),
			prep:   func(clientset.Interface) error { return nil },
			verify: func(client clientset.Interface) error {
				return verifyClusterRoleBinding(client, KarmadaAgentAutoApproveBootstrapClusterRoleBinding, CSRAutoApprovalClusterRoleName)
			},
		},
		{
			name:   "AutoApproveKarmadaAgentBootstrapTokens_ClusterRoleBindingAlreadyExists_Updated",
			client: fakeclientset.NewSimpleClientset(),
			prep: func(client clientset.Interface) error {
				return createClusterRoleBinding(client, KarmadaAgentAutoApproveBootstrapClusterRoleBinding, CSRAutoApprovalClusterRoleName, KarmadaAgentBootstrapTokenAuthGroup)
			},
			verify: func(client clientset.Interface) error {
				return verifyClusterRoleBinding(client, KarmadaAgentAutoApproveBootstrapClusterRoleBinding, CSRAutoApprovalClusterRoleName)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client); err != nil {
				t.Fatalf("failed to prep before auto-approve karmada agent bootstrap tokens, got: %v", err)
			}
			if err := AutoApproveKarmadaAgentBootstrapTokens(test.client); err != nil {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err := test.verify(test.client); err != nil {
				t.Errorf("failed to verify the creation of cluster role bindings, got error: %v", err)
			}
		})
	}
}

func TestAutoApproveAgentCertificateRotation(t *testing.T) {
	tests := []struct {
		name   string
		client clientset.Interface
		prep   func(clientset.Interface) error
		verify func(clientset.Interface) error
	}{
		{
			name:   "AutoApproveAgentCertificateRotation_CreateClusterRoleBindings_Created",
			client: fakeclientset.NewSimpleClientset(),
			prep:   func(clientset.Interface) error { return nil },
			verify: func(client clientset.Interface) error {
				return verifyClusterRoleBinding(client, KarmadaAgentAutoApproveCertificateRotationClusterRoleBinding, KarmadaAgentSelfCSRAutoApprovalClusterRoleName)
			},
		},
		{
			name:   "AutoApproveAgentCertificateRotation_ClusterRoleBindingAlreadyExists_Updated",
			client: fakeclientset.NewSimpleClientset(),
			prep: func(client clientset.Interface) error {
				return createClusterRoleBinding(client, KarmadaAgentAutoApproveCertificateRotationClusterRoleBinding, KarmadaAgentSelfCSRAutoApprovalClusterRoleName, KarmadaAgentGroup)
			},
			verify: func(client clientset.Interface) error {
				return verifyClusterRoleBinding(client, KarmadaAgentAutoApproveCertificateRotationClusterRoleBinding, KarmadaAgentSelfCSRAutoApprovalClusterRoleName)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client); err != nil {
				t.Fatalf("failed to prep before auto-approve agent certificate rotation, got: %v", err)
			}
			if err := AutoApproveAgentCertificateRotation(test.client); err != nil {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err := test.verify(test.client); err != nil {
				t.Errorf("failed to verify the creation of cluster role bindings, got error: %v", err)
			}
		})
	}
}

func createClusterRoleBinding(client clientset.Interface, crbName, crName, subjectName string) error {
	clusterRoleBinding := utils.ClusterRoleBindingFromSubjects(crbName, crName,
		[]rbacv1.Subject{
			{
				Kind: rbacv1.GroupKind,
				Name: subjectName,
			},
		}, nil)
	if _, err := client.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create cluster role binding, got: %v", err)
	}
	return nil
}

func verifyClusterRoleBinding(client clientset.Interface, crbName, crName string) error {
	clusterRoleBinding, err := client.RbacV1().ClusterRoleBindings().Get(context.TODO(), crbName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get cluster role binding, got: %v", err)
	}
	if clusterRoleBinding.RoleRef.Name != crName {
		return fmt.Errorf("expected cluster role ref name to be %s, but got %s", crName, clusterRoleBinding.RoleRef.Name)
	}
	return nil
}
