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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"

	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

func TestNewNamespaceTask(t *testing.T) {
	tests := []struct {
		name     string
		wantTask workflow.Task
	}{
		{
			name: "NewNamespaceTask_IsCalled_ExpectedWorkflowTask",
			wantTask: workflow.Task{
				Name: "Namespace",
				Run:  runNamespace,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			namespaceTask := NewNamespaceTask()
			if err := util.DeepEqualTasks(namespaceTask, test.wantTask); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunNamespace(t *testing.T) {
	name, namespace := "karmada-demo", "test"
	tests := []struct {
		name    string
		runData workflow.RunData
		verify  func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunNamespace_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			verify:  func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "namespace task invoked with an invalid data struct",
		},
		{
			name: "RunNamespace_CreateNamespace_NamespaceIsCreated",
			runData: &TestInitData{
				Name:                  name,
				Namespace:             namespace,
				RemoteClientConnector: fakeclientset.NewSimpleClientset(),
			},
			verify: func(rd workflow.RunData) error {
				_, ok := rd.(*TestInitData)
				if !ok {
					return fmt.Errorf("unexpected err, rundata interface doesn't implement TestInitData")
				}

				client := rd.(*TestInitData).RemoteClient()
				actions := client.(*fakeclientset.Clientset).Actions()
				if len(actions) != 2 {
					return fmt.Errorf("expected 2 actions for getting and creating namespaces, but got %d", len(actions))
				}

				ns, err := client.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("failed to get %s namespace with err %w", namespace, err)
				}

				if ns.Name != namespace {
					return fmt.Errorf("expected %s namespace to be %s namespace", ns.Namespace, namespace)
				}

				return nil
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runNamespace(test.runData)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected %s error msg to contain %s", err.Error(), test.errMsg)
			}
			if err := test.verify(test.runData); err != nil {
				t.Errorf("failed to verify the namespace running task, got err: %v", err)
			}
		})
	}
}
