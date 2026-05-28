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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/featuregate"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
)

// setFeatureGateDuringTest temporarily sets a feature gate to the given value
// and returns a cleanup function that restores the original value.
func setFeatureGateDuringTest(tb testing.TB, gate featuregate.FeatureGate, f featuregate.Feature, value bool) func() {
	originalValue := gate.Enabled(f)
	if err := gate.(featuregate.MutableFeatureGate).Set(fmt.Sprintf("%s=%v", f, value)); err != nil {
		tb.Errorf("error setting %s=%v: %v", f, value, err)
	}
	return func() {
		if err := gate.(featuregate.MutableFeatureGate).Set(fmt.Sprintf("%s=%v", f, originalValue)); err != nil {
			tb.Errorf("error restoring %s=%v: %v", f, originalValue, err)
		}
	}
}

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
			replicas, err := getMaximumReplicasBasedOnResourceModels(&tt.cluster, &tt.replicaRequirements, nil)
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
		BIGU int32               = 100 // define a large upper bound so we can test model decision algo
	)

	tests := []struct {
		name         string
		cluster      clusterv1alpha1.Cluster
		components   []workv1alpha2.Component
		upperBound   int32
		expectError  bool
		expectedSets int32
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
			got, err := getMaximumSetsBasedOnResourceModels(&tt.cluster, tt.components, nil, tt.upperBound)
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
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.SchedulingOvercommitProtection, true)()
	tests := []struct {
		name                string
		clusters            []*clusterv1alpha1.Cluster
		replicaRequirements *workv1alpha2.ReplicaRequirements
		assumedWorkloads    map[string][]AssumedWorkload
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
		{
			name: "assumed workloads reduce replicas",
			clusters: []*clusterv1alpha1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "member1"},
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
			assumedWorkloads: map[string][]AssumedWorkload{
				"member1": {
					{
						Namespace: "default",
						Components: []workv1alpha2.Component{
							{
								Replicas: 2,
								ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
									ResourceRequest: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("500m"),
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
								},
							},
						},
					},
				},
			},
			expectedTargets: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 4,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimator := NewGeneralEstimator()
			targets, err := estimator.MaxAvailableReplicas(context.Background(), ReplicaEstimationRequest{
				Clusters:            tt.clusters,
				ReplicaRequirements: tt.replicaRequirements,
				AssumedWorkloads:    tt.assumedWorkloads,
			})
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedTargets, targets)
		})
	}
}

