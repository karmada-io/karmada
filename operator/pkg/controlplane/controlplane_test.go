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

package controlplane

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

func TestEnsureAllControlPlaneComponents(t *testing.T) {
	var replicas int32 = 2
	name, namespace := "karmada-demo", "test"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	extraArgs := map[string]string{"cmd1": "arg1", "cmd2": "arg2"}

	cfg := &operatorv1alpha1.KarmadaComponents{
		KubeControllerManager: &operatorv1alpha1.KubeControllerManager{
			CommonSettings: operatorv1alpha1.CommonSettings{
				Image: operatorv1alpha1.Image{
					ImageRepository: "registry.k8s.io/kube-controller-manager",
					ImageTag:        "latest",
				},
				Replicas:        ptr.To[int32](replicas),
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
				Replicas:        ptr.To[int32](replicas),
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
				Replicas:        ptr.To[int32](replicas),
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
				Replicas:        ptr.To[int32](replicas),
				Annotations:     annotations,
				Labels:          labels,
				Resources:       corev1.ResourceRequirements{},
				ImagePullPolicy: imagePullPolicy,
			},
			ExtraArgs: extraArgs,
		},
	}

	fakeClient := fakeclientset.NewSimpleClientset()

	components := []string{
		constants.KubeControllerManagerComponent,
		constants.KarmadaControllerManagerComponent,
		constants.KarmadaSchedulerComponent,
		constants.KarmadaDeschedulerComponent,
	}

	for _, component := range components {
		err := EnsureControlPlaneComponent(component, name, namespace, map[string]bool{}, fakeClient, cfg)
		if err != nil {
			t.Fatalf("failed to ensure %s controlplane component: %v", component, err)
		}
	}

	actions := fakeClient.Actions()
	if len(actions) != len(components) {
		t.Fatalf("expected %d actions, but got %d", len(components), len(actions))
	}

	for _, action := range actions {
		createAction, ok := action.(coretesting.CreateAction)
		if !ok {
			t.Errorf("expected CreateAction, but got %T", action)
		}

		if createAction.GetResource().Resource != "deployments" {
			t.Errorf("expected action on 'deployments', but got '%s'", createAction.GetResource().Resource)
		}
	}
}

