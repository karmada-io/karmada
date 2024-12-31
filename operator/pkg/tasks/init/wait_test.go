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
	"net/http"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	fakerest "k8s.io/client-go/rest/fake"

	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

func TestNewCheckApiserverHealthTask(t *testing.T) {
	tests := []struct {
		name     string
		wantTask *workflow.Task
	}{
		{
			name: "NewCheckApiserverHealthTask",
			wantTask: &workflow.Task{
				Name: "check-apiserver-health",
				Run:  runWaitApiserver,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checkAPIServerHealthTask := NewCheckApiserverHealthTask()
			if err := util.DeepEqualTasks(checkAPIServerHealthTask, *test.wantTask); err != nil {
				t.Errorf("unexpected error, got: %v", err)
			}
		})
	}
}

func TestRunWaitAPIServer(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		prep    func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunWaitAPIServer_InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "check-apiserver-health task invoked with an invalid data struct",
		},
		{
			name: "RunWaitAPIServer_WaitingForAPIServerHealthyStatus_Timeout",
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
				RemoteClientConnector: &apiclient.MockK8SRESTClient{
					RESTClientConnector: &fakerest.RESTClient{
						NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{}),
						Client: fakerest.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
							return nil, fmt.Errorf("unexpected error, endpoint %s does not exist", req.URL.Path)
						}),
					},
				},
				ControlplaneConfigREST: &rest.Config{},
			},
			prep: func(workflow.RunData) error {
				componentBeReadyTimeout, failureThreshold = time.Second, 1
				return nil
			},
			wantErr: true,
			errMsg:  "the karmada apiserver is unhealthy",
		},
		{
			name: "RunWaitAPIServer_WaitingForAPIServerHealthyStatus_APIServerIsHealthy",
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
				RemoteClientConnector: &apiclient.MockK8SRESTClient{
					RESTClientConnector: &fakerest.RESTClient{
						NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{}),
						Client: fakerest.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
							if req.URL.Path == "/healthz" {
								// Return a fake 200 OK response.
								return &http.Response{
									StatusCode: http.StatusOK,
									Body:       http.NoBody,
								}, nil
							}
							return nil, fmt.Errorf("unexpected error, endpoint %s does not exist", req.URL.Path)
						}),
					},
				},
				ControlplaneConfigREST: &rest.Config{},
			},
			prep:    func(workflow.RunData) error { return nil },
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData); err != nil {
				t.Errorf("failed to prep waiting for APIServer: %v", err)
			}
			err := runWaitApiserver(test.runData)
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

func TestNewWaitControlPlaneTask(t *testing.T) {
	tests := []struct {
		name     string
		wantTask *workflow.Task
	}{
		{
			name: "NewCheckApiserverHealthTask",
			wantTask: &workflow.Task{
				Name:        "wait-controlPlane",
				Run:         runWaitControlPlane,
				RunSubTasks: true,
				Tasks: []workflow.Task{
					newWaitControlPlaneSubTask("KubeControllerManager", kubeControllerManagerLabels),
					newWaitControlPlaneSubTask("KarmadaControllerManager", karmadaControllerManagerLabels),
					newWaitControlPlaneSubTask("KarmadaScheduler", karmadaSchedulerLabels),
					newWaitControlPlaneSubTask("KarmadaWebhook", karmadaWebhookLabels),
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			waitControlPlaneTask := NewWaitControlPlaneTask()
			if err := util.DeepEqualTasks(waitControlPlaneTask, *test.wantTask); err != nil {
				t.Errorf("unexpected error, got: %v", err)
			}
		})
	}
}

func TestRunWaitControlPlane(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		prep    func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunWaitControlPlane_InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "wait-controlPlane task invoked with an invalid data struct",
		},
		{
			name: "RunWaitControlPlane_ValidTypeAssertion_TypeAssertionSucceeded",
			runData: &TestInitData{
				Name:      "test-karmada",
				Namespace: "test",
			},
			prep:    func(workflow.RunData) error { return nil },
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData); err != nil {
				t.Errorf("failed to prep waiting control plane: %v", err)
			}
			err := runWaitControlPlane(test.runData)
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

func TestRunWaitControlPlaneSubTask(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		prep    func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunWaitControlPlaneSubTask_InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "wait-controlPlane task invoked with an invalid data struct",
		},
		{
			name: "RunWaitControlPlaneSubTask_WaitingForSomeKarmadaControllerManagerPods_Timeout",
			runData: &TestInitData{
				Name:                   "karmada-demo",
				Namespace:              "test",
				RemoteClientConnector:  fakeclientset.NewSimpleClientset(),
				ControlplaneConfigREST: &rest.Config{},
			},
			prep: func(workflow.RunData) error {
				componentBeReadyTimeout = time.Second
				return nil
			},
			wantErr: true,
			errMsg:  "waiting for karmada-demo-controller-manager to ready timeout",
		},
		{
			name: "RunWaitControlPlaneSubTask_WaitingForSomeKarmadaControllerManagerPods_KarmadaControllerManagerIsReady",
			runData: &TestInitData{
				Name:                   "karmada-demo",
				Namespace:              "test",
				RemoteClientConnector:  fakeclientset.NewSimpleClientset(),
				ControlplaneConfigREST: &rest.Config{},
			},
			prep: func(rd workflow.RunData) error {
				data := rd.(*TestInitData)
				if _, err := apiclient.CreatePods(data.RemoteClient(), data.GetNamespace(), util.KarmadaControllerManagerName(data.GetName()), 3, karmadaControllerManagerLabels, true); err != nil {
					return fmt.Errorf("failed to create pods: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData); err != nil {
				t.Errorf("failed to prep waiting for Karmada Controller Manager: %v", err)
			}
			karmadaControllerManagerName := getKarmadaControllerManagerName(test.runData)
			waitForKarmadaControllerManager := runWaitControlPlaneSubTask(karmadaControllerManagerName, karmadaControllerManagerLabels)
			err := waitForKarmadaControllerManager(test.runData)
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

// getKarmadaControllerManagerName returns the Karmada controller manager name from the provided RunData.
// It checks if RunData is *TestInitData, otherwise retrieves it from *MyTestData.
func getKarmadaControllerManagerName(rd workflow.RunData) string {
	data, ok := rd.(*TestInitData)
	if ok {
		return util.KarmadaControllerManagerName(data.GetName())
	}
	return rd.(*MyTestData).Data
}