func TestDeductAssumedWorkloadsFromSummary(t *testing.T) {
	tests := []struct {
		name             string
		summary          clusterv1alpha1.ResourceSummary
		assumedWorkloads []AssumedWorkload
		wantAllocating   corev1.ResourceList
	}{
		{
			name: "empty assumed workloads — summary unchanged",
			summary: clusterv1alpha1.ResourceSummary{
				Allocating: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("500m"),
				},
			},
			assumedWorkloads: nil,
			wantAllocating: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("500m"),
			},
		},
		{
			name:    "nil Allocating is initialized before deduction",
			summary: clusterv1alpha1.ResourceSummary{},
			assumedWorkloads: []AssumedWorkload{
				{
					Components: []workv1alpha2.Component{
						{
							Replicas: 1,
							ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
								ResourceRequest: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("100m"),
								},
							},
						},
					},
				},
			},
			wantAllocating: corev1.ResourceList{
				corev1.ResourcePods: resource.MustParse("1"),
				corev1.ResourceCPU:  resource.MustParse("100m"),
			},
		},
		{
			name:    "component with zero replicas is skipped",
			summary: clusterv1alpha1.ResourceSummary{},
			assumedWorkloads: []AssumedWorkload{
				{
					Components: []workv1alpha2.Component{
						{
							Replicas: 0,
							ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
								ResourceRequest: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("100m"),
								},
							},
						},
					},
				},
			},
			wantAllocating: corev1.ResourceList{},
		},
		{
			name:    "component with nil ReplicaRequirements only deducts pod count",
			summary: clusterv1alpha1.ResourceSummary{},
			assumedWorkloads: []AssumedWorkload{
				{
					Components: []workv1alpha2.Component{
						{Replicas: 3, ReplicaRequirements: nil},
					},
				},
			},
			wantAllocating: corev1.ResourceList{
				corev1.ResourcePods: resource.MustParse("3"),
			},
		},
		{
			name:    "multiple replicas — resources multiplied correctly",
			summary: clusterv1alpha1.ResourceSummary{},
			assumedWorkloads: []AssumedWorkload{
				{
					Components: []workv1alpha2.Component{
						{
							Replicas: 3,
							ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
								ResourceRequest: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
			wantAllocating: corev1.ResourceList{
				corev1.ResourcePods:   resource.MustParse("3"),
				corev1.ResourceCPU:    resource.MustParse("600m"),  // 200m × 3
				corev1.ResourceMemory: resource.MustParse("768Mi"), // 256Mi × 3
			},
		},
		{
			name:    "multiple workloads accumulate into Allocating",
			summary: clusterv1alpha1.ResourceSummary{},
			assumedWorkloads: []AssumedWorkload{
				{
					Components: []workv1alpha2.Component{
						{
							Replicas: 1,
							ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
								ResourceRequest: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("100m"),
								},
							},
						},
					},
				},
				{
					Components: []workv1alpha2.Component{
						{
							Replicas: 2,
							ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
								ResourceRequest: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("100m"),
								},
							},
						},
					},
				},
			},
			wantAllocating: corev1.ResourceList{
				corev1.ResourcePods: resource.MustParse("3"),    // 1 + 2
				corev1.ResourceCPU:  resource.MustParse("300m"), // 100m + 200m
			},
		},
		{
			// Validates that memory scaling uses Value() not MilliValue() to avoid int64 overflow.
			// 1Ti × 10000 replicas: MilliValue()-based math (1Ti×1000×10000 ≈ 1.1e19) would overflow int64 (max 9.2e18).
			name:    "large memory replicas do not overflow",
			summary: clusterv1alpha1.ResourceSummary{},
			assumedWorkloads: []AssumedWorkload{
				{
					Components: []workv1alpha2.Component{
						{
							Replicas: 10000,
							ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
								ResourceRequest: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1Ti"),
								},
							},
						},
					},
				},
			},
			wantAllocating: corev1.ResourceList{
				corev1.ResourcePods:   resource.MustParse("10000"),
				corev1.ResourceMemory: resource.MustParse("10000Ti"),
			},
		},
		{
			name: "existing Allocating values are accumulated, not replaced",
			summary: clusterv1alpha1.ResourceSummary{
				Allocating: corev1.ResourceList{
					corev1.ResourceCPU:  resource.MustParse("400m"),
					corev1.ResourcePods: resource.MustParse("5"),
				},
			},
			assumedWorkloads: []AssumedWorkload{
				{
					Components: []workv1alpha2.Component{
						{
							Replicas: 2,
							ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
								ResourceRequest: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("100m"),
								},
							},
						},
					},
				},
			},
			wantAllocating: corev1.ResourceList{
				corev1.ResourcePods: resource.MustParse("7"),    // 5 + 2
				corev1.ResourceCPU:  resource.MustParse("600m"), // 400m + 200m
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := tt.summary.DeepCopy()
			deductAssumedWorkloadsFromSummary(summary, tt.assumedWorkloads)

			assert.Equal(t, len(tt.wantAllocating), len(summary.Allocating),
				"Allocating map length mismatch")
			for resName, wantQty := range tt.wantAllocating {
				gotQty, ok := summary.Allocating[resName]
				assert.True(t, ok, "resource %s missing from Allocating", resName)
				assert.True(t, wantQty.Cmp(gotQty) == 0,
					"resource %s: want %s, got %s", resName, wantQty.String(), gotQty.String())
			}
		})
	}
}