func TestGetKubeControllerManagerManifest(t *testing.T) {
	var replicas int32 = 2
	name, namespace := "karmada-demo", "test"
	image, imageTag := "registry.k8s.io/kube-controller-manager", "latest"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	extraArgs := map[string]string{"cmd1": "arg1", "cmd2": "arg2"}
	priorityClassName := "system-cluster-critical"

	cfg := &operatorv1alpha1.KubeControllerManager{
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

	deployment, err := getKubeControllerManagerManifest(name, namespace, cfg)
	if err != nil {
		t.Fatalf("failed to get kube controller manager manifest: %v", err)
	}

	deployment, _, err = verifyDeploymentDetails(
		deployment, replicas, imagePullPolicy, extraArgs, namespace,
		image, imageTag, util.KubeControllerManagerName(name), priorityClassName,
	)
	if err != nil {
		t.Errorf("failed to verify kube controller manager deployment details: %v", err)
	}

	expectedSecrets := []string{
		util.ComponentKarmadaConfigSecretName(util.KubeControllerManagerName(name)),
		util.KarmadaCertSecretName(name),
	}
	err = verifySecrets(deployment, expectedSecrets)
	if err != nil {
		t.Errorf("failed to verify kube controller manager secrets: %v", err)
	}
}

func TestGetKarmadaControllerManagerManifest(t *testing.T) {
	var replicas int32 = 2
	name, namespace := "karmada-demo", "test"
	image, imageTag := "docker.io/karmada/karmada-controller-manager", "latest"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	extraArgs := map[string]string{"cmd1": "arg1", "cmd2": "arg2"}
	priorityClassName := "system-cluster-critical"

	cfg := &operatorv1alpha1.KarmadaControllerManager{
		CommonSettings: operatorv1alpha1.CommonSettings{
			Image: operatorv1alpha1.Image{
				ImageRepository: image,
				ImageTag:        imageTag,
			},
			Replicas:          ptr.To[int32](replicas),
			Annotations:       annotations,
			Labels:            labels,
			ImagePullPolicy:   imagePullPolicy,
			PriorityClassName: priorityClassName,
		},
		ExtraArgs: extraArgs,
	}

	featureGates := map[string]bool{"FeatureA": true}

	deployment, err := getKarmadaControllerManagerManifest(name, namespace, featureGates, cfg)
	if err != nil {
		t.Fatalf("failed to get karmada controller manager manifest: %v", err)
	}

	deployment, container, err := verifyDeploymentDetails(
		deployment, replicas, imagePullPolicy, extraArgs, namespace,
		image, imageTag, util.KarmadaControllerManagerName(name), priorityClassName,
	)
	if err != nil {
		t.Errorf("failed to verify karmada controller manager deployment details: %v", err)
	}

	err = verifyFeatureGates(container, featureGates)
	if err != nil {
		t.Errorf("failed to verify karmada controller manager feature gates: %v", err)
	}

	err = verifySystemNamespace(container)
	if err != nil {
		t.Errorf("failed to verify karmada controller manager system namespace: %v", err)
	}

	expectedSecrets := []string{util.ComponentKarmadaConfigSecretName(util.KarmadaControllerManagerName(name))}
	err = verifySecrets(deployment, expectedSecrets)
	if err != nil {
		t.Errorf("failed to verify karmada controller manager secrets: %v", err)
	}
}

func TestGetKarmadaSchedulerManifest(t *testing.T) {
	var replicas int32 = 2
	name, namespace := "karmada-demo", "test"
	image, imageTag := "docker.io/karmada/karmada-scheduler", "latest"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	extraArgs := map[string]string{"cmd1": "arg1", "cmd2": "arg2"}
	priorityClassName := "system-cluster-critical"

	cfg := &operatorv1alpha1.KarmadaScheduler{
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

	featureGates := map[string]bool{"FeatureA": true}

	deployment, err := getKarmadaSchedulerManifest(name, namespace, featureGates, cfg)
	if err != nil {
		t.Fatalf("failed to get karmada scheduler manifest: %v", err)
	}

	deployment, container, err := verifyDeploymentDetails(
		deployment, replicas, imagePullPolicy, extraArgs, namespace,
		image, imageTag, util.KarmadaSchedulerName(name), priorityClassName,
	)
	if err != nil {
		t.Errorf("failed to verify karmada scheduler deployment details: %v", err)
	}

	err = verifyFeatureGates(container, featureGates)
	if err != nil {
		t.Errorf("failed to verify karmada scheduler feature gates: %v", err)
	}

	err = verifySystemNamespace(container)
	if err != nil {
		t.Errorf("failed to verify karmada scheduler system namespace: %v", err)
	}

	expectedSecrets := []string{
		util.ComponentKarmadaConfigSecretName(util.KarmadaSchedulerName(name)),
		util.KarmadaCertSecretName(name),
	}
	err = verifySecrets(deployment, expectedSecrets)
	if err != nil {
		t.Errorf("failed to verify karmada scheduler secrets: %v", err)
	}
}

func TestGetKarmadaDeschedulerManifest(t *testing.T) {
	var replicas int32 = 2
	name, namespace := "karmada-demo", "test"
	image, imageTag := "docker.io/karmada/karmada-descheduler", "latest"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	extraArgs := map[string]string{"cmd1": "arg1", "cmd2": "arg2"}
	priorityClassName := "system-cluster-critical"

	cfg := &operatorv1alpha1.KarmadaDescheduler{
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

	featureGates := map[string]bool{"FeatureA": true}

	deployment, err := getKarmadaDeschedulerManifest(name, namespace, featureGates, cfg)
	if err != nil {
		t.Fatalf("failed to get karmada descheduler manifest: %v", err)
	}

	deployment, container, err := verifyDeploymentDetails(
		deployment, replicas, imagePullPolicy, extraArgs, namespace,
		image, imageTag, util.KarmadaDeschedulerName(name), priorityClassName,
	)
	if err != nil {
		t.Errorf("failed to verify karmada descheduler deployment details: %v", err)
	}

	err = verifyFeatureGates(container, featureGates)
	if err != nil {
		t.Errorf("failed to verify karmada descheduler feature gates: %v", err)
	}

	err = verifySystemNamespace(container)
	if err != nil {
		t.Errorf("failed to verify karmada descheduler system namespace: %v", err)
	}

	expectedSecrets := []string{
		util.ComponentKarmadaConfigSecretName(util.KarmadaDeschedulerName(name)),
		util.KarmadaCertSecretName(name),
	}
	err = verifySecrets(deployment, expectedSecrets)
	if err != nil {
		t.Errorf("failed to verify karmada descheduler secrets: %v", err)
	}
}

// verifyDeploymentDetails ensures that the specified deployment contains the
// correct configuration for replicas, image pull policy, extra args, and image.
// It validates that the deployment matches the expected Karmada Controlplane settings.
// It could be against Kube Controller Manager, Karmada Controller Manager, Karmada Scheduler,
// and Karmada Descheduler.
func verifyDeploymentDetails(deployment *appsv1.Deployment, replicas int32, imagePullPolicy corev1.PullPolicy, extraArgs map[string]string, namespace, image, imageTag, expectedDeploymentName, priorityClassName string) (*appsv1.Deployment, *corev1.Container, error) {
	if deployment.Name != expectedDeploymentName {
		return nil, nil, fmt.Errorf("expected deployment name '%s', but got '%s'", expectedDeploymentName, deployment.Name)
	}

	if deployment.Spec.Template.Spec.PriorityClassName != priorityClassName {
		return nil, nil, fmt.Errorf("expected priorityClassName to be set to %s, but got %s", priorityClassName, deployment.Spec.Template.Spec.PriorityClassName)
	}

	if deployment.Namespace != namespace {
		return nil, nil, fmt.Errorf("expected deployment namespace '%s', but got '%s'", namespace, deployment.Namespace)
	}

	if _, exists := deployment.Annotations["annotationKey"]; !exists {
		return nil, nil, fmt.Errorf("expected annotation with key 'annotationKey' and value 'annotationValue', but it was missing")
	}

	if _, exists := deployment.Labels["labelKey"]; !exists {
		return nil, nil, fmt.Errorf("expected label with key 'labelKey' and value 'labelValue', but it was missing")
	}

	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != replicas {
		return nil, nil, fmt.Errorf("expected replicas to be %d, but got %d", replicas, deployment.Spec.Replicas)
	}

	containers := deployment.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		return nil, nil, fmt.Errorf("expected exactly 1 container, but got %d", len(containers))
	}

	expectedImage := fmt.Sprintf("%s:%s", image, imageTag)
	container := containers[0]
	if container.Image != expectedImage {
		return nil, nil, fmt.Errorf("expected container image '%s', but got '%s'", expectedImage, container.Image)
	}

	if container.ImagePullPolicy != imagePullPolicy {
		return nil, nil, fmt.Errorf("expected image pull policy '%s', but got '%s'", imagePullPolicy, container.ImagePullPolicy)
	}

	err := verifyExtraArgs(&container, extraArgs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to verify extra args: %v", err)
	}

	return deployment, &container, nil
}

// verifySystemNamespace validates that expected system namespace is present in the container commands.
func verifySystemNamespace(container *corev1.Container) error {
	leaderElectResourceSystemNamespaceArg := fmt.Sprintf("--leader-elect-resource-namespace=%s", constants.KarmadaSystemNamespace)
	if !contains(container.Command, leaderElectResourceSystemNamespaceArg) {
		return fmt.Errorf("leader elect resource namespace argument '%s' not found in container command with value %s", leaderElectResourceSystemNamespaceArg, constants.KarmadaSystemNamespace)
	}
	return nil
}

// verifySecrets validates that the expected secrets are present in the Deployment's volumes.
func verifySecrets(deployment *appsv1.Deployment, expectedSecrets []string) error {
	var extractedSecrets []string
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		extractedSecrets = append(extractedSecrets, volume.Secret.SecretName)
	}
	for _, expectedSecret := range expectedSecrets {
		if !contains(extractedSecrets, expectedSecret) {
			return fmt.Errorf("expected secret '%s' not found in extracted secrets", expectedSecret)
		}
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

// verifyFeatureGates ensures the container's command includes the specified feature gates.
func verifyFeatureGates(container *corev1.Container, featureGates map[string]bool) error {
	var featureGatesArg string
	for key, value := range featureGates {
		featureGatesArg += fmt.Sprintf("%s=%t,", key, value)
	}
	featureGatesArg = fmt.Sprintf("--feature-gates=%s", featureGatesArg[:len(featureGatesArg)-1])
	if !contains(container.Command, featureGatesArg) {
		return fmt.Errorf("expected container commands to include '%s', but it was missing", featureGatesArg)
	}
	return nil
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
