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

package util

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	testingutil "github.com/karmada-io/karmada/pkg/util/testing"
)

func TestBuildClientFromSecretRef(t *testing.T) {
	name, namespace := "test-secret", "test"
	token := "my-sample-token"
	kubeconfig := `
apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
    certificate-authority-data: %s
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
    token: %s
`
	tests := []struct {
		name    string
		client  clientset.Interface
		ref     *operatorv1alpha1.LocalSecretReference
		wantErr bool
		prep    func(clientset.Interface) error
		errMsg  string
	}{
		{
			name:   "BuildClientFromSecretRef_GotNetworkIssue_FailedToBuildClient",
			client: fakeclientset.NewSimpleClientset(),
			ref: &operatorv1alpha1.LocalSecretReference{
				Name:      name,
				Namespace: namespace,
			},
			prep: func(client clientset.Interface) error {
				client.(*fakeclientset.Clientset).Fake.PrependReactor("get", "secrets", func(coretesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("unexpected error: encountered a network issue while getting the secrets")
				})
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while getting the secrets",
		},
		{
			name:   "BuildClientFromSecretRef_WithoutKubeConfig_KubeConfigIsNotFound",
			client: fakeclientset.NewSimpleClientset(),
			ref: &operatorv1alpha1.LocalSecretReference{
				Name:      name,
				Namespace: namespace,
			},
			prep: func(client clientset.Interface) error {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
					Data: map[string][]byte{},
				}
				_, err := client.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create secret %s in %s namespace, got err: %v", name, namespace, err)
				}
				return nil
			},
			wantErr: true,
			errMsg:  "the kubeconfig or data key 'kubeconfig' is not found",
		},
		{
			name:   "BuildClientFromSecretRef_WithKubeconfig_ClientIsBuilt",
			client: fakeclientset.NewSimpleClientset(),
			ref: &operatorv1alpha1.LocalSecretReference{
				Name:      name,
				Namespace: namespace,
			},
			prep: func(client clientset.Interface) error {
				// Generate kubeconfig bytes.
				caCert, _, err := testingutil.GenerateTestCACertificate()
				if err != nil {
					return fmt.Errorf("failed to generate CA certificate: %v", err)
				}
				base64CACert := base64.StdEncoding.EncodeToString([]byte(caCert))
				base64Token := base64.StdEncoding.EncodeToString([]byte(token))
				kubeconfigBytes := []byte(fmt.Sprintf(kubeconfig, base64CACert, base64Token))

				// Create secret with kubeconfig data.
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
					Data: map[string][]byte{"kubeconfig": kubeconfigBytes},
				}
				_, err = client.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create secret %s in %s namespace, got err: %v", name, namespace, err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client); err != nil {
				t.Errorf("failed to prep before building client from secret ref: %v", err)
			}
			_, err := BuildClientFromSecretRef(test.client, test.ref)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
		})
	}
}

func TestIsInCluster(t *testing.T) {
	tests := []struct {
		name        string
		hostCluster *operatorv1alpha1.HostCluster
		want        bool
	}{
		{
			name:        "IsInCluster_WithoutHostCluster_ItIsLocal",
			hostCluster: nil,
			want:        true,
		},
		{
			name: "IsInCluster_WithoutSecretRef_ItIsLocal",
			hostCluster: &operatorv1alpha1.HostCluster{
				SecretRef: nil,
			},
			want: true,
		},
		{
			name: "IsInCluster_WithoutSecretRefName_ItIsLocal",
			hostCluster: &operatorv1alpha1.HostCluster{
				SecretRef: &operatorv1alpha1.LocalSecretReference{
					Name: "",
				},
			},
			want: true,
		},
		{
			name: "IsInCluster_WithAllRemoteClusterConfigurations_ItIsRemote",
			hostCluster: &operatorv1alpha1.HostCluster{
				SecretRef: &operatorv1alpha1.LocalSecretReference{
					Name:      "remote-secret",
					Namespace: "test",
				},
			},
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsInCluster(test.hostCluster); got != test.want {
				t.Errorf("expected host cluster local status to be %t, but got %t", test.want, got)
			}
		})
	}
}
