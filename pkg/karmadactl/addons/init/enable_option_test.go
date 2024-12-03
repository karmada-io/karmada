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

package init

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	cmdinit "github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/kubernetes"
)

func TestValidateEnableAddon(t *testing.T) {
	clusterName, namespace := "test-cluster", "test"
	tests := []struct {
		name         string
		enableOpts   *CommandAddonsEnableOption
		validateArgs []string
		prep         func(client clientset.Interface, cfgFile string) error
		cleanup      func() error
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "Validate_EmptyAddonNames_AddonNamesMustBeNotNull",
			enableOpts:   &CommandAddonsEnableOption{},
			validateArgs: nil,
			prep:         func(clientset.Interface, string) error { return nil },
			cleanup:      func() error { return nil },
			wantErr:      true,
			errMsg:       "addonNames must be not be null",
		},
		{
			name: "Validate_WithoutKubeConfigSecretAndMountName_NotFound",
			enableOpts: &CommandAddonsEnableOption{
				GlobalCommandOptions: GlobalCommandOptions{
					Namespace:     namespace,
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
			},
			validateArgs: []string{DeschedulerResourceName},
			prep: func(clientset.Interface, string) error {
				Addons["karmada-descheduler"] = &Addon{Name: DeschedulerResourceName}
				return nil
			},
			cleanup: func() error {
				Addons = map[string]*Addon{}
				return nil
			},
			wantErr: true,
			errMsg:  fmt.Sprintf("secrets `kubeconfig` is not found in namespace %s", namespace),
		},
		{
			name: "Validate_WithoutMemberClusterAndKarmadaSchedulerEstimatorArg_MemberClusterIsNeeded",
			enableOpts: &CommandAddonsEnableOption{
				GlobalCommandOptions: GlobalCommandOptions{
					Namespace:     namespace,
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
			},
			validateArgs: []string{EstimatorResourceName},
			prep: func(client clientset.Interface, _ string) error {
				Addons["karmada-scheduler-estimator"] = &Addon{Name: EstimatorResourceName}

				kubeConfigSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cmdinit.KubeConfigSecretAndMountName,
						Namespace: namespace,
					},
				}
				_, err := client.CoreV1().Secrets(namespace).Create(context.TODO(), kubeConfigSecret, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create secret %s in namespace %s, got an error: %v", kubeConfigSecret, namespace, err)
				}

				return nil
			},
			cleanup: func() error {
				Addons = map[string]*Addon{}
				return nil
			},
			wantErr: true,
			errMsg:  "member cluster is needed when enable karmada-scheduler-estimator",
		},
		{
			name: "Validate_WithMemberClusterAndKubeConfigSecret_Validated",
			enableOpts: &CommandAddonsEnableOption{
				GlobalCommandOptions: GlobalCommandOptions{
					Namespace:     namespace,
					KubeClientSet: fakeclientset.NewSimpleClientset(),
					Cluster:       clusterName,
				},
				MemberKubeConfig: filepath.Join(os.TempDir(), "member-kube-test.config"),
				MemberContext:    "member1",
			},
			validateArgs: []string{EstimatorResourceName},
			prep: func(client clientset.Interface, _ string) error {
				Addons["karmada-scheduler-estimator"] = &Addon{Name: EstimatorResourceName}

				// Create Karmada Kubeconfig secret.
				kubeConfigSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cmdinit.KubeConfigSecretAndMountName,
						Namespace: namespace,
					},
				}
				_, err := client.CoreV1().Secrets(namespace).Create(context.TODO(), kubeConfigSecret, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create secret %s in namespace %s, got an error: %v", kubeConfigSecret, namespace, err)
				}

				// Mock validateMemberConfig.
				validateMemberConfig = func(string, string) (*rest.Config, error) {
					return &rest.Config{}, nil
				}

				// Mock memberKubeClientBuilder.
				memberClient := fakeclientset.NewSimpleClientset()
				memberKubeClientBuilder = func(*rest.Config) clientset.Interface {
					return memberClient
				}

				// Create test node.
				newNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
					},
				}
				_, err = memberClient.CoreV1().Nodes().Create(context.TODO(), newNode, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create node %s, got an error: %v", newNode, err)
				}

				return nil
			},
			cleanup: func() error {
				Addons = map[string]*Addon{}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.enableOpts.KubeClientSet, test.enableOpts.MemberKubeConfig); err != nil {
				t.Fatalf("failed to prep for validating enable addon, got an error: %v", err)
			}
			err := test.enableOpts.Validate(test.validateArgs)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.cleanup(); err != nil {
				t.Errorf("failed to clean up the test environment, got an error: %v", err)
			}
		})
	}
}

func TestRunEnableAddon(t *testing.T) {
	tests := []struct {
		name         string
		enableOpts   *CommandAddonsEnableOption
		validateArgs []string
		prep         func() error
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "Run_EnableKarmadaDeschedulerAddon_FailedToEnableWithNetworkIssue",
			enableOpts:   &CommandAddonsEnableOption{},
			validateArgs: []string{DeschedulerResourceName},
			prep: func() error {
				Addons["karmada-descheduler"] = &Addon{
					Name: string(DeschedulerResourceName),
					Enable: func(*CommandAddonsEnableOption) error {
						return fmt.Errorf("got network issue while enabling %s addon", DeschedulerResourceName)
					},
				}
				return nil
			},
			wantErr: true,
			errMsg:  fmt.Sprintf("got network issue while enabling %s addon", DeschedulerResourceName),
		},
		{
			name:         "Run_EnableKarmadaDeschedulerAddon_Enabled",
			enableOpts:   &CommandAddonsEnableOption{},
			validateArgs: []string{DeschedulerResourceName},
			prep: func() error {
				Addons["karmada-descheduler"] = &Addon{
					Name:   string(DeschedulerResourceName),
					Enable: func(*CommandAddonsEnableOption) error { return nil },
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(); err != nil {
				t.Fatalf("failed to prep before enabling addon, got an error: %v", err)
			}
			err := test.enableOpts.Run(test.validateArgs)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
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
