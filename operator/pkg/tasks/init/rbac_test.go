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

func TestNewRBACTask(t *testing.T) {
	tests := []struct {
		name     string
		wantTask workflow.Task
	}{
		{
			name: "NewRBACTask_IsCalled_ExpectedWorkflowTask",
			wantTask: workflow.Task{
				Name: "rbac",
				Run:  runRBAC,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rbacTask := NewRBACTask()
			if err := util.DeepEqualTasks(rbacTask, test.wantTask); err != nil {
				t.Errorf("unexpected err: %v", err)
			}
		})
	}
}

func TestRunRBAC(t *testing.T) {
	name, namespace := "karmada-demo", "test"
	tests := []struct {
		name    string
		runData workflow.RunData
		verify  func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunRBAC_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			verify:  func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "RBAC task invoked with an invalid data struct",
		},
		{
			name: "RunRBAC_InstallKarmadaRBAC_KarmadaRBACInstalled",
			runData: &TestInitData{
				Name:                   name,
				Namespace:              namespace,
				KarmadaClientConnector: fakeclientset.NewSimpleClientset(),
			},
			verify: func(rd workflow.RunData) error {
				_, ok := rd.(*TestInitData)
				if !ok {
					return fmt.Errorf("unexpected err, rundata interface doesn't implement TestInitData")
				}

				client := rd.(*TestInitData).KarmadaClient()
				actions := client.(*fakeclientset.Clientset).Actions()
				if len(actions) != 4 {
					return fmt.Errorf("expected 4 actions, but got %d", len(actions))
				}

				rolesToCheck := []string{"cluster-proxy-admin", "karmada-edit", "karmada-view"}
				for _, role := range rolesToCheck {
					_, err := client.RbacV1().ClusterRoles().Get(context.TODO(), role, metav1.GetOptions{})
					if err != nil {
						return fmt.Errorf("failed to get ClusterRole: %s: %v", role, err)
					}
				}

				return nil
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runRBAC(test.runData)
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
