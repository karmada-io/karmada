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

package apiserver

import (
	"fmt"
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

func TestEnsureKarmadaAPIServer(t *testing.T) {
	var replicas int32 = 3
	image := "karmada-apiserver-image"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	name := "karmada-apiserver"
	namespace := "test-namespace"
	serviceSubnet := "10.96.0.0/12"

	cfg := &operatorv1alpha1.KarmadaComponents{
		KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
			CommonSettings: operatorv1alpha1.CommonSettings{
				Image:           operatorv1alpha1.Image{ImageTag: image},
				Replicas:        ptr.To[int32](replicas),
				Annotations:     annotations,
				Labels:          labels,
				Resources:       corev1.ResourceRequirements{},
				ImagePullPolicy: imagePullPolicy,
			},
			ServiceSubnet: ptr.To(serviceSubnet),
			ExtraArgs:     map[string]string{"cmd1": "arg1", "cmd2": "arg2"},
		},
		Etcd: &operatorv1alpha1.Etcd{
			Local: &operatorv1alpha1.LocalEtcd{},
		},
	}

	fakeClient := fakeclientset.NewSimpleClientset()

	err := EnsureKarmadaAPIServer(fakeClient, cfg, name, namespace, map[string]bool{})
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}

	actions := fakeClient.Actions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, but got %d", len(actions))
	}
}

func TestEnsureKarmadaAggregatedAPIServer(t *testing.T) {
	var replicas int32 = 2
	image := "karmada-aggregated-apiserver-image"
	imagePullPolicy := corev1.PullIfNotPresent
	annotationValues := map[string]string{"annotationKey": "annotationValue"}
	labelValues := map[string]string{"labelKey": "labelValue"}
	name := "test-agg-server"
	namespace := "test-namespace"

	cfg := &operatorv1alpha1.KarmadaComponents{
		KarmadaAggregatedAPIServer: &operatorv1alpha1.KarmadaAggregatedAPIServer{
			CommonSettings: operatorv1alpha1.CommonSettings{
				Image:           operatorv1alpha1.Image{ImageTag: image},
				Replicas:        ptr.To[int32](replicas),
				Annotations:     annotationValues,
				Labels:          labelValues,
				Resources:       corev1.ResourceRequirements{},
				ImagePullPolicy: imagePullPolicy,
			},
			ExtraArgs: map[string]string{"cmd1": "arg1", "cmd2": "arg2"},
		},
		Etcd: &operatorv1alpha1.Etcd{
			Local: &operatorv1alpha1.LocalEtcd{},
		},
	}

	featureGates := map[string]bool{"FeatureA": true}

	fakeClient := fakeclientset.NewSimpleClientset()

	err := EnsureKarmadaAggregatedAPIServer(fakeClient, cfg, name, namespace, featureGates)
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}

	actions := fakeClient.Actions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, but got %d", len(actions))
	}
}

func TestInstallKarmadaAPIServer(t *testing.T) {
	var replicas int32 = 3
	image := "karmada-apiserver-image"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	name := "karmada-apiserver"
	namespace := "test-namespace"
	serviceSubnet := "10.96.0.0/12"
	priorityClassName := "system-cluster-critical"

	// Create fake clientset.
	fakeClient := fakeclientset.NewSimpleClientset()

	// Define a valid KarmadaAPIServer configuration.
	cfg := &operatorv1alpha1.KarmadaAPIServer{
		CommonSettings: operatorv1alpha1.CommonSettings{
			Image:             operatorv1alpha1.Image{ImageTag: image},
			Replicas:          ptr.To[int32](replicas),
			Annotations:       annotations,
			Labels:            labels,
			Resources:         corev1.ResourceRequirements{},
			ImagePullPolicy:   imagePullPolicy,
			PriorityClassName: priorityClassName,
		},
		ServiceSubnet: ptr.To(serviceSubnet),
		ExtraArgs:     map[string]string{"cmd1": "arg1", "cmd2": "arg2"},
	}
	etcdCfg := &operatorv1alpha1.Etcd{
		Local: &operatorv1alpha1.LocalEtcd{},
	}
	featureGates := map[string]bool{"FeatureA": true}

	// Call the function under test.
	err := installKarmadaAPIServer(fakeClient, cfg, etcdCfg, name, namespace, featureGates)
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}

	deployment, err := verifyDeploymentCreation(fakeClient, &replicas, imagePullPolicy, cfg.ExtraArgs, name, namespace, image, util.KarmadaAPIServerName(name), priorityClassName)
	if err != nil {
		t.Fatalf("failed to verify karmada apiserver correct deployment creation correct details: %v", err)
	}

	err = verifyAPIServerDeploymentAdditionalDetails(deployment, name, serviceSubnet)
	if err != nil {
		t.Errorf("failed to verify karmada apiserver additional deployment details: %v", err)
	}
}

