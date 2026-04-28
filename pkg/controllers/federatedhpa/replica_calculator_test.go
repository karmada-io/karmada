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

package federatedhpa

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"

	metricsclient "github.com/karmada-io/karmada/pkg/controllers/federatedhpa/metrics"
)

// MockQueryClient implements a mock for the metrics client
type MockQueryClient struct {
	mock.Mock
}

func (m *MockQueryClient) GetResourceMetric(ctx context.Context, resource corev1.ResourceName, namespace string, selector labels.Selector, container string) (metricsclient.PodMetricsInfo, time.Time, error) {
	args := m.Called(ctx, resource, namespace, selector, container)
	return args.Get(0).(metricsclient.PodMetricsInfo), args.Get(1).(time.Time), args.Error(2)
}

func (m *MockQueryClient) GetRawMetric(metricName string, namespace string, selector labels.Selector, metricSelector labels.Selector) (metricsclient.PodMetricsInfo, time.Time, error) {
	args := m.Called(metricName, namespace, selector, metricSelector)
	return args.Get(0).(metricsclient.PodMetricsInfo), args.Get(1).(time.Time), args.Error(2)
}

func (m *MockQueryClient) GetObjectMetric(metricName string, namespace string, objectRef *autoscalingv2.CrossVersionObjectReference, metricSelector labels.Selector) (int64, time.Time, error) {
	args := m.Called(metricName, namespace, objectRef, metricSelector)
	return args.Get(0).(int64), args.Get(1).(time.Time), args.Error(2)
}

func (m *MockQueryClient) GetExternalMetric(metricName string, namespace string, selector labels.Selector) ([]int64, time.Time, error) {
	args := m.Called(metricName, namespace, selector)
	return args.Get(0).([]int64), args.Get(1).(time.Time), args.Error(2)
}

// TestNewReplicaCalculator verifies the creation of a new ReplicaCalculator
func TestNewReplicaCalculator(t *testing.T) {
	const (
		defaultTolerance                = 0.1
		defaultCPUInitPeriod            = 5 * time.Minute
		defaultDelayInitReadinessStatus = 30 * time.Second
	)

	tests := []struct {
		name                     string
		tolerance                float64
		cpuInitPeriod            time.Duration
		delayInitReadinessStatus time.Duration
	}{
		{
			name:                     "Default values",
			tolerance:                defaultTolerance,
			cpuInitPeriod:            defaultCPUInitPeriod,
			delayInitReadinessStatus: defaultDelayInitReadinessStatus,
		},
		{
			name:                     "Zero values",
			tolerance:                0,
			cpuInitPeriod:            0,
			delayInitReadinessStatus: 0,
		},
		{
			name:                     "Custom values",
			tolerance:                0.2,
			cpuInitPeriod:            10 * time.Minute,
			delayInitReadinessStatus: 1 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockQueryClient{}
			calculator := NewReplicaCalculator(mockClient,
				tt.tolerance,
				tt.cpuInitPeriod,
				tt.delayInitReadinessStatus)

			assert.NotNil(t, calculator, "Calculator should not be nil")
			assert.Equal(t, mockClient, calculator.metricsClient, "Metrics client should match")
			assert.Equal(t, tt.tolerance, calculator.tolerance, "Tolerance should match")
			assert.Equal(t, tt.cpuInitPeriod, calculator.cpuInitializationPeriod, "CPU initialization period should match")
			assert.Equal(t, tt.delayInitReadinessStatus, calculator.delayOfInitialReadinessStatus, "Delay of initial readiness status should match")
		})
	}
}

