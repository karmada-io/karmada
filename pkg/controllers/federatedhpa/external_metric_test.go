/*
Copyright 2026 The Karmada Authors.

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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestGetExternalMetricReplicas(t *testing.T) {
	selector := labels.SelectorFromSet(labels.Set{"queue": "orders"})
	podList := []*corev1.Pod{createPod("pod1", 100, 200), createPod("pod2", 100, 200)}

	testCases := []struct {
		name             string
		currentReplicas  int32
		targetUsage      int64
		metrics          []int64
		calibration      float64
		metricsErr       error
		expectedReplicas int32
		expectedUsage    int64
		expectError      bool
	}{
		{
			name:             "values from several clusters are added up",
			currentReplicas:  2,
			targetUsage:      10,
			metrics:          []int64{20, 10},
			calibration:      1.0,
			expectedReplicas: 6,
			expectedUsage:    30,
		},
		{
			name:             "no change within the tolerance",
			currentReplicas:  2,
			targetUsage:      10,
			metrics:          []int64{10},
			calibration:      1.0,
			expectedReplicas: 2,
			expectedUsage:    10,
		},
		{
			name:            "metrics client error",
			currentReplicas: 2,
			targetUsage:     10,
			metrics:         []int64{},
			calibration:     1.0,
			metricsErr:      errors.New("adapter unavailable"),
			expectError:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &MockQueryClient{}
			mockClient.On("GetExternalMetric", "queue_length", "default", selector).Return(tc.metrics, time.Now(), tc.metricsErr).Once()
			calculator := NewReplicaCalculator(mockClient, 0.1, 5*time.Minute, 30*time.Second)

			replicas, usage, _, err := calculator.GetExternalMetricReplicas(tc.currentReplicas, tc.targetUsage, "queue_length", "default", selector, podList, tc.calibration)
			if tc.expectError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedReplicas, replicas)
			assert.Equal(t, tc.expectedUsage, usage)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestGetExternalPerPodMetricReplicas(t *testing.T) {
	selector := labels.Everything()

	testCases := []struct {
		name              string
		statusReplicas    int32
		targetUsagePerPod int64
		metrics           []int64
		calibration       float64
		expectedReplicas  int32
		expectedUsage     int64
	}{
		{
			name:              "scale up to the total divided by the per-pod target",
			statusReplicas:    2,
			targetUsagePerPod: 10,
			metrics:           []int64{25, 25},
			calibration:       1.0,
			expectedReplicas:  5,
			expectedUsage:     25,
		},
		{
			name:              "no change within the tolerance",
			statusReplicas:    4,
			targetUsagePerPod: 10,
			metrics:           []int64{41},
			calibration:       1.0,
			expectedReplicas:  4,
			expectedUsage:     11,
		},
		{
			name:              "calibration is applied once",
			statusReplicas:    2,
			targetUsagePerPod: 10,
			metrics:           []int64{60},
			calibration:       2.0,
			expectedReplicas:  3,
			expectedUsage:     30,
		},
		{
			name:              "calibration is applied before rounding",
			statusReplicas:    1,
			targetUsagePerPod: 10,
			metrics:           []int64{21},
			calibration:       0.5,
			expectedReplicas:  5,
			expectedUsage:     21,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &MockQueryClient{}
			mockClient.On("GetExternalMetric", "queue_length", "default", selector).Return(tc.metrics, time.Now(), nil).Once()
			calculator := NewReplicaCalculator(mockClient, 0.1, 5*time.Minute, 30*time.Second)

			replicas, usage, _, err := calculator.GetExternalPerPodMetricReplicas(tc.statusReplicas, tc.targetUsagePerPod, "queue_length", "default", selector, tc.calibration)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedReplicas, replicas)
			assert.Equal(t, tc.expectedUsage, usage)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestGetExternalPerPodMetricReplicasZeroStatusReplicas(t *testing.T) {
	calculator := NewReplicaCalculator(&MockQueryClient{}, 0.1, 5*time.Minute, 30*time.Second)

	_, _, _, err := calculator.GetExternalPerPodMetricReplicas(0, 10, "queue_length", "default", labels.Everything(), 1.0)
	require.Error(t, err)
}
