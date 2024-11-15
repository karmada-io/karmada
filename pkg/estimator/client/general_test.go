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
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestGetMaximumReplicasBasedOnResourceModels(t *testing.T) {
	tests := []struct {
		name                string
		cluster             clusterv1alpha1.Cluster
		replicaRequirements workv1alpha2.ReplicaRequirements
		expectError         bool
		expectedReplicas    int64
	}{
		{
			name:    "No grade defined should result in an error",
			cluster: clusterv1alpha1.Cluster{},
			replicaRequirements: workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
				},
			},
			expectError:      true,
			expectedReplicas: -1,
		},
		{
			name: "Partially compliant grades",
			cluster: clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{
						{
							Grade: 0, Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: resource.MustParse("0"), Max: resource.MustParse("1")}},
						},
						{
							Grade: 1, Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: resource.MustParse("1"), Max: resource.MustParse("2")}},
						},
						{
							Grade: 2, Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: resource.MustParse("2"), Max: resource.MustParse("4")}},
						},
					},
				},
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						AllocatableModelings: []clusterv1alpha1.AllocatableModeling{
							{Grade: 0, Count: 1},
							{Grade: 1, Count: 1},
							{Grade: 2, Count: 1},
						},
					},
				},
			},
			replicaRequirements: workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1.5"),
				},
			},
			expectError:      false,
			expectedReplicas: 1,
		},
		{
			name: "No compliant grades",
			cluster: clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{
						{
							Grade: 0, Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: resource.MustParse("0"), Max: resource.MustParse("1")},
							},
						},
						{
							Grade: 1, Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: resource.MustParse("1"), Max: resource.MustParse("2")},
							},
						},
						{
							Grade: 2, Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: resource.MustParse("2"), Max: resource.MustParse("4")},
							},
						},
					},
				},
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						AllocatableModelings: []clusterv1alpha1.AllocatableModeling{
							{Grade: 0, Count: 1},
							{Grade: 1, Count: 1},
							{Grade: 2, Count: 1},
						},
					},
				},
			},
			replicaRequirements: workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("3"),
				},
			},
			expectError:      false,
			expectedReplicas: 0,
		},
		{
			name: "Multi resource request",
			cluster: clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{
						{
							Grade: 0, Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: resource.MustParse("0"), Max: resource.MustParse("1")},
								{Name: corev1.ResourceMemory, Min: resource.MustParse("0"), Max: resource.MustParse("1Gi")},
							},
						},
						{
							Grade: 1, Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: resource.MustParse("1"), Max: resource.MustParse("2")},
								{Name: corev1.ResourceMemory, Min: resource.MustParse("1Gi"), Max: resource.MustParse("2Gi")},
							},
						},
						{
							Grade: 2, Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: resource.MustParse("2"), Max: resource.MustParse("4")},
								{Name: corev1.ResourceMemory, Min: resource.MustParse("2Gi"), Max: resource.MustParse("4Gi")},
							},
						},
					},
				},
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						AllocatableModelings: []clusterv1alpha1.AllocatableModeling{
							{Grade: 0, Count: 1},
							{Grade: 1, Count: 1},
							{Grade: 2, Count: 1},
						},
					},
				},
			},
			replicaRequirements: workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					// When looking CPU, grade 1 meets, then looking memory, grade 2 meets.
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1.5Gi"),
				},
			},
			expectError:      false,
			expectedReplicas: 1,
		},
		{
			name: "request exceeds highest grade",
			cluster: clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{
						{
							Grade: 0, Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: resource.MustParse("0"), Max: resource.MustParse("1")},
								{Name: corev1.ResourceMemory, Min: resource.MustParse("0"), Max: resource.MustParse("1Gi")},
							},
						},
						{
							Grade: 1, Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: resource.MustParse("1"), Max: resource.MustParse("2")},
								{Name: corev1.ResourceMemory, Min: resource.MustParse("1Gi"), Max: resource.MustParse("2Gi")},
							},
						},
						{
							Grade: 2, Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: resource.MustParse("2"), Max: resource.MustParse("4")},
								{Name: corev1.ResourceMemory, Min: resource.MustParse("2Gi"), Max: resource.MustParse("4Gi")},
							},
						},
					},
				},
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						AllocatableModelings: []clusterv1alpha1.AllocatableModeling{
							{Grade: 0, Count: 1},
							{Grade: 1, Count: 1},
							{Grade: 2, Count: 1},
						},
					},
				},
			},
			replicaRequirements: workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("2.5Gi"), // no grade can provide sufficient memories.
				},
			},
			expectError:      false,
			expectedReplicas: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replicas, err := getMaximumReplicasBasedOnResourceModels(&tt.cluster, &tt.replicaRequirements)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedReplicas, replicas)
		})
	}
}