// TestGetResourceReplicas checks the calculation of resource-based replicas
func TestGetResourceReplicas(t *testing.T) {
	const (
		defaultNamespace   = "default"
		defaultContainer   = ""
		defaultCalibration = 1.0
		defaultTolerance   = 0.1
	)

	mockTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	testCases := []struct {
		name                   string
		currentReplicas        int32
		targetUtilization      int32
		pods                   []*corev1.Pod
		metrics                metricsclient.PodMetricsInfo
		expectedReplicas       int32
		expectedUtilization    int32
		expectedRawUtilization int64
		calibration            float64
		tolerance              float64
		expectError            bool
	}{
		{
			name:              "Scale up",
			currentReplicas:   2,
			targetUtilization: 50,
			pods: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 150},
				"pod2": {Value: 150},
			},
			expectedReplicas:       6,
			expectedUtilization:    150,
			expectedRawUtilization: 150,
			calibration:            defaultCalibration,
			tolerance:              defaultTolerance,
		},
		{
			name:              "Scale down",
			currentReplicas:   4,
			targetUtilization: 50,
			pods: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
				createPod("pod3", 100, 200),
				createPod("pod4", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 50},
				"pod2": {Value: 50},
				"pod3": {Value: 50},
				"pod4": {Value: 50},
			},
			expectedReplicas:       4,
			expectedUtilization:    50,
			expectedRawUtilization: 50,
			calibration:            defaultCalibration,
			tolerance:              defaultTolerance,
		},
		{
			name:              "No change (within tolerance)",
			currentReplicas:   2,
			targetUtilization: 50,
			pods: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 52},
				"pod2": {Value: 48},
			},
			expectedReplicas:       2,
			expectedUtilization:    50,
			expectedRawUtilization: 50,
			calibration:            defaultCalibration,
			tolerance:              defaultTolerance,
		},
		{
			name:              "Scale up with unready pods",
			currentReplicas:   3,
			targetUtilization: 50,
			pods: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
				createUnreadyPod("pod3", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 150},
				"pod2": {Value: 150},
			},
			expectedReplicas:       6,
			expectedUtilization:    150,
			expectedRawUtilization: 150,
			calibration:            defaultCalibration,
			tolerance:              defaultTolerance,
		},
		{
			name:              "Scale with calibration",
			currentReplicas:   2,
			targetUtilization: 50,
			pods: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 150},
				"pod2": {Value: 150},
			},
			expectedReplicas:       12,
			expectedUtilization:    150,
			expectedRawUtilization: 150,
			calibration:            0.5,
			tolerance:              defaultTolerance,
		},
		{
			name:              "Error: No pods",
			currentReplicas:   2,
			targetUtilization: 50,
			pods:              []*corev1.Pod{},
			metrics:           metricsclient.PodMetricsInfo{},
			expectError:       true,
		},
		{
			name:              "Error: No metrics for ready pods",
			currentReplicas:   2,
			targetUtilization: 50,
			pods: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			metrics:     metricsclient.PodMetricsInfo{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &MockQueryClient{}
			calculator := NewReplicaCalculator(mockClient,
				tc.tolerance,
				5*time.Minute,
				30*time.Second)

			mockClient.On("GetResourceMetric", mock.Anything, corev1.ResourceCPU, defaultNamespace, labels.Everything(), defaultContainer).
				Return(tc.metrics, mockTime, nil).Once()

			replicas, utilization, rawUtilization, timestamp, err := calculator.GetResourceReplicas(
				context.Background(), tc.currentReplicas, tc.targetUtilization, corev1.ResourceCPU,
				defaultNamespace, labels.Everything(), defaultContainer, tc.pods, tc.calibration)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedReplicas, replicas)
				assert.Equal(t, tc.expectedUtilization, utilization)
				assert.Equal(t, tc.expectedRawUtilization, rawUtilization)
				assert.Equal(t, mockTime, timestamp)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestGetRawResourceReplicas verifies the calculation of raw resource-based replicas
func TestGetRawResourceReplicas(t *testing.T) {
	const (
		defaultNamespace              = "default"
		defaultContainer              = ""
		cpuInitializationPeriod       = 5 * time.Minute
		delayOfInitialReadinessStatus = 30 * time.Second
	)

	mockTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	testCases := []struct {
		name             string
		currentReplicas  int32
		targetUsage      int64
		resource         corev1.ResourceName
		namespace        string
		selector         labels.Selector
		container        string
		podList          []*corev1.Pod
		metrics          metricsclient.PodMetricsInfo
		calibration      float64
		expectedReplicas int32
		expectedUsage    int64
		expectError      bool
	}{
		{
			name:            "Scale up based on raw metrics",
			currentReplicas: 2,
			targetUsage:     100,
			resource:        corev1.ResourceCPU,
			namespace:       defaultNamespace,
			selector:        labels.Everything(),
			container:       defaultContainer,
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 150},
				"pod2": {Value: 150},
			},
			calibration:      1.0,
			expectedReplicas: 3,
			expectedUsage:    150,
			expectError:      false,
		},
		{
			name:            "Scale down based on raw metrics",
			currentReplicas: 4,
			targetUsage:     100,
			resource:        corev1.ResourceCPU,
			namespace:       defaultNamespace,
			selector:        labels.Everything(),
			container:       defaultContainer,
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
				createPod("pod3", 100, 200),
				createPod("pod4", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 50},
				"pod2": {Value: 50},
				"pod3": {Value: 50},
				"pod4": {Value: 50},
			},
			calibration:      1.0,
			expectedReplicas: 2,
			expectedUsage:    50,
			expectError:      false,
		},
		{
			name:            "No change (at target usage)",
			currentReplicas: 2,
			targetUsage:     100,
			resource:        corev1.ResourceCPU,
			namespace:       defaultNamespace,
			selector:        labels.Everything(),
			container:       defaultContainer,
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 100},
				"pod2": {Value: 100},
			},
			calibration:      1.0,
			expectedReplicas: 2,
			expectedUsage:    100,
			expectError:      false,
		},
		{
			name:            "Scale with calibration",
			currentReplicas: 2,
			targetUsage:     100,
			resource:        corev1.ResourceCPU,
			namespace:       defaultNamespace,
			selector:        labels.Everything(),
			container:       defaultContainer,
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 150},
				"pod2": {Value: 150},
			},
			calibration:      0.8,
			expectedReplicas: 4,
			expectedUsage:    150,
			expectError:      false,
		},
		{
			name:            "Error: No pods",
			currentReplicas: 2,
			targetUsage:     100,
			resource:        corev1.ResourceCPU,
			namespace:       defaultNamespace,
			selector:        labels.Everything(),
			container:       defaultContainer,
			podList:         []*corev1.Pod{},
			metrics:         metricsclient.PodMetricsInfo{},
			calibration:     1.0,
			expectError:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &MockQueryClient{}
			calculator := NewReplicaCalculator(
				mockClient,
				0.1,
				cpuInitializationPeriod,
				delayOfInitialReadinessStatus,
			)

			mockClient.On("GetResourceMetric",
				mock.Anything,
				tc.resource,
				tc.namespace,
				tc.selector,
				tc.container,
			).Return(tc.metrics, mockTime, nil).Once()

			replicas, usage, timestamp, err := calculator.GetRawResourceReplicas(
				context.Background(),
				tc.currentReplicas,
				tc.targetUsage,
				tc.resource,
				tc.namespace,
				tc.selector,
				tc.container,
				tc.podList,
				tc.calibration,
			)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedReplicas, replicas)
				assert.Equal(t, tc.expectedUsage, usage)
				assert.Equal(t, mockTime, timestamp)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestGetMetricReplicas checks the calculation of metric-based replicas