func TestCreateKarmadaAPIServerService(t *testing.T) {
	// Initialize fake clientset.
	client := fakeclientset.NewSimpleClientset()

	// Define inputs.
	name := "test-apiserver"
	namespace := "test-namespace"
	cfg := &operatorv1alpha1.KarmadaAPIServer{
		ServiceAnnotations: map[string]string{"annotationKey": "annotationValue"},
	}

	// Call the function under test.
	err := createKarmadaAPIServerService(client, cfg, name, namespace)
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}

	// Ensure the expected action (service creation) occurred.
	actions := client.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d actions", len(actions))
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
	expectedServiceName := util.KarmadaAPIServerName(name)
	if service.Name != expectedServiceName {
		t.Fatalf("expected service name '%s', but got '%s'", expectedServiceName, service.Name)
	}

	if service.Namespace != namespace {
		t.Fatalf("expected service namespace '%s', but got '%s'", namespace, service.Namespace)
	}

	if _, exists := service.Annotations["annotationKey"]; !exists {
		t.Errorf("expected annotation with key 'annotationKey' and value 'annotationValue', but it was missing")
	}
}

func TestInstallKarmadaAggregatedAPIServer(t *testing.T) {
	var replicas int32 = 2
	image := "karmada-aggregated-apiserver-image"
	imagePullPolicy := corev1.PullIfNotPresent
	annotationValues := map[string]string{"annotationKey": "annotationValue"}
	labelValues := map[string]string{"labelKey": "labelValue"}
	name := "test-agg-server"
	namespace := "test-namespace"
	priorityClassName := "system-cluster-critical"

	// Use fake clientset.
	fakeClient := fakeclientset.NewSimpleClientset()

	// Define valid inputs.
	cfg := &operatorv1alpha1.KarmadaAggregatedAPIServer{
		CommonSettings: operatorv1alpha1.CommonSettings{
			Image:             operatorv1alpha1.Image{ImageTag: image},
			Replicas:          ptr.To[int32](replicas),
			Annotations:       annotationValues,
			Labels:            labelValues,
			Resources:         corev1.ResourceRequirements{},
			ImagePullPolicy:   imagePullPolicy,
			PriorityClassName: priorityClassName,
		},
		ExtraArgs: map[string]string{"cmd1": "arg1", "cmd2": "arg2"},
	}

	featureGates := map[string]bool{"FeatureA": true}
	etcdCfg := &operatorv1alpha1.Etcd{
		Local: &operatorv1alpha1.LocalEtcd{},
	}
	err := installKarmadaAggregatedAPIServer(fakeClient, cfg, etcdCfg, name, namespace, featureGates)
	if err != nil {
		t.Fatalf("Failed to install Karmada Aggregated API Server: %v", err)
	}

	deployment, err := verifyDeploymentCreation(fakeClient, &replicas, imagePullPolicy, cfg.ExtraArgs, name, namespace, image, util.KarmadaAggregatedAPIServerName(name), priorityClassName)
	if err != nil {
		t.Fatalf("failed to verify karmada aggregated apiserver deployment creation correct details: %v", err)
	}

	err = verifyAggregatedAPIServerDeploymentAdditionalDetails(featureGates, deployment, name)
	if err != nil {
		t.Errorf("failed to verify karmada aggregated apiserver additional deployment details: %v", err)
	}
}

