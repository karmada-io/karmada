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

package clusterinfo

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"

	"github.com/karmada-io/karmada/operator/pkg/certs"
)

func TestCreateBootstrapConfigMapIfNotExists(t *testing.T) {
	kubeAdminConfig := `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: https://test-cluster:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
kind: Config
preferences: {}
users:
- name: test-user
  user:
    client-certificate-data: %s
    client-key-data: %s
`
	tests := []struct {
		name    string
		client  kubernetes.Interface
		cfgFile string
		prep    func(cfgFile string) error
		verify  func(kubernetes.Interface) error
		cleanup func(cfgFile string) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "CreateBootstrapConfigMapIfNotExists_NonExistentConfigFile_FailedToLoadAdminKubeConfig",
			prep:    func(string) error { return nil },
			verify:  func(kubernetes.Interface) error { return nil },
			cleanup: func(string) error { return nil },
			wantErr: true,
			errMsg:  "failed to load admin kubeconfig",
		},
		{
			name:    "CreateBootstrapConfigMapIfNotExists_WithConfigFile_ConfigMapCreatedInKubePublicNamespace",
			client:  fakeclientset.NewClientset(),
			cfgFile: filepath.Join(os.TempDir(), "config-temp.txt"),
			prep: func(cfgFile string) error {
				caKarmadaCert, err := certs.NewCertificateAuthority(certs.KarmadaCertAdmin())
				if err != nil {
					t.Fatalf("NewCertificateAuthority() returned an error: %v", err)
				}

				kubeAdminConfig = fmt.Sprintf(
					kubeAdminConfig,
					base64.StdEncoding.EncodeToString(caKarmadaCert.CertData()),
					base64.StdEncoding.EncodeToString(caKarmadaCert.CertData()),
					base64.StdEncoding.EncodeToString(caKarmadaCert.KeyData()),
				)

				err = os.WriteFile(cfgFile, []byte(kubeAdminConfig), 0600)
				if err != nil {
					return fmt.Errorf("failed to write kubeAdminConfig to file, got error: %v", err)
				}

				return nil
			},
			cleanup: func(cfgFile string) error {
				if err := os.Remove(cfgFile); err != nil {
					return fmt.Errorf("failed to remove config file %s, got an error: %v", cfgFile, err)
				}
				return nil
			},
			verify: func(c kubernetes.Interface) error {
				return verifyKubeAdminKubeConfig(c)
			},
			wantErr: false,
		},
		// TODO: Update ConfigMap if it exists.
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.cfgFile); err != nil {
				t.Fatalf("failed prep before creating bootstrap config map, got error: %v", err)
			}
			defer func() {
				if err := test.cleanup(test.cfgFile); err != nil {
					t.Errorf("deferred cleanup failed: %v", err)
				}
			}()

			err := CreateBootstrapConfigMapIfNotExists(test.client, test.cfgFile)
			if err == nil && test.wantErr {
				t.Fatal("expected and error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client); err != nil {
				t.Errorf("failed to verify creating bootstrap config map, got an error: %v", err)
			}
		})
	}
}

func TestCreateClusterInfoRBACRules(t *testing.T) {
	tests := []struct {
		name   string
		client kubernetes.Interface
		verify func(kubernetes.Interface) error
	}{
		{
			name:   "CreateClusterInfoRBACRules_CreateRolesAndRoleBindings_Created",
			client: fakeclientset.NewClientset(),
			verify: func(c kubernetes.Interface) error {
				// Verify that roles are created as expected.
				role, err := c.RbacV1().Roles(metav1.NamespacePublic).Get(context.TODO(), BootstrapSignerClusterRoleName, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("failed to get role %s, got an error: %v", BootstrapSignerClusterRoleName, err)
				}
				expectedPolicyRoles := []rbacv1.PolicyRule{
					{
						Verbs:         []string{"get"},
						APIGroups:     []string{""},
						Resources:     []string{"configmaps"},
						ResourceNames: []string{bootstrapapi.ConfigMapClusterInfo},
					},
				}
				if !reflect.DeepEqual(role.Rules, expectedPolicyRoles) {
					return fmt.Errorf("expected policy roles to be equal, expected %v but got %v", expectedPolicyRoles, role.Rules)
				}

				// Verify that role bindings are created as expected.
				roleBinding, err := c.RbacV1().RoleBindings(metav1.NamespacePublic).Get(context.TODO(), BootstrapSignerClusterRoleName, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("failed to get role binding %s, got an error: %v", BootstrapSignerClusterRoleName, err)
				}
				if roleBinding.RoleRef.Name != BootstrapSignerClusterRoleName {
					return fmt.Errorf("expected rolebinding ref name to be %s, but got %s", BootstrapSignerClusterRoleName, roleBinding.RoleRef.Name)
				}
				if roleBinding.Subjects[0].Kind == rbacv1.UserKind && roleBinding.Subjects[0].Name != user.Anonymous {
					return fmt.Errorf("expected role binding subject user to be %s, but got %s", user.Anonymous, roleBinding.Subjects[0].Name)
				}

				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := CreateClusterInfoRBACRules(test.client); err != nil {
				t.Fatalf("failed to create cluster info RBAC rules, got error: %v", err)
			}
			if err := test.verify(test.client); err != nil {
				t.Errorf("failed to verify creating cluster info RBAC rules, got error: %v", err)
			}
		})
	}
}

func verifyKubeAdminKubeConfig(client kubernetes.Interface) error {
	configMap, err := client.CoreV1().ConfigMaps(metav1.NamespacePublic).Get(context.TODO(), bootstrapapi.ConfigMapClusterInfo, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get configmap %s, got an error: %v", bootstrapapi.ConfigMapClusterInfo, err)
	}
	if _, ok := configMap.Data[bootstrapapi.KubeConfigKey]; !ok {
		return fmt.Errorf("expected key %s to exist on the data field", bootstrapapi.KubeConfigKey)
	}
	return nil
}