func TestGetMetricReplicas(t *testing.T) {
	const (
		defaultNamespace              = "default"
		cpuInitializationPeriod       = 5 * time.Minute
		delayOfInitialReadinessStatus = 30 * time.Second
	)

	mockTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	testCases := []struct {
		name             string
		currentReplicas  int32
		targetUsage      int64
		metricName       string
		namespace        string
		selector         labels.Selector
		metricSelector   labels.Selector
		podList          []*corev1.Pod
		metrics          metricsclient.PodMetricsInfo
		calibration      float64
		expectedReplicas int32
		expectedUsage    int64
		expectError      bool
	}{
		{
			name:            "Scale up based on custom metrics",
			currentReplicas: 2,
			targetUsage:     10,
			metricName:      "custom_metric",
			namespace:       defaultNamespace,
			selector:        labels.Everything(),
			metricSelector:  labels.Everything(),
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 15},
				"pod2": {Value: 15},
			},
			calibration:      1.0,
			expectedReplicas: 3,
			expectedUsage:    15,
			expectError:      false,
		},
		{
			name:            "Scale down based on custom metrics",
			currentReplicas: 4,
			targetUsage:     20,
			metricName:      "custom_metric",
			namespace:       defaultNamespace,
			selector:        labels.Everything(),
			metricSelector:  labels.Everything(),
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
				createPod("pod3", 100, 200),
				createPod("pod4", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 10},
				"pod2": {Value: 10},
				"pod3": {Value: 10},
				"pod4": {Value: 10},
			},
			calibration:      1.0,
			expectedReplicas: 2,
			expectedUsage:    10,
			expectError:      false,
		},
		{
			name:            "No change (at target usage)",
			currentReplicas: 2,
			targetUsage:     15,
			metricName:      "custom_metric",
			namespace:       defaultNamespace,
			selector:        labels.Everything(),
			metricSelector:  labels.Everything(),
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 15},
				"pod2": {Value: 15},
			},
			calibration:      1.0,
			expectedReplicas: 2,
			expectedUsage:    15,
			expectError:      false,
		},
		{
			name:            "Scale with calibration",
			currentReplicas: 2,
			targetUsage:     10,
			metricName:      "custom_metric",
			namespace:       defaultNamespace,
			selector:        labels.Everything(),
			metricSelector:  labels.Everything(),
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 15},
				"pod2": {Value: 15},
			},
			calibration:      0.8,
			expectedReplicas: 4,
			expectedUsage:    15,
			expectError:      false,
		},
		{
			name:            "Error: No metrics",
			currentReplicas: 2,
			targetUsage:     10,
			metricName:      "custom_metric",
			namespace:       defaultNamespace,
			selector:        labels.Everything(),
			metricSelector:  labels.Everything(),
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			metrics:     metricsclient.PodMetricsInfo{},
			calibration: 1.0,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &MockQueryClient{}
			calculator := NewReplicaCalculator(
				mockClient,
				0.1,
				cpuInitializationPeriod,
				delayOfInitialReadinessStatus,
			)

			mockClient.On("GetRawMetric",
				tc.metricName,
				tc.namespace,
				tc.selector,
				tc.metricSelector,
			).Return(tc.metrics, mockTime, nil).Once()

			replicas, usage, timestamp, err := calculator.GetMetricReplicas(
				tc.currentReplicas,
				tc.targetUsage,
				tc.metricName,
				tc.namespace,
				tc.selector,
				tc.metricSelector,
				tc.podList,
				tc.calibration,
			)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedReplicas, replicas)
				assert.Equal(t, tc.expectedUsage, usage)
				assert.Equal(t, mockTime, timestamp)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestCalcPlainMetricReplicas verifies the calculation of plain metric-based replicas
