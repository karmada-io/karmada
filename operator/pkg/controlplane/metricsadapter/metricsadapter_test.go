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

package metricsadapter

import (
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/util"
)

func TestEnsureKarmadaMetricAdapter(t *testing.T) {
	var replicas int32 = 2
	image, imageTag := "docker.io/karmada/karmada-metrics-adapter", "latest"
	name := "karmada-demo"
	namespace := "test"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	priorityClassName := "system-cluster-critical"

	cfg := &operatorv1alpha1.KarmadaMetricsAdapter{
		CommonSettings: operatorv1alpha1.CommonSettings{
			Image: operatorv1alpha1.Image{
				ImageRepository: image,
				ImageTag:        imageTag,
			},
			Replicas:          ptr.To[int32](replicas),
			Annotations:       annotations,
			Labels:            labels,
			Resources:         corev1.ResourceRequirements{},
			ImagePullPolicy:   imagePullPolicy,
			PriorityClassName: priorityClassName,
		},
	}

	// Create fake clientset.
	fakeClient := fakeclientset.NewClientset()

	err := EnsureKarmadaMetricAdapter(fakeClient, cfg, name, namespace)
	if err != nil {
		t.Fatalf("failed to ensure karmada metrics adapter, but got: %v", err)
	}

	actions := fakeClient.Actions()
	// We now create deployment, service and PDB, so expect 4 actions
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions, but got %d", len(actions))
	}

	// Check that we have deployment, service, and PDB
	deploymentCount := 0
	serviceCount := 0
	pdbCount := 0
	for _, action := range actions {
		if action.GetResource().Resource == "deployments" {
			deploymentCount++
		} else if action.GetResource().Resource == "services" {
			serviceCount++
		} else if action.GetResource().Resource == "poddisruptionbudgets" {
			pdbCount++
		}
	}

	if deploymentCount != 1 {
		t.Errorf("expected 1 deployment actions, but got %d", deploymentCount)
	}

	if serviceCount != 1 {
		t.Errorf("expected 1 service action, but got %d", serviceCount)
	}

	if pdbCount != 1 {
		t.Errorf("expected 1 PDB action, but got %d", pdbCount)
	}
}

func TestInstallKarmadaMetricAdapter(t *testing.T) {
	var replicas int32 = 2
	image, imageTag := "docker.io/karmada/karmada-metrics-adapter", "latest"
	name := "karmada-demo"
	namespace := "test"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	priorityClassName := "system-cluster-critical"

	cfg := &operatorv1alpha1.KarmadaMetricsAdapter{
		CommonSettings: operatorv1alpha1.CommonSettings{
			Image: operatorv1alpha1.Image{
				ImageRepository: image,
				ImageTag:        imageTag,
			},
			Replicas:          ptr.To[int32](replicas),
			Annotations:       annotations,
			Labels:            labels,
			Resources:         corev1.ResourceRequirements{},
			ImagePullPolicy:   imagePullPolicy,
			PriorityClassName: priorityClassName,
		},
	}

	// Create fake clientset.
	fakeClient := fakeclientset.NewClientset()

	err := installKarmadaMetricAdapter(fakeClient, cfg, name, namespace)
	if err != nil {
		t.Fatalf("failed to install karmada metrics adapter: %v", err)
	}

	_, err = verifyDeploymentCreation(fakeClient)
	if err != nil {
		t.Fatalf("failed to verify deployment creation: %v", err)
	}
}

func TestCreateKarmadaMetricAdapterService(t *testing.T) {
	// Define inputs.
	name := "karmada-demo"
	namespace := "test"

	// Initialize fake clientset.
	client := fakeclientset.NewClientset()

	err := createKarmadaMetricAdapterService(client, name, namespace)
	if err != nil {
		t.Fatalf("failed to create karmada metrics adapter service %v", err)
	}

	// Ensure the expected action (service creation) occurred.
	actions := client.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 actions, but got %d actions", len(actions))
	}

	// Validate the action is a CreateAction and it's for the correct resource (Service).
	createAction, ok := actions[0].(coretesting.CreateAction)
	if !ok {
		t.Fatalf("expected CreateAction, but got %T", actions[0])
	}

	if createAction.GetResource().Resource != "services" {
		t.Fatalf("expected action on 'services', but got '%s'", createAction.GetResource().Resource)
	}

	// Validate the created service object.
	service := createAction.GetObject().(*corev1.Service)
	expectedServiceName := util.KarmadaMetricsAdapterName(name)
	if service.Name != expectedServiceName {
		t.Fatalf("expected service name '%s', but got '%s'", expectedServiceName, service.Name)
	}

	if service.Namespace != namespace {
		t.Fatalf("expected service namespace '%s', but got '%s'", namespace, service.Namespace)
	}
}

// verifyDeploymentCreation validates that a Deployment and PDB were created and returns the deployment.
func verifyDeploymentCreation(client *fakeclientset.Clientset) (*appsv1.Deployment, error) {
	// Assert that a Deployment and PDB were created.
	actions := client.Actions()
	// We now create deployment and PDB, so expect 2 actions
	if len(actions) != 2 {
		return nil, fmt.Errorf("expected exactly 2 actions (deployment + PDB), but got %d actions", len(actions))
	}

	// Find the deployment action
	var deployment *appsv1.Deployment
	for _, action := range actions {
		if action.GetResource().Resource == "deployments" {
			createAction, ok := action.(coretesting.CreateAction)
			if !ok {
				return nil, fmt.Errorf("expected a CreateAction for deployment, but got %T", action)
			}
			deployment = createAction.GetObject().(*appsv1.Deployment)
			break
		}
	}

	if deployment == nil {
		return nil, fmt.Errorf("expected deployment action, but none found")
	}

	return deployment, nil
}
