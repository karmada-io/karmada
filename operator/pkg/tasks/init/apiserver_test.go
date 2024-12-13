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
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

func TestNewAPIServerTask(t *testing.T) {
	tests := []struct {
		name     string
		wantTask workflow.Task
	}{
		{
			name: "NewKarmadaApiserverTask_IsCalled_ExpectedWorkflowTask",
			wantTask: workflow.Task{
				Name:        "apiserver",
				Run:         runApiserver,
				RunSubTasks: true,
				Tasks: []workflow.Task{
					{
						Name: constants.KarmadaAPIserverComponent,
						Run:  runKarmadaAPIServer,
					},
					{
						Name: fmt.Sprintf("%s-%s", "wait", constants.KarmadaAPIserverComponent),
						Run:  runWaitKarmadaAPIServer,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			karmadaAPIServerTask := NewKarmadaApiserverTask()
			err := util.DeepEqualTasks(karmadaAPIServerTask, test.wantTask)
			if err != nil {
				t.Errorf("unexpected error, got %v", err)
			}
		})
	}
}

func TestNewKarmadaAggregatedApiserverTask(t *testing.T) {
	tests := []struct {
		name     string
		wantTask workflow.Task
	}{
		{
			name: "NewKarmadaAggregatedApiserverTask_IsCalled_ExpectedWorkflowTask",
			wantTask: workflow.Task{
				Name:        "aggregated-apiserver",
				Run:         runAggregatedApiserver,
				RunSubTasks: true,
				Tasks: []workflow.Task{
					{
						Name: constants.KarmadaAggregatedAPIServerComponent,
						Run:  runKarmadaAggregatedAPIServer,
					},
					{
						Name: fmt.Sprintf("%s-%s", "wait", constants.KarmadaAggregatedAPIServerComponent),
						Run:  runWaitKarmadaAggregatedAPIServer,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			karmadaAggregatedAPIServerTask := NewKarmadaAggregatedApiserverTask()
			err := util.DeepEqualTasks(karmadaAggregatedAPIServerTask, test.wantTask)
			if err != nil {
				t.Errorf("unexpected error, got %v", err)
			}
		})
	}
}

func TestRunAggregatedAPIServer(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunAggregatedApiserver_InvalidTypeAssertion_TypeAssertionFailed",
			runData: MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "aggregated-apiserver task invoked with an invalid data struct",
		},
		{
			name:    "RunAggregatedApiserver_ValidTypeAssertion_TypeAssertionSucceeded",
			runData: &TestInitData{},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runAggregatedApiserver(test.runData)
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

func TestRunAPIServer(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunAPIServer_InvalidTypeAssertion_TypeAssertionFailed",
			runData: MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "apiserver task invoked with an invalid data struct",
		},
		{
			name:    "RunAPIServer_ValidTypeAssertion_TypeAssertionSucceeded",
			runData: &TestInitData{},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runApiserver(test.runData)
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

func TestRunKarmadaAPIServer(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunKarmadaAPIServer_InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "KarmadaApiserver task invoked with an invalid data struct",
		},
		{
			name: "RunKarmadaAPIServer_NilKarmadaAPIServer_RunIsCompletedWithoutErrors",
			runData: &TestInitData{
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					Etcd: &operatorv1alpha1.Etcd{
						Local: &operatorv1alpha1.LocalEtcd{},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "RunKarmadaAPIServer_InitializeKarmadaAPIServer_KarmadaAPIServerEnsured",
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
						CommonSettings: operatorv1alpha1.CommonSettings{
							Image:           operatorv1alpha1.Image{ImageTag: "karmada-apiserver-image"},
							Replicas:        ptr.To[int32](2),
							Resources:       corev1.ResourceRequirements{},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						ServiceSubnet: ptr.To("10.96.0.0/12"),
					},
					Etcd: &operatorv1alpha1.Etcd{
						Local: &operatorv1alpha1.LocalEtcd{},
					},
				},
				RemoteClientConnector: fakeclientset.NewSimpleClientset(),
				FeatureGatesOptions: map[string]bool{
					"Feature1": true,
				},
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runKarmadaAPIServer(test.runData)
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

func TestRunWaitKarmadaAPIServer(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		prep    func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunWaitKarmadaAPIServer_InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "wait-KarmadaAPIServer task invoked with an invalid data struct",
		},
		{
			name: "RunWaitKarmadaAPIServer_TimeoutWaitingForSomeKarmadaAPIServersPods_Timeout",
			runData: &TestInitData{
				Name:                   "karmada-demo",
				Namespace:              "test",
				RemoteClientConnector:  fakeclientset.NewSimpleClientset(),
				ControlplaneConfigREST: &rest.Config{},
				FeatureGatesOptions: map[string]bool{
					"Feature1": true,
				},
			},
			prep: func(workflow.RunData) error {
				componentBeReadyTimeout = time.Second
				return nil
			},
			wantErr: true,
			errMsg:  "waiting for karmada-apiserver to ready timeout",
		},
		{
			name: "RunWaitKarmadaAPIServer_WaitingForSomeKarmadaAPIServersPods_KarmadaAPIServerIsReady",
			runData: &TestInitData{
				Name:                   "karmada-demo",
				Namespace:              "test",
				RemoteClientConnector:  fakeclientset.NewSimpleClientset(),
				ControlplaneConfigREST: &rest.Config{},
				FeatureGatesOptions: map[string]bool{
					"Feature1": true,
				},
			},
			prep: func(rd workflow.RunData) error {
				data := rd.(*TestInitData)
				_, err := apiclient.CreatePods(data.RemoteClient(), data.Namespace, util.KarmadaAPIServerName(data.Name), 2, karmadaApiserverLabels, true)
				return err
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData); err != nil {
				t.Errorf("failed to prep waiting for Karmada APIServer: %v", err)
			}
			err := runWaitKarmadaAPIServer(test.runData)
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

func TestRunKarmadaAggregatedAPIServer(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		prep    func() error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunKarmadaAggregatedAPIServer_InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			prep:    func() error { return nil },
			wantErr: true,
			errMsg:  "KarmadaAggregatedAPIServer task invoked with an invalid data struct",
		},
		{
			name: "RunKarmadaAggregatedAPIServer_NilKarmadaAggregatedAPIServer_RunIsCompletedWithoutErrors",
			runData: &TestInitData{
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					Etcd: &operatorv1alpha1.Etcd{
						Local: &operatorv1alpha1.LocalEtcd{},
					},
				},
			},
			prep:    func() error { return nil },
			wantErr: false,
		},
		{
			name: "RunKarmadaAggregatedAPIServer_InitializeKarmadaAggregatedAPIServer_KarmadaAggregatedAPIServerEnsured",
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					KarmadaAggregatedAPIServer: &operatorv1alpha1.KarmadaAggregatedAPIServer{
						CommonSettings: operatorv1alpha1.CommonSettings{
							Image:           operatorv1alpha1.Image{ImageTag: "karmada-aggregated-apiserver-image"},
							Replicas:        ptr.To[int32](2),
							Resources:       corev1.ResourceRequirements{},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					},
					Etcd: &operatorv1alpha1.Etcd{
						Local: &operatorv1alpha1.LocalEtcd{},
					},
				},
				RemoteClientConnector: fakeclientset.NewSimpleClientset(),
				FeatureGatesOptions: map[string]bool{
					"Feature1": true,
				},
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runKarmadaAggregatedAPIServer(test.runData)
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

func TestRunWaitKarmadaAggregatedAPIServer(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		prep    func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunWaitKarmadaAggregatedAPIServer_InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			prep: func(workflow.RunData) error {
				return nil
			},
			wantErr: true,
			errMsg:  "wait-KarmadaAggregatedAPIServer task invoked with an invalid data struct",
		},
		{
			name: "RunWaitKarmadaAggregatedAPIServer_TimeoutWaitingForSomeKarmadaAggregatedAPIServersPods_Timeout",
			runData: &TestInitData{
				Name:                   "karmada-demo",
				Namespace:              "test",
				RemoteClientConnector:  fakeclientset.NewSimpleClientset(),
				ControlplaneConfigREST: &rest.Config{},
				FeatureGatesOptions: map[string]bool{
					"Feature1": true,
				},
			},
			prep: func(workflow.RunData) error {
				componentBeReadyTimeout = time.Second
				return nil
			},
			wantErr: true,
			errMsg:  "waiting for karmada-aggregated-apiserver to ready timeout",
		},
		{
			name: "RunWaitKarmadaAggregatedAPIServer_WaitingForSomeKarmadaAggregatedAPIServersPods_KarmadaAggregatedAPIServerIsReady",
			runData: &TestInitData{
				Name:                   "karmada-demo",
				Namespace:              "test",
				RemoteClientConnector:  fakeclientset.NewSimpleClientset(),
				ControlplaneConfigREST: &rest.Config{},
				FeatureGatesOptions: map[string]bool{
					"Feature1": true,
				},
			},
			prep: func(rd workflow.RunData) error {
				data := rd.(*TestInitData)
				_, err := apiclient.CreatePods(data.RemoteClient(), data.Namespace, util.KarmadaAggregatedAPIServerName(data.Name), 2, karmadaAggregatedAPIServerLabels, true)
				return err
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData); err != nil {
				t.Errorf("failed to prep waiting for Karmada Aggregated APIServer: %v", err)
			}
			err := runWaitKarmadaAggregatedAPIServer(test.runData)
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
