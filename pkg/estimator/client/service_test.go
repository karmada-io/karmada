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

package client

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResolveCluster(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		id          string
		port        int32
		service     *corev1.Service
		expectError bool
		expected    []string
	}{
		{
			name:      "Service not found",
			namespace: "default",
			id:        "nonexistent",
			port:      80,
			service:   nil,
			expected:  []string{"nonexistent.default.svc.cluster.local:80", "nonexistent.default.svc:80"},
		},
		{
			name:      "Unsupported service type",
			namespace: "default",
			id:        "myservice",
			port:      80,
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myservice",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
				},
			},
			expectError: true,
		},
		{
			name:      "ExternalName service with int target port",
			namespace: "default",
			id:        "myservice",
			port:      80,
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myservice",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type:         corev1.ServiceTypeExternalName,
					ExternalName: "example.com",
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			},
			expected: []string{"example.com:8080"},
		},
		{
			name:      "ExternalName service with non-int target port",
			namespace: "default",
			id:        "myservice",
			port:      80,
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myservice",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type:         corev1.ServiceTypeExternalName,
					ExternalName: "example.com",
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromString("http"),
						},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			if tt.service != nil {
				_, err := clientset.CoreV1().Services(tt.namespace).Create(context.TODO(), tt.service, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create service: %v", err)
				}
			}

			result, err := resolveCluster(clientset, tt.namespace, tt.id, tt.port)
			if (err != nil) != tt.expectError {
				t.Errorf("expected error: %v, got: %v", tt.expectError, err)
			}
			if !reflect.DeepEqual(tt.expected, result) {
				t.Errorf("expected: %v, got: %v", tt.expected, result)
			}
		})
	}
}

func TestFindServicePort(t *testing.T) {
	tests := []struct {
		name        string
		service     *corev1.Service
		port        int32
		expectError bool
	}{
		{
			name: "Port found",
			service: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Port: 80},
					},
				},
			},
			port: 80,
		},
		{
			name: "Port not found",
			service: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Port: 8080},
					},
				},
			},
			port:        80,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := findServicePort(tt.service, tt.port)
			if (err != nil) != tt.expectError {
				t.Errorf("expected error: %v, got: %v", tt.expectError, err)
			}
		})
	}
}
