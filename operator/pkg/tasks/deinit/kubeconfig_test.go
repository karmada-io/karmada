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

package tasks

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
	"github.com/karmada-io/karmada/test/helper"
)

func TestNewCleanupKubeconfigTask(t *testing.T) {
	tests := []struct {
		name     string
		wantTask *workflow.Task
	}{
		{
			name: "NewCleanupKubeconfigTask",
			wantTask: &workflow.Task{
				Name: "cleanup-kubeconfig",
				Run:  runCleanupKubeconfig,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cleanupKubeconfigTask := NewCleanupKubeconfigTask()
			if err := util.DeepEqualTasks(cleanupKubeconfigTask, *test.wantTask); err != nil {
				t.Errorf("unexpected error, got: %v", err)
			}
		})
	}
}

func TestRunCleanupKubeconfig(t *testing.T) {
	name, namespace := "karmada-demo", "test"
	tests := []struct {
		name    string
		runData workflow.RunData
		secret  *corev1.Secret
		prep    func(workflow.RunData, *corev1.Secret) error
		verify  func(rd workflow.RunData, s *corev1.Secret) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunCleanupKubeconfig_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData, *corev1.Secret) error { return nil },
			verify:  func(workflow.RunData, *corev1.Secret) error { return nil },
			wantErr: true,
			errMsg:  "cleanup-karmada-config task invoked with an invalid data struct",
		},
		{
			name: "RunCleanupKubeconfig_DeleteSecretWithKarmadaOperatorLabel_SecretDeleted",
			runData: &TestDeInitData{
				name:         name,
				namespace:    namespace,
				remoteClient: fakeclientset.NewSimpleClientset(),
			},
			secret: helper.NewSecret(namespace, util.AdminKarmadaConfigSecretName(name), map[string][]byte{}),
			prep: func(rd workflow.RunData, s *corev1.Secret) error {
				data := rd.(*TestDeInitData)
				s.Labels = constants.KarmadaOperatorLabel
				if err := apiclient.CreateOrUpdateSecret(data.RemoteClient(), s); err != nil {
					return fmt.Errorf("failed to create secret, got err: %v", err)
				}
				return nil
			},
			verify: func(rd workflow.RunData, s *corev1.Secret) error {
				data := rd.(*TestDeInitData)
				_, err := data.RemoteClient().CoreV1().Secrets(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("expected secret to be deleted, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData, test.secret); err != nil {
				t.Errorf("failed to prep the test env before cleaning the kubeconfig, got error: %v", err)
			}
			err := runCleanupKubeconfig(test.runData)
			if err == nil && test.wantErr {
				t.Error("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.runData, test.secret); err != nil {
				t.Errorf("failed to verify the deletion of secret, got error: %v", err)
			}
		})
	}
}
