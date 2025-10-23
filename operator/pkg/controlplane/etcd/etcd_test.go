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

package etcd

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
)

func TestEnsureKarmadaEtcd(t *testing.T) {
	var replicas int32 = 2
	image, imageTag := "registry.k8s.io/etcd", "latest"
	name := "karmada-demo"
	namespace := "test"
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

	// Create fake clientset.
	fakeClient := fakeclientset.NewSimpleClientset()

	err := EnsureKarmadaEtcd(fakeClient, cfg, name, namespace)
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}

	actions := fakeClient.Actions()
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions, but got %d", len(actions))
	}
}

func TestInstallKarmadaEtcd(t *testing.T) {
	var replicas int32 = 2
	image, imageTag := "registry.k8s.io/etcd", "latest"
	name := "karmada-demo"
	namespace := "test"
	imagePullPolicy := corev1.PullIfNotPresent
	annotations := map[string]string{"annotationKey": "annotationValue"}
	labels := map[string]string{"labelKey": "labelValue"}
	priorityClassName := "system-cluster-critical"

	// Define a valid Etcd configuration.
	cfg := &operatorv1alpha1.LocalEtcd{
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
	fakeClient := fakeclientset.NewSimpleClientset()

	err := installKarmadaEtcd(fakeClient, name, namespace, cfg)
	if err != nil {
		t.Fatalf("failed to install karmada etcd, got: %v", err)
	}

	err = verifyStatefulSetCreation(
		fakeClient, replicas, imagePullPolicy, name, namespace, image, imageTag, priorityClassName,
	)
	if err != nil {
		t.Fatalf("failed to verify statefulset creation: %v", err)
	}
}

func TestCreateEtcdService(t *testing.T) {
	// Define inputs.
	name := "karmada-demo"
	namespace := "test"

	// Initialize fake clientset.
	client := fakeclientset.NewSimpleClientset()

	err := createEtcdService(client, name, namespace)
	if err != nil {
		t.Fatalf("failed to create etcd service %v", err)
	}

	// Ensure the expected actions are two creations for etcd peer and client services.
	actions := client.Actions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, but got %d actions", len(actions))
	}

	// Validate the actions is of type CreateAction and it's for the correct resource (Service).
	for i, action := range actions {
		createAction, ok := action.(coretesting.CreateAction)
		if !ok {
			t.Fatalf("expected CreateAction, but got %d at index %d", action, i)
		}

		if createAction.GetResource().Resource != "services" {
			t.Fatalf("expected action on 'services', but got '%s' at action index %d", createAction.GetResource().Resource, i)
		}

		service := createAction.GetObject().(*corev1.Service)

		if service.Name != util.KarmadaEtcdName(name) && service.Name != util.KarmadaEtcdClientName(name) {
			t.Fatalf("expected created actions to be performed on etcd peer and client services, but found one on: %s", service.Name)
		}

		if service.Namespace != namespace {
			t.Fatalf("expected service namespace '%s', but got '%s'", namespace, service.Namespace)
		}

		if service.Name == util.KarmadaEtcdName(name) {
			peerServicePortsExpected := []corev1.ServicePort{
				{
					Name:     "client",
					Port:     constants.EtcdListenClientPort,
					Protocol: corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: constants.EtcdListenClientPort,
					},
				},
				{
					Name:     "server",
					Port:     constants.EtcdListenPeerPort,
					Protocol: corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: constants.EtcdListenPeerPort,
					},
				},
			}
			err := verifyEtcdPeerOrClientService(service, peerServicePortsExpected)
			if err != nil {
				t.Errorf("failed to verify etcd peer service: %v", err)
			}
		}

		if service.Name == util.KarmadaEtcdClientName(name) {
			clientServicePortsExpected := []corev1.ServicePort{
				{
					Name:     "client",
					Port:     constants.EtcdListenClientPort,
					Protocol: corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: constants.EtcdListenClientPort,
					},
				},
			}
			err := verifyEtcdPeerOrClientService(service, clientServicePortsExpected)
			if err != nil {
				t.Errorf("failed to verify etcd client service: %v", err)
			}
		}
	}
}