func TestMaxAvailableReplicas(t *testing.T) {
	tests := []struct {
		name                string
		clusters            []*clusterv1alpha1.Cluster
		replicaRequirements *workv1alpha2.ReplicaRequirements
		expectedTargets     []workv1alpha2.TargetCluster
	}{
		{
			name: "single cluster with resources",
			clusters: []*clusterv1alpha1.Cluster{
				{
					Status: clusterv1alpha1.ClusterStatus{
						ResourceSummary: &clusterv1alpha1.ResourceSummary{
							Allocatable: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4"),
								corev1.ResourceMemory: resource.MustParse("8Gi"),
								corev1.ResourcePods:   resource.MustParse("100"),
							},
							Allocated: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
								corev1.ResourcePods:   resource.MustParse("20"),
							},
						},
					},
				},
			},
			replicaRequirements: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			expectedTargets: []workv1alpha2.TargetCluster{
				{
					Name:     "",
					Replicas: 6,
				},
			},
		},
		{
			name: "cluster with no resources",
			clusters: []*clusterv1alpha1.Cluster{
				{
					Status: clusterv1alpha1.ClusterStatus{},
				},
			},
			replicaRequirements: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
				},
			},
			expectedTargets: []workv1alpha2.TargetCluster{
				{
					Name:     "",
					Replicas: 0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimator := NewGeneralEstimator()
			targets, err := estimator.MaxAvailableReplicas(context.Background(), tt.clusters, tt.replicaRequirements)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedTargets, targets)
		})
	}
}

