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

package estimator

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/util"
	schedulerframework "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework"
)

func TestMatchNode(t *testing.T) {
	tests := []struct {
		name                string
		replicaRequirements pb.ReplicaRequirements
		node                *schedulerframework.NodeInfo
		expected            bool
	}{
		{
			name: "no enough information to perform the match operation - should match",
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
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
			node: func() *schedulerframework.NodeInfo {
				return schedulerframework.NewNodeInfo()
			}(),
			expected: true,
		},
		{
			name: "no constraints - should match",
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
				},
			},
			node: func() *schedulerframework.NodeInfo {
				nodeInfo := schedulerframework.NewNodeInfo()
				nodeInfo.SetNode(makeNode("node1", map[string]string{}, corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("4"),
				}))
				return nodeInfo
			}(),
			expected: true,
		},
		{
			name: "node affinity matches",
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
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
			node: func() *schedulerframework.NodeInfo {
				nodeInfo := schedulerframework.NewNodeInfo()
				nodeInfo.SetNode(makeNode("node1", map[string]string{"zone": "us-west"}, corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("4"),
				}))
				return nodeInfo
			}(),
			expected: true,
		},
		{
			name: "node affinity does not match",
			replicaRequirements: pb.ReplicaRequirements{
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("1"),
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
			node: func() *schedulerframework.NodeInfo {
				nodeInfo := schedulerframework.NewNodeInfo()
				nodeInfo.SetNode(makeNode("node1", map[string]string{"zone": "us-east"}, corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("4"),
				}))
				return nodeInfo
			}(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchNode(tt.replicaRequirements.NodeClaim, tt.node)
			if result != tt.expected {
				t.Errorf("matchNode() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestSchedulingSimulator_SimulateSchedulingFF(t *testing.T) {
	tests := []struct {
		name         string
		nodes        []*schedulerframework.NodeInfo
		components   []pb.Component
		upperBound   int32
		expectedSets int32
	}{
		{
			name: "single component fits perfectly",
			nodes: []*schedulerframework.NodeInfo{
				createNodeInfo("node1", corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
			},
			components: []pb.Component{
				{
					Name:     "web",
					Replicas: 2,
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
			upperBound:   10,
			expectedSets: 2, // 4 CPU / 1 CPU per replica = 4 replicas, 4 / 2 replicas per set = 2 sets
		},
		{
			name: "multiple components with resource constraints",
			nodes: []*schedulerframework.NodeInfo{
				createNodeInfo("node1", corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("16Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
			},
			components: []pb.Component{
				{
					Name:     "web",
					Replicas: 2,
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
				{
					Name:     "db",
					Replicas: 1,
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				},
			},
			upperBound:   10,
			expectedSets: 2, // Each set needs 4 CPU (2*1 + 1*2) and 8Gi memory (2*2 + 1*4), so 2 sets fit
		},
		{
			name: "no resources available",
			nodes: []*schedulerframework.NodeInfo{
				createNodeInfo("node1", corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0"),
					corev1.ResourceMemory: resource.MustParse("0"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
			},
			components: []pb.Component{
				{
					Name:     "web",
					Replicas: 1,
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
			},
			upperBound:   10,
			expectedSets: 0,
		},
		{
			name: "upper bound limits scheduling",
			nodes: []*schedulerframework.NodeInfo{
				createNodeInfo("node1", corev1.ResourceList{
					corev1.ResourceCPU:  resource.MustParse("100"),
					corev1.ResourcePods: resource.MustParse("10"),
				}),
			},
			components: []pb.Component{
				{
					Name:     "web",
					Replicas: 1,
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
			},
			upperBound:   3,
			expectedSets: 3, // Limited by upper bound, not resources
		},
		{
			name: "multiple nodes distribution",
			nodes: []*schedulerframework.NodeInfo{
				createNodeInfo("node1", corev1.ResourceList{
					corev1.ResourceCPU:  resource.MustParse("2"),
					corev1.ResourcePods: resource.MustParse("10"),
				}),
				createNodeInfo("node2", corev1.ResourceList{
					corev1.ResourceCPU:  resource.MustParse("2"),
					corev1.ResourcePods: resource.MustParse("10"),
				}),
			},
			components: []pb.Component{
				{
					Name:     "web",
					Replicas: 3,
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
			},
			upperBound:   10,
			expectedSets: 1, // Total 4 CPU, need 3 CPU per set, so 1 set fits
		},
		{
			name: "nodes with different capacities",
			nodes: []*schedulerframework.NodeInfo{
				createNodeInfo("small-node", corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
					corev1.ResourcePods:   resource.MustParse("10"),
				}),
				createNodeInfo("medium-node", corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("20"),
				}),
				createNodeInfo("large-node", corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("16Gi"),
					corev1.ResourcePods:   resource.MustParse("30"),
				}),
			},
			components: []pb.Component{
				{
					Name:     "frontend",
					Replicas: 2,
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
				{
					Name:     "backend",
					Replicas: 1,
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				},
			},
			upperBound:   10,
			expectedSets: 4,
		},
		{
			name: "many small nodes",
			nodes: []*schedulerframework.NodeInfo{
				createNodeInfo("node1", corev1.ResourceList{
					corev1.ResourceCPU:  resource.MustParse("1"),
					corev1.ResourcePods: resource.MustParse("5"),
				}),
				createNodeInfo("node2", corev1.ResourceList{
					corev1.ResourceCPU:  resource.MustParse("1"),
					corev1.ResourcePods: resource.MustParse("5"),
				}),
				createNodeInfo("node3", corev1.ResourceList{
					corev1.ResourceCPU:  resource.MustParse("1"),
					corev1.ResourcePods: resource.MustParse("5"),
				}),
				createNodeInfo("node4", corev1.ResourceList{
					corev1.ResourceCPU:  resource.MustParse("1"),
					corev1.ResourcePods: resource.MustParse("5"),
				}),
			},
			components: []pb.Component{
				{
					Name:     "microservice",
					Replicas: 1,
					ReplicaRequirements: pb.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"),
						},
					},
				},
			},
			upperBound:   10,
			expectedSets: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			simulator := NewSchedulingSimulator(tt.nodes)
			result := simulator.SimulateScheduling(tt.components, tt.upperBound)
			if result != tt.expectedSets {
				t.Errorf("SimulateScheduling() = %d, expected %d", result, tt.expectedSets)
			}
		})
	}
}

// Helper functions

func createNodeInfo(name string, allocatable corev1.ResourceList) *schedulerframework.NodeInfo {
	nodeInfo := schedulerframework.NewNodeInfo()
	nodeInfo.SetNode(makeNode(name, map[string]string{}, allocatable))
	// Set the Allocatable field using util.NewResource
	nodeInfo.Allocatable = util.NewResource(allocatable)
	return nodeInfo
}

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
