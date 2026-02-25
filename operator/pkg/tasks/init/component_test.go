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
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

func TestNewComponentTask(t *testing.T) {
	tests := []struct {
		name     string
		wantTask workflow.Task
	}{
		{
			name: "NewComponentTask_IsCalled_ExpectedWorkflowTask",
			wantTask: workflow.Task{
				Name:        "components",
				Run:         runComponents,
				RunSubTasks: true,
				Tasks: []workflow.Task{
					newComponentSubTask(constants.KubeControllerManagerComponent),
					newComponentSubTask(constants.KarmadaControllerManagerComponent),
					newComponentSubTask(constants.KarmadaSchedulerComponent),
					{
						Name: constants.KarmadaWebhookComponent,
						Run:  runKarmadaWebhook,
					},
					newComponentSubTask(constants.KarmadaDeschedulerComponent),
					newKarmadaMetricsAdapterSubTask(),
					newKarmadaSearchSubTask(),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			componentTask := NewComponentTask()
			err := util.DeepEqualTasks(componentTask, test.wantTask)
			if err != nil {
				t.Errorf("unexpected error, got %v", err)
			}
		})
	}
}

func TestRunComponents(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunComponents_InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "components task invoked with an invalid data struct",
		},
		{
			name: "RunComponents_ValidTypeAssertion_TypeAssertionSuceeded",
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runComponents(test.runData)
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

func TestNewComponentSubTask(t *testing.T) {
	component := constants.KubeControllerManagerComponent
	tests := []struct {
		name      string
		component string
		wantTask  workflow.Task
	}{
		{
			name: "NewComponentSubTask_IsCalled_ExpectedWorkflowTask",
			wantTask: workflow.Task{
				Name: component,
				Run:  runComponentSubTask(component),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			componentTask := newComponentSubTask(component)
			err := util.DeepEqualTasks(componentTask, test.wantTask)
			if err != nil {
				t.Errorf("unexpected error, got %v", err)
			}
		})
	}
}

func TestRunComponentSubTask(t *testing.T) {
	var replicas int32 = 2
	name, namespace := "karmada-demo", "test"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	extraArgs := map[string]string{"cmd1": "arg1", "cmd2": "arg2"}

	components := &operatorv1alpha1.KarmadaComponents{
		KubeControllerManager: &operatorv1alpha1.KubeControllerManager{
			CommonSettings: operatorv1alpha1.CommonSettings{
				Image: operatorv1alpha1.Image{
					ImageRepository: "registry.k8s.io/kube-controller-manager",
					ImageTag:        "latest",
				},
				Replicas:        ptr.To(replicas),
				Annotations:     annotations,
				Labels:          labels,
				Resources:       corev1.ResourceRequirements{},
				ImagePullPolicy: imagePullPolicy,
			},
			ExtraArgs: extraArgs,
		},
		KarmadaControllerManager: &operatorv1alpha1.KarmadaControllerManager{
			CommonSettings: operatorv1alpha1.CommonSettings{
				Image: operatorv1alpha1.Image{
					ImageRepository: "docker.io/karmada/karmada-controller-manager",
					ImageTag:        "latest",
				},
				Replicas:        ptr.To(replicas),
				Annotations:     annotations,
				Labels:          labels,
				ImagePullPolicy: imagePullPolicy,
			},
			ExtraArgs: extraArgs,
		},
		KarmadaScheduler: &operatorv1alpha1.KarmadaScheduler{
			CommonSettings: operatorv1alpha1.CommonSettings{
				Image: operatorv1alpha1.Image{
					ImageRepository: "docker.io/karmada/karmada-scheduler",
					ImageTag:        "latest",
				},
				Replicas:        ptr.To(replicas),
				Annotations:     annotations,
				Labels:          labels,
				Resources:       corev1.ResourceRequirements{},
				ImagePullPolicy: imagePullPolicy,
			},
			ExtraArgs: extraArgs,
		},
		KarmadaDescheduler: &operatorv1alpha1.KarmadaDescheduler{
			CommonSettings: operatorv1alpha1.CommonSettings{
				Image: operatorv1alpha1.Image{
					ImageRepository: "docker.io/karmada/karmada-descheduler",
					ImageTag:        "latest",
				},
				Replicas:        ptr.To(replicas),
				Annotations:     annotations,
				Labels:          labels,
				Resources:       corev1.ResourceRequirements{},
				ImagePullPolicy: imagePullPolicy,
			},
			ExtraArgs: extraArgs,
		},
	}

	tests := []struct {
		name      string
		component string
		runData   workflow.RunData
		verify    func(workflow.RunData) error
		wantErr   bool
		errMsg    string
	}{
		{
			name:    "RunComponentSubTask_InvalidTypeAssertion_TypeAssertionFailed",
			runData: MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "components task invoked with an invalid data struct",
			verify:  func(workflow.RunData) error { return nil },
		},
		{
			name:      "RunComponentSubTask_NonExistentComponent_SkipInstallingThatComponent",
			component: "non-existent-component",
			runData: &TestInitData{
				Name:            name,
				Namespace:       namespace,
				ComponentsUnits: components,
			},
			wantErr: false,
			verify:  func(workflow.RunData) error { return nil },
		},
		{
			name:      "RunComponentSubTask_InstallKubeControllerManagerComponent_KubeControllerManagerComponentInstalled",
			component: constants.KubeControllerManagerComponent,
			runData: &TestInitData{
				Name:                  name,
				Namespace:             namespace,
				ComponentsUnits:       components,
				RemoteClientConnector: fakeclientset.NewSimpleClientset(),
			},
			verify: func(runData workflow.RunData) error {
				data := runData.(*TestInitData)
				client := data.RemoteClient()
				actions := client.(*fakeclientset.Clientset).Actions()
				if len(actions) != 1 {
					return fmt.Errorf("expected 1 action, but got %d", len(actions))
				}

				// Verify that the component deployment has been created in the given namespace.
				deploymentName := util.KubeControllerManagerName(name)
				_, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("failed to find deployment %s in namespace %s: %w", deploymentName, namespace, err)
				}

				return nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runComponentSubTask := runComponentSubTask(test.component)
			err := runComponentSubTask(test.runData)
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
				t.Errorf("failed to verify the existence of %s deployment, got err: %v", test.component, err)
			}
		})
	}
}

