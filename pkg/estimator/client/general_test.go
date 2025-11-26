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

func q(s string) resource.Quantity { return resource.MustParse(s) }

func comp(name string, replicas int32, rl corev1.ResourceList) workv1alpha2.Component {
	return workv1alpha2.Component{
		Name:     name,
		Replicas: replicas,
		ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
			ResourceRequest: rl,
		},
	}
}

func TestGetMaximumSetsBasedOnResourceModels(t *testing.T) {
	const (
		GPU  corev1.ResourceName = "nvidia.com/gpu"
		BIGU int64               = 100 // define a large upper bound so we can test model decision algo
	)

	tests := []struct {
		name         string
		cluster      clusterv1alpha1.Cluster
		components   []workv1alpha2.Component
		upperBound   int64
		expectError  bool
		expectedSets int64
	}{
		{
			name: "No grades defined → error",
			cluster: clusterv1alpha1.Cluster{
				Spec:   clusterv1alpha1.ClusterSpec{ResourceModels: nil},
				Status: clusterv1alpha1.ClusterStatus{ResourceSummary: &clusterv1alpha1.ResourceSummary{}},
			},
			components: []workv1alpha2.Component{
				comp("cpu-worker", 1, corev1.ResourceList{
					corev1.ResourceCPU: q("1500m"),
				}),
			},
			upperBound:   BIGU,
			expectError:  true,
			expectedSets: -1,
		},
		{
			name: "No resource requirement",
			cluster: clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{
						{Grade: uint(0), Ranges: []clusterv1alpha1.ResourceModelRange{{Name: corev1.ResourceCPU, Min: q("500m")}}},
						{Grade: uint(1), Ranges: []clusterv1alpha1.ResourceModelRange{{Name: corev1.ResourceCPU, Min: q("1000m")}}},
						{Grade: uint(2), Ranges: []clusterv1alpha1.ResourceModelRange{{Name: corev1.ResourceCPU, Min: q("2000m")}}}, // (first compliant grade for 1500m)
					},
				},
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						AllocatableModelings: []clusterv1alpha1.AllocatableModeling{
							{Grade: uint(0), Count: 5},
							{Grade: uint(1), Count: 0},
							{Grade: uint(2), Count: 10},
						},
					},
				},
			},
			components: []workv1alpha2.Component{
				comp("no-resource", 1, corev1.ResourceList{}),
			},
			upperBound:   BIGU,
			expectError:  false,
			expectedSets: BIGU,
		},
		{
			name: "CPU-only: 3 grades, only highest compliant",
			cluster: clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{
						{Grade: uint(0), Ranges: []clusterv1alpha1.ResourceModelRange{{Name: corev1.ResourceCPU, Min: q("500m")}}},
						{Grade: uint(1), Ranges: []clusterv1alpha1.ResourceModelRange{{Name: corev1.ResourceCPU, Min: q("1000m")}}},
						{Grade: uint(2), Ranges: []clusterv1alpha1.ResourceModelRange{{Name: corev1.ResourceCPU, Min: q("2000m")}}},
					},
				},
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						AllocatableModelings: []clusterv1alpha1.AllocatableModeling{
							{Grade: uint(0), Count: 5},
							{Grade: uint(1), Count: 0},
							{Grade: uint(2), Count: 10},
						},
					},
				},
			},
			components: []workv1alpha2.Component{
				comp("cpu-job", 1, corev1.ResourceList{
					corev1.ResourceCPU: q("1500m"),
				}),
			},
			upperBound:   BIGU,
			expectError:  false,
			expectedSets: 10, // 10 grade-2 nodes × 1 replica each = 10 sets (1 replica per set)
		},
		{
			name: "Multi-resource with GPUs across grades",
			cluster: clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{
						{Grade: uint(0), Ranges: []clusterv1alpha1.ResourceModelRange{
							{Name: corev1.ResourceCPU, Min: q("1000m")},
							{Name: corev1.ResourceMemory, Min: q("2Gi")},
							{Name: GPU, Min: q("0")},
						}},
						{Grade: uint(1), Ranges: []clusterv1alpha1.ResourceModelRange{
							{Name: corev1.ResourceCPU, Min: q("2000m")},
							{Name: corev1.ResourceMemory, Min: q("4Gi")},
							{Name: GPU, Min: q("1")},
						}},
						{Grade: uint(2), Ranges: []clusterv1alpha1.ResourceModelRange{
							{Name: corev1.ResourceCPU, Min: q("4000m")},
							{Name: corev1.ResourceMemory, Min: q("8Gi")},
							{Name: GPU, Min: q("2")},
						}},
					},
				},
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						AllocatableModelings: []clusterv1alpha1.AllocatableModeling{
							{Grade: uint(0), Count: 4},
							{Grade: uint(1), Count: 3},
							{Grade: uint(2), Count: 2},
						},
					},
				},
			},
			components: []workv1alpha2.Component{
				// One set has 2 replicas; each replica needs 2 CPU, 4Gi, 1 GPU.
				comp("gpu-worker", 2, corev1.ResourceList{
					corev1.ResourceCPU:    q("2000m"),
					corev1.ResourceMemory: q("4Gi"),
					GPU:                   q("1"),
				}),
			},
			upperBound:   BIGU,
			expectError:  false,
			expectedSets: 3,
		},
		{
			name: "No compliant grades = 0 sets",
			cluster: clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{
						{Grade: uint(0), Ranges: []clusterv1alpha1.ResourceModelRange{{Name: corev1.ResourceCPU, Min: q("1000m")}}},
						{Grade: uint(1), Ranges: []clusterv1alpha1.ResourceModelRange{{Name: corev1.ResourceCPU, Min: q("2000m")}}},
					},
				},
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						AllocatableModelings: []clusterv1alpha1.AllocatableModeling{
							{Grade: uint(0), Count: 10},
							{Grade: uint(1), Count: 10},
						},
					},
				},
			},
			components: []workv1alpha2.Component{
				comp("too-heavy", 1, corev1.ResourceList{
					corev1.ResourceCPU: q("3000m"),
				}),
			},
			upperBound:   BIGU,
			expectError:  false,
			expectedSets: 0,
		},
		{
			name: "Multi-component JM/TM across grades (CPU+Mem)",
			cluster: clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{
						{ // grade 0: fits JM (1 CPU, 2Gi)
							Grade: uint(0),
							Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: q("1000m")},
								{Name: corev1.ResourceMemory, Min: q("2Gi")},
							},
						},
						{ // grade 1: fits TM (3 CPU, 10Gi)
							Grade: uint(1),
							Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: q("3000m")},
								{Name: corev1.ResourceMemory, Min: q("10Gi")},
							},
						},
						{ // grade 2: fits both JM and a TM (4 CPU, 12Gi)
							Grade: uint(2),
							Ranges: []clusterv1alpha1.ResourceModelRange{
								{Name: corev1.ResourceCPU, Min: q("4000m")},
								{Name: corev1.ResourceMemory, Min: q("12Gi")},
							},
						},
					},
				},
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						AllocatableModelings: []clusterv1alpha1.AllocatableModeling{
							{Grade: uint(0), Count: 2}, // g0 nodes -> 2 JMs max
							{Grade: uint(1), Count: 6}, // g1 nodes -> 6 TMs max
							{Grade: uint(2), Count: 3}, // g2 nodes -> can fit 1 JM and 1 TM each
						},
						// Aggregate total resources for the cluster (example realistic values)
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    q("32"),    // ~32 cores
							corev1.ResourceMemory: q("100Gi"), // ~100 Gi memory
							corev1.ResourcePods:   q("1000"),  // arbitrary
						},
						// Some usage already allocated (these won't change the feasibility logic much)
						Allocated: corev1.ResourceList{
							corev1.ResourceCPU:    q("4"),
							corev1.ResourceMemory: q("4Gi"),
							corev1.ResourcePods:   q("50"),
						},
					},
				},
			},
			components: []workv1alpha2.Component{
				{
					Name:     "jobmanager",
					Replicas: 1,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    q("1"),
							corev1.ResourceMemory: q("2Gi"),
						},
					},
				},
				{
					Name:     "taskmanager",
					Replicas: 3,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    q("3"),
							corev1.ResourceMemory: q("10Gi"),
						},
					},
				},
			},
			upperBound:   100,
			expectError:  false,
			expectedSets: 3,
		},
		{
			name: "Three-component CRD across 4 grades (CPU+Mem)",
			cluster: clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{
						{Grade: uint(0), Ranges: []clusterv1alpha1.ResourceModelRange{
							{Name: corev1.ResourceCPU, Min: q("1000m")},
							{Name: corev1.ResourceMemory, Min: q("2Gi")},
						}},
						{Grade: uint(1), Ranges: []clusterv1alpha1.ResourceModelRange{
							{Name: corev1.ResourceCPU, Min: q("2000m")},
							{Name: corev1.ResourceMemory, Min: q("4Gi")},
						}},
						{Grade: uint(2), Ranges: []clusterv1alpha1.ResourceModelRange{
							{Name: corev1.ResourceCPU, Min: q("3000m")},
							{Name: corev1.ResourceMemory, Min: q("6Gi")},
						}},
						{Grade: uint(3), Ranges: []clusterv1alpha1.ResourceModelRange{
							{Name: corev1.ResourceCPU, Min: q("8000m")},
							{Name: corev1.ResourceMemory, Min: q("12Gi")},
						}},
					},
				},
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						AllocatableModelings: []clusterv1alpha1.AllocatableModeling{
							{Grade: uint(0), Count: 2},
							{Grade: uint(1), Count: 3},
							{Grade: uint(2), Count: 2},
							{Grade: uint(3), Count: 6},
						},
					},
				},
			},
			components: []workv1alpha2.Component{
				{
					Name:     "jobmanager",
					Replicas: 1,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    q("1"),
							corev1.ResourceMemory: q("2Gi"),
						},
					},
				},
				{
					Name:     "taskmanager",
					Replicas: 3,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    q("3"),
							corev1.ResourceMemory: q("10Gi"),
						},
					},
				},
				{
					Name:     "cache",
					Replicas: 1,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    q("500m"),
							corev1.ResourceMemory: q("1Gi"),
						},
					},
				},
			},
			upperBound:   100,
			expectError:  false,
			expectedSets: 2, // Two sets max, bound by TMs which can only fit on largest nodes.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getMaximumSetsBasedOnResourceModels(&tt.cluster, tt.components, tt.upperBound)
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedSets, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSets, got)
			}
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