func TestCalcPlainMetricReplicas(t *testing.T) {
	const (
		cpuInitializationPeriod       = 5 * time.Minute
		delayOfInitialReadinessStatus = 30 * time.Second
		defaultTolerance              = 0.1
	)

	testCases := []struct {
		name             string
		metrics          metricsclient.PodMetricsInfo
		currentReplicas  int32
		targetUsage      int64
		resource         corev1.ResourceName
		podList          []*corev1.Pod
		calibration      float64
		expectedReplicas int32
		expectedUsage    int64
		expectError      bool
	}{
		{
			name: "Scale up",
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 15},
				"pod2": {Value: 15},
			},
			currentReplicas: 2,
			targetUsage:     10,
			resource:        corev1.ResourceCPU,
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			calibration:      1.0,
			expectedReplicas: 3,
			expectedUsage:    15,
			expectError:      false,
		},
		{
			name: "Scale down",
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 5},
				"pod2": {Value: 5},
				"pod3": {Value: 5},
				"pod4": {Value: 5},
			},
			currentReplicas: 4,
			targetUsage:     10,
			resource:        corev1.ResourceCPU,
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
				createPod("pod3", 100, 200),
				createPod("pod4", 100, 200),
			},
			calibration:      1.0,
			expectedReplicas: 2,
			expectedUsage:    5,
			expectError:      false,
		},
		{
			name: "No change (within tolerance)",
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 10},
				"pod2": {Value: 10},
			},
			currentReplicas: 2,
			targetUsage:     10,
			resource:        corev1.ResourceCPU,
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			calibration:      1.0,
			expectedReplicas: 2,
			expectedUsage:    10,
			expectError:      false,
		},
		{
			name: "Scale up with unready pods",
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 15},
				"pod2": {Value: 15},
			},
			currentReplicas: 3,
			targetUsage:     10,
			resource:        corev1.ResourceCPU,
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
				createUnreadyPod("pod3", 100, 200),
			},
			calibration:      1.0,
			expectedReplicas: 3,
			expectedUsage:    15,
			expectError:      false,
		},
		{
			name: "Scale down with missing pods",
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 5},
				"pod2": {Value: 5},
			},
			currentReplicas: 3,
			targetUsage:     10,
			resource:        corev1.ResourceCPU,
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
				createPod("pod3", 100, 200),
			},
			calibration:      1.0,
			expectedReplicas: 2,
			expectedUsage:    5,
			expectError:      false,
		},
		{
			name: "Scale with calibration",
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 15},
				"pod2": {Value: 15},
			},
			currentReplicas: 2,
			targetUsage:     10,
			resource:        corev1.ResourceCPU,
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			calibration:      0.8,
			expectedReplicas: 4,
			expectedUsage:    15,
			expectError:      false,
		},
		{
			name:            "Error: No pods",
			metrics:         metricsclient.PodMetricsInfo{},
			currentReplicas: 2,
			targetUsage:     10,
			resource:        corev1.ResourceCPU,
			podList:         []*corev1.Pod{},
			calibration:     1.0,
			expectError:     true,
		},
		{
			name:            "Error: No metrics for ready pods",
			metrics:         metricsclient.PodMetricsInfo{},
			currentReplicas: 2,
			targetUsage:     10,
			resource:        corev1.ResourceCPU,
			podList: []*corev1.Pod{
				createUnreadyPod("pod1", 100, 200),
				createUnreadyPod("pod2", 100, 200),
			},
			calibration: 1.0,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			calculator := NewReplicaCalculator(
				nil, // metrics client not needed for this test
				defaultTolerance,
				cpuInitializationPeriod,
				delayOfInitialReadinessStatus,
			)

			replicas, usage, err := calculator.calcPlainMetricReplicas(
				tc.metrics,
				tc.currentReplicas,
				tc.targetUsage,
				tc.resource,
				tc.podList,
				tc.calibration,
			)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedReplicas, replicas)
				assert.Equal(t, tc.expectedUsage, usage)
			}
		})
	}
}

