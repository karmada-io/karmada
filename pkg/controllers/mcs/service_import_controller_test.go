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

package mcs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

func TestDeleteDerivedService(t *testing.T) {
	tests := []struct {
		name            string
		existingService *corev1.Service
		svcImportName   types.NamespacedName
		mockClientErr   error
		wantErr         bool
	}{
		{
			name: "Successfully delete existing service",
			existingService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "derived-test-service",
					Namespace: "default",
				},
			},
			svcImportName: types.NamespacedName{
				Name:      "test-service",
				Namespace: "default",
			},
			wantErr: false,
		},
		{
			name: "Service doesn't exist",
			svcImportName: types.NamespacedName{
				Name:      "non-existent-service",
				Namespace: "default",
			},
			wantErr: false,
		},
		{
			name: "Error occurs during deletion",
			existingService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "derived-test-service",
					Namespace: "default",
				},
			},
			svcImportName: types.NamespacedName{
				Name:      "test-service",
				Namespace: "default",
			},
			mockClientErr: errors.New("mock deletion error"),
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := setupController(t, tt.existingService, tt.mockClientErr)

			_, err := c.deleteDerivedService(context.Background(), tt.svcImportName)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.existingService != nil {
					assertServiceDeleted(t, c, tt.existingService)
				}
			}
		})
	}
}

func TestDeriveServiceFromServiceImport(t *testing.T) {
	tests := []struct {
		name            string
		existingService *corev1.Service
		svcImport       *mcsv1alpha1.ServiceImport
		wantSvc         *corev1.Service
		wantErr         bool
	}{
		{
			name: "Create new derived service",
			svcImport: &mcsv1alpha1.ServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: mcsv1alpha1.ServiceImportSpec{
					Type: mcsv1alpha1.ClusterSetIP,
					Ports: []mcsv1alpha1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
						},
					},
					IPs: []string{"10.0.0.1"},
				},
			},
			wantSvc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "derived-test-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
						},
					},
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{IP: "10.0.0.1"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Update existing derived service",
			existingService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "derived-test-service",
					Namespace:       "default",
					ResourceVersion: "1000",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.100",
					Type:      corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     8080,
						},
					},
				},
			},
			svcImport: &mcsv1alpha1.ServiceImport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: mcsv1alpha1.ServiceImportSpec{
					Type: mcsv1alpha1.ClusterSetIP,
					Ports: []mcsv1alpha1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
						},
					},
					IPs: []string{"10.0.0.1", "10.0.0.2"},
				},
			},
			wantSvc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "derived-test-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.100",
					Type:      corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
						},
					},
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{IP: "10.0.0.1"},
							{IP: "10.0.0.2"},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := setupControllerWithServiceImport(t, tt.existingService, tt.svcImport)

			err := c.deriveServiceFromServiceImport(context.Background(), tt.svcImport)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assertDerivedService(t, c, tt.wantSvc, tt.existingService)
			}
		})
	}
}

