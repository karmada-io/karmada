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
	"k8s.io/utils/ptr"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

func TestNewEtcd(t *testing.T) {
	tests := []struct {
		name     string
		wantTask workflow.Task
	}{
		{
			name: "NewEtcdTask_IsCalled_ExpectedWorkflowTask",
			wantTask: workflow.Task{
				Name:        "Etcd",
				Run:         runEtcd,
				RunSubTasks: true,
				Tasks: []workflow.Task{
					{
						Name: "deploy-etcd",
						Run:  runDeployEtcd,
					},
					{
						Name: "wait-etcd",
						Run:  runWaitEtcd,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			etcdTask := NewEtcdTask()
			if err := util.DeepEqualTasks(etcdTask, test.wantTask); err != nil {
				t.Errorf("unexpected error, got %v", err)
			}
		})
	}
}

func TestRunEtcd(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunEtcd_InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "etcd task invoked with an invalid data struct",
		},
		{
			name: "RunEtcd_ValidTypeAssertion_TypeAssertionSucceeded",
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runEtcd(test.runData)
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

func TestRunDeployEtcd(t *testing.T) {
	var replicas int32 = 2
	image, imageTag := "registry.k8s.io/etcd", "latest"
	name, namespace := "karmada-demo", "test"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}

	cfg := &operatorv1alpha1.LocalEtcd{
		CommonSettings: operatorv1alpha1.CommonSettings{
			Image: operatorv1alpha1.Image{
				ImageRepository: image,
				ImageTag:        imageTag,
			},
			Replicas:        ptr.To[int32](replicas),
			Annotations:     annotations,
			Labels:          labels,
			Resources:       corev1.ResourceRequirements{},
			ImagePullPolicy: imagePullPolicy,
		},
	}

	tests := []struct {
		name    string
		runData workflow.RunData
		verify  func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunDeployEtcd_InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			verify:  func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "deploy-etcd task invoked with an invalid data struct",
		},
		{
			name: "RunDeployEtcd_WithExternalEtcd_ExternalEtcdJobInstallationSkipped",
			runData: &TestInitData{
				Name:      name,
				Namespace: namespace,
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					Etcd: &operatorv1alpha1.Etcd{
						External: &operatorv1alpha1.ExternalEtcd{},
					},
				},
			},
			verify:  func(workflow.RunData) error { return nil },
			wantErr: false,
		},
		{
			name: "RunDeployEtcd_WithoutLocalEtcd_UnexpectedEtcdLocalConfig",
			runData: &TestInitData{
				Name:      name,
				Namespace: namespace,
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					Etcd: &operatorv1alpha1.Etcd{},
				},
			},
			verify:  func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "unexpected empty etcd local configuration",
		},
		{
			name: "RunDeployEtcd_InstallLocalEtcd_LocalEtcdInstalled",
			runData: &TestInitData{
				Name:      name,
				Namespace: namespace,
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					Etcd: &operatorv1alpha1.Etcd{
						Local: cfg,
					},
				},
				RemoteClientConnector: fakeclientset.NewSimpleClientset(),
			},
			verify: func(runData workflow.RunData) error {
				data := runData.(*TestInitData)
				// Verify that the component statefulset has been created in the given namespace.
				client := data.RemoteClient()
				componentName := util.KarmadaEtcdName(name)
				_, err := client.AppsV1().StatefulSets(namespace).Get(context.TODO(), componentName, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("failed to find statefulset %s in namespace %s: %w", componentName, namespace, err)
				}

				// Verify that the component service has been created in the given namespace.
				_, err = client.CoreV1().Services(namespace).Get(context.TODO(), componentName, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("failed to find service %s in namespace %s: %w", componentName, namespace, err)
				}

				return nil
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runDeployEtcd(test.runData)
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
				t.Errorf("failed to verify run data: %v", err)
			}
		})
	}
}

func TestRunWaitEtcd(t *testing.T) {
	var replicas int32 = 2
	name, namespace := "karmada-demo", "test"
	tests := []struct {
		name    string
		runData workflow.RunData
		prep    func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunWaitEtcd_InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "wait-etcd task invoked with an invalid data struct",
		},
		{
			name: "RunWaitEtcd_WaitingForEtcdPods_EtcdPodsAreReady",
			runData: &TestInitData{
				Name:                  name,
				Namespace:             namespace,
				RemoteClientConnector: fakeclientset.NewSimpleClientset(),
			},
			prep: func(rd workflow.RunData) error {
				data := rd.(*TestInitData)
				if _, err := apiclient.CreatePods(data.RemoteClient(), namespace, util.KarmadaEtcdName(name), replicas, etcdLabels, true); err != nil {
					return fmt.Errorf("failed to create pods, got error: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.prep(test.runData)
			if err != nil {
				t.Errorf("failed to prep before waiting for etcd: %v", err)
			}
			err = runWaitEtcd(test.runData)
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
