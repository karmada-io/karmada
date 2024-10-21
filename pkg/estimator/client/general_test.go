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
	"testing"

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
			if tt.expectError && err == nil {
				t.Errorf("Expects an error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("getMaximumReplicasBasedOnResourceModels() returned an unexpected error: %v", err)
			}
			if replicas != tt.expectedReplicas {
				t.Errorf("getMaximumReplicasBasedOnResourceModels() = %v, expectedReplicas %v", replicas, tt.expectedReplicas)
			}
		})
	}
}