// Helper function to create an unready pod
func createUnreadyPod(name string, request, limit int64) *corev1.Pod {
	pod := createPod(name, request, limit)
	pod.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionFalse,
		},
	}
	return pod
}

func TestGetObjectMetricReplicas(t *testing.T) {
	mockClient := &MockQueryClient{}
	calculator := NewReplicaCalculator(mockClient, 0.1, 5*time.Minute, 30*time.Second)

	testCases := []struct {
		name             string
		currentReplicas  int32
		targetUsage      int64
		metricName       string
		namespace        string
		objectRef        *autoscalingv2.CrossVersionObjectReference
		metricSelector   labels.Selector
		podList          []*corev1.Pod
		objectMetric     int64
		calibration      float64
		expectedReplicas int32
		expectedError    bool
	}{
		{
			name:            "Scale up based on object metrics",
			currentReplicas: 2,
			targetUsage:     10,
			metricName:      "queue_length",
			namespace:       "default",
			objectRef:       &autoscalingv2.CrossVersionObjectReference{Kind: "Service", Name: "my-svc"},
			metricSelector:  labels.Everything(),
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			objectMetric:     30,
			calibration:      1.0,
			expectedReplicas: 6,
			expectedError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient.On("GetObjectMetric", tc.metricName, tc.namespace, tc.objectRef, tc.metricSelector).Return(tc.objectMetric, time.Now(), nil).Once()

			replicas, _, _, err := calculator.GetObjectMetricReplicas(tc.currentReplicas, tc.targetUsage, tc.metricName, tc.namespace, tc.objectRef, tc.metricSelector, tc.podList, tc.calibration)

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedReplicas, replicas)
			}
		})
	}
}

