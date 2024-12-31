/*
Copyright 2024 The Kubernetes Authors.

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

package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework"
)

// TestNode represents a test node configuration
type TestNode struct {
	name      string
	labels    map[string]string
	images    []corev1.ContainerImage
	resources corev1.ResourceList
}

// TestPod represents a test pod configuration
type TestPod struct {
	name      string
	namespace string
	uid       string
	nodeName  string
	labels    map[string]string
	volumes   []corev1.Volume
	affinity  *corev1.Affinity
}

func TestCacheNode(t *testing.T) {
	tests := []struct {
		name    string
		nodes   []TestNode
		ops     func(*testing.T, Cache, []*corev1.Node)
		verify  func(*testing.T, Cache)
		wantErr bool
	}{
		{
			name: "add single node",
			nodes: []TestNode{
				{
					name: "node1",
					resources: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				},
			},
			ops: func(t *testing.T, c Cache, nodes []*corev1.Node) {
				nodeInfo := c.AddNode(nodes[0])
				require.NotNil(t, nodeInfo)
				assert.Equal(t, nodes[0].Name, nodeInfo.Node().Name)
			},
			verify: func(t *testing.T, c Cache) {
				assert.Equal(t, 1, c.NodeCount())
			},
		},
		{
			name: "update node resources",
			nodes: []TestNode{
				{
					name: "node1",
					resources: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				},
			},
			ops: func(t *testing.T, c Cache, nodes []*corev1.Node) {
				c.AddNode(nodes[0])

				// Update node with new resources
				updatedNode := nodes[0].DeepCopy()
				updatedNode.Status.Allocatable = corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("2"),
				}

				nodeInfo := c.UpdateNode(nodes[0], updatedNode)
				require.NotNil(t, nodeInfo)
				assert.Equal(t, "2", nodeInfo.Node().Status.Allocatable.Cpu().String())
			},
			verify: func(t *testing.T, c Cache) {
				assert.Equal(t, 1, c.NodeCount())
			},
		},
		{
			name: "remove node",
			nodes: []TestNode{
				{name: "node1"},
			},
			ops: func(t *testing.T, c Cache, nodes []*corev1.Node) {
				c.AddNode(nodes[0])
				err := c.RemoveNode(nodes[0])
				assert.NoError(t, err)
			},
			verify: func(t *testing.T, c Cache) {
				assert.Equal(t, 0, c.NodeCount())
			},
		},
		{
			name: "remove non-existent node",
			nodes: []TestNode{
				{name: "node1"},
			},
			ops: func(t *testing.T, c Cache, nodes []*corev1.Node) {
				err := c.RemoveNode(nodes[0])
				assert.Error(t, err)
			},
			verify: func(t *testing.T, c Cache) {
				assert.Equal(t, 0, c.NodeCount())
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := newCache(time.Second, time.Second, nil)
			nodes := make([]*corev1.Node, len(tt.nodes))
			for i, n := range tt.nodes {
				nodes[i] = newTestNode(n)
			}

			tt.ops(t, cache, nodes)
			tt.verify(t, cache)
		})
	}
}

func TestCachePod(t *testing.T) {
	tests := []struct {
		name    string
		node    TestNode
		pods    []TestPod
		ops     func(*testing.T, Cache, *corev1.Node, []*corev1.Pod)
		verify  func(*testing.T, Cache)
		wantErr bool
	}{
		{
			name: "add pod",
			node: TestNode{name: "node1"},
			pods: []TestPod{
				{
					name:      "pod1",
					namespace: "default",
					uid:       "pod1",
					nodeName:  "node1",
				},
			},
			ops: func(t *testing.T, c Cache, node *corev1.Node, pods []*corev1.Pod) {
				c.AddNode(node)
				err := c.AddPod(pods[0])
				assert.NoError(t, err)
			},
			verify: func(t *testing.T, c Cache) {
				count, err := c.PodCount()
				assert.NoError(t, err)
				assert.Equal(t, 1, count)
			},
		},
		{
			name: "assume pod",
			node: TestNode{name: "node1"},
			pods: []TestPod{
				{
					name:      "pod1",
					namespace: "default",
					uid:       "pod1",
					nodeName:  "node1",
				},
			},
			ops: func(t *testing.T, c Cache, node *corev1.Node, pods []*corev1.Pod) {
				c.AddNode(node)
				err := c.AssumePod(pods[0])
				assert.NoError(t, err)

				assumed, err := c.IsAssumedPod(pods[0])
				assert.NoError(t, err)
				assert.True(t, assumed)
			},
			verify: func(t *testing.T, c Cache) {
				count, err := c.PodCount()
				assert.NoError(t, err)
				assert.Equal(t, 1, count)
			},
		},
		{
			name: "forget assumed pod",
			node: TestNode{name: "node1"},
			pods: []TestPod{
				{
					name:      "pod1",
					namespace: "default",
					uid:       "pod1",
					nodeName:  "node1",
				},
			},
			ops: func(t *testing.T, c Cache, node *corev1.Node, pods []*corev1.Pod) {
				c.AddNode(node)
				_ = c.AssumePod(pods[0])
				err := c.ForgetPod(pods[0])
				assert.NoError(t, err)

				assumed, err := c.IsAssumedPod(pods[0])
				assert.NoError(t, err)
				assert.False(t, assumed)
			},
			verify: func(t *testing.T, c Cache) {
				count, err := c.PodCount()
				assert.NoError(t, err)
				assert.Equal(t, 0, count)
			},
		},
		{
			name: "update pod",
			node: TestNode{name: "node1"},
			pods: []TestPod{
				{
					name:      "pod1",
					namespace: "default",
					uid:       "pod1",
					nodeName:  "node1",
				},
			},
			ops: func(t *testing.T, c Cache, node *corev1.Node, pods []*corev1.Pod) {
				c.AddNode(node)
				_ = c.AddPod(pods[0])

				updatedPod := pods[0].DeepCopy()
				updatedPod.Labels = map[string]string{"key": "value"}

				err := c.UpdatePod(pods[0], updatedPod)
				assert.NoError(t, err)

				pod, err := c.GetPod(updatedPod)
				assert.NoError(t, err)
				assert.Equal(t, "value", pod.Labels["key"])
			},
			verify: func(t *testing.T, c Cache) {
				count, err := c.PodCount()
				assert.NoError(t, err)
				assert.Equal(t, 1, count)
			},
		},
		{
			name: "remove pod",
			node: TestNode{name: "node1"},
			pods: []TestPod{
				{
					name:      "pod1",
					namespace: "default",
					uid:       "pod1",
					nodeName:  "node1",
				},
			},
			ops: func(t *testing.T, c Cache, node *corev1.Node, pods []*corev1.Pod) {
				c.AddNode(node)
				err := c.AddPod(pods[0])
				assert.NoError(t, err)

				err = c.RemovePod(pods[0])
				assert.NoError(t, err)
			},
			verify: func(t *testing.T, c Cache) {
				count, err := c.PodCount()
				assert.NoError(t, err)
				assert.Equal(t, 0, count)
			},
		},
		{
			name: "remove non-existent pod",
			node: TestNode{name: "node1"},
			pods: []TestPod{
				{
					name:      "pod1",
					namespace: "default",
					uid:       "pod1",
					nodeName:  "node1",
				},
			},
			ops: func(t *testing.T, c Cache, node *corev1.Node, pods []*corev1.Pod) {
				c.AddNode(node)
				err := c.RemovePod(pods[0])
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "is not found in scheduler cache")
			},
			verify: func(t *testing.T, c Cache) {
				count, err := c.PodCount()
				assert.NoError(t, err)
				assert.Equal(t, 0, count)
			},
		},
		{
			name: "remove pod with empty node name",
			node: TestNode{name: "node1"},
			pods: []TestPod{
				{
					name:      "pod1",
					namespace: "default",
					uid:       "pod1",
					nodeName:  "node1",
				},
			},
			ops: func(t *testing.T, c Cache, node *corev1.Node, pods []*corev1.Pod) {
				c.AddNode(node)
				err := c.AddPod(pods[0])
				assert.NoError(t, err)

				// Create a pod with same identity but empty node name
				podWithEmptyNode := pods[0].DeepCopy()
				podWithEmptyNode.Spec.NodeName = ""

				err = c.RemovePod(podWithEmptyNode)
				assert.NoError(t, err)
			},
			verify: func(t *testing.T, c Cache) {
				count, err := c.PodCount()
				assert.NoError(t, err)
				assert.Equal(t, 0, count)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := newCache(time.Hour, time.Second, nil) // Using longer TTL to avoid expiration
			node := newTestNode(tt.node)
			pods := make([]*corev1.Pod, len(tt.pods))
			for i, p := range tt.pods {
				pods[i] = newTestPod(p)
			}
			tt.ops(t, cache, node, pods)
			tt.verify(t, cache)
		})
	}
}

func TestCacheSnapshot(t *testing.T) {
	tests := []struct {
		name           string
		node           TestNode
		pods           []TestPod
		setup          func(*testing.T, Cache, *corev1.Node, []*corev1.Pod)
		verifySnapshot func(*testing.T, *Snapshot)
	}{
		{
			name: "basic snapshot",
			node: TestNode{name: "node1"},
			pods: []TestPod{
				{
					name:      "pod1",
					namespace: "default",
					uid:       "pod1",
					nodeName:  "node1",
				},
			},
			setup: func(t *testing.T, c Cache, node *corev1.Node, pods []*corev1.Pod) {
				c.AddNode(node)
				err := c.AddPod(pods[0])
				assert.NoError(t, err)
			},
			verifySnapshot: func(t *testing.T, snapshot *Snapshot) {
				assert.Equal(t, 1, len(snapshot.nodeInfoMap))
				assert.Equal(t, 1, len(snapshot.nodeInfoList))
				assert.Greater(t, snapshot.generation, int64(0))
			},
		},
		{
			name: "snapshot with pod affinity",
			node: TestNode{name: "node1"},
			pods: []TestPod{
				{
					name:      "pod1",
					namespace: "default",
					uid:       "pod1",
					nodeName:  "node1",
					affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"web"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			},
			setup: func(t *testing.T, c Cache, node *corev1.Node, pods []*corev1.Pod) {
				c.AddNode(node)
				err := c.AddPod(pods[0])
				assert.NoError(t, err)
			},
			verifySnapshot: func(t *testing.T, snapshot *Snapshot) {
				assert.Equal(t, 1, len(snapshot.havePodsWithAffinityNodeInfoList))
			},
		},
		{
			name: "snapshot with pod anti-affinity",
			node: TestNode{name: "node1"},
			pods: []TestPod{
				{
					name:      "pod1",
					namespace: "default",
					uid:       "pod1",
					nodeName:  "node1",
					affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{"web"},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			},
			setup: func(t *testing.T, c Cache, node *corev1.Node, pods []*corev1.Pod) {
				c.AddNode(node)
				err := c.AddPod(pods[0])
				assert.NoError(t, err)
			},
			verifySnapshot: func(t *testing.T, snapshot *Snapshot) {
				assert.Equal(t, 1, len(snapshot.havePodsWithRequiredAntiAffinityNodeInfoList))
			},
		},
		{
			name: "snapshot with PVC reference",
			node: TestNode{name: "node1"},
			pods: []TestPod{
				{
					name:      "pod1",
					namespace: "default",
					uid:       "pod1",
					nodeName:  "node1",
					volumes: []corev1.Volume{
						{
							Name: "pvc-volume",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "test-pvc",
								},
							},
						},
					},
				},
			},
			setup: func(t *testing.T, c Cache, node *corev1.Node, pods []*corev1.Pod) {
				c.AddNode(node)
				err := c.AddPod(pods[0])
				assert.NoError(t, err)
			},
			verifySnapshot: func(t *testing.T, snapshot *Snapshot) {
				assert.True(t, snapshot.usedPVCSet.Has("default/test-pvc"))
			},
		},
		{
			name: "snapshot with deleted node",
			node: TestNode{name: "node1"},
			pods: []TestPod{
				{
					name:      "pod1",
					namespace: "default",
					uid:       "pod1",
					nodeName:  "node1",
				},
			},
			setup: func(t *testing.T, c Cache, node *corev1.Node, pods []*corev1.Pod) {
				c.AddNode(node)
				err := c.AddPod(pods[0])
				assert.NoError(t, err)
				snapshot1 := NewSnapshot(nil, nil)
				err = c.UpdateSnapshot(snapshot1)
				assert.NoError(t, err)

				// Remove node and verify that removeDeletedNodesFromSnapshot is triggered
				err = c.RemoveNode(node)
				assert.NoError(t, err)
			},
			verifySnapshot: func(t *testing.T, snapshot *Snapshot) {
				assert.Equal(t, 0, len(snapshot.nodeInfoMap))
			},
		},
		{
			name: "snapshot consistency check",
			node: TestNode{name: "node1"},
			setup: func(_ *testing.T, _ Cache, _ *corev1.Node, _ []*corev1.Pod) {
				// Don't add the node, this will create an inconsistent state
			},
			verifySnapshot: func(t *testing.T, snapshot *Snapshot) {
				// The snapshot should be empty but consistent
				assert.Equal(t, 0, len(snapshot.nodeInfoMap))
				assert.Equal(t, 0, len(snapshot.nodeInfoList))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := newCache(time.Hour, time.Second, nil)
			node := newTestNode(tt.node)
			pods := make([]*corev1.Pod, len(tt.pods))
			for i, p := range tt.pods {
				pods[i] = newTestPod(p)
			}

			tt.setup(t, cache, node, pods)

			snapshot := NewSnapshot(nil, nil)
			err := cache.UpdateSnapshot(snapshot)
			assert.NoError(t, err)

			tt.verifySnapshot(t, snapshot)
		})
	}
}

func TestUpdateNodeInfoSnapshotList(t *testing.T) {
	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
		},
	}
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
		},
	}

	// Create a cache
	cache := newCache(time.Hour, time.Second, nil)
	cache.nodeTree.addNode(node1)
	cache.nodeTree.addNode(node2)

	// Create and populate nodeInfoMap in snapshot
	snapshot := NewSnapshot(nil, nil)
	snapshot.nodeInfoMap = make(map[string]*framework.NodeInfo)

	// Create NodeInfos
	info1 := framework.NewNodeInfo()
	info2 := framework.NewNodeInfo()
	info1.SetNode(node1)
	info2.SetNode(node2)

	// Add them to snapshot
	snapshot.nodeInfoMap["node1"] = info1
	snapshot.nodeInfoMap["node2"] = info2

	cache.updateNodeInfoSnapshotList(snapshot, true)

	assert.Equal(t, 2, len(snapshot.nodeInfoList), "should have both nodes in list")
	assert.Equal(t, 0, len(snapshot.havePodsWithAffinityNodeInfoList), "should have no nodes with affinity")
	assert.Equal(t, 0, len(snapshot.havePodsWithRequiredAntiAffinityNodeInfoList), "should have no nodes with anti-affinity")
	assert.Empty(t, snapshot.usedPVCSet, "should have no PVC references")
}

func TestRemoveDeletedNodesFromSnapshot(t *testing.T) {
	cache := newCache(time.Hour, time.Second, nil)

	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
		},
	}
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
		},
	}

	// Set up the cache nodeTree
	cache.nodeTree.addNode(node1)
	cache.nodeTree.addNode(node2)

	// Create node infos in cache
	nodeInfo1 := framework.NewNodeInfo()
	nodeInfo2 := framework.NewNodeInfo()
	nodeInfo1.SetNode(node1)
	nodeInfo2.SetNode(node2)
	cache.nodes["node1"] = newNodeInfoListItem(nodeInfo1)
	cache.nodes["node2"] = newNodeInfoListItem(nodeInfo2)

	// Create snapshot with both nodes
	snapshot := NewSnapshot(nil, nil)
	snapshot.nodeInfoMap = map[string]*framework.NodeInfo{
		"node1": nodeInfo1.Clone(),
		"node2": nodeInfo2.Clone(),
	}

	// Initial state check
	assert.Equal(t, 2, len(snapshot.nodeInfoMap), "should start with two nodes in snapshot")

	// Remove node1 from cache
	delete(cache.nodes, "node1")
	err := cache.nodeTree.removeNode(node1)
	require.NoError(t, err)

	// Run the function being tested
	cache.removeDeletedNodesFromSnapshot(snapshot)

	// Verify the results
	assert.Equal(t, 1, len(snapshot.nodeInfoMap), "should have one node after removal")
	_, exists := snapshot.nodeInfoMap["node2"]
	assert.True(t, exists, "node2 should still exist in snapshot")
	_, exists = snapshot.nodeInfoMap["node1"]
	assert.False(t, exists, "node1 should be removed from snapshot")
}

func TestCacheImageStates(t *testing.T) {
	tests := []struct {
		name   string
		node   TestNode
		verify func(*testing.T, Cache)
	}{
		{
			name: "add node with images",
			node: TestNode{
				name: "node1",
				images: []corev1.ContainerImage{
					{
						Names:     []string{"test-image:latest"},
						SizeBytes: 1000,
					},
				},
			},
			verify: func(t *testing.T, c Cache) {
				nodeInfo := c.AddNode(newTestNode(TestNode{
					name: "node1",
					images: []corev1.ContainerImage{
						{
							Names:     []string{"test-image:latest"},
							SizeBytes: 1000,
						},
					},
				}))

				require.NotNil(t, nodeInfo)
				assert.Equal(t, 1, len(nodeInfo.ImageStates))

				imageState, ok := nodeInfo.ImageStates["test-image:latest"]
				assert.True(t, ok)
				assert.Equal(t, int64(1000), imageState.Size)
				assert.Equal(t, 1, imageState.NumNodes)
			},
		},
		{
			name: "remove node images",
			node: TestNode{
				name: "node1",
				images: []corev1.ContainerImage{
					{
						Names:     []string{"test-image:latest"},
						SizeBytes: 1000,
					},
				},
			},
			verify: func(t *testing.T, c Cache) {
				node := newTestNode(TestNode{
					name: "node1",
					images: []corev1.ContainerImage{
						{
							Names:     []string{"test-image:latest"},
							SizeBytes: 1000,
						},
					},
				})

				c.AddNode(node)

				err := c.RemoveNode(node)
				require.NoError(t, err, "failed to remove node")

				// Verify imageStates is empty
				impl := c.(*cacheImpl)
				assert.Equal(t, 0, len(impl.imageStates))
			},
		},
		{
			name: "update node images",
			node: TestNode{
				name: "node1",
				images: []corev1.ContainerImage{
					{
						Names:     []string{"test-image:v1"},
						SizeBytes: 1000,
					},
				},
			},
			verify: func(t *testing.T, c Cache) {
				oldNode := newTestNode(TestNode{
					name: "node1",
					images: []corev1.ContainerImage{
						{
							Names:     []string{"test-image:v1"},
							SizeBytes: 1000,
						},
					},
				})

				newNode := oldNode.DeepCopy()
				newNode.Status.Images = []corev1.ContainerImage{
					{
						Names:     []string{"test-image:v2"},
						SizeBytes: 2000,
					},
				}

				c.AddNode(oldNode)

				nodeInfo := c.UpdateNode(oldNode, newNode)

				// Verify new image state
				require.NotNil(t, nodeInfo)
				assert.Equal(t, 1, len(nodeInfo.ImageStates))

				imageState, ok := nodeInfo.ImageStates["test-image:v2"]
				assert.True(t, ok)
				assert.Equal(t, int64(2000), imageState.Size)
				assert.Equal(t, 1, imageState.NumNodes)

				// Verify old image was removed
				_, ok = nodeInfo.ImageStates["test-image:v1"]
				assert.False(t, ok)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := newCache(time.Second, time.Second, nil)
			tt.verify(t, cache)
		})
	}
}

func TestCacheAssumedPodExpiration(t *testing.T) {
	tests := []struct {
		name     string
		ttl      time.Duration
		setup    func(*testing.T, Cache, *corev1.Node, *corev1.Pod)
		validate func(*testing.T, Cache, *corev1.Pod)
	}{
		{
			name: "assumed pod expires",
			ttl:  time.Second,
			setup: func(t *testing.T, c Cache, node *corev1.Node, pod *corev1.Pod) {
				c.AddNode(node)
				err := c.AssumePod(pod)
				require.NoError(t, err)

				err = c.FinishBinding(pod)
				require.NoError(t, err)

				// Wait for TTL to expire and add extra time for cleanup
				time.Sleep(2 * time.Second)

				// Trigger cleanup explicitly
				impl := c.(*cacheImpl)
				now := time.Now()
				impl.cleanupAssumedPods(now)
			},
			validate: func(t *testing.T, c Cache, pod *corev1.Pod) {
				assumed, err := c.IsAssumedPod(pod)
				assert.NoError(t, err)
				assert.False(t, assumed, "pod should no longer be assumed after expiration")

				_, err = c.GetPod(pod)
				assert.Error(t, err, "pod should be removed from cache after expiration")
			},
		},
		{
			name: "assumed pod does not expire with zero TTL",
			ttl:  0,
			setup: func(t *testing.T, c Cache, node *corev1.Node, pod *corev1.Pod) {
				c.AddNode(node)
				err := c.AssumePod(pod)
				require.NoError(t, err)

				err = c.FinishBinding(pod)
				require.NoError(t, err)

				// Wait some time
				time.Sleep(2 * time.Second)

				// Trigger cleanup explicitly
				impl := c.(*cacheImpl)
				now := time.Now()
				impl.cleanupAssumedPods(now)
			},
			validate: func(t *testing.T, c Cache, pod *corev1.Pod) {
				assumed, err := c.IsAssumedPod(pod)
				assert.NoError(t, err)
				assert.True(t, assumed, "pod should remain assumed with zero TTL")

				foundPod, err := c.GetPod(pod)
				assert.NoError(t, err)
				assert.Equal(t, pod.UID, foundPod.UID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create stop channel to prevent background cleanup
			stop := make(chan struct{})
			defer close(stop)

			cache := newCache(tt.ttl, time.Second, stop)
			node := newTestNode(TestNode{name: "node1"})
			pod := newTestPod(TestPod{
				name:      "pod1",
				namespace: "default",
				uid:       "pod1",
				nodeName:  "node1",
			})

			tt.setup(t, cache, node, pod)
			tt.validate(t, cache, pod)
		})
	}
}

func TestCacheDump(t *testing.T) {
	cache := newCache(time.Second, time.Second, nil)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	}
	cache.AddNode(node)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-pod",
			UID:       types.UID("test-pod"),
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
		},
	}
	_ = cache.AssumePod(pod)

	dump := cache.Dump()
	assert.Len(t, dump.Nodes, 1, "Expected 1 node in dump")
	assert.Len(t, dump.AssumedPods, 1, "Expected 1 assumed pod in dump")
}

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		ttl    time.Duration
		verify func(*testing.T, Cache)
	}{
		{
			name: "create cache with zero TTL",
			ttl:  0,
			verify: func(t *testing.T, c Cache) {
				impl := c.(*cacheImpl)
				assert.Equal(t, time.Duration(0), impl.ttl)
				assert.Equal(t, cleanAssumedPeriod, impl.period)
				assert.NotNil(t, impl.stop)
			},
		},
		{
			name: "create cache with non-zero TTL",
			ttl:  5 * time.Minute,
			verify: func(t *testing.T, c Cache) {
				impl := c.(*cacheImpl)
				assert.Equal(t, 5*time.Minute, impl.ttl)
				assert.Equal(t, cleanAssumedPeriod, impl.period)
				assert.NotNil(t, impl.stop)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stop := make(chan struct{})
			defer close(stop)

			cache := New(tt.ttl, stop)
			tt.verify(t, cache)
		})
	}
}

func TestCacheRun(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*testing.T, *cacheImpl)
		validate func(*testing.T, *cacheImpl)
	}{
		{
			name: "cleanup of expired assumed pods",
			setup: func(t *testing.T, c *cacheImpl) {
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				}
				c.AddNode(node)

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						UID:       "test-pod-uid",
					},
					Spec: corev1.PodSpec{
						NodeName: "node1",
					},
				}

				err := c.AssumePod(pod)
				assert.NoError(t, err)

				// Mark it as finished binding
				err = c.FinishBinding(pod)
				assert.NoError(t, err)

				assumed, err := c.IsAssumedPod(pod)
				assert.NoError(t, err)
				assert.True(t, assumed)
			},
			validate: func(t *testing.T, c *cacheImpl) {
				// Wait for cleanup to happen
				time.Sleep(2 * time.Second)

				// Count pods - should be 0 after cleanup
				count, err := c.PodCount()
				assert.NoError(t, err)
				assert.Equal(t, 0, count)

				// Verify assumedPods is empty
				assert.Equal(t, 0, len(c.assumedPods))
			},
		},
		{
			name: "non-expired assumed pods remain",
			setup: func(t *testing.T, c *cacheImpl) {
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				}
				c.AddNode(node)

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						UID:       "test-pod-uid",
					},
					Spec: corev1.PodSpec{
						NodeName: "node1",
					},
				}

				err := c.AssumePod(pod)
				assert.NoError(t, err)

				// Mark it as finished binding but don't expire it
				err = c.FinishBinding(pod)
				assert.NoError(t, err)

				// Set a future deadline
				future := time.Now().Add(1 * time.Hour)
				c.podStates[string(pod.UID)].deadline = &future
			},
			validate: func(t *testing.T, c *cacheImpl) {
				// Wait for potential cleanup
				time.Sleep(2 * time.Second)

				// Count should still be 1
				count, err := c.PodCount()
				assert.NoError(t, err)
				assert.Equal(t, 1, count)

				// Should still be in assumedPods
				assert.Equal(t, 1, len(c.assumedPods))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stop := make(chan struct{})
			cache := newCache(1*time.Second, 500*time.Millisecond, stop)

			tt.setup(t, cache)

			cache.run()

			tt.validate(t, cache)

			// Cleanup
			close(stop)
		})
	}
}

// Helper Functions

func newTestNode(config TestNode) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   config.name,
			Labels: config.labels,
		},
		Status: corev1.NodeStatus{
			Images:      config.images,
			Allocatable: config.resources,
		},
	}
}

func newTestPod(config TestPod) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.name,
			Namespace: config.namespace,
			UID:       types.UID(config.uid),
			Labels:    config.labels,
		},
		Spec: corev1.PodSpec{
			NodeName: config.nodeName,
			Volumes:  config.volumes,
			Affinity: config.affinity,
		},
	}
}