func TestMaxAvailableReplicasGeneral(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.SchedulingOvercommitProtection, true)()
	tests := []struct {
		name                string
		cluster             *clusterv1alpha1.Cluster
		replicaRequirements *workv1alpha2.ReplicaRequirements
		assumedWorkloads    []AssumedWorkload
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
		{
			name: "assumed workloads are deducted",
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
			assumedWorkloads: []AssumedWorkload{
				{
					Namespace: "default",
					Components: []workv1alpha2.Component{
						{
							Replicas: 2,
							ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
								ResourceRequest: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
			expected: 4,
		},
	}

	estimator := NewGeneralEstimator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimator.maxAvailableReplicas(tt.cluster, tt.replicaRequirements, tt.assumedWorkloads)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMaxAvailableReplicasGeneralWithResourceModels verifies that when
// CustomizedClusterResourceModeling is enabled and assumed workloads are
// present, the cluster-summary bound (computed from the deducted
// resourceSummary) is applied in addition to the model-based result so that
// in-flight resource demands are not overlooked.
func TestMaxAvailableReplicasGeneralWithResourceModels(t *testing.T) {
	// Cluster topology:
	//   3 nodes each at Grade 1 (CPU min=1 core).
	//   Per-node capacity: 1 CPU → fits 2 replicas of 500m each.
	//   Total cluster capacity: 3 CPU, 100 pod slots.
	//
	// Assumed workload: 4 replicas × 500m CPU each → 2 CPU in-flight.
	//   After deduction: 1 CPU remaining → only 2 replicas fit.
	//   The ResourceModels path returns 6 (3 nodes × 2) but the cluster-summary
	//   bound (2) is also applied, so the result is 2.
	cluster := &clusterv1alpha1.Cluster{
		Spec: clusterv1alpha1.ClusterSpec{
			ResourceModels: []clusterv1alpha1.ResourceModel{
				{
					Grade: 0,
					Ranges: []clusterv1alpha1.ResourceModelRange{
						{Name: corev1.ResourceCPU, Min: resource.MustParse("0"), Max: resource.MustParse("1")},
					},
				},
				{
					Grade: 1,
					Ranges: []clusterv1alpha1.ResourceModelRange{
						{Name: corev1.ResourceCPU, Min: resource.MustParse("1"), Max: resource.MustParse("2")},
					},
				},
			},
		},
		Status: clusterv1alpha1.ClusterStatus{
			ResourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocatable: corev1.ResourceList{
					corev1.ResourcePods: resource.MustParse("100"),
					corev1.ResourceCPU:  resource.MustParse("3"),
				},
				AllocatableModelings: []clusterv1alpha1.AllocatableModeling{
					{Grade: 0, Count: 0},
					{Grade: 1, Count: 3},
				},
			},
		},
	}
	reqs := &workv1alpha2.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("500m"),
		},
	}
	assumed := []AssumedWorkload{
		{
			Namespace: "default",
			Components: []workv1alpha2.Component{
				{
					Replicas: 4,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("500m"),
						},
					},
				},
			},
		},
	}

	defer setFeatureGateDuringTest(t, features.FeatureGate, features.CustomizedClusterResourceModeling, true)()

	estimator := NewGeneralEstimator()

	// Without assumed workloads: model path returns 6 (3 nodes × 2 replicas/node).
	assert.Equal(t, int32(6), estimator.maxAvailableReplicas(cluster, reqs, nil),
		"baseline without assumed workloads should equal model result")

	// With assumed workloads: cluster-summary bound (1 CPU left → 2 replicas) must win.
	assert.Equal(t, int32(2), estimator.maxAvailableReplicas(cluster, reqs, assumed),
		"assumed workloads should reduce result via cluster-summary bound")
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
			result := estimator.maxAvailableComponentSets(tt.cluster, tt.components, nil)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMaxAvailableComponentSets_AssumedWorkloadDeduction(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.SchedulingOvercommitProtection, true)()
	// Base cluster: 100 pods, 10 CPU, 8Gi memory available.
	baseCluster := func(cpuAlloc, memAllocGi string) *clusterv1alpha1.Cluster {
		return &clusterv1alpha1.Cluster{
			Status: clusterv1alpha1.ClusterStatus{
				ResourceSummary: &clusterv1alpha1.ResourceSummary{
					Allocatable: corev1.ResourceList{
						corev1.ResourcePods:   resource.MustParse("100"),
						corev1.ResourceCPU:    resource.MustParse(cpuAlloc),
						corev1.ResourceMemory: resource.MustParse(memAllocGi),
					},
				},
			},
		}
	}

	// Component set: 1 jobmanager (1 CPU) + 2 taskmanagers (1.5 CPU each) = 4 CPU per set
	finkComponents := []workv1alpha2.Component{
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
					corev1.ResourceCPU: resource.MustParse("1500m"),
				},
			},
		},
	}

	tests := []struct {
		name             string
		cluster          *clusterv1alpha1.Cluster
		components       []workv1alpha2.Component
		assumedWorkloads []AssumedWorkload
		expected         int32
	}{
		{
			name:       "no assumed workloads — baseline unchanged",
			cluster:    baseCluster("10", "8Gi"),
			components: finkComponents,
			// 10 CPU / 4 CPU-per-set = 2 sets
			expected: 2,
		},
		{
			name:       "assumed workload deducts CPU, reduces set count",
			cluster:    baseCluster("10", "8Gi"),
			components: finkComponents,
			assumedWorkloads: []AssumedWorkload{
				{
					Namespace: "ns",
					Components: []workv1alpha2.Component{
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
									corev1.ResourceCPU: resource.MustParse("1500m"),
								},
							},
						},
					},
				},
			},
			// Assumed uses 4 CPU → remaining 6 CPU / 4 per-set = 1 set
			expected: 1,
		},
		{
			name:       "assumed workload exhausts all CPU → 0 sets",
			cluster:    baseCluster("8", "8Gi"),
			components: finkComponents,
			assumedWorkloads: []AssumedWorkload{
				{
					Namespace: "ns",
					Components: []workv1alpha2.Component{
						{
							Name:     "taskmanager",
							Replicas: 4,
							ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
								ResourceRequest: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("2"),
								},
							},
						},
					},
				},
			},
			// Assumed uses 8 CPU → 0 remaining → 0 sets
			expected: 0,
		},
		{
			name: "assumed workload exhausts pod budget → 0 sets",
			cluster: &clusterv1alpha1.Cluster{
				Status: clusterv1alpha1.ClusterStatus{
					ResourceSummary: &clusterv1alpha1.ResourceSummary{
						Allocatable: corev1.ResourceList{
							corev1.ResourcePods: resource.MustParse("3"),
							corev1.ResourceCPU:  resource.MustParse("100"),
						},
					},
				},
			},
			components: finkComponents, // 3 pods per set
			assumedWorkloads: []AssumedWorkload{
				{
					Components: []workv1alpha2.Component{
						{Name: "worker", Replicas: 3},
					},
				},
			},
			// 3 pods assumed, 0 remaining → 0 sets
			expected: 0,
		},
		{
			name:       "multiple assumed workloads — cumulative deduction",
			cluster:    baseCluster("12", "8Gi"),
			components: finkComponents,
			assumedWorkloads: []AssumedWorkload{
				{
					Namespace: "ns-a",
					Components: []workv1alpha2.Component{
						{
							Name:     "jobmanager",
							Replicas: 1,
							ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
								ResourceRequest: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("1"),
								},
							},
						},
					},
				},
				{
					Namespace: "ns-b",
					Components: []workv1alpha2.Component{
						{
							Name:     "taskmanager",
							Replicas: 2,
							ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
								ResourceRequest: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("1500m"),
								},
							},
						},
					},
				},
			},
			// Assumed total: 1 CPU + 3 CPU = 4 CPU → remaining 8 CPU / 4 per-set = 2 sets
			expected: 2,
		},
		{
			name:       "assumed workload with no resource requirements — only pod deduction",
			cluster:    baseCluster("10", "8Gi"),
			components: finkComponents,
			assumedWorkloads: []AssumedWorkload{
				{
					Components: []workv1alpha2.Component{
						{Name: "no-req-worker", Replicas: 3},
					},
				},
			},
			// Pods: 100 - 3 = 97; 97/3 = 32 pod-bound sets; CPU: 10/4 = 2 sets → min = 2
			expected: 2,
		},
	}

	ge := NewGeneralEstimator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ge.maxAvailableComponentSets(tt.cluster, tt.components, tt.assumedWorkloads)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMaxAvailableComponentSets_ResourceModelsWithAssumedWorkloads verifies that