// TestGetObjectPerPodMetricReplicas verifies the calculation of per-pod object metric-based replicas
func TestGetObjectPerPodMetricReplicas(t *testing.T) {
	const (
		defaultNamespace              = "default"
		cpuInitializationPeriod       = 5 * time.Minute
		delayOfInitialReadinessStatus = 30 * time.Second
		defaultTolerance              = 0.1
	)

	defaultObjectRef := &autoscalingv2.CrossVersionObjectReference{Kind: "Service", Name: "my-svc"}

	testCases := []struct {
		name               string
		statusReplicas     int32
		targetAverageUsage int64
		metricName         string
		namespace          string
		objectRef          *autoscalingv2.CrossVersionObjectReference
		metricSelector     labels.Selector
		objectMetric       int64
		calibration        float64
		tolerance          float64
		expectedReplicas   int32
		expectedUsage      int64
		expectError        bool
	}{
		{
			name:               "Scale up based on per-pod object metrics",
			statusReplicas:     2,
			targetAverageUsage: 10,
			metricName:         "requests_per_pod",
			namespace:          defaultNamespace,
			objectRef:          defaultObjectRef,
			metricSelector:     labels.Everything(),
			objectMetric:       30,
			calibration:        1.0,
			tolerance:          defaultTolerance,
			expectedReplicas:   3,
			expectedUsage:      15,
			expectError:        false,
		},
		{
			name:               "Scale down based on per-pod object metrics",
			statusReplicas:     4,
			targetAverageUsage: 20,
			metricName:         "requests_per_pod",
			namespace:          defaultNamespace,
			objectRef:          defaultObjectRef,
			metricSelector:     labels.Everything(),
			objectMetric:       60,
			calibration:        1.0,
			tolerance:          defaultTolerance,
			expectedReplicas:   3,
			expectedUsage:      15,
			expectError:        false,
		},
		{
			name:               "No change due to tolerance",
			statusReplicas:     3,
			targetAverageUsage: 10,
			metricName:         "requests_per_pod",
			namespace:          defaultNamespace,
			objectRef:          defaultObjectRef,
			metricSelector:     labels.Everything(),
			objectMetric:       32, // Just within tolerance (10% of 30)
			calibration:        1.0,
			tolerance:          defaultTolerance,
			expectedReplicas:   3,
			expectedUsage:      11,
			expectError:        false,
		},
		{
			name:               "Scale with calibration",
			statusReplicas:     2,
			targetAverageUsage: 10,
			metricName:         "requests_per_pod",
			namespace:          defaultNamespace,
			objectRef:          defaultObjectRef,
			metricSelector:     labels.Everything(),
			objectMetric:       30,
			calibration:        0.5,
			tolerance:          defaultTolerance,
			expectedReplicas:   12,
			expectedUsage:      15,
			expectError:        false,
		},
		{
			name:               "Error getting metric",
			statusReplicas:     2,
			targetAverageUsage: 10,
			metricName:         "requests_per_pod",
			namespace:          defaultNamespace,
			objectRef:          defaultObjectRef,
			metricSelector:     labels.Everything(),
			objectMetric:       0,
			calibration:        1.0,
			tolerance:          defaultTolerance,
			expectError:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &MockQueryClient{}
			calculator := NewReplicaCalculator(
				mockClient,
				tc.tolerance,
				cpuInitializationPeriod,
				delayOfInitialReadinessStatus,
			)

			if tc.expectError {
				mockClient.On("GetObjectMetric", tc.metricName, tc.namespace, tc.objectRef, tc.metricSelector).
					Return(int64(0), time.Time{}, fmt.Errorf("metric not available")).Once()
			} else {
				mockClient.On("GetObjectMetric", tc.metricName, tc.namespace, tc.objectRef, tc.metricSelector).
					Return(tc.objectMetric, time.Now(), nil).Once()
			}

			replicas, usage, timestamp, err := calculator.GetObjectPerPodMetricReplicas(
				tc.statusReplicas,
				tc.targetAverageUsage,
				tc.metricName,
				tc.namespace,
				tc.objectRef,
				tc.metricSelector,
				tc.calibration,
			)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedReplicas, replicas, "Unexpected replica count")
				assert.Equal(t, tc.expectedUsage, usage, "Unexpected usage value")
				assert.False(t, timestamp.IsZero(), "Timestamp should not be zero")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestGetUsageRatioReplicaCount checks the calculation of usage ratio-based replica count
func TestGetUsageRatioReplicaCount(t *testing.T) {
	const (
		cpuInitializationPeriod       = 5 * time.Minute
		delayOfInitialReadinessStatus = 30 * time.Second
		defaultTolerance              = 0.1
	)

	testCases := []struct {
		name             string
		currentReplicas  int32
		usageRatio       float64
		podList          []*corev1.Pod
		calibration      float64
		tolerance        float64
		expectedReplicas int32
		expectError      bool
	}{
		{
			name:             "Scale up",
			currentReplicas:  2,
			usageRatio:       1.5,
			podList:          []*corev1.Pod{createPod("pod1", 100, 200), createPod("pod2", 100, 200)},
			calibration:      1.0,
			tolerance:        defaultTolerance,
			expectedReplicas: 3,
			expectError:      false,
		},
		{
			name:            "Scale down",
			currentReplicas: 4,
			usageRatio:      0.5,
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
				createPod("pod3", 100, 200),
				createPod("pod4", 100, 200),
			},
			calibration:      1.0,
			tolerance:        defaultTolerance,
			expectedReplicas: 2,
			expectError:      false,
		},
		{
			name:            "No change due to tolerance",
			currentReplicas: 3,
			usageRatio:      1.05,
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
				createPod("pod3", 100, 200),
			},
			calibration:      1.0,
			tolerance:        defaultTolerance,
			expectedReplicas: 3,
			expectError:      false,
		},
		{
			name:             "Scale to zero",
			currentReplicas:  0,
			usageRatio:       0.0,
			podList:          []*corev1.Pod{},
			calibration:      1.0,
			tolerance:        defaultTolerance,
			expectedReplicas: 0,
			expectError:      false,
		},
		{
			name:             "Scale from zero",
			currentReplicas:  0,
			usageRatio:       1.5,
			podList:          []*corev1.Pod{},
			calibration:      1.0,
			tolerance:        defaultTolerance,
			expectedReplicas: 2,
			expectError:      false,
		},
		{
			name:             "Scale with calibration",
			currentReplicas:  2,
			usageRatio:       1.5,
			podList:          []*corev1.Pod{createPod("pod1", 100, 200), createPod("pod2", 100, 200)},
			calibration:      0.5,
			tolerance:        defaultTolerance,
			expectedReplicas: 6,
			expectError:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &MockQueryClient{}
			calculator := NewReplicaCalculator(
				mockClient,
				tc.tolerance,
				cpuInitializationPeriod,
				delayOfInitialReadinessStatus,
			)

			replicas, timestamp, err := calculator.getUsageRatioReplicaCount(
				tc.currentReplicas,
				tc.usageRatio,
				tc.podList,
				tc.calibration,
			)

			assert.NoError(t, err, "Unexpected error: %v", err)
			assert.Equal(t, tc.expectedReplicas, replicas, "Unexpected replica count")
			assert.True(t, timestamp.IsZero(), "Expected zero timestamp, but got: %v", timestamp)
		})
	}
}

