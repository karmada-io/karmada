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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/helper"
)

func TestRunRemoveComponent(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunRemoveComponent_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "remove-component task invoked with an invalid data struct",
		},
		{
			name: "RunRemoveComponent_ValidTypeAssertion_TypeAssertionIsValid",
			runData: &TestDeInitData{
				name:      "karmada-demo",
				namespace: "test",
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runRemoveComponent(test.runData)
			if err == nil && test.wantErr {
				t.Error("expected an error, but got none")
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

func TestRunRemoveComponentSubTask(t *testing.T) {
	name, namespace := "karmada-demo", "test"
	tests := []struct {
		name             string
		component        string
		runData          workflow.RunData
		workloadNameFunc util.Namefunc
		deployment       *appsv1.Deployment
		service          *corev1.Service
		prep             func(workflow.RunData, *appsv1.Deployment, *corev1.Service) error
		verify           func(workflow.RunData, *appsv1.Deployment, *corev1.Service) error
		hasService       bool
		wantErr          bool
		errMsg           string
	}{
		{
			name:      "RunRemoveComponentSubTask_InvalidTypeAssertion_TypeAssertionIsInvalid",
			component: names.KarmadaControllerManagerComponentName,
			runData:   &MyTestData{Data: "test"},
			prep:      func(workflow.RunData, *appsv1.Deployment, *corev1.Service) error { return nil },
			verify:    func(workflow.RunData, *appsv1.Deployment, *corev1.Service) error { return nil },
			wantErr:   true,
			errMsg:    fmt.Sprintf("remove-%s task invoked with an invalid data struct", names.KarmadaControllerManagerComponentName),
		},
		{
			name:             "RunRemoveComponentSubTask_DeleteKarmadaControllerManagerDeploymentWithSecret_DeploymentAndSecretDeleted",
			component:        names.KarmadaControllerManagerComponentName,
			workloadNameFunc: util.KarmadaControllerManagerName,
			deployment:       helper.NewDeployment(namespace, util.KarmadaControllerManagerName(name)),
			service:          helper.NewService(namespace, util.KarmadaControllerManagerName(name), corev1.ServiceTypeClusterIP),
			prep: func(rd workflow.RunData, d *appsv1.Deployment, s *corev1.Service) error {
				data := rd.(*TestDeInitData)
				client := data.RemoteClient()

				// Create Karmada Controller Manager deployment with given labels.
				d.Labels = constants.KarmadaOperatorLabel
				if err := apiclient.CreateOrUpdateDeployment(client, d); err != nil {
					return fmt.Errorf("failed to create deployment, got: %v", err)
				}

				// Create Karmada Controller Manager service with given labels.
				s.Labels = constants.KarmadaOperatorLabel
				if err := apiclient.CreateOrUpdateService(client, s); err != nil {
					return fmt.Errorf("failed to create service, got: %v", err)
				}

				return nil
			},
			verify: func(rd workflow.RunData, d *appsv1.Deployment, s *corev1.Service) error {
				data := rd.(*TestDeInitData)
				client := data.RemoteClient()

				// Verify that the Karmada Controller Manager deployment is deleted.
				_, err := client.AppsV1().Deployments(d.GetNamespace()).Get(context.TODO(), d.GetName(), metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("expected deployment to be deleted, but got err: %v", err)
				}

				// Verify that the Karmada Controller Manager service is deleted.
				_, err = client.CoreV1().Services(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("expected service to be deleted, but got err: %v", err)
				}

				return nil
			},
			runData: &TestDeInitData{
				name:         name,
				namespace:    namespace,
				remoteClient: fakeclientset.NewSimpleClientset(),
			},
			hasService: true,
			wantErr:    false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData, test.deployment, test.service); err != nil {
				t.Errorf("failed to prep before removing component subtask %s, got: %v", test.component, err)
			}
			runRemoveComponentSubTask := runRemoveComponentSubTask(test.component, test.workloadNameFunc, true)
			err := runRemoveComponentSubTask(test.runData)
			if err == nil && test.wantErr {
				t.Error("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.runData, test.deployment, test.service); err != nil {
				t.Errorf("failed to verify the deletion of deployments and services for %s component, got: %v", test.component, err)
			}
		})
	}
}

func TestRunRemoveEtcd(t *testing.T) {
	name, namespace := "karmada-demo", "test"
	tests := []struct {
		name        string
		runData     workflow.RunData
		statefulset *appsv1.StatefulSet
		service     *corev1.Service
		prep        func(workflow.RunData, *appsv1.StatefulSet, *corev1.Service) error
		verify      func(workflow.RunData, *appsv1.StatefulSet, *corev1.Service) error
		wantErr     bool
		errMsg      string
	}{
		{
			name:    "RunRemoveEtcd_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData, *appsv1.StatefulSet, *corev1.Service) error { return nil },
			verify:  func(workflow.RunData, *appsv1.StatefulSet, *corev1.Service) error { return nil },
			wantErr: true,
			errMsg:  "remove-etcd task invoked with an invalid data struct",
		},
		{
			name:        "RunRemoveEtcd_DeleteEtcdStatefulSetWithService_StatefulSetAndServiceDeleted",
			statefulset: helper.NewStatefulSet(namespace, util.KarmadaEtcdName(name)),
			service:     helper.NewService(namespace, util.KarmadaEtcdName(name), corev1.ServiceTypeClusterIP),
			prep: func(rd workflow.RunData, staetfulset *appsv1.StatefulSet, service *corev1.Service) error {
				data := rd.(*TestDeInitData)
				client := data.RemoteClient()

				// Create Etcd statefulset with given labels.
				staetfulset.Labels = constants.KarmadaOperatorLabel
				if err := apiclient.CreateOrUpdateStatefulSet(client, staetfulset); err != nil {
					return fmt.Errorf("failed to create statefulset, got: %v", err)
				}

				// Create Etcd service with given labels.
				service.Labels = constants.KarmadaOperatorLabel
				if err := apiclient.CreateOrUpdateService(client, service); err != nil {
					return fmt.Errorf("failed to create service, got: %v", err)
				}

				return nil
			},
			verify: func(rd workflow.RunData, statefulset *appsv1.StatefulSet, service *corev1.Service) error {
				data := rd.(*TestDeInitData)
				client := data.RemoteClient()

				// Verify that the Etcd statefulset is deleted.
				_, err := client.AppsV1().StatefulSets(statefulset.GetNamespace()).Get(context.TODO(), statefulset.GetName(), metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("expected statefulset to be deleted, but got err: %v", err)
				}

				// Verify that the Etcd service is deleted.
				_, err = client.CoreV1().Services(service.GetNamespace()).Get(context.TODO(), service.GetName(), metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("expected service to be deleted, but got err: %v", err)
				}

				return nil
			},
			runData: &TestDeInitData{
				name:         name,
				namespace:    namespace,
				remoteClient: fakeclientset.NewSimpleClientset(),
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData, test.statefulset, test.service); err != nil {
				t.Errorf("failed to prep before removing etcd, got: %v", err)
			}
			err := runRemoveEtcd(test.runData)
			if err == nil && test.wantErr {
				t.Error("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.runData, test.statefulset, test.service); err != nil {
				t.Errorf("failed to verify the deletion of statefulsets and services for etcd component, got: %v", err)
			}
		})
	}
}
