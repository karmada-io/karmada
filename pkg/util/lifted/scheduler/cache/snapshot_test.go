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

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewEmptySnapshot(t *testing.T) {
	t.Run("verify empty snapshot initialization", func(t *testing.T) {
		s := NewEmptySnapshot()
		assert.NotNil(t, s)
		assert.NotNil(t, s.nodeInfoMap)
		assert.NotNil(t, s.usedPVCSet)
		assert.Empty(t, s.nodeInfoList)
		assert.Empty(t, s.havePodsWithAffinityNodeInfoList)
		assert.Empty(t, s.havePodsWithRequiredAntiAffinityNodeInfoList)
	})
}

func TestNewSnapshot(t *testing.T) {
	tests := []struct {
		name                      string
		nodes                     []*corev1.Node
		pods                      []*corev1.Pod
		expectedNodeCount         int
		expectedPVCCount          int
		expectedAffinityCount     int
		expectedAntiAffinityCount int
		expectedImageCount        int
	}{
		{
			name:                      "empty snapshot",
			expectedNodeCount:         0,
			expectedPVCCount:          0,
			expectedAffinityCount:     0,
			expectedAntiAffinityCount: 0,
			expectedImageCount:        0,
		},
		{
			name: "single node without pods",
			nodes: []*corev1.Node{
				makeNode("node1", makeResourceList(1000, 2000)),
			},
			expectedNodeCount:         1,
			expectedPVCCount:          0,
			expectedAffinityCount:     0,
			expectedAntiAffinityCount: 0,
			expectedImageCount:        0,
		},
		{
			name: "multiple nodes with various pod configurations",
			nodes: []*corev1.Node{
				makeNode("node1", makeResourceList(1000, 2000)),
				makeNode("node2", makeResourceList(2000, 4000)),
			},
			pods: []*corev1.Pod{
				makePod("pod1", "node1", true, false), // with affinity
				makePod("pod2", "node2", false, true), // with anti-affinity
				makePodWithPVC("pod3", "node1", "pvc1"),
			},
			expectedNodeCount:         2,
			expectedPVCCount:          1,
			expectedAffinityCount:     2,
			expectedAntiAffinityCount: 1,
			expectedImageCount:        0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSnapshot(tt.pods, tt.nodes)
			assert.NotNil(t, s)
			assert.Equal(t, tt.expectedNodeCount, len(s.nodeInfoMap))
			assert.Equal(t, tt.expectedNodeCount, len(s.nodeInfoList))
			assert.Equal(t, tt.expectedPVCCount, s.usedPVCSet.Len())

			affinityList, err := s.HavePodsWithAffinityList()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedAffinityCount, len(affinityList))

			antiAffinityList, err := s.HavePodsWithRequiredAntiAffinityList()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedAntiAffinityCount, len(antiAffinityList))

			if tt.expectedImageCount > 0 {
				nodeInfo, err := s.Get("node1")
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedImageCount, len(nodeInfo.ImageStates))
			}
		})
	}
}

func TestNodeInfoOperations(t *testing.T) {
	tests := []struct {
		name       string
		nodes      []*corev1.Node
		pods       []*corev1.Pod
		operations func(t *testing.T, s *Snapshot)
	}{
		{
			name: "basic node operations",
			nodes: []*corev1.Node{
				makeNode("node1", makeResourceList(1000, 2000)),
			},
			pods: []*corev1.Pod{
				makePod("pod1", "node1", false, false),
			},
			operations: func(t *testing.T, s *Snapshot) {
				nodeInfos := s.NodeInfos()
				assert.NotNil(t, nodeInfos)

				// Test List
				list, err := nodeInfos.List()
				assert.NoError(t, err)
				assert.Equal(t, 1, len(list))

				// Test Get existing node
				info, err := nodeInfos.Get("node1")
				assert.NoError(t, err)
				assert.NotNil(t, info)
				assert.Equal(t, "node1", info.Node().Name)

				// Test Get non-existent node
				info, err = nodeInfos.Get("node-non-existent")
				assert.Error(t, err)
				assert.Nil(t, info)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSnapshot(tt.pods, tt.nodes)
			tt.operations(t, s)
		})
	}
}