// TestGetReadyPodsCount verifies the counting of ready pods
func TestGetReadyPodsCount(t *testing.T) {
	testCases := []struct {
		name          string
		podList       []*corev1.Pod
		expectedCount int64
		expectError   bool
	}{
		{
			name: "All pods ready",
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
				createPod("pod3", 100, 200),
			},
			expectedCount: 3,
			expectError:   false,
		},
		{
			name: "Mixed ready and unready pods",
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createUnreadyPod("pod2", 100, 200),
				createPod("pod3", 100, 200),
				createUnreadyPod("pod4", 100, 200),
			},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name: "All pods unready",
			podList: []*corev1.Pod{
				createUnreadyPod("pod1", 100, 200),
				createUnreadyPod("pod2", 100, 200),
			},
			expectedCount: 0,
			expectError:   false,
		},
		{
			name:          "Empty pod list",
			podList:       []*corev1.Pod{},
			expectedCount: 0,
			expectError:   true,
		},
		{
			name: "Pods with different phases",
			podList: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPodWithPhase("pod2", 100, 200, corev1.PodPending),
				createPodWithPhase("pod3", 100, 200, corev1.PodSucceeded),
				createPodWithPhase("pod4", 100, 200, corev1.PodFailed),
			},
			expectedCount: 1,
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			calculator := &ReplicaCalculator{} // Don't need to initialize other fields for this test

			count, err := calculator.getReadyPodsCount(tc.podList)

			if tc.expectError {
				assert.Error(t, err, "Expected an error, but got none")
			} else {
				assert.NoError(t, err, "Unexpected error: %v", err)
				assert.Equal(t, tc.expectedCount, count, "Unexpected ready pod count")
			}
		})
	}
}

// TestGroupPods checks the grouping of pods based on their status and metrics
func TestGroupPods(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name                          string
		pods                          []*corev1.Pod
		metrics                       metricsclient.PodMetricsInfo
		resource                      corev1.ResourceName
		cpuInitializationPeriod       time.Duration
		delayOfInitialReadinessStatus time.Duration
		expectedReadyCount            int
		expectedUnreadyPods           []string
		expectedMissingPods           []string
		expectedIgnoredPods           []string
	}{
		{
			name: "All pods ready and with metrics",
			pods: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 150, Window: time.Minute, Timestamp: now},
				"pod2": {Value: 150, Window: time.Minute, Timestamp: now},
			},
			resource:                      corev1.ResourceCPU,
			cpuInitializationPeriod:       5 * time.Minute,
			delayOfInitialReadinessStatus: 30 * time.Second,
			expectedReadyCount:            2,
		},
		{
			name: "One pod unready (Pending)",
			pods: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPodWithPhase("pod2", 100, 200, corev1.PodPending),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 150, Window: time.Minute, Timestamp: now},
				"pod2": {Value: 150, Window: time.Minute, Timestamp: now},
			},
			resource:                      corev1.ResourceCPU,
			cpuInitializationPeriod:       5 * time.Minute,
			delayOfInitialReadinessStatus: 30 * time.Second,
			expectedReadyCount:            1,
			expectedUnreadyPods:           []string{"pod2"},
		},
		{
			name: "One pod missing metrics",
			pods: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 150, Window: time.Minute, Timestamp: now},
			},
			resource:                      corev1.ResourceCPU,
			cpuInitializationPeriod:       5 * time.Minute,
			delayOfInitialReadinessStatus: 30 * time.Second,
			expectedReadyCount:            1,
			expectedMissingPods:           []string{"pod2"},
		},
		{
			name: "One pod ignored (Failed)",
			pods: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPodWithPhase("pod2", 100, 200, corev1.PodFailed),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 150, Window: time.Minute, Timestamp: now},
				"pod2": {Value: 150, Window: time.Minute, Timestamp: now},
			},
			resource:                      corev1.ResourceCPU,
			cpuInitializationPeriod:       5 * time.Minute,
			delayOfInitialReadinessStatus: 30 * time.Second,
			expectedReadyCount:            1,
			expectedIgnoredPods:           []string{"pod2"},
		},
		{
			name: "Pod within CPU initialization period",
			pods: []*corev1.Pod{
				createPodWithStartTime("pod1", 100, 200, now.Add(-2*time.Minute)),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 150, Window: time.Minute, Timestamp: now},
			},
			resource:                      corev1.ResourceCPU,
			cpuInitializationPeriod:       5 * time.Minute,
			delayOfInitialReadinessStatus: 30 * time.Second,
			expectedReadyCount:            1,
			expectedUnreadyPods:           []string{},
		},
		{
			name: "Non-CPU resource",
			pods: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createUnreadyPod("pod2", 100, 200),
			},
			metrics: metricsclient.PodMetricsInfo{
				"pod1": {Value: 150, Window: time.Minute, Timestamp: now},
				"pod2": {Value: 150, Window: time.Minute, Timestamp: now},
			},
			resource:                      corev1.ResourceMemory,
			cpuInitializationPeriod:       5 * time.Minute,
			delayOfInitialReadinessStatus: 30 * time.Second,
			expectedReadyCount:            2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			readyCount, unreadyPods, missingPods, ignoredPods := groupPods(tc.pods, tc.metrics, tc.resource, tc.cpuInitializationPeriod, tc.delayOfInitialReadinessStatus)

			assert.Equal(t, tc.expectedReadyCount, readyCount, "Ready pod count mismatch")
			assertSetContains(t, unreadyPods, tc.expectedUnreadyPods, "Unready pods mismatch")
			assertSetContains(t, missingPods, tc.expectedMissingPods, "Missing pods mismatch")
			assertSetContains(t, ignoredPods, tc.expectedIgnoredPods, "Ignored pods mismatch")
		})
	}
}