func TestRunKarmadaWebook(t *testing.T) {
	var replicas int32 = 2
	image, imageTag := "docker.io/karmada/karmada-webhook", "latest"
	name := "karmada-demo"
	namespace := "test"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	extraArgs := map[string]string{"cmd1": "arg1", "cmd2": "arg2"}

	webhookComponent := &operatorv1alpha1.KarmadaWebhook{
		CommonSettings: operatorv1alpha1.CommonSettings{
			Image: operatorv1alpha1.Image{
				ImageRepository: image,
				ImageTag:        imageTag,
			},
			Replicas:        ptr.To(replicas),
			Annotations:     annotations,
			Labels:          labels,
			Resources:       corev1.ResourceRequirements{},
			ImagePullPolicy: imagePullPolicy,
		},
		ExtraArgs: extraArgs,
	}

	tests := []struct {
		name    string
		runData workflow.RunData
		prep    func(workflow.RunData) error
		verify  func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunKarmadaWebook_InvalidTypeAssertion_TypeAssertionFailed",
			runData: MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "KarmadaWebhook task invoked with an invalid data struct",
			verify:  func(workflow.RunData) error { return nil },
		},
		{
			name: "RunKarmadaWebook_NilKarmadaWebhook_KarmadaWebhookInstallationSkipped",
			runData: &TestInitData{
				Name:      name,
				Namespace: namespace,
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					KarmadaWebhook: nil,
				},
			},
			wantErr: true,
			errMsg:  "skip install karmada webhook",
			verify:  func(workflow.RunData) error { return nil },
		},
		{
			name: "RunKarmadaWebook_InstallKarmadaWebhook_KarmadaWebhookInstalled",
			runData: &TestInitData{
				Name:      name,
				Namespace: namespace,
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					KarmadaWebhook: webhookComponent,
				},
				RemoteClientConnector: fakeclientset.NewSimpleClientset(),
			},
			verify: func(runData workflow.RunData) error {
				data := runData.(*TestInitData)
				client := data.RemoteClient()
				actions := client.(*fakeclientset.Clientset).Actions()
				if len(actions) != 2 {
					return fmt.Errorf("expected 2 actions, but got %d", len(actions))
				}

				// Verify that the component deployment has been created in the given namespace.
				componentName := util.KarmadaWebhookName(name)
				_, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), componentName, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("failed to find deployment %s in namespace %s: %w", componentName, namespace, err)
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
			err := runKarmadaWebhook(test.runData)
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
				t.Errorf("failed to verify the existence of %s deployment, got err: %v", util.KarmadaWebhookName(test.name), err)
			}
		})
	}
}