func TestPVCOperations(t *testing.T) {
	tests := []struct {
		name   string
		nodes  []*corev1.Node
		pods   []*corev1.Pod
		checks func(t *testing.T, s *Snapshot)
	}{
		{
			name: "PVC tracking scenarios",
			nodes: []*corev1.Node{
				makeNode("node1", makeResourceList(1000, 2000)),
			},
			pods: []*corev1.Pod{
				makePodWithPVC("scheduled-pod", "node1", "pvc1"),
				makePodWithPVC("unscheduled-pod", "", "pvc2"),
				makePodWithoutPVC("pod-without-pvc", "node1"),
			},
			checks: func(t *testing.T, s *Snapshot) {
				assert.True(t, s.IsPVCUsedByPods("default/pvc1"))
				assert.False(t, s.IsPVCUsedByPods("default/pvc2"))
				assert.False(t, s.IsPVCUsedByPods("default/non-existent"))
				assert.Equal(t, 1, s.usedPVCSet.Len())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSnapshot(tt.pods, tt.nodes)
			tt.checks(t, s)
		})
	}
}

func TestImageHandling(t *testing.T) {
	tests := []struct {
		name                string
		nodes               []*corev1.Node
		expectedImageStates map[string]struct {
			size     int64
			numNodes int
		}
	}{
		{
			name: "single node with multiple image tags",
			nodes: []*corev1.Node{
				makeNodeWithImages("node1", makeResourceList(1000, 2000), []corev1.ContainerImage{
					{
						Names:     []string{"image1:latest", "image1:v1"},
						SizeBytes: 1000,
					},
				}),
			},
			expectedImageStates: map[string]struct {
				size     int64
				numNodes int
			}{
				"image1:latest": {1000, 1},
				"image1:v1":     {1000, 1},
			},
		},
		{
			name: "multiple nodes with same image",
			nodes: []*corev1.Node{
				makeNodeWithImages("node1", makeResourceList(1000, 2000), []corev1.ContainerImage{
					{
						Names:     []string{"image1:latest"},
						SizeBytes: 1000,
					},
				}),
				makeNodeWithImages("node2", makeResourceList(1000, 2000), []corev1.ContainerImage{
					{
						Names:     []string{"image1:latest"},
						SizeBytes: 1000,
					},
				}),
			},
			expectedImageStates: map[string]struct {
				size     int64
				numNodes int
			}{
				"image1:latest": {1000, 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSnapshot(nil, tt.nodes)
			nodeInfo, err := s.Get(tt.nodes[0].Name)
			assert.NoError(t, err)

			for imageName, expected := range tt.expectedImageStates {
				state := nodeInfo.ImageStates[imageName]
				assert.Equal(t, expected.size, state.Size)
				assert.Equal(t, expected.numNodes, state.NumNodes)
			}
		})
	}
}

func TestStorageInfos(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []*corev1.Node
		pods      []*corev1.Pod
		pvcChecks map[string]bool // map of PVC keys to expected existence
	}{
		{
			name: "empty snapshot",
			pvcChecks: map[string]bool{
				"default/non-existent-pvc": false,
			},
		},
		{
			name: "snapshot with PVCs",
			nodes: []*corev1.Node{
				makeNode("node1", makeResourceList(1000, 2000)),
			},
			pods: []*corev1.Pod{
				makePodWithPVC("pod1", "node1", "pvc1"),
				makePodWithPVC("pod2", "node1", "pvc2"),
				makePodWithPVC("pod3", "", "pvc3"), // Unscheduled pod
			},
			pvcChecks: map[string]bool{
				"default/pvc1":             true,
				"default/pvc2":             true,
				"default/pvc3":             false, // false as pod is unscheduled
				"default/non-existent-pvc": false,
			},
		},
		{
			name: "snapshot with mixed volume types",
			nodes: []*corev1.Node{
				makeNode("node1", makeResourceList(1000, 2000)),
			},
			pods: []*corev1.Pod{
				makePodWithPVC("pod1", "node1", "pvc1"),
				makePodWithoutPVC("pod2", "node1"),
				makePodWithPVC("pod3", "node1", "pvc3"),
			},
			pvcChecks: map[string]bool{
				"default/pvc1": true,
				"default/pvc3": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSnapshot(tt.pods, tt.nodes)
			storageInfos := s.StorageInfos()

			assert.NotNil(t, storageInfos, "StorageInfos should not return nil")

			// Check PVC existence
			for pvcKey, shouldExist := range tt.pvcChecks {
				exists := storageInfos.IsPVCUsedByPods(pvcKey)
				assert.Equal(t, shouldExist, exists, "PVC %s existence mismatch", pvcKey)
			}
		})
	}
}