// verifyStatefulSetCreation asserts that a StatefulSet was created in the given clientset.
// It checks that exactly one action was recorded, verifies that it is a creation action for a StatefulSet,
// and then validates the details of the created StatefulSet against the expected parameters.
func verifyStatefulSetCreation(client *fakeclientset.Clientset, replicas int32, imagePullPolicy corev1.PullPolicy, name, namespace, image, imageTag, priorityClassName string) error {
	// Assert that a Statefulset was created.
	actions := client.Actions()
	if len(actions) != 1 {
		return fmt.Errorf("expected exactly 1 action either create or update, but got %d actions", len(actions))
	}

	// Check that the action was a Statefulset creation.
	createAction, ok := actions[0].(coretesting.CreateAction)
	if !ok {
		return fmt.Errorf("expected a CreateAction, but got %T", actions[0])
	}

	if createAction.GetResource().Resource != "statefulsets" {
		return fmt.Errorf("expected action on 'statefulsets', but got '%s'", createAction.GetResource().Resource)
	}

	statefulSet := createAction.GetObject().(*appsv1.StatefulSet)

	if statefulSet.Spec.Template.Spec.PriorityClassName != priorityClassName {
		return fmt.Errorf("expected priorityClassName to be set to %s, but got %s", priorityClassName, statefulSet.Spec.Template.Spec.PriorityClassName)
	}

	return verifyStatefulSetDetails(
		statefulSet, replicas, imagePullPolicy, name, namespace, image, imageTag,
	)
}

// verifyStatefulSetDetails validates the details of a StatefulSet against the expected parameters.
func verifyStatefulSetDetails(statefulSet *appsv1.StatefulSet, replicas int32, imagePullPolicy corev1.PullPolicy, name, namespace, image, imageTag string) error {
	expectedStatefulsetName := util.KarmadaEtcdName(name)
	if statefulSet.Name != expectedStatefulsetName {
		return fmt.Errorf("expected statefulset name '%s', but got '%s'", expectedStatefulsetName, statefulSet.Name)
	}

	if statefulSet.Namespace != namespace {
		return fmt.Errorf("expected statefulset namespace '%s', but got '%s'", namespace, statefulSet.Namespace)
	}

	if _, exists := statefulSet.Annotations["annotationKey"]; !exists {
		return fmt.Errorf("expected annotation with key 'annotationKey' and value 'annotationValue', but it was missing")
	}

	if _, exists := statefulSet.Labels["labelKey"]; !exists {
		return fmt.Errorf("expected label with key 'labelKey' and value 'labelValue', but it was missing")
	}

	if statefulSet.Spec.Replicas == nil || *statefulSet.Spec.Replicas != replicas {
		return fmt.Errorf("expected replicas to be %d, but got %d", replicas, statefulSet.Spec.Replicas)
	}

	containers := statefulSet.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		return fmt.Errorf("expected exactly 1 container, but got %d", len(containers))
	}
	container := containers[0]

	expectedImage := fmt.Sprintf("%s:%s", image, imageTag)
	if container.Image != expectedImage {
		return fmt.Errorf("expected container image '%s', but got '%s'", expectedImage, container.Image)
	}

	if container.ImagePullPolicy != imagePullPolicy {
		return fmt.Errorf("expected image pull policy '%s', but got '%s'", imagePullPolicy, container.ImagePullPolicy)
	}

	err := verifyEtcdServers(&container, name, namespace)
	if err != nil {
		return fmt.Errorf("failed to verify etcd servers %v", err)
	}

	err = verifySecrets(statefulSet, name)
	if err != nil {
		return fmt.Errorf("failed to verify secrets %v", err)
	}

	err = verifyVolumeMounts(&container)
	if err != nil {
		return fmt.Errorf("failed to verify mounts %v", err)
	}

	err = verifyInitialClusters(&container, replicas, name, namespace)
	if err != nil {
		return fmt.Errorf("failed to verify initial clusters %v", err)
	}

	err = verifyEtcdCipherSuite(&container)
	if err != nil {
		return fmt.Errorf("failed to verify etcd cipher suite")
	}

	return nil
}