func TestGetMaxAvailableComponentSetsGeneral(t *testing.T) {
	tests := []struct {
		name       string
		cluster    *clusterv1alpha1.Cluster
		components []workv1alpha2.Component
		expected   int32
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
			name: "empty component list should return max pod allowance",
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
							corev1.ResourceCPU:    resource.MustParse("10"),
							corev1.ResourceMemory: resource.MustParse("8Gi"),
						},
						Allocated: corev1.ResourceList{
							corev1.ResourcePods:   resource.MustParse("20"),
							corev1.ResourceCPU:    resource.MustParse("0"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
			components: []workv1alpha2.Component{
				{
					Name:     "jobmanager",
					Replicas: 1,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
				{
					Name:     "taskmanager",
					Replicas: 2,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1.5"),
						},
					},
				},
			},
			expected: 2,
		},
		{
			name: "resource estimation with mixed components",
			cluster: &clusterv1alpha1.Cluster{
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						Allocatable: corev1.ResourceList{
							corev1.ResourcePods:   resource.MustParse("100"),
							corev1.ResourceCPU:    resource.MustParse("10"),
							corev1.ResourceMemory: resource.MustParse("8Gi"),
						},
						Allocated: corev1.ResourceList{
							corev1.ResourcePods:   resource.MustParse("20"),
							corev1.ResourceCPU:    resource.MustParse("0"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
			components: []workv1alpha2.Component{
				{
					Name:     "jobmanager",
					Replicas: 1,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
				{
					Name:     "taskmanager",
					Replicas: 2,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2000m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
			// Per-set demand: 2 replicas × (2 CPU, 2Gi) + 1 replica x (1 CPU, 2Gi) = (5 CPU, 6Gi)
			// Cluster allocatable: 10 CPU, 6Gi
			// 10/5 = 2 sets (CPU), 6Gi/6 = 1 set (Mem)
			// min(2,1) = 1 set total
			expected: 1,
		},
		{
			name: "estimation limited by pod count",
			cluster: &clusterv1alpha1.Cluster{
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						Allocatable: corev1.ResourceList{
							corev1.ResourcePods:   resource.MustParse("3"),
							corev1.ResourceCPU:    resource.MustParse("100"),
							corev1.ResourceMemory: resource.MustParse("1Ti"),
						},
					},
				},
			},
			components: []workv1alpha2.Component{
				{
					Name:     "small-component",
					Replicas: 3,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("1Mi"),
						},
					},
				},
			},
			expected: 1, // limited by pods
		},
		{
			name: "custom resource estimation with GPUs",
			cluster: &clusterv1alpha1.Cluster{
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						Allocatable: corev1.ResourceList{
							corev1.ResourcePods:   resource.MustParse("20"),
							corev1.ResourceCPU:    resource.MustParse("40"),
							corev1.ResourceMemory: resource.MustParse("64Gi"),
							"nvidia.com/gpu":      resource.MustParse("8"),
						},
						Allocated: corev1.ResourceList{
							corev1.ResourcePods:   resource.MustParse("0"),
							corev1.ResourceCPU:    resource.MustParse("0"),
							corev1.ResourceMemory: resource.MustParse("0Gi"),
							"nvidia.com/gpu":      resource.MustParse("0"),
						},
					},
				},
			},
			components: []workv1alpha2.Component{
				{
					Name:     "gpu-worker",
					Replicas: 2,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("8Gi"),
							"nvidia.com/gpu":      resource.MustParse("1"),
						},
					},
				},
			},
			// Per-set demand: 2 replicas × (4 CPU, 8Gi, 1 GPU) = (8 CPU, 16Gi, 2 GPUs)
			// Cluster allocatable: 40 CPU, 64Gi, 8 GPUs
			// 40/8 = 5 sets (CPU), 64/16 = 4 sets (Mem), 8/2 = 4 sets (GPU)
			// min(5, 4, 4) = 4 sets total
			expected: 4,
		},
	}

	estimator := NewGeneralEstimator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimator.maxAvailableComponentSets(tt.cluster, tt.components)
			assert.Equal(t, tt.expected, result)
		})
	}
}
