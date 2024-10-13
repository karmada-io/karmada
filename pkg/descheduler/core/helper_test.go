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

package core

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/estimator/client"
)

func TestNewSchedulingResultHelper(t *testing.T) {
	tests := []struct {
		name     string
		binding  *workv1alpha2.ResourceBinding
		expected *SchedulingResultHelper
	}{
		{
			name: "Valid binding with ready replicas",
			binding: &workv1alpha2.ResourceBinding{
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1", Replicas: 3},
						{Name: "cluster2", Replicas: 2},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status:      &runtime.RawExtension{Raw: []byte(`{"readyReplicas": 2}`)},
						},
						{
							ClusterName: "cluster2",
							Status:      &runtime.RawExtension{Raw: []byte(`{"readyReplicas": 1}`)},
						},
					},
				},
			},
			expected: &SchedulingResultHelper{
				TargetClusters: []*TargetClusterWrapper{
					{ClusterName: "cluster1", Spec: 3, Ready: 2},
					{ClusterName: "cluster2", Spec: 2, Ready: 1},
				},
			},
		},
		{
			name: "Binding with invalid status",
			binding: &workv1alpha2.ResourceBinding{
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1", Replicas: 3},
						{Name: "cluster2", Replicas: 2},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status:      &runtime.RawExtension{Raw: []byte(`{"readyReplicas": 2}`)},
						},
						{
							ClusterName: "cluster2",
							Status:      &runtime.RawExtension{Raw: []byte(`invalid json`)},
						},
					},
				},
			},
			expected: &SchedulingResultHelper{
				TargetClusters: []*TargetClusterWrapper{
					{ClusterName: "cluster1", Spec: 3, Ready: 2},
					{ClusterName: "cluster2", Spec: 2, Ready: client.UnauthenticReplica},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helper := NewSchedulingResultHelper(tt.binding)
			helper.ResourceBinding = nil
			if !reflect.DeepEqual(helper, tt.expected) {
				t.Errorf("NewSchedulingResultHelper() = %v, want %v", helper, tt.expected)
			}
		})
	}
}

func TestSchedulingResultHelper_FillUnschedulableReplicas(t *testing.T) {
	tests := []struct {
		name           string
		helper         *SchedulingResultHelper
		mockEstimator  *mockUnschedulableReplicaEstimator
		expected       []*TargetClusterWrapper
		expectedErrLog string
	}{
		{
			name: "Successful fill",
			helper: &SchedulingResultHelper{
				ResourceBinding: &workv1alpha2.ResourceBinding{
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
							Namespace:  "default",
						},
					},
				},
				TargetClusters: []*TargetClusterWrapper{
					{ClusterName: "cluster1", Spec: 3, Ready: 2},
					{ClusterName: "cluster2", Spec: 2, Ready: 1},
				},
			},
			mockEstimator: &mockUnschedulableReplicaEstimator{},
			expected: []*TargetClusterWrapper{
				{ClusterName: "cluster1", Spec: 3, Ready: 2, Unschedulable: 1},
				{ClusterName: "cluster2", Spec: 2, Ready: 1, Unschedulable: 1},
			},
		},
		{
			name: "Estimator error",
			helper: &SchedulingResultHelper{
				ResourceBinding: &workv1alpha2.ResourceBinding{
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
							Namespace:  "default",
						},
					},
				},
				TargetClusters: []*TargetClusterWrapper{
					{ClusterName: "cluster1", Spec: 3, Ready: 2},
				},
			},
			mockEstimator: &mockUnschedulableReplicaEstimator{shouldError: true},
			expected: []*TargetClusterWrapper{
				{ClusterName: "cluster1", Spec: 3, Ready: 2, Unschedulable: 0},
			},
			expectedErrLog: "Max cluster unschedulable replicas error: mock error",
		},
		{
			name: "UnauthenticReplica handling",
			helper: &SchedulingResultHelper{
				ResourceBinding: &workv1alpha2.ResourceBinding{
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
							Namespace:  "default",
						},
					},
				},
				TargetClusters: []*TargetClusterWrapper{
					{ClusterName: "cluster1", Spec: 3, Ready: 2},
					{ClusterName: "cluster2", Spec: 2, Ready: 1},
				},
			},
			mockEstimator: &mockUnschedulableReplicaEstimator{unauthenticCluster: "cluster2"},
			expected: []*TargetClusterWrapper{
				{ClusterName: "cluster1", Spec: 3, Ready: 2, Unschedulable: 1},
				{ClusterName: "cluster2", Spec: 2, Ready: 1, Unschedulable: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client.GetUnschedulableReplicaEstimators()["mock"] = tt.mockEstimator
			defer delete(client.GetUnschedulableReplicaEstimators(), "mock")

			tt.helper.FillUnschedulableReplicas(time.Minute)

			if !reflect.DeepEqual(tt.helper.TargetClusters, tt.expected) {
				t.Errorf("FillUnschedulableReplicas() = %v, want %v", tt.helper.TargetClusters, tt.expected)
			}
		})
	}
}

