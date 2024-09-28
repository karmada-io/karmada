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

package apiclient

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// MockK8SRESTClient is a struct that implements clientset.Interface.
type MockK8SRESTClient struct {
	clientset.Interface
	RESTClientConnector rest.Interface
}

// Discovery returns a mocked discovery interface.
func (m *MockK8SRESTClient) Discovery() discovery.DiscoveryInterface {
	return &MockDiscovery{
		RESTClientConnector: m.RESTClientConnector,
	}
}

// MockDiscovery is a mock implementation of DiscoveryInterface.
type MockDiscovery struct {
	discovery.DiscoveryInterface
	RESTClientConnector rest.Interface
}

// RESTClient returns a restClientConnector that is used to communicate with API server
// by this client implementation.
func (m *MockDiscovery) RESTClient() rest.Interface {
	return m.RESTClientConnector
}

// CreatePods creates a specified number of pods in the given namespace
// with the provided component name and optional labels. It uses a
// Kubernetes client to interact with the API and can mark the pods as
// running if the `markRunningState` flag is set.
//
// Parameters:
//   - client: Kubernetes client interface for API requests.
//   - namespace: Namespace for pod creation.
//   - componentName: Base name for the pods and their containers.
//   - replicaCount: Number of pods to create.
//   - labels: Labels to apply to the pods.
//   - markRunningState: If true, updates the pods' status to running.
//
// Returns:
//   - A slice of pointers to corev1.Pod representing the created pods.
//   - An error if pod creation fails.
func CreatePods(client clientset.Interface, namespace string, componentName string, replicaCount int32, labels map[string]string, markRunningState bool) ([]*corev1.Pod, error) {
	pods := make([]*corev1.Pod, 0, replicaCount)
	for i := int32(0); i < replicaCount; i++ {
		podName := fmt.Sprintf("%s-pod-%d", componentName, i)
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
				Labels:    labels,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  fmt.Sprintf("my-%s-container-%d", componentName, i),
					Image: fmt.Sprintf("my-%s-image:latest", componentName),
					Ports: []corev1.ContainerPort{{ContainerPort: 80}},
				}},
			},
		}
		_, err := client.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create pod %s: %w", podName, err)
		}

		// Mark the pod as in running state if flag is set.
		if markRunningState {
			if err := UpdatePodStatus(client, pod); err != nil {
				return nil, fmt.Errorf("failed to update pod status, got err: %v", err)
			}
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

// UpdatePodStatus updates the status of a pod to PodRunning and sets the PodReady condition.
func UpdatePodStatus(client clientset.Interface, pod *corev1.Pod) error {
	// Mark the pod as in running state.
	pod.Status = corev1.PodStatus{
		Phase: corev1.PodRunning,
		Conditions: []corev1.PodCondition{
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
		},
	}

	// Update the pod status in the Kubernetes cluster.
	_, err := client.CoreV1().Pods(pod.GetNamespace()).UpdateStatus(context.TODO(), pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update status of the pod %s: %w", pod.GetName(), err)
	}

	return nil
}