// when CustomizedClusterResourceModeling is enabled, assumed workloads are
// pre-simulated onto the virtual model nodes before estimating how many target
// component sets can still be placed.
func TestMaxAvailableComponentSets_ResourceModelsWithAssumedWorkloads(t *testing.T) {
	// Cluster topology:
	//   2 nodes each at Grade 1 (CPU min=2 cores, pods=110).
	//   Total aggregate capacity: 4 CPU.
	//
	// Target component set: 1 × 1-CPU replica → 1 CPU per set.
	//   Without assumed workloads:   4 CPU / 1 = 4 sets.
	//   podBound: 220 pods / 1 = 220. resourceBound = 4. maxSets = 4.
	//   Model simulation (nodes at full capacity): fits 4 sets (2 per node × 2 nodes).
	//
	// Assumed workload: 1 set of 2 replicas × 1 CPU each → 2 CPU consumed.
	//   After aggregate deduction: 2 CPU remaining → maxSets = 2.
	//   Model simulation pre-consuming assumed workload also gives 2.
	cluster := &clusterv1alpha1.Cluster{
		Spec: clusterv1alpha1.ClusterSpec{
			ResourceModels: []clusterv1alpha1.ResourceModel{
				{
					Grade: 0,
					Ranges: []clusterv1alpha1.ResourceModelRange{
						{Name: corev1.ResourceCPU, Min: resource.MustParse("0"), Max: resource.MustParse("2")},
					},
				},
				{
					Grade: 1,
					Ranges: []clusterv1alpha1.ResourceModelRange{
						{Name: corev1.ResourceCPU, Min: resource.MustParse("2"), Max: resource.MustParse("4")},
					},
				},
			},
		},
		Status: clusterv1alpha1.ClusterStatus{
			ResourceSummary: &clusterv1alpha1.ResourceSummary{
				Allocatable: corev1.ResourceList{
					corev1.ResourcePods: resource.MustParse("220"),
					corev1.ResourceCPU:  resource.MustParse("4"),
				},
				AllocatableModelings: []clusterv1alpha1.AllocatableModeling{
					{Grade: 0, Count: 0},
					{Grade: 1, Count: 2}, // 2 nodes × 2 CPU min each
				},
			},
		},
	}
	components := []workv1alpha2.Component{
		{
			Name:     "worker",
			Replicas: 1,
			ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
				},
			},
		},
	}
	// Assumed workload consumes 2 replicas × 1 CPU = 2 CPU in total (one full node's worth).
	assumed := []AssumedWorkload{
		{
			Components: []workv1alpha2.Component{
				{
					Name:     "assumed-worker",
					Replicas: 2,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
			},
		},
	}

	defer setFeatureGateDuringTest(t, features.FeatureGate, features.CustomizedClusterResourceModeling, true)()

	ge := NewGeneralEstimator()

	// Baseline: no assumed workloads → model simulation gives 4 sets (2 per node × 2 nodes).
	assert.Equal(t, int32(4), ge.maxAvailableComponentSets(cluster, components, nil),
		"baseline without assumed workloads should equal model capacity")

	// With assumed workload: 2 CPU consumed on one node, leaving 2 CPU (1 node) → 2 sets.
	assert.Equal(t, int32(2), ge.maxAvailableComponentSets(cluster, components, assumed),
		"assumed workloads must reduce model simulation result")
}

