/*
Copyright 2024 The Kubernetes Authors.

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

package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	customapi "k8s.io/metrics/pkg/apis/custom_metrics/v1beta2"
	externalapi "k8s.io/metrics/pkg/apis/external_metrics/v1beta1"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	resourceclient "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	customclient "k8s.io/metrics/pkg/client/custom_metrics"
	externalclient "k8s.io/metrics/pkg/client/external_metrics"
)

// Mock clients and interfaces
type mockResourceClient struct {
	resourceclient.PodMetricsesGetter
}

type mockCustomClient struct {
	customclient.CustomMetricsClient
}

type mockExternalClient struct {
	externalclient.ExternalMetricsClient
}

type mockExternalMetricsClient struct {
	externalclient.ExternalMetricsClient
	metrics *externalapi.ExternalMetricValueList
	err     error
}

type mockExternalMetricsInterface struct {
	externalclient.MetricsInterface
	metrics *externalapi.ExternalMetricValueList
	err     error
}

type mockCustomMetricsClient struct {
	customclient.CustomMetricsClient
	metrics *customapi.MetricValueList
	err     error
}

type mockCustomMetricsInterface struct {
	customclient.MetricsInterface
	metrics *customapi.MetricValueList
	err     error
}

type mockPodMetricsGetter struct {
	metrics *v1beta1.PodMetricsList
	err     error
}

type mockPodMetricsInterface struct {
	resourceclient.PodMetricsInterface
	metrics *v1beta1.PodMetricsList
	err     error
}

func (m *mockExternalMetricsClient) NamespacedMetrics(_ string) externalclient.MetricsInterface {
	return &mockExternalMetricsInterface{metrics: m.metrics, err: m.err}
}

func (m *mockExternalMetricsInterface) List(_ string, _ labels.Selector) (*externalapi.ExternalMetricValueList, error) {
	return m.metrics, m.err
}

func (m *mockCustomMetricsClient) NamespacedMetrics(_ string) customclient.MetricsInterface {
	return &mockCustomMetricsInterface{metrics: m.metrics, err: m.err}
}

func (m *mockCustomMetricsInterface) GetForObjects(_ schema.GroupKind, _ labels.Selector, _ string, _ labels.Selector) (*customapi.MetricValueList, error) {
	return m.metrics, m.err
}

func (m *mockCustomMetricsInterface) GetForObject(_ schema.GroupKind, _ string, _ string, _ labels.Selector) (*customapi.MetricValue, error) {
	if len(m.metrics.Items) > 0 {
		return &m.metrics.Items[0], m.err
	}
	return nil, m.err
}

func (m *mockPodMetricsGetter) PodMetricses(_ string) resourceclient.PodMetricsInterface {
	return &mockPodMetricsInterface{metrics: m.metrics, err: m.err}
}

func (m *mockPodMetricsInterface) List(_ context.Context, _ metav1.ListOptions) (*v1beta1.PodMetricsList, error) {
	return m.metrics, m.err
}

// Test functions

// NewRESTMetricsClient creates a new REST metrics client with the given clients.
func TestNewRESTMetricsClient(t *testing.T) {
	resourceClient := &mockResourceClient{}
	customClient := &mockCustomClient{}
	externalClient := &mockExternalClient{}

	client := NewRESTMetricsClient(resourceClient, customClient, externalClient)

	if client == nil {
		t.Error("Expected non-nil client, got nil")
	}
}

// TestGetResourceMetric tests the GetResourceMetric function with various scenarios.
func TestGetResourceMetric(t *testing.T) {
	tests := []struct {
		name           string
		mockMetrics    *v1beta1.PodMetricsList
		mockError      error
		container      string
		expectedError  string
		expectedResult PodMetricsInfo
	}{
		{
			name: "Successful retrieval",
			mockMetrics: &v1beta1.PodMetricsList{
				Items: []v1beta1.PodMetrics{
					createPodMetrics("pod1", "container1", 100),
				},
			},
			expectedResult: PodMetricsInfo{
				"pod1": {Value: 100},
			},
		},
		{
			name:          "API error",
			mockError:     errors.New("API error"),
			expectedError: "unable to fetch metrics from resource metrics API: API error",
		},
		{
			name:          "Empty metrics",
			mockMetrics:   &v1beta1.PodMetricsList{},
			expectedError: "no metrics returned from resource metrics API",
		},
		{
			name: "Container-specific metrics",
			mockMetrics: &v1beta1.PodMetricsList{
				Items: []v1beta1.PodMetrics{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "default"},
						Containers: []v1beta1.ContainerMetrics{
							createPodMetrics("pod1", "container1", 100).Containers[0],
							createPodMetrics("pod1", "container2", 200).Containers[0],
						},
					},
				},
			},
			container: "container2",
			expectedResult: PodMetricsInfo{
				"pod1": {Value: 200},
			},
		},
		{
			name: "Container not found",
			mockMetrics: &v1beta1.PodMetricsList{
				Items: []v1beta1.PodMetrics{
					createPodMetrics("pod1", "container1", 100),
				},
			},
			container:     "nonexistent",
			expectedError: "failed to get container metrics: container nonexistent not present in metrics for pod default/pod1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := setupMockResourceClient(tt.mockMetrics, tt.mockError)
			result, _, err := client.GetResourceMetric(context.Background(), corev1.ResourceCPU, "default", labels.Everything(), tt.container)

			assertError(t, err, tt.expectedError)
			assertPodMetricsInfoEqual(t, result, tt.expectedResult)
		})
	}
}

// TestGetExternalMetric tests the retrieval of external metrics.
func TestGetExternalMetric(t *testing.T) {
	tests := []struct {
		name           string
		mockMetrics    *externalapi.ExternalMetricValueList
		mockError      error
		expectedValues []int64
		expectedError  string
	}{
		{
			name: "Successful retrieval",
			mockMetrics: &externalapi.ExternalMetricValueList{
				Items: []externalapi.ExternalMetricValue{
					{Value: *resource.NewQuantity(100, resource.DecimalSI)},
					{Value: *resource.NewQuantity(200, resource.DecimalSI)},
				},
			},
			expectedValues: []int64{100000, 200000},
		},
		{
			name:          "API error",
			mockError:     errors.New("API error"),
			expectedError: "unable to fetch metrics from external metrics API: API error",
		},
		{
			name:          "Empty metrics",
			mockMetrics:   &externalapi.ExternalMetricValueList{},
			expectedError: "no metrics returned from external metrics API",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := setupMockExternalClient(tt.mockMetrics, tt.mockError)
			values, _, err := client.GetExternalMetric("test-metric", "default", labels.Everything())

			assertError(t, err, tt.expectedError)
			assertInt64SliceEqual(t, values, tt.expectedValues)
		})
	}
}

// TestGetRawMetric tests the retrieval of raw custom metrics.
func TestGetRawMetric(t *testing.T) {
	tests := []struct {
		name           string
		mockMetrics    *customapi.MetricValueList
		mockError      error
		expectedResult PodMetricsInfo
		expectedError  string
	}{
		{
			name: "Successful retrieval",
			mockMetrics: &customapi.MetricValueList{
				Items: []customapi.MetricValue{
					{
						DescribedObject: corev1.ObjectReference{
							Kind:       "Pod",
							Name:       "pod1",
							APIVersion: "v1",
						},
						Metric: customapi.MetricIdentifier{
							Name: "test-metric",
						},
						Timestamp: metav1.Time{Time: time.Now()},
						Value:     *resource.NewQuantity(100, resource.DecimalSI),
					},
				},
			},
			expectedResult: PodMetricsInfo{
				"pod1": {Value: 100000},
			},
		},
		{
			name:          "API error",
			mockError:     errors.New("API error"),
			expectedError: "unable to fetch metrics from custom metrics API: API error",
		},
		{
			name:          "Empty metrics",
			mockMetrics:   &customapi.MetricValueList{},
			expectedError: "no metrics returned from custom metrics API",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := setupMockCustomClient(tt.mockMetrics, tt.mockError)
			result, _, err := client.GetRawMetric("test-metric", "default", labels.Everything(), labels.Everything())

			assertError(t, err, tt.expectedError)
			assertPodMetricsInfoEqual(t, result, tt.expectedResult)
		})
	}
}

// TestGetObjectMetric tests the retrieval of object-specific custom metrics.
func TestGetObjectMetric(t *testing.T) {
	tests := []struct {
		name          string
		mockMetrics   *customapi.MetricValueList
		mockError     error
		objectRef     *autoscalingv2.CrossVersionObjectReference
		expectedValue int64
		expectedError string
	}{
		{
			name: "Successful retrieval",
			mockMetrics: &customapi.MetricValueList{
				Items: []customapi.MetricValue{
					{
						DescribedObject: corev1.ObjectReference{
							Kind:       "Deployment",
							Name:       "test-deployment",
							APIVersion: "apps/v1",
						},
						Metric: customapi.MetricIdentifier{
							Name: "test-metric",
						},
						Timestamp: metav1.Time{Time: time.Now()},
						Value:     *resource.NewQuantity(100, resource.DecimalSI),
					},
				},
			},
			objectRef: &autoscalingv2.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "test-deployment",
				APIVersion: "apps/v1",
			},
			expectedValue: 100000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := setupMockCustomClient(tt.mockMetrics, tt.mockError)
			value, _, err := client.GetObjectMetric("test-metric", "default", tt.objectRef, labels.Everything())

			assertError(t, err, tt.expectedError)
			assertInt64Equal(t, value, tt.expectedValue)
		})
	}
}

// Helper functions

// createPodMetrics creates a PodMetrics object with specified name, container name, and CPU value
func createPodMetrics(name string, containerName string, cpuValue int64) v1beta1.PodMetrics {
	return v1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Timestamp:  metav1.Time{Time: time.Now()},
		Window:     metav1.Duration{Duration: time.Minute},
		Containers: []v1beta1.ContainerMetrics{
			{
				Name: containerName,
				Usage: corev1.ResourceList{
					corev1.ResourceCPU: *resource.NewMilliQuantity(cpuValue, resource.DecimalSI),
				},
			},
		},
	}
}

// setupMockResourceClient creates a mock resource metrics client for testing
func setupMockResourceClient(mockMetrics *v1beta1.PodMetricsList, mockError error) *resourceMetricsClient {
	mockClient := &mockResourceClient{}
	mockClient.PodMetricsesGetter = &mockPodMetricsGetter{
		metrics: mockMetrics,
		err:     mockError,
	}
	return &resourceMetricsClient{client: mockClient}
}

// setupMockExternalClient creates a mock external metrics client for testing
func setupMockExternalClient(mockMetrics *externalapi.ExternalMetricValueList, mockError error) *externalMetricsClient {
	mockClient := &mockExternalMetricsClient{
		metrics: mockMetrics,
		err:     mockError,
	}
	return &externalMetricsClient{client: mockClient}
}

// setupMockCustomClient creates a mock custom metrics client for testing
func setupMockCustomClient(mockMetrics *customapi.MetricValueList, mockError error) *customMetricsClient {
	mockClient := &mockCustomMetricsClient{
		metrics: mockMetrics,
		err:     mockError,
	}
	return &customMetricsClient{client: mockClient}
}

// assertError checks if the error matches the expected error string
func assertError(t *testing.T, got error, want string) {
	if want == "" {
		if got != nil {
			t.Errorf("Unexpected error: %v", got)
		}
	} else if got == nil || got.Error() != want {
		t.Errorf("Expected error '%s', got '%v'", want, got)
	}
}

// assertPodMetricsInfoEqual compares two PodMetricsInfo objects for equality
func assertPodMetricsInfoEqual(t *testing.T, got, want PodMetricsInfo) {
	if !podMetricsInfoEqual(got, want) {
		t.Errorf("Expected result %v, got %v", want, got)
	}
}

// assertInt64SliceEqual compares two int64 slices for equality
func assertInt64SliceEqual(t *testing.T, got, want []int64) {
	if !int64SliceEqual(got, want) {
		t.Errorf("Expected values %v, got %v", want, got)
	}
}

// assertInt64Equal compares two int64 values for equality
func assertInt64Equal(t *testing.T, got, want int64) {
	if got != want {
		t.Errorf("Expected value %d, got %d", want, got)
	}
}

// int64SliceEqual checks if two int64 slices are equal
func int64SliceEqual(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// podMetricsInfoEqual checks if two PodMetricsInfo objects are equal
func podMetricsInfoEqual(a, b PodMetricsInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || v.Value != bv.Value {
			return false
		}
	}
	return true
}
