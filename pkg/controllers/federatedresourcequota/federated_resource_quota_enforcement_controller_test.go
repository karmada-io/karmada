/*
Copyright 2025 The Karmada Authors.

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

package federatedresourcequota

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestCalculateUsedWithResourceBinding(t *testing.T) {
	tests := []struct {
		name     string
		bindings []workv1alpha2.ResourceBinding
		overall  corev1.ResourceList
		expected corev1.ResourceList
	}{
		{
			name: "single binding",
			bindings: []workv1alpha2.ResourceBinding{
				makeBinding("500m", "128Mi", []workv1alpha2.TargetCluster{
					{
						Name:     "Cluster1",
						Replicas: int32(1),
					},
					{
						Name:     "Cluster2",
						Replicas: int32(2),
					},
				}),
			},
			overall: makeResourceRequest("2000m", "1Gi"),
			expected: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1500m"),
				corev1.ResourceMemory: resource.MustParse("384Mi"),
			},
		},
		{
			name: "multiple bindings",
			bindings: []workv1alpha2.ResourceBinding{
				makeBinding("1", "2Gi", []workv1alpha2.TargetCluster{
					{
						Name:     "Cluster1",
						Replicas: int32(1),
					},
					{
						Name:     "Cluster2",
						Replicas: int32(1),
					},
				}),
				makeBinding("500m", "500Mi", []workv1alpha2.TargetCluster{
					{
						Name:     "Cluster1",
						Replicas: int32(2),
					},
					{
						Name:     "Cluster2",
						Replicas: int32(2),
					},
				}),
				makeBinding("2", "1Gi", []workv1alpha2.TargetCluster{
					{
						Name:     "Cluster1",
						Replicas: int32(1),
					},
					{
						Name:     "Cluster2",
						Replicas: int32(1),
					},
				}),
			},
			overall: makeResourceRequest("10", "10Gi"),
			expected: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("8144Mi"),
			},
		},
		{
			name: "single binding, overall only includes cpu",
			bindings: []workv1alpha2.ResourceBinding{
				makeBinding("500m", "1Gi", []workv1alpha2.TargetCluster{
					{
						Name:     "Cluster1",
						Replicas: int32(1),
					},
					{
						Name:     "Cluster2",
						Replicas: int32(2),
					},
				}),
			},
			overall: makeResourceRequest("10", ""),
			expected: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1500m"),
			},
		},
		{
			name: "skip binding with nil ResourceRequest",
			bindings: []workv1alpha2.ResourceBinding{
				{
					Spec: workv1alpha2.ResourceBindingSpec{
						Clusters: []workv1alpha2.TargetCluster{
							{Name: "cluster1"},
							{Name: "cluster2"},
						},
						ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
							ResourceRequest: nil,
						},
					},
				},
				makeBinding("200m", "128Mi", []workv1alpha2.TargetCluster{
					{
						Name:     "Cluster1",
						Replicas: int32(1),
					},
					{
						Name:     "Cluster2",
						Replicas: int32(1),
					},
				}),
			},
			overall: makeResourceRequest("10", "10Gi"),
			expected: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("400m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
		{
			name:     "empty binding list",
			bindings: []workv1alpha2.ResourceBinding{},
			overall:  makeResourceRequest("10", "10Gi"),
			expected: corev1.ResourceList{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			used := calculateUsedWithResourceBinding(tt.bindings, tt.overall)
			for res, expectedQuantity := range tt.expected {
				actualQuantity, exists := used[res]
				require.True(t, exists, "expected resource %s to exist", res)
				require.Equal(t, expectedQuantity.String(), actualQuantity.String(), "resource %s mismatch", res)
			}
			// Double check there are no unexpected resources
			require.Equal(t, len(tt.expected), len(used), "expected resource count mismatch")
		})
	}
}

func makeBinding(cpu string, memory string, clusters []workv1alpha2.TargetCluster) workv1alpha2.ResourceBinding {
	return workv1alpha2.ResourceBinding{
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: clusters,
			ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: makeResourceRequest(cpu, memory),
			},
		},
	}
}

func makeResourceRequest(cpu string, memory string) map[corev1.ResourceName]resource.Quantity {
	if cpu != "" && memory != "" {
		return map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse(cpu),
			corev1.ResourceMemory: resource.MustParse(memory),
		}
	} else if cpu != "" {
		return map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU: resource.MustParse(cpu),
		}
	}
	return map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceMemory: resource.MustParse(memory),
	}
}