func TestMaxAvailableReplicasGeneral(t *testing.T) {
	tests := []struct {
		name                string
		cluster             *clusterv1alpha1.Cluster
		replicaRequirements *workv1alpha2.ReplicaRequirements
		expected            int32
	}{
		{
			name: "nil resource summary",
			cluster: &clusterv1alpha1.Cluster{
				Status: clusterv1alpha1.ClusterStatus{},
			},
			expected: 0,
		},
		{
			name: "no allowed pods",
			cluster: &clusterv1alpha1.Cluster{
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						Allocatable: corev1.ResourceList{
							corev1.ResourcePods: resource.MustParse("10"),
						},
						Allocated: corev1.ResourceList{
							corev1.ResourcePods: resource.MustParse("10"),
						},
					},
				},
			},
			expected: 0,
		},
		{
			name: "nil replica requirements",
			cluster: &clusterv1alpha1.Cluster{
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						Allocatable: corev1.ResourceList{
							corev1.ResourcePods: resource.MustParse("10"),
						},
					},
				},
			},
			expected: 10,
		},
		{
			name: "basic resource estimation",
			cluster: &clusterv1alpha1.Cluster{
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						Allocatable: corev1.ResourceList{
							corev1.ResourcePods:   resource.MustParse("100"),
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("8Gi"),
						},
						Allocated: corev1.ResourceList{
							corev1.ResourcePods:   resource.MustParse("20"),
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
			replicaRequirements: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			expected: 6,
		},
	}

	estimator := NewGeneralEstimator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimator.maxAvailableReplicas(tt.cluster, tt.replicaRequirements)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAllowedPodNumber(t *testing.T) {
	tests := []struct {
		name            string
		resourceSummary *clusterv1alpha1.ResourceSummary
		expected        int64
	}{
		{
			name: "normal case",
			resourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocatable: corev1.ResourceList{
					corev1.ResourcePods: resource.MustParse("100"),
				},
				Allocated: corev1.ResourceList{
					corev1.ResourcePods: resource.MustParse("40"),
				},
				Allocating: corev1.ResourceList{
					corev1.ResourcePods: resource.MustParse("10"),
				},
			},
			expected: 50,
		},
		{
			name: "no allocatable pods",
			resourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocated: corev1.ResourceList{
					corev1.ResourcePods: resource.MustParse("10"),
				},
			},
			expected: 0,
		},
		{
			name: "over allocation",
			resourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocatable: corev1.ResourceList{
					corev1.ResourcePods: resource.MustParse("100"),
				},
				Allocated: corev1.ResourceList{
					corev1.ResourcePods: resource.MustParse("90"),
				},
				Allocating: corev1.ResourceList{
					corev1.ResourcePods: resource.MustParse("20"),
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getAllowedPodNumber(tt.resourceSummary)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertToResourceModelsMinMap(t *testing.T) {
	tests := []struct {
		name              string
		models            []clusterv1alpha1.ResourceModel
		expectedMinRanges map[corev1.ResourceName][]resource.Quantity
	}{
		{
			name:              "empty models",
			models:            []clusterv1alpha1.ResourceModel{},
			expectedMinRanges: map[corev1.ResourceName][]resource.Quantity{},
		},
		{
			name: "single resource type across models",
			models: []clusterv1alpha1.ResourceModel{
				{
					Grade: 0,
					Ranges: []clusterv1alpha1.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  resource.MustParse("1"),
							Max:  resource.MustParse("2"),
						},
					},
				},
				{
					Grade: 1,
					Ranges: []clusterv1alpha1.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  resource.MustParse("2"),
							Max:  resource.MustParse("4"),
						},
					},
				},
			},
			expectedMinRanges: map[corev1.ResourceName][]resource.Quantity{
				corev1.ResourceCPU: {
					resource.MustParse("1"),
					resource.MustParse("2"),
				},
			},
		},
		{
			name: "multiple resource types",
			models: []clusterv1alpha1.ResourceModel{
				{
					Grade: 0,
					Ranges: []clusterv1alpha1.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  resource.MustParse("1"),
							Max:  resource.MustParse("2"),
						},
						{
							Name: corev1.ResourceMemory,
							Min:  resource.MustParse("1Gi"),
							Max:  resource.MustParse("2Gi"),
						},
					},
				},
				{
					Grade: 1,
					Ranges: []clusterv1alpha1.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  resource.MustParse("2"),
							Max:  resource.MustParse("4"),
						},
						{
							Name: corev1.ResourceMemory,
							Min:  resource.MustParse("2Gi"),
							Max:  resource.MustParse("4Gi"),
						},
					},
				},
			},
			expectedMinRanges: map[corev1.ResourceName][]resource.Quantity{
				corev1.ResourceCPU: {
					resource.MustParse("1"),
					resource.MustParse("2"),
				},
				corev1.ResourceMemory: {
					resource.MustParse("1Gi"),
					resource.MustParse("2Gi"),
				},
			},
		},
		{
			name: "models with missing resource types",
			models: []clusterv1alpha1.ResourceModel{
				{
					Grade: 0,
					Ranges: []clusterv1alpha1.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  resource.MustParse("1"),
							Max:  resource.MustParse("2"),
						},
					},
				},
				{
					Grade: 1,
					Ranges: []clusterv1alpha1.ResourceModelRange{
						{
							Name: corev1.ResourceMemory,
							Min:  resource.MustParse("1Gi"),
							Max:  resource.MustParse("2Gi"),
						},
					},
				},
			},
			expectedMinRanges: map[corev1.ResourceName][]resource.Quantity{
				corev1.ResourceCPU: {
					resource.MustParse("1"),
				},
				corev1.ResourceMemory: {
					resource.MustParse("1Gi"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToResourceModelsMinMap(tt.models)

			// Check map length matches
			assert.Equal(t, len(tt.expectedMinRanges), len(result))

			// Check each resource type and its quantities
			for resourceName, expectedQuantities := range tt.expectedMinRanges {
				resultQuantities, exists := result[resourceName]
				assert.True(t, exists, "Resource %v should exist in result", resourceName)

				// Check quantities length matches
				assert.Equal(t, len(expectedQuantities), len(resultQuantities))

				// Check each quantity matches
				for i, expectedQty := range expectedQuantities {
					assert.True(t, expectedQty.Equal(resultQuantities[i]),
						"Quantity mismatch for resource %v at index %d: expected %v, got %v",
						resourceName, i, expectedQty.String(), resultQuantities[i].String())
				}
			}
		})
	}
}

