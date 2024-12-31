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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"

	"github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

func TestNewCleanupCertTask(t *testing.T) {
	tests := []struct {
		name     string
		wantTask workflow.Task
	}{
		{
			name: "NewCleanupCertTask_IsCalled_ExpectedWorkflowTask",
			wantTask: workflow.Task{
				Name:        "cleanup-cert",
				Run:         runCleanupCert,
				RunSubTasks: true,
				Tasks: []workflow.Task{
					newCleanupCertSubTask("karmada", util.KarmadaCertSecretName),
					newCleanupCertSubTask("webhook", util.WebhookCertSecretName),
					newCleanupCertSubTask("etcd", util.EtcdCertSecretName),
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			karmada := &v1alpha1.Karmada{
				Spec: v1alpha1.KarmadaSpec{
					Components: &v1alpha1.KarmadaComponents{
						Etcd: &v1alpha1.Etcd{
							Local: &v1alpha1.LocalEtcd{},
						},
					},
				},
			}
			cleanupCertTask := NewCleanupCertTask(karmada)
			if err := util.DeepEqualTasks(cleanupCertTask, test.wantTask); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunCleanupCert(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunCleanupCert_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "cleanup-cert task invoked with an invalid data struct",
		},
		{
			name: "RunCleanupCert_ValidTypeAssertion_TypeAssertionIsValid",
			runData: &TestDeInitData{
				name:      "karmada-demo",
				namespace: "test",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runCleanupCert(test.runData)
			if err == nil && test.wantErr {
				t.Errorf("expected error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected %s error msg to contain %s", err.Error(), test.errMsg)
			}
		})
	}
}

func TestRunCleanupCertSubTask(t *testing.T) {
	tests := []struct {
		name           string
		owner          string
		secretNameFunc util.Namefunc
		runData        workflow.RunData
		prep           func(workflow.RunData) error
		verify         func(workflow.RunData) error
		wantErr        bool
		errMsg         string
	}{
		{
			name:    "RunCleanupCertSubTask_InvalidTypeAssertion_TypeAssertionIsInvalid",
			owner:   "karmada",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData) error { return nil },
			verify:  func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  fmt.Sprintf("cleanup-%s-cert task invoked with an invalid data struct", "karmada"),
		},
		{
			name:           "RunCleanupCertSubTask_WithKarmadaCertSecret_CertsHaveBeenCleanedUp",
			owner:          "karmada",
			secretNameFunc: util.KarmadaCertSecretName,
			prep:           prepareKarmadaCertSecret,
			verify:         verifyKarmadaCertSecretDeleted,
			runData: &TestDeInitData{
				name:         "karmada-demo",
				namespace:    "test",
				remoteClient: fakeclientset.NewSimpleClientset(),
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData); err != nil {
				t.Errorf("failed to prep the run cleanup cert subtask: %v", err)
			}
			cleanupCertSubTask := runCleanupCertSubTask(test.owner, test.secretNameFunc)
			err := cleanupCertSubTask(test.runData)
			if err == nil && test.wantErr {
				t.Errorf("expected error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected %s error msg to contain %s", err.Error(), test.errMsg)
			}
			if err := test.verify(test.runData); err != nil {
				t.Errorf("failed to verify run cleanup cert subtask: %v", err)
			}
		})
	}
}

func prepareKarmadaCertSecret(rd workflow.RunData) error {
	data := rd.(*TestDeInitData)
	secretName := util.KarmadaCertSecretName(data.name)
	_, err := data.remoteClient.CoreV1().Secrets(data.namespace).Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: data.namespace,
			Labels:    constants.KarmadaOperatorLabel,
		},
		Type: corev1.SecretTypeOpaque,
	}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret: %v", err)
	}
	return nil
}

func verifyKarmadaCertSecretDeleted(rd workflow.RunData) error {
	data := rd.(*TestDeInitData)
	secretName := util.KarmadaCertSecretName(data.name)
	secret, err := data.remoteClient.CoreV1().Secrets(data.namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err == nil {
		if val, exists := secret.Labels[constants.KarmadaOperatorLabelKeyName]; exists && val != "" {
			return fmt.Errorf("expected secret %s to be deleted, but it still exists with label %s=%s", secretName, constants.KarmadaOperatorLabelKeyName, val)
		}
		return fmt.Errorf("expected secret %s to be deleted, but it still exists", secretName)
	}
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
