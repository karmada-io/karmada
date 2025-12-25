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

package noderesource

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
	schedcache "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/cache"
)

func TestNodeResourceEstimator_EstimateComponents(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		nodes      []*corev1.Node
		pods       []*corev1.Pod
		components []pb.Component
		expected   int32
		wantCode   framework.Code
	}{
		{
			name:    "plugin disabled",
			enabled: false,
			nodes: []*corev1.Node{
				makeNode("node1", map[string]string{}, corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
			},
			components: []pb.Component{
				{
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
					Replicas: 2,
				},
			},
			expected: 2147483647, // math.MaxInt32
			wantCode: framework.Noopperation,
		},
		{
			name:    "single component single replica fits in single node",
			enabled: true,
			nodes: []*corev1.Node{
				makeNode("node1", map[string]string{}, corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
			},
			components: []pb.Component{
				{
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
					Replicas: 1,
				},
			},
			expected: 4, // node can fit 4 sets
			wantCode: framework.Success,
		},
		{
			name:    "single component multiple replicas fits in single node",
			enabled: true,
			nodes: []*corev1.Node{
				makeNode("node1", map[string]string{}, corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
			},
			components: []pb.Component{
				{
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
					Replicas: 2,
				},
			},
			expected: 2, // each set needs 2 replicas, node can fit 2 sets
			wantCode: framework.Success,
		},
		{
			name:    "multiple components fit in single node",
			enabled: true,
			nodes: []*corev1.Node{
				makeNode("node1", map[string]string{}, corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
			},
			components: []pb.Component{
				{
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
					Replicas: 1,
				},
				{
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("3"),
							corev1.ResourceMemory: resource.MustParse("3Gi"),
						},
					},
					Replicas: 1,
				},
			},
			expected: 2, // each set needs 5 CPU + 5Gi memory, node can fit 2 sets
			wantCode: framework.Success,
		},
		{
			name:    "components spread across multiple nodes",
			enabled: true,
			nodes: []*corev1.Node{
				makeNode("node1", map[string]string{}, corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("6"),
					corev1.ResourceMemory: resource.MustParse("6Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
				makeNode("node2", map[string]string{}, corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("6"),
					corev1.ResourceMemory: resource.MustParse("6Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
			},
			components: []pb.Component{
				{
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("3"),
							corev1.ResourceMemory: resource.MustParse("3Gi"),
						},
					},
					Replicas: 2,
				},
				{
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
					Replicas: 1,
				},
			},
			expected: 1, // each set needs 8 CPU + 8Gi memory, can fit 1 set across nodes
			wantCode: framework.Success,
		},
		{
			name:    "insufficient resources",
			enabled: true,
			nodes: []*corev1.Node{
				makeNode("node1", map[string]string{}, corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
			},
			components: []pb.Component{
				{
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("3"),
							corev1.ResourceMemory: resource.MustParse("3Gi"),
						},
					},
					Replicas: 1,
				},
			},
			expected: 0, // not enough resources
			wantCode: framework.Unschedulable,
		},
		{
			name:    "node with existing pods",
			enabled: true,
			nodes: []*corev1.Node{
				makeNode("node1", map[string]string{}, corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
			},
			pods: []*corev1.Pod{
				makePod("pod1", "node1", corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				}),
			},
			components: []pb.Component{
				{
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
					Replicas: 1,
				},
			},
			expected: 3, // remaining: 3 CPU + 6Gi memory, can fit 3 sets
			wantCode: framework.Success,
		},
		{
			name:    "node affinity constraints",
			enabled: true,
			nodes: []*corev1.Node{
				makeNode("node1", map[string]string{"zone": "us-west"}, corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
				makeNode("node2", map[string]string{"zone": "us-east"}, corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
			},
			components: []pb.Component{
				{
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						NodeClaim: &pb.NodeClaim{
							NodeAffinity: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "zone",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"us-west"},
											},
										},
									},
								},
							},
						},
					},
					Replicas: 1,
				},
			},
			expected: 4, // only node1 matches affinity, can fit 4 sets
			wantCode: framework.Success,
		},
		{
			name:    "no component match node affinity constraints",
			enabled: true,
			nodes: []*corev1.Node{
				makeNode("node1", map[string]string{"zone": "us-west"}, corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
				makeNode("node2", map[string]string{"zone": "us-east"}, corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
			},
			components: []pb.Component{
				{
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						NodeClaim: &pb.NodeClaim{
							NodeAffinity: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "zone",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"us-south"},
											},
										},
									},
								},
							},
						},
					},
					Replicas: 4,
				},
			},
			expected: 0,
			wantCode: framework.Unschedulable,
		},
		{
			name:    "empty components",
			enabled: true,
			nodes: []*corev1.Node{
				makeNode("node1", map[string]string{}, corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
			},
			components: []pb.Component{},
			expected:   noNodeConstraint,
			wantCode:   framework.Noopperation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create snapshot
			snapshot := schedcache.NewSnapshot(tt.pods, tt.nodes)

			// Create plugin
			pl := &nodeResourceEstimator{
				enabled: tt.enabled,
			}

			// Execute test
			result, status := pl.EstimateComponents(context.Background(), snapshot, tt.components, "")

			// Verify results
			if result != tt.expected {
				t.Errorf("EstimateComponents() result = %v, expected %v", result, tt.expected)
			}

			if status.Code() != tt.wantCode {
				t.Errorf("EstimateComponents() status code = %v, expected %v", status.Code(), tt.wantCode)
			}
		})
	}
}

// Helper functions

func makeNode(name string, labels map[string]string, allocatable corev1.ResourceList) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Status: corev1.NodeStatus{
			Allocatable: allocatable,
			Capacity:    allocatable,
		},
	}
}

func makePod(name, nodeName string, requests corev1.ResourceList) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: requests,
					},
				},
			},
		},
	}
}
