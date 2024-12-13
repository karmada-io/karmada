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
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewNodeTree(t *testing.T) {
	tests := []struct {
		name          string
		nodes         []*corev1.Node
		expectedZones []string
		expectedCount int
	}{
		{
			name:          "empty node list",
			nodes:         []*corev1.Node{},
			expectedZones: nil,
			expectedCount: 0,
		},
		{
			name: "single node",
			nodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			},
			expectedZones: []string{getExpectedZoneKey("us-east", "us-east-1a")},
			expectedCount: 1,
		},
		{
			name: "multiple nodes same zone",
			nodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
				makeNodeWithTopology("node2", "us-east", "us-east-1a"),
			},
			expectedZones: []string{getExpectedZoneKey("us-east", "us-east-1a")},
			expectedCount: 2,
		},
		{
			name: "multiple nodes different zones",
			nodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
				makeNodeWithTopology("node2", "us-east", "us-east-1b"),
			},
			expectedZones: []string{
				getExpectedZoneKey("us-east", "us-east-1a"),
				getExpectedZoneKey("us-east", "us-east-1b"),
			},
			expectedCount: 2,
		},
		{
			name: "nodes without region/zone labels",
			nodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node2"}},
			},
			expectedZones: []string{""},
			expectedCount: 2,
		},
		{
			name: "mixed labeled and unlabeled nodes",
			nodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
				{ObjectMeta: metav1.ObjectMeta{Name: "node2"}},
			},
			expectedZones: []string{
				"",
				getExpectedZoneKey("us-east", "us-east-1a"),
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nt := newNodeTree(tt.nodes)
			assert.ElementsMatch(t, tt.expectedZones, nt.zones)
			assert.Equal(t, tt.expectedCount, nt.numNodes)
		})
	}
}

func TestAddNode(t *testing.T) {
	tests := []struct {
		name           string
		existingNodes  []*corev1.Node
		nodeToAdd      *corev1.Node
		expectedZones  []string
		expectedCount  int
		expectedInZone map[string]int
	}{
		{
			name:          "add to empty tree",
			existingNodes: []*corev1.Node{},
			nodeToAdd:     makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			expectedZones: []string{getExpectedZoneKey("us-east", "us-east-1a")},
			expectedCount: 1,
			expectedInZone: map[string]int{
				getExpectedZoneKey("us-east", "us-east-1a"): 1,
			},
		},
		{
			name: "add to existing zone",
			existingNodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			},
			nodeToAdd:     makeNodeWithTopology("node2", "us-east", "us-east-1a"),
			expectedZones: []string{getExpectedZoneKey("us-east", "us-east-1a")},
			expectedCount: 2,
			expectedInZone: map[string]int{
				getExpectedZoneKey("us-east", "us-east-1a"): 2,
			},
		},
		{
			name: "add to new zone",
			existingNodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			},
			nodeToAdd: makeNodeWithTopology("node2", "us-east", "us-east-1b"),
			expectedZones: []string{
				getExpectedZoneKey("us-east", "us-east-1a"),
				getExpectedZoneKey("us-east", "us-east-1b"),
			},
			expectedCount: 2,
			expectedInZone: map[string]int{
				getExpectedZoneKey("us-east", "us-east-1a"): 1,
				getExpectedZoneKey("us-east", "us-east-1b"): 1,
			},
		},
		{
			name: "add unlabeled node",
			existingNodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			},
			nodeToAdd: &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node2"}},
			expectedZones: []string{
				"",
				getExpectedZoneKey("us-east", "us-east-1a"),
			},
			expectedCount: 2,
			expectedInZone: map[string]int{
				"": 1,
				getExpectedZoneKey("us-east", "us-east-1a"): 1,
			},
		},
		{
			name: "add duplicate node",
			existingNodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			},
			nodeToAdd:     makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			expectedZones: []string{getExpectedZoneKey("us-east", "us-east-1a")},
			expectedCount: 1,
			expectedInZone: map[string]int{
				getExpectedZoneKey("us-east", "us-east-1a"): 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nt := newNodeTree(tt.existingNodes)
			nt.addNode(tt.nodeToAdd)

			assert.ElementsMatch(t, tt.expectedZones, nt.zones)
			assert.Equal(t, tt.expectedCount, nt.numNodes)

			for zone, expectedCount := range tt.expectedInZone {
				assert.Equal(t, expectedCount, len(nt.tree[zone]))
			}

			// For duplicate node case, verify the node only appears once
			if tt.name == "add duplicate node" {
				nodeZone := getExpectedZoneKey("us-east", "us-east-1a")
				nodeCount := 0
				for _, nodeName := range nt.tree[nodeZone] {
					if nodeName == tt.nodeToAdd.Name {
						nodeCount++
					}
				}
				assert.Equal(t, 1, nodeCount, "duplicate node should only appear once")
			}
		})
	}
}