func TestCreateKarmadaAggregatedAPIServerService(t *testing.T) {
	// Initialize fake clientset.
	client := fakeclientset.NewSimpleClientset()

	// Define inputs.
	name := "test-agg-server"
	namespace := "test-namespace"

	// Call the function under test.
	err := createKarmadaAggregatedAPIServerService(client, name, namespace)
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}

	// Ensure the expected action (service creation) occurred.
	actions := client.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d actions", len(actions))
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
	expectedServiceName := util.KarmadaAggregatedAPIServerName(name)
	if service.Name != expectedServiceName {
		t.Fatalf("expected service name '%s', but got '%s'", expectedServiceName, service.Name)
	}

	if service.Namespace != namespace {
		t.Fatalf("expected service namespace '%s', but got '%s'", namespace, service.Namespace)
	}
}

// contains check if a slice contains a specific string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// verifyDeploymentCreation verifies the creation of a Kubernetes deployment
// based on the given parameters. It ensures that the deployment has the correct
// number of replicas, image pull policy, extra arguments, and labels, as well
// as the correct image for the Karmada API server.
func verifyDeploymentCreation(client *fakeclientset.Clientset, replicas *int32, imagePullPolicy corev1.PullPolicy, extraArgs map[string]string, name, namespace, image, expectedDeploymentName, priorityClassName string) (*appsv1.Deployment, error) {
	// Assert that a Deployment was created.
	actions := client.Actions()
	if len(actions) != 1 {
		return nil, fmt.Errorf("expected exactly 1 action either create or update, but got %d actions", len(actions))
	}

	// Check that the action was a Deployment creation.
	createAction, ok := actions[0].(coretesting.CreateAction)
	if !ok {
		return nil, fmt.Errorf("expected a CreateAction, but got %T", actions[0])
	}

	// Check that the action was performed on the correct resource.
	if createAction.GetResource().Resource != "deployments" {
		return nil, fmt.Errorf("expected action on 'deployments', but got '%s'", createAction.GetResource().Resource)
	}

	deployment := createAction.GetObject().(*appsv1.Deployment)
	err := verifyDeploymentDetails(deployment, replicas, imagePullPolicy, extraArgs, name, namespace, image, expectedDeploymentName, priorityClassName)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

// verifyDeploymentDetails ensures that the specified deployment contains the
// correct configuration for replicas, image pull policy, extra args, and image.
// It validates that the deployment matches the expected Karmada API server settings.
func verifyDeploymentDetails(deployment *appsv1.Deployment, replicas *int32, imagePullPolicy corev1.PullPolicy, extraArgs map[string]string, name, namespace, image, expectedDeploymentName, priorityClassName string) error {
	if deployment.Name != expectedDeploymentName {
		return fmt.Errorf("expected deployment name '%s', but got '%s'", expectedDeploymentName, deployment.Name)
	}

	if deployment.Spec.Template.Spec.PriorityClassName != priorityClassName {
		return fmt.Errorf("expected priorityClassName to be set to %s, but got %s", priorityClassName, deployment.Spec.Template.Spec.PriorityClassName)
	}

	expectedNamespace := "test-namespace"
	if deployment.Namespace != expectedNamespace {
		return fmt.Errorf("expected deployment namespace '%s', but got '%s'", expectedNamespace, deployment.Namespace)
	}

	if _, exists := deployment.Annotations["annotationKey"]; !exists {
		return fmt.Errorf("expected annotation with key 'annotationKey' and value 'annotationValue', but it was missing")
	}

	if _, exists := deployment.Labels["labelKey"]; !exists {
		return fmt.Errorf("expected label with key 'labelKey' and value 'labelValue', but it was missing")
	}

	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != *replicas {
		return fmt.Errorf("expected replicas to be %d, but got %d", replicas, *deployment.Spec.Replicas)
	}

	containers := deployment.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		return fmt.Errorf("expected exactly 1 container, but got %d", len(containers))
	}

	expectedImage := fmt.Sprintf(":%s", image)
	container := containers[0]
	if container.Image != expectedImage {
		return fmt.Errorf("expected container image '%s', but got '%s'", expectedImage, container.Image)
	}

	if container.ImagePullPolicy != imagePullPolicy {
		return fmt.Errorf("expected image pull policy '%s', but got '%s'", imagePullPolicy, container.ImagePullPolicy)
	}

	for key, value := range extraArgs {
		expectedArg := fmt.Sprintf("--%s=%s", key, value)
		if !contains(container.Command, expectedArg) {
			return fmt.Errorf("expected container commands to include '%s', but it was missing", expectedArg)
		}
	}

	etcdServersArg := fmt.Sprintf("https://%s.%s.svc.cluster.local:%d,", util.KarmadaEtcdClientName(name), namespace, constants.EtcdListenClientPort)
	etcdServersArg = fmt.Sprintf("--etcd-servers=%s", etcdServersArg[:len(etcdServersArg)-1])
	if !contains(container.Command, etcdServersArg) {
		return fmt.Errorf("etcd servers argument '%s' not found in container command", etcdServersArg)
	}

	return nil
}

