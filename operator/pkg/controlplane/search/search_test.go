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

package search

import (
	"fmt"
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
)

func TestEnsureKarmadaSearch(t *testing.T) {
	var replicas int32 = 2
	image, imageTag := "docker.io/karmada/karmada-search", "latest"
	name := "karmada-demo"
	namespace := "test"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	extraArgs := map[string]string{"cmd1": "arg1", "cmd2": "arg2"}

	cfg := &operatorv1alpha1.KarmadaSearch{
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
		ExtraArgs: extraArgs,
	}

	// Create fake clientset.
	fakeClient := fakeclientset.NewClientset()
	etcdCfg := &operatorv1alpha1.Etcd{
		Local: &operatorv1alpha1.LocalEtcd{},
	}
	err := EnsureKarmadaSearch(fakeClient, cfg, etcdCfg, name, namespace, map[string]bool{})
	if err != nil {
		t.Fatalf("failed to ensure karmada search, but got: %v", err)
	}

	actions := fakeClient.Actions()
	// We now create deployment, service and PDB, so expect 3 actions
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

func TestInstallKarmadaSearch(t *testing.T) {
	var replicas int32 = 2
	image, imageTag := "docker.io/karmada/karmada-search", "latest"
	name := "karmada-demo"
	namespace := "test"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	extraArgs := map[string]string{"cmd1": "arg1", "cmd2": "arg2"}
	priorityClassName := "system-cluster-critical"

	cfg := &operatorv1alpha1.KarmadaSearch{
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
		ExtraArgs: extraArgs,
	}

	// Create fake clientset.
	fakeClient := fakeclientset.NewClientset()
	etcdCfg := &operatorv1alpha1.Etcd{
		Local: &operatorv1alpha1.LocalEtcd{},
	}
	err := installKarmadaSearch(fakeClient, cfg, etcdCfg, name, namespace, map[string]bool{})
	if err != nil {
		t.Fatalf("failed to install karmada search: %v", err)
	}

	deployment, err := verifyDeploymentCreation(fakeClient)
	if err != nil {
		t.Fatalf("failed to verify karmada search deployment creation: %v", err)
	}

	// Verify deployment details
	if err := verifyDeploymentDetails(deployment, replicas, imagePullPolicy, extraArgs, name, namespace, image, imageTag, priorityClassName); err != nil {
		t.Fatalf("failed to verify deployment details: %v", err)
	}
}

func TestCreateKarmadaSearchService(t *testing.T) {
	// Define inputs.
	name := "karmada-demo"
	namespace := "test"

	// Initialize fake clientset.
	client := fakeclientset.NewClientset()

	err := createKarmadaSearchService(client, name, namespace)
	if err != nil {
		t.Fatalf("failed to create karmada search service")
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
	expectedServiceName := util.KarmadaSearchName(name)
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

// verifyDeploymentDetails validates the details of a Deployment against the expected parameters.
func verifyDeploymentDetails(deployment *appsv1.Deployment, replicas int32, imagePullPolicy corev1.PullPolicy, extraArgs map[string]string, name, namespace, image, imageTag, priorityClassName string) error {
	expectedDeploymentName := util.KarmadaSearchName(name)
	if deployment.Name != expectedDeploymentName {
		return fmt.Errorf("expected deployment name '%s', but got '%s'", expectedDeploymentName, deployment.Name)
	}

	if deployment.Spec.Template.Spec.PriorityClassName != priorityClassName {
		return fmt.Errorf("expected priorityClassName to be set to %s, but got %s", priorityClassName, deployment.Spec.Template.Spec.PriorityClassName)
	}

	if deployment.Namespace != namespace {
		return fmt.Errorf("expected deployment namespace '%s', but got '%s'", namespace, deployment.Namespace)
	}

	if _, exists := deployment.Annotations["annotationKey"]; !exists {
		return fmt.Errorf("expected annotation with key 'annotationKey' and value 'annotationValue', but it was missing")
	}

	if _, exists := deployment.Labels["labelKey"]; !exists {
		return fmt.Errorf("expected label with key 'labelKey' and value 'labelValue', but it was missing")
	}

	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != replicas {
		return fmt.Errorf("expected replicas to be %d, but got %d", replicas, deployment.Spec.Replicas)
	}

	containers := deployment.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		return fmt.Errorf("expected exactly 1 container, but got %d", len(containers))
	}

	expectedImage := fmt.Sprintf("%s:%s", image, imageTag)
	container := containers[0]
	if container.Image != expectedImage {
		return fmt.Errorf("expected container image '%s', but got '%s'", expectedImage, container.Image)
	}

	if container.ImagePullPolicy != imagePullPolicy {
		return fmt.Errorf("expected image pull policy '%s', but got '%s'", imagePullPolicy, container.ImagePullPolicy)
	}

	err := verifyExtraArgs(&container, extraArgs)
	if err != nil {
		return fmt.Errorf("failed to verify extra args: %v", err)
	}

	err = verifySecrets(deployment, name)
	if err != nil {
		return fmt.Errorf("failed to verify secrets: %v", err)
	}

	err = verifyEtcdServers(&container, name, namespace)
	if err != nil {
		return fmt.Errorf("failed to verify etcd servers: %v", err)
	}

	return nil
}

// verifySecrets validates that the expected secrets are present in the Deployment's volumes.
func verifySecrets(deployment *appsv1.Deployment, name string) error {
	var extractedSecrets []string
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		extractedSecrets = append(extractedSecrets, volume.Secret.SecretName)
	}
	expectedSecrets := []string{
		util.ComponentKarmadaConfigSecretName(util.KarmadaSearchName(name)),
		util.KarmadaCertSecretName(name),
	}
	for _, expectedSecret := range expectedSecrets {
		if !contains(extractedSecrets, expectedSecret) {
			return fmt.Errorf("expected secret '%s' not found in extracted secrets", expectedSecret)
		}
	}
	return nil
}

// verifyEtcdServers checks that the container command includes the correct etcd server argument.
func verifyEtcdServers(container *corev1.Container, name, namespace string) error {
	etcdServersArg := fmt.Sprintf("https://%s.%s.svc.cluster.local:%d,", util.KarmadaEtcdClientName(name), namespace, constants.EtcdListenClientPort)
	etcdServersArg = fmt.Sprintf("--etcd-servers=%s", etcdServersArg[:len(etcdServersArg)-1])
	if !contains(container.Command, etcdServersArg) {
		return fmt.Errorf("etcd servers argument '%s' not found in container command", etcdServersArg)
	}
	return nil
}

// verifyExtraArgs checks that the container command includes the extra arguments.
func verifyExtraArgs(container *corev1.Container, extraArgs map[string]string) error {
	for key, value := range extraArgs {
		expectedArg := fmt.Sprintf("--%s=%s", key, value)
		if !contains(container.Command, expectedArg) {
			return fmt.Errorf("expected container commands to include '%s', but it was missing", expectedArg)
		}
	}
	return nil
}

// contains check if a slice contains a specific string.
func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}