func TestRemoveNode(t *testing.T) {
	tests := []struct {
		name          string
		existingNodes []*corev1.Node
		nodeToRemove  *corev1.Node
		expectedZones []string
		expectedCount int
		expectedError bool
	}{
		{
			name: "remove existing node",
			existingNodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
				makeNodeWithTopology("node2", "us-east", "us-east-1b"),
			},
			nodeToRemove: makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			expectedZones: []string{
				getExpectedZoneKey("us-east", "us-east-1b"),
			},
			expectedCount: 1,
			expectedError: false,
		},
		{
			name: "remove last node in zone",
			existingNodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			},
			nodeToRemove:  makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			expectedZones: []string{},
			expectedCount: 0,
			expectedError: false,
		},
		{
			name: "remove non-existing node",
			existingNodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			},
			nodeToRemove: makeNodeWithTopology("node2", "us-east", "us-east-1a"),
			expectedZones: []string{
				getExpectedZoneKey("us-east", "us-east-1a"),
			},
			expectedCount: 1,
			expectedError: true,
		},
		{
			name: "remove unlabeled node",
			existingNodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
			},
			nodeToRemove:  &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
			expectedZones: []string{},
			expectedCount: 0,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nt := newNodeTree(tt.existingNodes)
			err := nt.removeNode(tt.nodeToRemove)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.ElementsMatch(t, tt.expectedZones, nt.zones)
			assert.Equal(t, tt.expectedCount, nt.numNodes)
		})
	}
}

func TestUpdateNode(t *testing.T) {
	tests := []struct {
		name          string
		existingNodes []*corev1.Node
		oldNode       *corev1.Node
		newNode       *corev1.Node
		expectedZones []string
		expectedCount int
	}{
		{
			name: "update to same zone",
			existingNodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			},
			oldNode:       makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			newNode:       makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			expectedZones: []string{getExpectedZoneKey("us-east", "us-east-1a")},
			expectedCount: 1,
		},
		{
			name: "update to different zone",
			existingNodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			},
			oldNode:       makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			newNode:       makeNodeWithTopology("node1", "us-east", "us-east-1b"),
			expectedZones: []string{getExpectedZoneKey("us-east", "us-east-1b")},
			expectedCount: 1,
		},
		{
			name: "update from unlabeled to labeled",
			existingNodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
			},
			oldNode:       &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
			newNode:       makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			expectedZones: []string{getExpectedZoneKey("us-east", "us-east-1a")},
			expectedCount: 1,
		},
		{
			name: "update from labeled to unlabeled",
			existingNodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			},
			oldNode:       makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			newNode:       &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
			expectedZones: []string{""},
			expectedCount: 1,
		},
		{
			name:          "update non-existent node",
			existingNodes: []*corev1.Node{},
			oldNode:       makeNodeWithTopology("node1", "us-east", "us-east-1a"),
			newNode:       makeNodeWithTopology("node1", "us-east", "us-east-1b"),
			expectedZones: []string{getExpectedZoneKey("us-east", "us-east-1b")},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nt := newNodeTree(tt.existingNodes)
			nt.updateNode(tt.oldNode, tt.newNode)

			assert.ElementsMatch(t, tt.expectedZones, nt.zones)
			assert.Equal(t, tt.expectedCount, nt.numNodes)
		})
	}
}

func TestList(t *testing.T) {
	tests := []struct {
		name         string
		nodes        []*corev1.Node
		expectedList []string
	}{
		{
			name:         "empty tree",
			nodes:        []*corev1.Node{},
			expectedList: nil,
		},
		{
			name: "single zone",
			nodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
				makeNodeWithTopology("node2", "us-east", "us-east-1a"),
			},
			expectedList: []string{"node1", "node2"},
		},
		{
			name: "multiple zones",
			nodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
				makeNodeWithTopology("node2", "us-east", "us-east-1b"),
				makeNodeWithTopology("node3", "us-east", "us-east-1a"),
			},
			expectedList: []string{"node1", "node2", "node3"},
		},
		{
			name: "attempt to add duplicate nodes",
			nodes: []*corev1.Node{
				makeNodeWithTopology("node1", "us-east", "us-east-1a"),
				makeNodeWithTopology("node2", "us-east", "us-east-1b"),
				makeNodeWithTopology("node2", "us-east", "us-east-1b"), // This duplicate will be ignored
				makeNodeWithTopology("node3", "us-east", "us-east-1a"),
			},
			expectedList: []string{"node1", "node2", "node3"}, // Only unique nodes appear
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nt := newNodeTree(tt.nodes)
			list, err := nt.list()

			assert.NoError(t, err)

			if tt.expectedList == nil {
				assert.Nil(t, list)
				return
			}

			// Sort both slices to ensure ordering doesn't affect comparison
			sort.Strings(tt.expectedList)
			sort.Strings(list)

			assert.Equal(t, tt.expectedList, list)
		})
	}
}

// Helper Functions

// Helper function to create a test node
func makeNodeWithTopology(name, region, zone string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				corev1.LabelTopologyRegion: region,
				corev1.LabelTopologyZone:   zone,
			},
		},
	}
}

// Helper function to get the expected zone key format
func getExpectedZoneKey(region, zone string) string {
	return region + ":\x00:" + zone
}
