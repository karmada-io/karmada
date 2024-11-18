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

package deinit

import (
	"context"
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"

	"github.com/karmada-io/karmada/pkg/util/names"
)

type resource struct {
	name         string
	resourceType string
}

func TestDeInitKarmada(t *testing.T) {
	labels := map[string]string{karmadaBootstrappingLabelKey: "app-defaults"}
	tests := []struct {
		name       string
		deInitOpts *CommandDeInitOption
		resources  []resource
		prep       func(client clientset.Interface, namespace string, resources []resource) error
		verify     func(client clientset.Interface, namespace string, resources []resource) error
		wantErr    bool
	}{
		{
			name: "DeInitKarmada_DeInitKarmadaAfterItIsInitialized_KarmadaDeinit",
			deInitOpts: &CommandDeInitOption{
				KubeClientSet:  fakeclientset.NewClientset(),
				Namespace:      names.NamespaceKarmadaSystem,
				PurgeNamespace: true,
				Force:          true,
			},
			resources: []resource{
				{"test-deployment-1", "deployment"},
				{"test-deployment-2", "deployment"},
				{"test-statefulset-1", "statefulset"},
				{"test-statefulset-2", "statefulset"},
				{"test-service-1", "service"},
				{"test-service-2", "service"},
				{"test-secret-1", "secret"},
				{"test-secret-2", "secret"},
				{"test-node-1", "node"},
				{"test-node-2", "node"},
			},
			prep: func(client clientset.Interface, namespace string, resources []resource) error {
				ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
				if ns, err := client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{}); err != nil {
					return fmt.Errorf("failed to create karmada namespace %s, got error: %v", ns.GetName(), err)
				}
				for _, resource := range resources {
					if err := createResource(client, namespace, resource.name, labels, resource.resourceType); err != nil {
						return fmt.Errorf("failed to create %s %s in namespace %s, got error: %v", resource.resourceType, resource.name, namespace, err)
					}
				}
				return nil
			},
			verify: func(client clientset.Interface, namespace string, resources []resource) error {
				for _, resource := range resources {
					if err := verifyResource(client, namespace, resource.name, resource.resourceType); err != nil {
						return err
					}
				}
				if ns, err := client.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{}); err == nil {
					return fmt.Errorf("expected namespace %s to be deleted, but it still exists", ns.GetName())
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.deInitOpts.KubeClientSet, test.deInitOpts.Namespace, test.resources); err != nil {
				t.Fatalf("failed to prep test environment, got: %v", err)
			}
			err := test.deInitOpts.Run()
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err := test.verify(test.deInitOpts.KubeClientSet, test.deInitOpts.Namespace, test.resources); err != nil {
				t.Errorf("failed to verify the deinit of Karmada, got error: %v", err)
			}
		})
	}
}

// createResource creates a Kubernetes resource of the specified type in the given namespace.
// Supported resource types include "deployment", "statefulset", "service", "secret", and "node".
// It sets the provided name and labels for the resource.
// Returns an error if the resource creation fails or if the resource type is unsupported.
func createResource(client clientset.Interface, namespace, name string, labels map[string]string, resourceType string) error {
	switch resourceType {
	case "deployment":
		resource := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
		}
		_, err := client.AppsV1().Deployments(namespace).Create(context.TODO(), resource, metav1.CreateOptions{})
		return err
	case "statefulset":
		resource := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
		}
		_, err := client.AppsV1().StatefulSets(namespace).Create(context.TODO(), resource, metav1.CreateOptions{})
		return err
	case "service":
		resource := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
		}
		_, err := client.CoreV1().Services(namespace).Create(context.TODO(), resource, metav1.CreateOptions{})
		return err
	case "secret":
		resource := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
		}
		_, err := client.CoreV1().Secrets(namespace).Create(context.TODO(), resource, metav1.CreateOptions{})
		return err
	case "node":
		resource := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{karmadaNodeLabel: ""},
			},
		}
		_, err := client.CoreV1().Nodes().Create(context.TODO(), resource, metav1.CreateOptions{})
		return err
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

// getResource retrieves a Kubernetes resource of the specified type from the given namespace.
// Supported resource types include "deployment", "statefulset", "service", "secret", and "node".
// For "node" resources, checks if the expected label is removed and returns an error if not.
// Returns an error if the resource retrieval fails or if the resource type is unsupported.
func getResource(client clientset.Interface, namespace, name string, resourceType string) error {
	switch resourceType {
	case "deployment":
		_, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		return err
	case "statefulset":
		_, err := client.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		return err
	case "service":
		_, err := client.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		return err
	case "secret":
		_, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		return err
	case "node":
		node, err := client.CoreV1().Nodes().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if _, ok := node.Labels[karmadaNodeLabel]; ok {
			return fmt.Errorf("expected node label %s to be removed from node %s", karmadaNodeLabel, name)
		}
		return nil
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

// verifyResource ensures the specified Kubernetes resource is deleted, except for "node" resources,
// where it checks for errors in retrieval. Returns an error if the resource still exists or verification fails.
func verifyResource(client clientset.Interface, namespace, resourceName, resourceType string) error {
	err := getResource(client, namespace, resourceName, resourceType)
	if resourceType != "node" && err == nil {
		return fmt.Errorf("expected %s %s in namespace %s to be deleted, but it still exists", resourceType, resourceName, namespace)
	}
	if resourceType == "node" && err != nil {
		return err
	}
	return nil
}