func TestRunKarmadaMetricsAdapter(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunKarmadaMetricsAdapter_InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "karmadaMetricsAdapter task invoked with an invalid data struct",
		},
		{
			name: "RunKarmadaMetricsAdapter_ValidTypeAssertion_TypeAssertionSuceeded",
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runKarmadaMetricsAdapter(test.runData)
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

func TestRunDeployMetricsAdapter(t *testing.T) {
	var replicas int32 = 2
	image, imageTag := "docker.io/karmada/karmada-metrics-adapter", "latest"
	name, namespace := "karmada-demo", "test"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}

	karmadaMetricsAdapter := &operatorv1alpha1.KarmadaMetricsAdapter{
		CommonSettings: operatorv1alpha1.CommonSettings{
			Image: operatorv1alpha1.Image{
				ImageRepository: image,
				ImageTag:        imageTag,
			},
			Replicas:        ptr.To(replicas),
			Annotations:     annotations,
			Labels:          labels,
			Resources:       corev1.ResourceRequirements{},
			ImagePullPolicy: imagePullPolicy,
		},
	}

	tests := []struct {
		name    string
		runData workflow.RunData
		prep    func(workflow.RunData) error
		verify  func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunDeployMetricsAdapter_InvalidTypeAssertion_TypeAssertionFailed",
			runData: MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "DeployMetricAdapter task invoked with an invalid data struct",
			prep:    func(workflow.RunData) error { return nil },
			verify:  func(workflow.RunData) error { return nil },
		},
		{
			name: "RunDeployMetricsAdapter_NilKarmadaMetricsAdapter_KarmadaMetricsAdapterInstallationSkipped",
			runData: &TestInitData{
				Name:      name,
				Namespace: namespace,
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					KarmadaMetricsAdapter: nil,
				},
			},
			wantErr: false,
			prep:    func(workflow.RunData) error { return nil },
			verify:  func(workflow.RunData) error { return nil },
		},
		{
			name: "RunDeployMetricsAdapter_InstallKarmadaMetricsAdapter_KarmadaMetricsAdapterInstalled",
			runData: &TestInitData{
				Name:      name,
				Namespace: namespace,
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					KarmadaMetricsAdapter: karmadaMetricsAdapter,
				},
				RemoteClientConnector: fakeclientset.NewSimpleClientset(),
			},
			prep: func(rd workflow.RunData) error {
				data := rd.(*TestInitData)
				if _, err := apiclient.CreatePods(data.RemoteClient(), namespace, util.KarmadaMetricsAdapterName(name), replicas, karmadaMetricAdapterLabels, true); err != nil {
					return fmt.Errorf("failed to create pods, got err: %v", err)
				}
				return nil
			},
			verify: func(runData workflow.RunData) error {
				data := runData.(*TestInitData)
				// Verify that the component deployment has been created in the given namespace.
				client := data.RemoteClient()
				componentName := util.KarmadaMetricsAdapterName(name)
				_, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), componentName, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("failed to find deployment %s in namespace %s: %w", componentName, namespace, err)
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
			if err := test.prep(test.runData); err != nil {
				t.Errorf("failed to prep deploying metrics adapter: %v", err)
			}
			err := runDeployMetricAdapter(test.runData)
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
				t.Errorf("failed to verify the existence of %s deployment, got err: %v", util.KarmadaMetricsAdapterName(name), err)
			}
		})
	}
}