// TestCalculatePodRequests verifies the calculation of pod resource requests
func TestCalculatePodRequests(t *testing.T) {
	testCases := []struct {
		name           string
		pods           []*corev1.Pod
		container      string
		resource       corev1.ResourceName
		expectedResult map[string]int64
		expectedError  bool
	}{
		{
			name: "Calculate CPU requests for all containers",
			pods: []*corev1.Pod{
				createPod("pod1", 100, 200),
				createPod("pod2", 200, 300),
			},
			container:      "",
			resource:       corev1.ResourceCPU,
			expectedResult: map[string]int64{"pod1": 100, "pod2": 200},
			expectedError:  false,
		},
		{
			name: "Calculate memory requests for specific container",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pod1"},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "container1",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("100Mi"),
									},
								},
							},
							{
								Name: "container2",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("200Mi"),
									},
								},
							},
						},
					},
				},
			},
			container:      "container2",
			resource:       corev1.ResourceMemory,
			expectedResult: map[string]int64{"pod1": 209715200000},
			expectedError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := calculatePodRequests(tc.pods, tc.container, tc.resource)

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}

// TestRemoveMetricsForPods checks the removal of metrics for specified pods
func TestRemoveMetricsForPods(t *testing.T) {
	metrics := metricsclient.PodMetricsInfo{
		"pod1": {Value: 100},
		"pod2": {Value: 200},
		"pod3": {Value: 300},
	}

	podsToRemove := sets.New("pod1", "pod3")

	removeMetricsForPods(metrics, podsToRemove)

	assert.Equal(t, 1, len(metrics))
	assert.Contains(t, metrics, "pod2")
	assert.NotContains(t, metrics, "pod1")
	assert.NotContains(t, metrics, "pod3")
}

// Helper Functions

// Helper function to create a pod
func createPod(name string, request, limit int64) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"test": "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "test-container",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: *resource.NewMilliQuantity(request, resource.DecimalSI),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: *resource.NewMilliQuantity(limit, resource.DecimalSI),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
			StartTime: &metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
		},
	}
}

// Helper function to create a pod with a specific phase
func createPodWithPhase(name string, request, limit int64, phase corev1.PodPhase) *corev1.Pod {
	pod := createPod(name, request, limit)
	pod.Status.Phase = phase
	return pod
}

// Helper function to assert that a set contains expected elements
func assertSetContains(t *testing.T, set sets.Set[string], expected []string, message string) {
	assert.Equal(t, len(expected), set.Len(), message)
	for _, item := range expected {
		assert.True(t, set.Has(item), fmt.Sprintf("%s: %s not found", message, item))
	}
}

// Helper function to create a pod with a specific start time
func createPodWithStartTime(name string, request, limit int64, startTime time.Time) *corev1.Pod {
	pod := createPod(name, request, limit)
	pod.Status.StartTime = &metav1.Time{Time: startTime}
	return pod
}
