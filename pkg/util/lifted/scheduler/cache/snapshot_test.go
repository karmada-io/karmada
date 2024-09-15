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

// This code is directly lifted from the Kubernetes codebase in order to avoid relying on the k8s.io/kubernetes package.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/scheduler/internal/cache/snapshot.go

package cache

import (
	"testing"

	"github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestNewEmptySnapshot(t *testing.T) {
	snapshot := NewEmptySnapshot()

	if len(snapshot.nodeInfoMap) != 0 {
		t.Errorf("expected empty nodeInfoMap, got %d", len(snapshot.nodeInfoMap))
	}

	if snapshot.usedPVCSet.Len() != 0 {
		t.Errorf("expected empty usedPVCSet, got %d", snapshot.usedPVCSet.Len())
	}
}

func TestNewSnapshot(t *testing.T) {
	pods := []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "default"},
			Spec: corev1.PodSpec{
				NodeName: "node1",
				Volumes: []corev1.Volume{
					{
						Name: "volume1",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "pvc1",
							},
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pod2", Namespace: "default"},
			Spec: corev1.PodSpec{
				NodeName: "", // Pod with empty NodeName
			},
		},
	}

	nodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node1"},
			Status: corev1.NodeStatus{
				Images: []corev1.ContainerImage{
					{
						Names:     []string{"image1"},
						SizeBytes: 12345,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node2"},
		},
	}

	snapshot := NewSnapshot(pods, nodes)
	if snapshot.usedPVCSet.Len() != 1 {
		t.Errorf("expected 1 used PVC, got %d", snapshot.usedPVCSet.Len())
	}

	if !snapshot.IsPVCUsedByPods("default/pvc1") {
		t.Errorf("expected PVC 'default/pvc1' to be used by pods")
	}
}

func TestSnapshotNodeInfos(t *testing.T) {
	snapshot := NewEmptySnapshot()
	nodeInfo := framework.NewNodeInfo()
	nodeInfo.SetNode(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}})
	snapshot.nodeInfoMap["node1"] = nodeInfo

	nodeInfos := snapshot.NodeInfos()
	if nodeInfos == nil {
		t.Error("expected NodeInfos to be non-nil")
	}

	nodeInfoRetrieved, err := nodeInfos.Get("node1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if nodeInfoRetrieved.Node().Name != "node1" {
		t.Errorf("expected node name 'node1', got %s", nodeInfoRetrieved.Node().Name)
	}

	// Test getting non-existing node
	_, err = nodeInfos.Get("node2")
	if err == nil {
		t.Error("expected error for non-existing node, got nil")
	}
}

func TestSnapshotList(t *testing.T) {
	snapshot := NewEmptySnapshot()
	nodeInfo := framework.NewNodeInfo()
	nodeInfo.SetNode(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}})
	snapshot.nodeInfoList = append(snapshot.nodeInfoList, nodeInfo)

	nodeList, err := snapshot.List()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(nodeList) != 1 {
		t.Errorf("expected 1 node in list, got %d", len(nodeList))
	}

	if nodeList[0].Node().Name != "node1" {
		t.Errorf("expected node name 'node1', got %s", nodeList[0].Node().Name)
	}
}

func TestSnapshotHavePodsWithAffinityList(t *testing.T) {
	snapshot := NewEmptySnapshot()
	nodeInfo := framework.NewNodeInfo()
	snapshot.havePodsWithAffinityNodeInfoList = append(snapshot.havePodsWithAffinityNodeInfoList, nodeInfo)

	affinityList, err := snapshot.HavePodsWithAffinityList()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(affinityList) != 1 {
		t.Errorf("expected 1 node with pods with affinity, got %d", len(affinityList))
	}
}

func TestSnapshotHavePodsWithRequiredAntiAffinityList(t *testing.T) {
	snapshot := NewEmptySnapshot()
	nodeInfo := framework.NewNodeInfo()
	snapshot.havePodsWithRequiredAntiAffinityNodeInfoList = append(snapshot.havePodsWithRequiredAntiAffinityNodeInfoList, nodeInfo)

	antiAffinityList, err := snapshot.HavePodsWithRequiredAntiAffinityList()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(antiAffinityList) != 1 {
		t.Errorf("expected 1 node with pods with required anti-affinity, got %d", len(antiAffinityList))
	}
}

func TestSnapshotIsPVCUsedByPods(t *testing.T) {
	snapshot := NewEmptySnapshot()
	snapshot.usedPVCSet = sets.NewString("default/pvc1")

	if !snapshot.IsPVCUsedByPods("default/pvc1") {
		t.Errorf("expected PVC 'default/pvc1' to be used by pods")
	}

	if snapshot.IsPVCUsedByPods("default/pvc2") {
		t.Errorf("expected PVC 'default/pvc2' to not be used by pods")
	}
}

func TestSnapshotGet(t *testing.T) {
	snapshot := NewEmptySnapshot()
	nodeInfo := framework.NewNodeInfo()
	nodeInfo.SetNode(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}})
	snapshot.nodeInfoMap["node1"] = nodeInfo

	// Test getting existing node
	nodeInfoRetrieved, err := snapshot.Get("node1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if nodeInfoRetrieved.Node().Name != "node1" {
		t.Errorf("expected node name 'node1', got %s", nodeInfoRetrieved.Node().Name)
	}

	_, err = snapshot.Get("node2")
	if err == nil {
		t.Error("expected error for non-existing node, got nil")
	}
}
