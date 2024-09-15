/*
Copyright 2018 The Kubernetes Authors.

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

// This code is directly lifted from the Kubernetes codebase in order to avoid relying on the k8s.io/kubernetes package.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/scheduler/internal/cache/node_tree.go

package cache

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNodeTree(t *testing.T) {

	t.Run("TestNewNodeTree", func(t *testing.T) {
		nodes := []*corev1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"zone": "zone1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: map[string]string{"zone": "zone1"}}},
		}

		nt := newNodeTree(nodes)

		if nt.numNodes != len(nodes) {
			t.Errorf("expected %d nodes, got %d", len(nodes), nt.numNodes)
		}
	})

	t.Run("TestAddNode", func(t *testing.T) {
		nt := newNodeTree(nil)

		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"zone": "zone1"}}}
		nt.addNode(node)

		if nt.numNodes != 1 {
			t.Errorf("expected 1 node, got %d", nt.numNodes)
		}
	})

	t.Run("TestAddExistingNode", func(t *testing.T) {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"zone": "zone1"}}}
		nt := newNodeTree([]*corev1.Node{node})

		nt.addNode(node)

		if nt.numNodes != 1 {
			t.Errorf("expected 1 node, got %d", nt.numNodes)
		}
	})

	t.Run("TestRemoveNode", func(t *testing.T) {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"zone": "zone1"}}}
		nt := newNodeTree([]*corev1.Node{node})

		err := nt.removeNode(node)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if nt.numNodes != 0 {
			t.Errorf("expected 0 nodes, got %d", nt.numNodes)
		}
	})

	t.Run("TestRemoveNonExistentNode", func(t *testing.T) {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"zone": "zone1"}}}
		nt := newNodeTree(nil)

		err := nt.removeNode(node)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("TestUpdateNode", func(t *testing.T) {
		oldNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"zone": "zone1"}}}
		newNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"zone": "zone2"}}}
		nt := newNodeTree([]*corev1.Node{oldNode})

		nt.updateNode(oldNode, newNode)

		if nt.numNodes != 1 {
			t.Errorf("expected 1 node, got %d", nt.numNodes)
		}
	})

	t.Run("TestList", func(t *testing.T) {
		nodes := []*corev1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"zone": "zone1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: map[string]string{"zone": "zone2"}}},
		}
		nt := newNodeTree(nodes)

		list, err := nt.list()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(list) != len(nodes) {
			t.Errorf("expected %d nodes, got %d", len(nodes), len(list))
		}
	})

	t.Run("TestListEmpty", func(t *testing.T) {
		nt := newNodeTree(nil)

		list, err := nt.list()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(list) != 0 {
			t.Errorf("expected 0 nodes, got %d", len(list))
		}
	})

	t.Run("TestListExhaustedZones", func(t *testing.T) {
		nodes := []*corev1.Node{
			{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"zone": "zone1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node2", Labels: map[string]string{"zone": "zone2"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node3", Labels: map[string]string{"zone": "zone1"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "node4", Labels: map[string]string{"zone": "zone2"}}},
		}

		nt := newNodeTree(nodes)
		nt.numNodes = 5

		list, err := nt.list()

		if err == nil || err.Error() != "all zones exhausted before reaching count of nodes expected" {
			t.Errorf("expected error about exhausted zones, got %v", err)
		}

		if len(list) != len(nodes) {
			t.Errorf("expected %d nodes, got %d", len(nodes), len(list))
		}
	})
}