// TestMaxAvailableComponentSets_PerClusterRouting verifies that AssumedWorkloads are routed
// per-cluster: each cluster only sees its own assumed workloads, not those of other clusters.
func TestMaxAvailableComponentSets_PerClusterRouting(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.SchedulingOvercommitProtection, true)()
	// Both clusters have identical capacity: 100 pods, 8 CPU.
	makeCluster := func(name string) *clusterv1alpha1.Cluster {
		return &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Status: clusterv1alpha1.ClusterStatus{
				ResourceSummary: &clusterv1alpha1.ResourceSummary{
					Allocatable: corev1.ResourceList{
						corev1.ResourcePods: resource.MustParse("100"),
						corev1.ResourceCPU:  resource.MustParse("8"),
					},
				},
			},
		}
	}

	// 1 set = 4 CPU; 8 CPU → 2 sets without any assumed workloads.
	components := []workv1alpha2.Component{
		{
			Name:     "worker",
			Replicas: 2,
			ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("2"),
				},
			},
		},
	}

	// Only cluster-a has 4 CPU assumed; cluster-b has none.
	assumed4CPU := []AssumedWorkload{
		{
			Components: []workv1alpha2.Component{
				{
					Name:     "assumed-worker",
					Replicas: 2,
					ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"),
						},
					},
				},
			},
		},
	}

	req := ComponentSetEstimationRequest{
		Clusters:   []*clusterv1alpha1.Cluster{makeCluster("cluster-a"), makeCluster("cluster-b")},
		Components: components,
		AssumedWorkloads: map[string][]AssumedWorkload{
			"cluster-a": assumed4CPU,
			// cluster-b intentionally absent
		},
	}

	ge := NewGeneralEstimator()
	resp, err := ge.MaxAvailableComponentSets(context.Background(), req)
	assert.NoError(t, err)
	assert.Len(t, resp, 2)

	byName := map[string]int32{}
	for _, r := range resp {
		byName[r.Name] = r.Sets
	}
	// cluster-a: 8 CPU - 4 assumed = 4 remaining → 4/4 = 1 set
	assert.Equal(t, int32(1), byName["cluster-a"], "cluster-a should have 1 set after deduction")
	// cluster-b: no assumptions → 8/4 = 2 sets
	assert.Equal(t, int32(2), byName["cluster-b"], "cluster-b should have 2 sets (unaffected)")
}