func TestNumNodes(t *testing.T) {
	tests := []struct {
		name             string
		nodes            []*corev1.Node
		pods             []*corev1.Pod
		expectedNumNodes int
	}{
		{
			name:             "empty snapshot",
			expectedNumNodes: 0,
		},
		{
			name: "single node",
			nodes: []*corev1.Node{
				makeNode("node1", makeResourceList(1000, 2000)),
			},
			expectedNumNodes: 1,
		},
		{
			name: "multiple nodes",
			nodes: []*corev1.Node{
				makeNode("node1", makeResourceList(1000, 2000)),
				makeNode("node2", makeResourceList(2000, 4000)),
				makeNode("node3", makeResourceList(3000, 6000)),
			},
			expectedNumNodes: 3,
		},
		{
			name: "nodes with pods",
			nodes: []*corev1.Node{
				makeNode("node1", makeResourceList(1000, 2000)),
				makeNode("node2", makeResourceList(2000, 4000)),
			},
			pods: []*corev1.Pod{
				makePod("pod1", "node1", false, false),
				makePod("pod2", "node2", false, false),
			},
			expectedNumNodes: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSnapshot(tt.pods, tt.nodes)

			numNodes := s.NumNodes()
			assert.Equal(t, tt.expectedNumNodes, numNodes, "NumNodes() returned incorrect count")

			assert.Equal(t, tt.expectedNumNodes, len(s.nodeInfoList), "nodeInfoList length mismatch")

			assert.Equal(t, tt.expectedNumNodes, len(s.nodeInfoMap), "nodeInfoMap length mismatch")

			// Verify List method returns same count
			list, err := s.List()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedNumNodes, len(list), "List() returned incorrect count")
		})
	}
}

// Helper functions

func makeNode(name string, res corev1.ResourceList) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Capacity:    res,
			Allocatable: res,
		},
	}
}

func makeNodeWithImages(name string, res corev1.ResourceList, images []corev1.ContainerImage) *corev1.Node {
	node := makeNode(name, res)
	node.Status.Images = images
	return node
}

func makeResourceList(cpu, memory int64) corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:    *resource.NewMilliQuantity(cpu, resource.DecimalSI),
		corev1.ResourceMemory: *resource.NewQuantity(memory, resource.BinarySI),
	}
}

func makePod(name, nodeName string, hasAffinity, hasAntiAffinity bool) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
	}

	if hasAffinity || hasAntiAffinity {
		pod.Spec.Affinity = &corev1.Affinity{}

		if hasAffinity {
			pod.Spec.Affinity.PodAffinity = &corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"key": "value"},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			}
		}

		if hasAntiAffinity {
			pod.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"key": "value"},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			}
		}
	}

	return pod
}

func makePodWithPVC(name, nodeName, pvcName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Volumes: []corev1.Volume{
				{
					Name: "vol1",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}
}

func makePodWithoutPVC(name, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Volumes: []corev1.Volume{
				{
					Name: "vol1",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
}