func TestSchedulingResultHelper_GetUndesiredClusters(t *testing.T) {
	tests := []struct {
		name             string
		helper           *SchedulingResultHelper
		expectedClusters []*TargetClusterWrapper
		expectedNames    []string
	}{
		{
			name: "Mixed desired and undesired clusters",
			helper: &SchedulingResultHelper{
				TargetClusters: []*TargetClusterWrapper{
					{ClusterName: "cluster1", Spec: 3, Ready: 2},
					{ClusterName: "cluster2", Spec: 2, Ready: 2},
					{ClusterName: "cluster3", Spec: 4, Ready: 1},
				},
			},
			expectedClusters: []*TargetClusterWrapper{
				{ClusterName: "cluster1", Spec: 3, Ready: 2},
				{ClusterName: "cluster3", Spec: 4, Ready: 1},
			},
			expectedNames: []string{"cluster1", "cluster3"},
		},
		{
			name: "All clusters desired",
			helper: &SchedulingResultHelper{
				TargetClusters: []*TargetClusterWrapper{
					{ClusterName: "cluster1", Spec: 2, Ready: 2},
					{ClusterName: "cluster2", Spec: 3, Ready: 3},
				},
			},
			expectedClusters: nil,
			expectedNames:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusters, names := tt.helper.GetUndesiredClusters()

			if !reflect.DeepEqual(clusters, tt.expectedClusters) {
				t.Errorf("GetUndesiredClusters() clusters = %v, want %v", clusters, tt.expectedClusters)
			}
			if !reflect.DeepEqual(names, tt.expectedNames) {
				t.Errorf("GetUndesiredClusters() names = %v, want %v", names, tt.expectedNames)
			}
		})
	}
}

func TestGetReadyReplicas(t *testing.T) {
	tests := []struct {
		name     string
		binding  *workv1alpha2.ResourceBinding
		expected map[string]int32
	}{
		{
			name: "Valid status with readyReplicas",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
				},
				Status: workv1alpha2.ResourceBindingStatus{
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status:      &runtime.RawExtension{Raw: []byte(`{"readyReplicas": 2}`)},
						},
						{
							ClusterName: "cluster2",
							Status:      &runtime.RawExtension{Raw: []byte(`{"readyReplicas": 3}`)},
						},
					},
				},
			},
			expected: map[string]int32{
				"cluster1": 2,
				"cluster2": 3,
			},
		},
		{
			name: "Status with missing readyReplicas field",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
				},
				Status: workv1alpha2.ResourceBindingStatus{
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status:      &runtime.RawExtension{Raw: []byte(`{"readyReplicas": 2}`)},
						},
						{
							ClusterName: "cluster2",
							Status:      &runtime.RawExtension{Raw: []byte(`{"someOtherField": 3}`)},
						},
					},
				},
			},
			expected: map[string]int32{
				"cluster1": 2,
			},
		},
		{
			name: "Status with invalid JSON",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
				},
				Status: workv1alpha2.ResourceBindingStatus{
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status:      &runtime.RawExtension{Raw: []byte(`{"readyReplicas": 2}`)},
						},
						{
							ClusterName: "cluster2",
							Status:      &runtime.RawExtension{Raw: []byte(`invalid json`)},
						},
					},
				},
			},
			expected: map[string]int32{
				"cluster1": 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getReadyReplicas(tt.binding)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("getReadyReplicas() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Mock implementation of UnschedulableReplicaEstimator
type mockUnschedulableReplicaEstimator struct {
	shouldError        bool
	unauthenticCluster string
}

func (m *mockUnschedulableReplicaEstimator) GetUnschedulableReplicas(_ context.Context, clusters []string, _ *workv1alpha2.ObjectReference, _ time.Duration) ([]workv1alpha2.TargetCluster, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}

	result := make([]workv1alpha2.TargetCluster, len(clusters))
	for i, cluster := range clusters {
		replicas := int32(1)
		if cluster == m.unauthenticCluster {
			replicas = client.UnauthenticReplica
		}
		result[i] = workv1alpha2.TargetCluster{Name: cluster, Replicas: replicas}
	}
	return result, nil
}