func TestGetNodeAvailableReplicas(t *testing.T) {
	tests := []struct {
		name                 string
		modelIndex           int
		replicaRequirements  *workv1alpha2.ReplicaRequirements
		resourceModelsMinMap map[corev1.ResourceName][]resource.Quantity
		expected             int64
	}{
		{
			name:       "empty resource request",
			modelIndex: 0,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{},
			},
			resourceModelsMinMap: map[corev1.ResourceName][]resource.Quantity{},
			expected:             math.MaxInt64,
		},
		{
			name:       "zero requested quantity should be ignored",
			modelIndex: 0,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			resourceModelsMinMap: map[corev1.ResourceName][]resource.Quantity{
				corev1.ResourceCPU:    {resource.MustParse("2")},
				corev1.ResourceMemory: {resource.MustParse("4Gi")},
			},
			expected: 4,
		},
		{
			name:       "CPU resource calculation in millicores",
			modelIndex: 0,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("500m"),
				},
			},
			resourceModelsMinMap: map[corev1.ResourceName][]resource.Quantity{
				corev1.ResourceCPU: {resource.MustParse("2")},
			},
			expected: 4, // 2000m / 500m = 4
		},
		{
			name:       "multiple resources with different ratios",
			modelIndex: 0,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
			resourceModelsMinMap: map[corev1.ResourceName][]resource.Quantity{
				corev1.ResourceCPU:    {resource.MustParse("2")},   // 4 replicas possible (2000m/500m)
				corev1.ResourceMemory: {resource.MustParse("8Gi")}, // 4 replicas possible (8Gi/2Gi)
			},
			expected: 4,
		},
		{
			name:       "memory limited scenario",
			modelIndex: 0,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("3Gi"),
				},
			},
			resourceModelsMinMap: map[corev1.ResourceName][]resource.Quantity{
				corev1.ResourceCPU:    {resource.MustParse("2")},   // 4 replicas possible (2000m/500m)
				corev1.ResourceMemory: {resource.MustParse("8Gi")}, // 2 replicas possible (8Gi/3Gi)
			},
			expected: 2,
		},
		{
			name:       "zero replicas possible but first model",
			modelIndex: 0,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("3"),
				},
			},
			resourceModelsMinMap: map[corev1.ResourceName][]resource.Quantity{
				corev1.ResourceCPU: {resource.MustParse("2")},
			},
			expected: 1, // Although 2/3=0, return 1 since it's the first model
		},
		{
			name:       "different model index",
			modelIndex: 1,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
				},
			},
			resourceModelsMinMap: map[corev1.ResourceName][]resource.Quantity{
				corev1.ResourceCPU: {
					resource.MustParse("2"),
					resource.MustParse("4"),
				},
			},
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNodeAvailableReplicas(tt.modelIndex, tt.replicaRequirements, tt.resourceModelsMinMap)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMaximumReplicasBasedOnClusterSummary(t *testing.T) {
	tests := []struct {
		name                string
		resourceSummary     *clusterv1alpha1.ResourceSummary
		replicaRequirements *workv1alpha2.ReplicaRequirements
		expected            int64
	}{
		{
			name: "sufficient resources",
			resourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
				},
				Allocated: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
			replicaRequirements: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			expected: 6,
		},
		{
			name: "insufficient memory",
			resourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
				Allocated: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1.5Gi"),
				},
			},
			replicaRequirements: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMaximumReplicasBasedOnClusterSummary(tt.resourceSummary, tt.replicaRequirements)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMinimumModelIndex(t *testing.T) {
	tests := []struct {
		name          string
		minimumGrades []resource.Quantity
		requestValue  resource.Quantity
		expected      int
	}{
		{
			name: "first grade sufficient",
			minimumGrades: []resource.Quantity{
				resource.MustParse("2"),
				resource.MustParse("4"),
				resource.MustParse("8"),
			},
			requestValue: resource.MustParse("1"),
			expected:     0,
		},
		{
			name: "middle grade sufficient",
			minimumGrades: []resource.Quantity{
				resource.MustParse("1"),
				resource.MustParse("4"),
				resource.MustParse("8"),
			},
			requestValue: resource.MustParse("2"),
			expected:     1,
		},
		{
			name: "no sufficient grade",
			minimumGrades: []resource.Quantity{
				resource.MustParse("1"),
				resource.MustParse("2"),
				resource.MustParse("4"),
			},
			requestValue: resource.MustParse("8"),
			expected:     -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := minimumModelIndex(tt.minimumGrades, tt.requestValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}