// verifyAggregatedAPIServerDeploymentAdditionalDetails validates the additional
// details of the Karmada Aggregated API server deployment, such as the service
// subnet and configuration related to aggregated API server behavior.
func verifyAggregatedAPIServerDeploymentAdditionalDetails(featureGates map[string]bool, deployment *appsv1.Deployment, expectedDeploymentName string) error {
	var featureGatesArg string
	for key, value := range featureGates {
		featureGatesArg += fmt.Sprintf("%s=%t,", key, value)
	}
	featureGatesArg = fmt.Sprintf("--feature-gates=%s", featureGatesArg[:len(featureGatesArg)-1])
	if !contains(deployment.Spec.Template.Spec.Containers[0].Command, featureGatesArg) {
		return fmt.Errorf("expected container commands to include '%s', but it was missing", featureGatesArg)
	}

	if len(deployment.Spec.Template.Spec.Volumes) != 3 {
		return fmt.Errorf("expected 3 volumes, but found %d", len(deployment.Spec.Template.Spec.Volumes))
	}

	var extractedSecrets []string
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		extractedSecrets = append(extractedSecrets, volume.Secret.SecretName)
	}
	expectedSecrets := []string{util.ComponentKarmadaConfigSecretName(util.KarmadaAggregatedAPIServerName(expectedDeploymentName)), util.KarmadaCertSecretName(expectedDeploymentName), util.EtcdCertSecretName(expectedDeploymentName)}
	for _, expectedSecret := range expectedSecrets {
		if !contains(extractedSecrets, expectedSecret) {
			return fmt.Errorf("expected secret '%s' not found in extracted secrets", expectedSecret)
		}
	}

	return nil
}

// verifyAPIServerDeploymentAdditionalDetails checks the additional configuration
// details of a Kubernetes deployment for the Karmada API server. It validates the
// service cluster IP range, the number of volumes, and ensures that the required
// secret volumes are mounted in the deployment.
func verifyAPIServerDeploymentAdditionalDetails(deployment *appsv1.Deployment, expectedDeploymentName, serviceSubnet string) error {
	serviceClusterIPRangeArg := fmt.Sprintf("--service-cluster-ip-range=%s", serviceSubnet)
	if !contains(deployment.Spec.Template.Spec.Containers[0].Command, serviceClusterIPRangeArg) {
		return fmt.Errorf("service cluster IP range argument '%s' not found in container command", serviceClusterIPRangeArg)
	}

	if len(deployment.Spec.Template.Spec.Volumes) != 2 {
		return fmt.Errorf("expected 2 volumes, but found %d", len(deployment.Spec.Template.Spec.Volumes))
	}

	var extractedSecrets []string
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		extractedSecrets = append(extractedSecrets, volume.Secret.SecretName)
	}
	expectedSecrets := []string{util.KarmadaCertSecretName(expectedDeploymentName), util.EtcdCertSecretName(expectedDeploymentName)}
	for _, expectedSecret := range expectedSecrets {
		if !contains(extractedSecrets, expectedSecret) {
			return fmt.Errorf("expected secret '%s' not found in extracted secrets", expectedSecret)
		}
	}

	return nil
}