// verifyEtcdServers checks that the container command includes the correct etcd server argument.
func verifyEtcdServers(container *corev1.Container, name, namespace string) error {
	etcdServersArg := fmt.Sprintf("https://%s.%s.svc.cluster.local:%d,", util.KarmadaEtcdClientName(name), namespace, constants.EtcdListenClientPort)
	etcdServersArg = fmt.Sprintf("--advertise-client-urls=%s", etcdServersArg[:len(etcdServersArg)-1])
	if !contains(container.Command, etcdServersArg) {
		return fmt.Errorf("etcd servers argument '%s' not found in container command", etcdServersArg)
	}

	return nil
}

// verifySecrets validates that the expected secrets are present in the StatefulSet's volumes.
func verifySecrets(statefulSet *appsv1.StatefulSet, name string) error {
	var extractedSecrets []string
	for _, volume := range statefulSet.Spec.Template.Spec.Volumes {
		extractedSecrets = append(extractedSecrets, volume.Secret.SecretName)
	}
	expectedSecrets := []string{util.EtcdCertSecretName(name)}
	for _, expectedSecret := range expectedSecrets {
		if !contains(extractedSecrets, expectedSecret) {
			return fmt.Errorf("expected secret '%s' not found in extracted secrets", expectedSecret)
		}
	}

	return nil
}

// verifyVolumeMounts checks that the expected volume mounts are present in the container.
func verifyVolumeMounts(container *corev1.Container) error {
	var extractedVolumeMounts []string
	for _, volumeMount := range container.VolumeMounts {
		extractedVolumeMounts = append(extractedVolumeMounts, volumeMount.Name)
	}
	expectedVolumeMounts := []string{constants.EtcdDataVolumeName}
	for _, expectedVolumeMount := range expectedVolumeMounts {
		if !contains(extractedVolumeMounts, expectedVolumeMount) {
			return fmt.Errorf("expected volume mount '%s' not found in extracted volume mounts", expectedVolumeMount)
		}
	}

	return nil
}

// verifyInitialClusters validates that the container command includes the correct initial cluster argument.
func verifyInitialClusters(container *corev1.Container, replicas int32, name, namespace string) error {
	expectedInitialClusters := make([]string, replicas)
	for i := 0; i < int(replicas); i++ {
		memberName := fmt.Sprintf("%s-%d", util.KarmadaEtcdName(name), i)
		memberPeerURL := fmt.Sprintf("http://%s.%s.%s.svc.cluster.local:%v",
			memberName,
			util.KarmadaEtcdName(name),
			namespace,
			constants.EtcdListenPeerPort,
		)
		expectedInitialClusters[i] = fmt.Sprintf("%s=%s", memberName, memberPeerURL)
	}
	initialClustersArg := fmt.Sprintf("--initial-cluster=%s", strings.Join(expectedInitialClusters, ","))
	if !contains(container.Command, initialClustersArg) {
		return fmt.Errorf("expected container commands to include '%s', but it was missing", initialClustersArg)
	}

	return nil
}

// verifyEtcdCipherSuite checks that the container command includes the correct cipher suites argument.
func verifyEtcdCipherSuite(container *corev1.Container) error {
	etcdCipherSuitesArg := fmt.Sprintf("--cipher-suites=%s", genEtcdCipherSuites())
	if !contains(container.Command, etcdCipherSuitesArg) {
		return fmt.Errorf("the cipher suites argument '%s' is missing from the container command", etcdCipherSuitesArg)
	}

	for _, command := range container.Command {
		if strings.HasPrefix(command, "--listen-client-urls") && !strings.HasSuffix(command, strconv.Itoa(constants.EtcdListenClientPort)) {
			return fmt.Errorf("expected '--listen-client-urls' command should end with %d", constants.EtcdListenClientPort)
		}

		if strings.HasPrefix(command, "--listen-peer-urls") && !strings.HasSuffix(command, strconv.Itoa(constants.EtcdListenPeerPort)) {
			return fmt.Errorf("expected '--listen-peer-urls' command should end with %d", constants.EtcdListenPeerPort)
		}
	}

	return nil
}

// verifyEtcdPeerOrClientService verifies that the expected ports are present in the etcd peer or client service.
func verifyEtcdPeerOrClientService(service *corev1.Service, expectedPorts []corev1.ServicePort) error {
	for _, servicePortExpected := range expectedPorts {
		found := false
		for _, port := range service.Spec.Ports {
			if port == servicePortExpected {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("expected port %v isn't found in etcd peer service ports", servicePortExpected)
		}
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