func TestServicePorts(t *testing.T) {
	tests := []struct {
		name      string
		svcImport *mcsv1alpha1.ServiceImport
		want      []corev1.ServicePort
	}{
		{
			name: "Single port",
			svcImport: &mcsv1alpha1.ServiceImport{
				Spec: mcsv1alpha1.ServiceImportSpec{
					Ports: []mcsv1alpha1.ServicePort{
						{
							Name:        "http",
							Protocol:    corev1.ProtocolTCP,
							Port:        80,
							AppProtocol: &[]string{"http"}[0],
						},
					},
				},
			},
			want: []corev1.ServicePort{
				{
					Name:        "http",
					Protocol:    corev1.ProtocolTCP,
					Port:        80,
					AppProtocol: &[]string{"http"}[0],
				},
			},
		},
		{
			name: "Multiple ports",
			svcImport: &mcsv1alpha1.ServiceImport{
				Spec: mcsv1alpha1.ServiceImportSpec{
					Ports: []mcsv1alpha1.ServicePort{
						{
							Name:     "http",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
						},
						{
							Name:     "https",
							Protocol: corev1.ProtocolTCP,
							Port:     443,
						},
					},
				},
			},
			want: []corev1.ServicePort{
				{
					Name:     "http",
					Protocol: corev1.ProtocolTCP,
					Port:     80,
				},
				{
					Name:     "https",
					Protocol: corev1.ProtocolTCP,
					Port:     443,
				},
			},
		}, {
			name: "Empty ServiceImport",
			svcImport: &mcsv1alpha1.ServiceImport{
				Spec: mcsv1alpha1.ServiceImportSpec{
					Ports: []mcsv1alpha1.ServicePort{},
				},
			},
			want: []corev1.ServicePort{},
		},
		{
			name: "Port without name",
			svcImport: &mcsv1alpha1.ServiceImport{
				Spec: mcsv1alpha1.ServiceImportSpec{
					Ports: []mcsv1alpha1.ServicePort{
						{
							Protocol: corev1.ProtocolTCP,
							Port:     8080,
						},
					},
				},
			},
			want: []corev1.ServicePort{
				{
					Protocol: corev1.ProtocolTCP,
					Port:     8080,
				},
			},
		},
		{
			name: "Different protocols",
			svcImport: &mcsv1alpha1.ServiceImport{
				Spec: mcsv1alpha1.ServiceImportSpec{
					Ports: []mcsv1alpha1.ServicePort{
						{
							Name:     "tcp-port",
							Protocol: corev1.ProtocolTCP,
							Port:     80,
						},
						{
							Name:     "udp-port",
							Protocol: corev1.ProtocolUDP,
							Port:     53,
						},
						{
							Name:     "sctp-port",
							Protocol: corev1.ProtocolSCTP,
							Port:     9999,
						},
					},
				},
			},
			want: []corev1.ServicePort{
				{
					Name:     "tcp-port",
					Protocol: corev1.ProtocolTCP,
					Port:     80,
				},
				{
					Name:     "udp-port",
					Protocol: corev1.ProtocolUDP,
					Port:     53,
				},
				{
					Name:     "sctp-port",
					Protocol: corev1.ProtocolSCTP,
					Port:     9999,
				},
			},
		},
		{
			name: "Nil AppProtocol",
			svcImport: &mcsv1alpha1.ServiceImport{
				Spec: mcsv1alpha1.ServiceImportSpec{
					Ports: []mcsv1alpha1.ServicePort{
						{
							Name:        "http",
							Protocol:    corev1.ProtocolTCP,
							Port:        80,
							AppProtocol: nil,
						},
					},
				},
			},
			want: []corev1.ServicePort{
				{
					Name:        "http",
					Protocol:    corev1.ProtocolTCP,
					Port:        80,
					AppProtocol: nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := servicePorts(tt.svcImport)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRetainServiceFields(t *testing.T) {
	tests := []struct {
		name    string
		oldSvc  *corev1.Service
		newSvc  *corev1.Service
		wantSvc *corev1.Service
	}{
		{
			name: "Retain ClusterIP and ResourceVersion",
			oldSvc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.1",
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "1000",
				},
			},
			newSvc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "",
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "",
				},
			},
			wantSvc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.1",
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "1000",
				},
			},
		},
		{
			name: "Empty old service",
			oldSvc: &corev1.Service{
				Spec:       corev1.ServiceSpec{},
				ObjectMeta: metav1.ObjectMeta{},
			},
			newSvc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.2",
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "2000",
				},
			},
			wantSvc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "",
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "",
				},
			},
		},
		{
			name: "Empty new service",
			oldSvc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.3",
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "3000",
				},
			},
			newSvc: &corev1.Service{
				Spec:       corev1.ServiceSpec{},
				ObjectMeta: metav1.ObjectMeta{},
			},
			wantSvc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.3",
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "3000",
				},
			},
		},
		{
			name: "Overwrite existing values",
			oldSvc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.4",
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "4000",
				},
			},
			newSvc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.5",
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "5000",
				},
			},
			wantSvc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.4",
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "4000",
				},
			},
		},
		{
			name: "Retain only ClusterIP",
			oldSvc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.6",
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "",
				},
			},
			newSvc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "",
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "6000",
				},
			},
			wantSvc: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.0.0.6",
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retainServiceFields(tt.oldSvc, tt.newSvc)
			assert.Equal(t, tt.wantSvc, tt.newSvc)
		})
	}
}

// Helper functions

func setupController(t *testing.T, existingService *corev1.Service, mockClientErr error) *ServiceImportController {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	objs := []client.Object{}
	if existingService != nil {
		objs = append(objs, existingService)
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

	mockClient := &mockClient{
		Client:        fakeClient,
		mockDeleteErr: mockClientErr,
	}

	return &ServiceImportController{
		Client:        mockClient,
		EventRecorder: &record.FakeRecorder{},
	}
}

func setupControllerWithServiceImport(t *testing.T, existingService *corev1.Service, svcImport *mcsv1alpha1.ServiceImport) *ServiceImportController {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, mcsv1alpha1.AddToScheme(scheme))

	objs := []client.Object{svcImport}
	if existingService != nil {
		objs = append(objs, existingService)
	}

	return &ServiceImportController{
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build(),
		EventRecorder: &record.FakeRecorder{},
	}
}

func assertServiceDeleted(t *testing.T, c *ServiceImportController, service *corev1.Service) {
	svc := &corev1.Service{}
	err := c.Client.Get(context.Background(), types.NamespacedName{
		Name:      service.Name,
		Namespace: service.Namespace,
	}, svc)
	assert.True(t, apierrors.IsNotFound(err), "Service should be deleted")
}

func assertDerivedService(t *testing.T, c *ServiceImportController, wantSvc, existingService *corev1.Service) {
	gotSvc := &corev1.Service{}
	err := c.Client.Get(context.Background(), types.NamespacedName{
		Name:      wantSvc.Name,
		Namespace: wantSvc.Namespace,
	}, gotSvc)
	require.NoError(t, err, "Failed to get derived service")

	assert.Equal(t, wantSvc.Spec, gotSvc.Spec, "Derived service spec mismatch")
	assert.Equal(t, wantSvc.Status, gotSvc.Status, "Derived service status mismatch")

	if existingService != nil {
		assert.Equal(t, existingService.Spec.ClusterIP, gotSvc.Spec.ClusterIP, "ClusterIP not retained")
		assert.Greater(t, gotSvc.ResourceVersion, existingService.ResourceVersion, "ResourceVersion not updated")
	}
}

// Mock implementations

// mockClient is a mock implementation of client.Client
type mockClient struct {
	client.Client
	mockDeleteErr error
}

// Delete implements client.Client
func (m *mockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if m.mockDeleteErr != nil {
		return m.mockDeleteErr
	}
	return m.Client.Delete(ctx, obj, opts...)
}

// Watch implements client.WithWatch
func (m *mockClient) Watch(ctx context.Context, list client.ObjectList, opts ...client.ListOption) (watch.Interface, error) {
	return m.Client.(client.WithWatch).Watch(ctx, list, opts...)
}
