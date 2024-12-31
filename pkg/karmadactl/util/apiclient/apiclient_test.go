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

package apiclient

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/client-go/rest"

	testingutil "github.com/karmada-io/karmada/pkg/util/testing"
)

func TestRestConfig(t *testing.T) {
	kubeconfig := `apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
    certificate-authority-data: %s
  name: member1
contexts:
- context:
    cluster: member1
    user: member1
  name: member1-context
current-context: member1-context
kind: Config
preferences: {}
users:
- name: test-user
  user:
    client-certificate-data: %s
    client-key-data: %s
`
	tests := []struct {
		name                    string
		context, kubeConfigPath string
		prep                    func(kubeConfigPath string) (string, error)
		verify                  func(restConfig *rest.Config, caCert string) error
		cleanup                 func(kubeConfigPath string) error
		wantErr                 bool
		errMsg                  string
	}{
		{
			name:           "RestConfig_KubeConfigNotExist_MissingConfigurationInfo",
			context:        "member1-context",
			kubeConfigPath: filepath.Join(os.TempDir(), "member1-cluster.config"),
			prep:           func(string) (string, error) { return "", nil },
			verify:         func(*rest.Config, string) error { return nil },
			cleanup:        func(string) error { return nil },
			wantErr:        true,
			errMsg:         ErrEmptyConfig.Error(),
		},
		{
			name:           "RestConfig_CreateRestConfig_RestConfigCreated",
			context:        "member1-context",
			kubeConfigPath: filepath.Join(os.TempDir(), "member1-cluster.config"),
			prep: func(kubeConfigPath string) (string, error) {
				return prepRestConfig(kubeconfig, kubeConfigPath)
			},
			verify: func(restConfig *rest.Config, caCert string) error {
				return verifyRestConfig(restConfig, caCert)
			},
			cleanup: func(kubeConfigPath string) error {
				if err := os.Remove(kubeConfigPath); err != nil {
					return fmt.Errorf("failed to clean up config file %s, got error: %v", kubeConfigPath, err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caCert, err := test.prep(test.kubeConfigPath)
			if err != nil {
				t.Fatalf("failed to prep test environment, got error: %v", err)
			}
			defer func() {
				if err := test.cleanup(test.kubeConfigPath); err != nil {
					t.Errorf("failed to cleanup test environment, got error: %v", err)
				}
			}()
			restConfig, err := RestConfig(test.context, test.kubeConfigPath)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(restConfig, caCert); err != nil {
				t.Errorf("failed to verify creating rest config, got error: %v", err)
			}
		})
	}
}

func prepRestConfig(kubeConfig, kubeConfigPath string) (string, error) {
	caCert, privateKey, err := testingutil.GenerateTestCACertificate()
	if err != nil {
		return "", fmt.Errorf("failed to generate CA certificate: %v", err)
	}
	base64CACert := base64.StdEncoding.EncodeToString([]byte(caCert))
	base64PrivateKey := base64.StdEncoding.EncodeToString([]byte(privateKey))
	kubeconfigFormatted := fmt.Sprintf(kubeConfig, base64CACert, base64CACert, base64PrivateKey)
	if err := os.WriteFile(kubeConfigPath, []byte(kubeconfigFormatted), 0600); err != nil {
		return "", fmt.Errorf("failed to write kubeconfig to file: %v", err)
	}
	return base64CACert, nil
}

func verifyRestConfig(restConfig *rest.Config, caCert string) error {
	if restConfig == nil {
		return errors.New("expected rest config, but got nil")
	}
	if restConfig.Host != "https://127.0.0.1:6443" {
		return fmt.Errorf("expected rest config host to be %s, but got %s", "https://127.0.0.1:6443", restConfig.Host)
	}

	caCertDecoded, err := base64.StdEncoding.DecodeString(caCert)
	if err != nil {
		return fmt.Errorf("failed to decode ca certificate, got error: %v", err)
	}
	if !bytes.Equal(restConfig.TLSClientConfig.CAData, caCertDecoded) {
		return fmt.Errorf("expected ca cert bytes %v to be %v", restConfig.TLSClientConfig.CAData, caCertDecoded)
	}

	return nil
}
