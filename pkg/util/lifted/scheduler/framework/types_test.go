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

package framework

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/karmada-io/karmada/pkg/util"
)

func TestClusterEventIsWildCard(t *testing.T) {
	tests := []struct {
		name     string
		event    ClusterEvent
		expected bool
	}{
		{
			name:     "WildCard event",
			event:    ClusterEvent{Resource: WildCard, ActionType: All},
			expected: true,
		},
		{
			name:     "Non-WildCard resource",
			event:    ClusterEvent{Resource: Pod, ActionType: All},
			expected: false,
		},
		{
			name:     "Non-WildCard action",
			event:    ClusterEvent{Resource: WildCard, ActionType: Add},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.event.IsWildCard()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQueuedPodInfoDeepCopy(t *testing.T) {
	now := time.Now()
	qpi := &QueuedPodInfo{
		PodInfo: &PodInfo{
			Pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
			},
		},
		Timestamp:               now,
		Attempts:                2,
		InitialAttemptTimestamp: now.Add(-time.Hour),
		UnschedulablePlugins:    sets.NewString("plugin1", "plugin2"),
		Gated:                   true,
	}
	podInfoCopy := qpi.DeepCopy()
	assert.Equal(t, qpi.PodInfo.Pod.Name, podInfoCopy.PodInfo.Pod.Name)
	assert.Equal(t, qpi.Timestamp, podInfoCopy.Timestamp)
	assert.Equal(t, qpi.Attempts, podInfoCopy.Attempts)
	assert.Equal(t, qpi.InitialAttemptTimestamp, podInfoCopy.InitialAttemptTimestamp)
	assert.Equal(t, qpi.UnschedulablePlugins, podInfoCopy.UnschedulablePlugins)
	assert.Equal(t, qpi.Gated, podInfoCopy.Gated)
}

func TestPodInfoDeepCopy(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}

	podInfo, _ := NewPodInfo(pod)
	copiedPodInfo := podInfo.DeepCopy()

	assert.NotSame(t, podInfo, copiedPodInfo)
	assert.Equal(t, podInfo.Pod.Name, copiedPodInfo.Pod.Name)
	assert.Equal(t, podInfo.Pod.Namespace, copiedPodInfo.Pod.Namespace)
	assert.Equal(t, podInfo.Pod.UID, copiedPodInfo.Pod.UID)
}

func TestPodInfoUpdate(t *testing.T) {
	originalPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}

	updatedPod := originalPod.DeepCopy()
	updatedPod.Spec.NodeName = "node-1"

	podInfo, _ := NewPodInfo(originalPod)
	err := podInfo.Update(updatedPod)

	assert.NoError(t, err)
	assert.Equal(t, updatedPod, podInfo.Pod)
	assert.Equal(t, "node-1", podInfo.Pod.Spec.NodeName)
}

func TestAffinityTermMatches(t *testing.T) {
	tests := []struct {
		name            string
		affinityTerm    AffinityTerm
		pod             *corev1.Pod
		nsLabels        labels.Set
		expectedMatches bool
	}{
		{
			name: "Matches namespace and labels",
			affinityTerm: AffinityTerm{
				Namespaces:        sets.NewString("test-ns"),
				Selector:          labels.SelectorFromSet(labels.Set{"app": "web"}),
				TopologyKey:       "kubernetes.io/hostname",
				NamespaceSelector: labels.Nothing(),
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-ns",
					Labels:    map[string]string{"app": "web"},
				},
			},
			nsLabels:        labels.Set{},
			expectedMatches: true,
		},
		{
			name: "Matches namespace selector",
			affinityTerm: AffinityTerm{
				Namespaces:        sets.NewString(),
				NamespaceSelector: labels.SelectorFromSet(labels.Set{"env": "prod"}),
				Selector:          labels.SelectorFromSet(labels.Set{"app": "db"}),
				TopologyKey:       "kubernetes.io/hostname",
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "prod-ns",
					Labels:    map[string]string{"app": "db"},
				},
			},
			nsLabels:        labels.Set{"env": "prod"},
			expectedMatches: true,
		},
		{
			name: "Does not match namespace",
			affinityTerm: AffinityTerm{
				Namespaces:        sets.NewString("test-ns"),
				Selector:          labels.SelectorFromSet(labels.Set{"app": "web"}),
				TopologyKey:       "kubernetes.io/hostname",
				NamespaceSelector: labels.Nothing(),
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "other-ns",
					Labels:    map[string]string{"app": "web"},
				},
			},
			nsLabels:        labels.Set{},
			expectedMatches: false,
		},
		{
			name: "Does not match labels",
			affinityTerm: AffinityTerm{
				Namespaces:        sets.NewString("test-ns"),
				Selector:          labels.SelectorFromSet(labels.Set{"app": "web"}),
				TopologyKey:       "kubernetes.io/hostname",
				NamespaceSelector: labels.Nothing(),
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-ns",
					Labels:    map[string]string{"app": "db"},
				},
			},
			nsLabels:        labels.Set{},
			expectedMatches: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := tt.affinityTerm.Matches(tt.pod, tt.nsLabels)
			assert.Equal(t, tt.expectedMatches, matches, "Unexpected match result")
		})
	}
}

func TestGetAffinityTerms(t *testing.T) {
	tests := []struct {
		name           string
		pod            *corev1.Pod
		affinityTerms  []corev1.PodAffinityTerm
		expectedTerms  int
		expectedErrMsg string
	}{
		{
			name: "Valid affinity terms",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-ns",
				},
			},
			affinityTerms: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "web"},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "db"},
					},
					TopologyKey: "kubernetes.io/zone",
				},
			},
			expectedTerms:  2,
			expectedErrMsg: "",
		},
		{
			name: "Invalid label selector",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-ns",
				},
			},
			affinityTerms: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "app",
								Operator: "InvalidOperator",
								Values:   []string{"web"},
							},
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
			expectedTerms:  0,
			expectedErrMsg: "\"InvalidOperator\" is not a valid label selector operator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terms, err := getAffinityTerms(tt.pod, tt.affinityTerms)

			if tt.expectedErrMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedTerms, len(terms), "Unexpected number of affinity terms")
			}
		})
	}
}

func TestGetWeightedAffinityTerms(t *testing.T) {
	tests := []struct {
		name           string
		pod            *corev1.Pod
		affinityTerms  []corev1.WeightedPodAffinityTerm
		expectedTerms  int
		expectedErrMsg string
	}{
		{
			name: "Valid weighted affinity terms",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-ns",
				},
			},
			affinityTerms: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "web"},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
				{
					Weight: 50,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "db"},
						},
						TopologyKey: "kubernetes.io/zone",
					},
				},
			},
			expectedTerms:  2,
			expectedErrMsg: "",
		},
		{
			name: "Invalid label selector",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-ns",
				},
			},
			affinityTerms: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "app",
									Operator: "InvalidOperator",
									Values:   []string{"web"},
								},
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
			expectedTerms:  0,
			expectedErrMsg: "\"InvalidOperator\" is not a valid label selector operator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terms, err := getWeightedAffinityTerms(tt.pod, tt.affinityTerms)

			if tt.expectedErrMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedTerms, len(terms), "Unexpected number of weighted affinity terms")
				for i, term := range terms {
					assert.Equal(t, tt.affinityTerms[i].Weight, term.Weight, "Unexpected weight for term")
				}
			}
		})
	}
}

func TestNewPodInfo(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
		Spec: corev1.PodSpec{
			Affinity: &corev1.Affinity{
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "web"},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
		},
	}

	podInfo, err := NewPodInfo(pod)

	assert.NoError(t, err)
	assert.NotNil(t, podInfo)
	assert.Equal(t, pod, podInfo.Pod)
	assert.Len(t, podInfo.RequiredAffinityTerms, 1)
	assert.Equal(t, "kubernetes.io/hostname", podInfo.RequiredAffinityTerms[0].TopologyKey)
}

func TestGetPodAffinityTerms(t *testing.T) {
	affinity := &corev1.Affinity{
		PodAffinity: &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "web"},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}

	terms := getPodAffinityTerms(affinity)

	assert.Len(t, terms, 1)
	assert.Equal(t, "kubernetes.io/hostname", terms[0].TopologyKey)
}

func TestGetPodAntiAffinityTerms(t *testing.T) {
	affinity := &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "db"},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}

	terms := getPodAntiAffinityTerms(affinity)

	assert.Len(t, terms, 1)
	assert.Equal(t, "kubernetes.io/hostname", terms[0].TopologyKey)
}

// disable `deprecation` check as sets.String is deprecated
//
//nolint:staticcheck
func TestGetNamespacesFromPodAffinityTerm(t *testing.T) {
	tests := []struct {
		name            string
		pod             *corev1.Pod
		podAffinityTerm *corev1.PodAffinityTerm
		expectedNS      sets.String
	}{
		{
			name: "No namespaces specified",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
			},
			podAffinityTerm: &corev1.PodAffinityTerm{},
			expectedNS:      sets.String{"default": sets.Empty{}},
		},
		{
			name: "Namespaces specified",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
			},
			podAffinityTerm: &corev1.PodAffinityTerm{
				Namespaces: []string{"ns1", "ns2"},
			},
			expectedNS: sets.String{"ns1": sets.Empty{}, "ns2": sets.Empty{}},
		},
		{
			name: "Namespace selector specified",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
			},
			podAffinityTerm: &corev1.PodAffinityTerm{
				NamespaceSelector: &metav1.LabelSelector{},
			},
			expectedNS: sets.String{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNamespacesFromPodAffinityTerm(tt.pod, tt.podAffinityTerm)
			assert.Equal(t, tt.expectedNS, result)
		})
	}
}

func TestNewNodeInfo(t *testing.T) {
	pods := []*corev1.Pod{
		{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
						},
					},
				},
			},
		},
		{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("300Mi"),
							},
						},
					},
				},
			},
		},
	}

	ni := NewNodeInfo(pods...)

	assert.Equal(t, 2, len(ni.Pods), "Expected 2 pods in NodeInfo")
	assert.Equal(t, int64(300), ni.Requested.MilliCPU, "Unexpected MilliCPU value")
	assert.Equal(t, int64(500*1024*1024), ni.Requested.Memory, "Unexpected Memory value")
	assert.NotNil(t, ni.UsedPorts, "UsedPorts should be initialized")
	assert.NotNil(t, ni.ImageStates, "ImageStates should be initialized")
	assert.NotNil(t, ni.PVCRefCounts, "PVCRefCounts should be initialized")
}

func TestNodeInfoNode(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	}
	nodeInfo := &NodeInfo{
		node: node,
	}

	assert.Equal(t, node, nodeInfo.Node())

	nilNodeInfo := (*NodeInfo)(nil)
	assert.Nil(t, nilNodeInfo.Node())
}

func TestNodeInfoClone(t *testing.T) {
	originalNI := &NodeInfo{
		Requested:        &util.Resource{MilliCPU: 100, Memory: 200},
		NonZeroRequested: &util.Resource{MilliCPU: 100, Memory: 200},
		Allocatable:      &util.Resource{MilliCPU: 1000, Memory: 2000},
		Generation:       42,
		UsedPorts: HostPortInfo{
			"0.0.0.0": {ProtocolPort{Protocol: "TCP", Port: 80}: {}},
		},
		ImageStates:  map[string]*ImageStateSummary{"image1": {Size: 1000, NumNodes: 1}},
		PVCRefCounts: map[string]int{"pvc1": 1},
	}

	clonedNI := originalNI.Clone()

	assert.Equal(t, originalNI.Requested, clonedNI.Requested, "Cloned Requested should be equal")
	assert.Equal(t, originalNI.NonZeroRequested, clonedNI.NonZeroRequested, "Cloned NonZeroRequested should be equal")
	assert.Equal(t, originalNI.Allocatable, clonedNI.Allocatable, "Cloned Allocatable should be equal")
	assert.Equal(t, originalNI.Generation, clonedNI.Generation, "Cloned Generation should be equal")
	assert.Equal(t, originalNI.UsedPorts, clonedNI.UsedPorts, "Cloned UsedPorts should be equal")
	assert.Equal(t, originalNI.ImageStates, clonedNI.ImageStates, "Cloned ImageStates should be equal")
	assert.Equal(t, originalNI.PVCRefCounts, clonedNI.PVCRefCounts, "Cloned PVCRefCounts should be equal")

	// Verify that modifying the clone doesn't affect the original
	clonedNI.Requested.MilliCPU = 200
	assert.NotEqual(t, originalNI.Requested.MilliCPU, clonedNI.Requested.MilliCPU, "Modifying clone should not affect original")
}

func TestNodeInfoString(t *testing.T) {
	nodeInfo := &NodeInfo{
		Requested: &util.Resource{
			MilliCPU: 1000,
			Memory:   2048,
		},
		NonZeroRequested: &util.Resource{
			MilliCPU: 1000,
			Memory:   2048,
		},
		Allocatable: &util.Resource{
			MilliCPU: 2000,
			Memory:   4096,
		},
		UsedPorts: HostPortInfo{
			"0.0.0.0": {
				ProtocolPort{Protocol: "TCP", Port: 80}: {},
			},
		},
	}

	nodeInfoString := nodeInfo.String()

	assert.Contains(t, nodeInfoString, "RequestedResource:&util.Resource{MilliCPU:1000, Memory:2048")
	assert.Contains(t, nodeInfoString, "NonZeroRequest: &util.Resource{MilliCPU:1000, Memory:2048")
	assert.Contains(t, nodeInfoString, "UsedPort: framework.HostPortInfo{\"0.0.0.0\":map[framework.ProtocolPort]struct {}{framework.ProtocolPort{Protocol:\"TCP\", Port:80}:struct {}{}}}")
	assert.Contains(t, nodeInfoString, "AllocatableResource:&util.Resource{MilliCPU:2000, Memory:4096")
}

func TestNodeInfoAddPodInfo(t *testing.T) {
	ni := NewNodeInfo()
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("200Mi"),
						},
					},
				},
			},
		},
	}
	podInfo, _ := NewPodInfo(pod)

	ni.AddPodInfo(podInfo)

	assert.Equal(t, 1, len(ni.Pods), "Expected 1 pod in NodeInfo")
	assert.Equal(t, int64(100), ni.Requested.MilliCPU, "Unexpected MilliCPU value")
	assert.Equal(t, int64(200*1024*1024), ni.Requested.Memory, "Unexpected Memory value")
}

func TestPodWithAffinity(t *testing.T) {
	tests := []struct {
		name           string
		pod            *corev1.Pod
		expectedResult bool
	}{
		{
			name: "Pod with affinity",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{"app": "web"},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			},
			expectedResult: true,
		},
		{
			name: "Pod with anti-affinity",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{"app": "db"},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			},
			expectedResult: true,
		},
		{
			name: "Pod without affinity",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{},
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := podWithAffinity(tt.pod)
			assert.Equal(t, tt.expectedResult, result, "Unexpected result for podWithAffinity")
		})
	}
}

func TestRemoveFromSlice(t *testing.T) {
	pods := []*PodInfo{
		{Pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{UID: "pod1"}}},
		{Pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{UID: "pod2"}}},
		{Pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{UID: "pod3"}}},
	}

	result := removeFromSlice(pods, "pod2")

	assert.Len(t, result, 2)
	assert.Equal(t, "pod1", string(result[0].Pod.UID))
	assert.Equal(t, "pod3", string(result[1].Pod.UID))
}

func TestCalculateResource(t *testing.T) {
	tests := []struct {
		name            string
		pod             *corev1.Pod
		expectedRes     util.Resource
		expectedNon0CPU int64
		expectedNon0Mem int64
	}{
		{
			name: "Pod with single container",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
							},
						},
					},
				},
			},
			expectedRes: util.Resource{
				MilliCPU: 100,
				Memory:   200 * 1024 * 1024,
			},
			expectedNon0CPU: 100,
			expectedNon0Mem: 200 * 1024 * 1024,
		},
		{
			name: "Pod with multiple containers",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
							},
						},
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("300Mi"),
								},
							},
						},
					},
				},
			},
			expectedRes: util.Resource{
				MilliCPU: 300,
				Memory:   500 * 1024 * 1024,
			},
			expectedNon0CPU: 300,
			expectedNon0Mem: 500 * 1024 * 1024,
		},
		{
			name: "Pod with init container",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
							},
						},
					},
				},
			},
			expectedRes: util.Resource{
				MilliCPU: 500,
				Memory:   1024 * 1024 * 1024,
			},
			expectedNon0CPU: 500,
			expectedNon0Mem: 1024 * 1024 * 1024,
		},
		{
			name: "Pod with overhead",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
							},
						},
					},
					Overhead: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
			},
			expectedRes: util.Resource{
				MilliCPU: 150,
				Memory:   300 * 1024 * 1024,
			},
			expectedNon0CPU: 150,
			expectedNon0Mem: 300 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, non0CPU, non0Mem := calculateResource(tt.pod)
			assert.Equal(t, tt.expectedRes.MilliCPU, res.MilliCPU, "Unexpected MilliCPU value")
			assert.Equal(t, tt.expectedRes.Memory, res.Memory, "Unexpected Memory value")
			assert.Equal(t, tt.expectedNon0CPU, non0CPU, "Unexpected non-zero CPU value")
			assert.Equal(t, tt.expectedNon0Mem, non0Mem, "Unexpected non-zero Memory value")
		})
	}
}

func TestNodeInfoUpdateUsedPorts(t *testing.T) {
	tests := []struct {
		name     string
		nodeInfo *NodeInfo
		pod      *corev1.Pod
		add      bool
		expected HostPortInfo
	}{
		{
			name: "Add ports",
			nodeInfo: &NodeInfo{
				UsedPorts: make(HostPortInfo),
			},
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Ports: []corev1.ContainerPort{
								{HostIP: "192.168.1.1", Protocol: "TCP", HostPort: 80},
								{HostIP: "192.168.1.1", Protocol: "UDP", HostPort: 53},
							},
						},
					},
				},
			},
			add: true,
			expected: HostPortInfo{
				"192.168.1.1": {
					ProtocolPort{Protocol: "TCP", Port: 80}: struct{}{},
					ProtocolPort{Protocol: "UDP", Port: 53}: struct{}{},
				},
			},
		},
		{
			name: "Remove ports",
			nodeInfo: &NodeInfo{
				UsedPorts: HostPortInfo{
					"192.168.1.1": {
						ProtocolPort{Protocol: "TCP", Port: 80}: struct{}{},
						ProtocolPort{Protocol: "UDP", Port: 53}: struct{}{},
					},
				},
			},
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Ports: []corev1.ContainerPort{
								{HostIP: "192.168.1.1", Protocol: "TCP", HostPort: 80},
							},
						},
					},
				},
			},
			add: false,
			expected: HostPortInfo{
				"192.168.1.1": {
					ProtocolPort{Protocol: "UDP", Port: 53}: struct{}{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.nodeInfo.updateUsedPorts(tt.pod, tt.add)
			assert.Equal(t, tt.expected, tt.nodeInfo.UsedPorts, "Unexpected UsedPorts")
		})
	}
}

func TestNodeInfoUpdatePVCRefCounts(t *testing.T) {
	ni := &NodeInfo{
		PVCRefCounts: make(map[string]int),
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "vol1",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "pvc1",
						},
					},
				},
				{
					Name: "vol2",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "pvc2",
						},
					},
				},
			},
		},
	}

	// Test adding PVC references
	ni.updatePVCRefCounts(pod, true)
	assert.Equal(t, 1, ni.PVCRefCounts["default/pvc1"])
	assert.Equal(t, 1, ni.PVCRefCounts["default/pvc2"])

	// Test removing PVC references
	ni.updatePVCRefCounts(pod, false)
	assert.NotContains(t, ni.PVCRefCounts, "default/pvc1")
	assert.NotContains(t, ni.PVCRefCounts, "default/pvc2")
}

func TestNodeInfoSetNode(t *testing.T) {
	ni := &NodeInfo{}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("4Gi"),
			},
		},
	}

	ni.SetNode(node)

	assert.Equal(t, node, ni.node)
	assert.Equal(t, int64(2000), ni.Allocatable.MilliCPU)
	assert.Equal(t, int64(4*1024*1024*1024), ni.Allocatable.Memory)
	assert.NotZero(t, ni.Generation)
}

func TestNodeInfoRemoveNode(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	}
	ni := &NodeInfo{
		node: node,
	}

	oldGeneration := ni.Generation
	ni.RemoveNode()

	assert.Nil(t, ni.node)
	assert.NotEqual(t, oldGeneration, ni.Generation)
}

func TestGetPodKey(t *testing.T) {
	tests := []struct {
		name        string
		pod         *corev1.Pod
		expectedKey string
		expectError bool
	}{
		{
			name: "Valid pod with UID",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					UID: "123456789",
				},
			},
			expectedKey: "123456789",
			expectError: false,
		},
		{
			name: "Pod without UID",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expectedKey: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := GetPodKey(tt.pod)

			if tt.expectError {
				assert.Error(t, err, "Expected an error for pod without UID")
			} else {
				assert.NoError(t, err, "Did not expect an error for valid pod")
				assert.Equal(t, tt.expectedKey, key, "Unexpected pod key")
			}
		})
	}
}

func TestGetNamespacedName(t *testing.T) {
	tests := []struct {
		testName     string
		namespace    string
		resourceName string
		expected     string
	}{
		{
			testName:     "Valid namespace and name",
			namespace:    "default",
			resourceName: "my-pod",
			expected:     "default/my-pod",
		},
		{
			testName:     "Empty namespace",
			namespace:    "",
			resourceName: "my-pod",
			expected:     "/my-pod",
		},
		{
			testName:     "Empty name",
			namespace:    "default",
			resourceName: "",
			expected:     "default/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			result := GetNamespacedName(tt.namespace, tt.resourceName)
			assert.Equal(t, tt.expected, result, "Unexpected namespaced name")
		})
	}
}

func TestNodeInfoRemovePod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("test-pod-uid"),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("200Mi"),
						},
					},
				},
			},
		},
	}

	ni := NewNodeInfo()
	ni.AddPod(pod)

	// Verify that the pod was added correctly
	assert.Equal(t, 1, len(ni.Pods), "Expected 1 pod in NodeInfo before removal")
	assert.Equal(t, int64(100), ni.Requested.MilliCPU, "Unexpected MilliCPU value before removal")
	assert.Equal(t, int64(200*1024*1024), ni.Requested.Memory, "Unexpected Memory value before removal")

	err := ni.RemovePod(pod)

	assert.NoError(t, err, "RemovePod should not return an error")
	assert.Equal(t, 0, len(ni.Pods), "Expected 0 pods in NodeInfo after removal")
	assert.Equal(t, int64(0), ni.Requested.MilliCPU, "MilliCPU should be 0 after pod removal")
	assert.Equal(t, int64(0), ni.Requested.Memory, "Memory should be 0 after pod removal")
}

func TestNewProtocolPort(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		port     int32
		expected ProtocolPort
	}{
		{
			name:     "TCP protocol",
			protocol: "TCP",
			port:     80,
			expected: ProtocolPort{Protocol: "TCP", Port: 80},
		},
		{
			name:     "UDP protocol",
			protocol: "UDP",
			port:     53,
			expected: ProtocolPort{Protocol: "UDP", Port: 53},
		},
		{
			name:     "Empty protocol defaults to TCP",
			protocol: "",
			port:     8080,
			expected: ProtocolPort{Protocol: "TCP", Port: 8080},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewProtocolPort(tt.protocol, tt.port)
			assert.Equal(t, tt.expected, *result, "NewProtocolPort(%s, %d) returned unexpected result", tt.protocol, tt.port)
		})
	}
}

func TestHostPortInfoAdd(t *testing.T) {
	tests := []struct {
		name     string
		initial  HostPortInfo
		ip       string
		protocol string
		port     int32
		expected HostPortInfo
	}{
		{
			name:     "Add new IP and port",
			initial:  HostPortInfo{},
			ip:       "192.168.1.1",
			protocol: "TCP",
			port:     80,
			expected: HostPortInfo{
				"192.168.1.1": {
					ProtocolPort{Protocol: "TCP", Port: 80}: {},
				},
			},
		},
		{
			name: "Add to existing IP",
			initial: HostPortInfo{
				"192.168.1.1": {
					ProtocolPort{Protocol: "TCP", Port: 80}: {},
				},
			},
			ip:       "192.168.1.1",
			protocol: "UDP",
			port:     53,
			expected: HostPortInfo{
				"192.168.1.1": {
					ProtocolPort{Protocol: "TCP", Port: 80}: {},
					ProtocolPort{Protocol: "UDP", Port: 53}: {},
				},
			},
		},
		{
			name:     "Add with empty protocol",
			initial:  HostPortInfo{},
			ip:       "10.0.0.1",
			protocol: "",
			port:     8080,
			expected: HostPortInfo{
				"10.0.0.1": {
					ProtocolPort{Protocol: "TCP", Port: 8080}: {},
				},
			},
		},
		{
			name:     "Add with empty IP",
			initial:  HostPortInfo{},
			ip:       "",
			protocol: "TCP",
			port:     443,
			expected: HostPortInfo{
				"0.0.0.0": {
					ProtocolPort{Protocol: "TCP", Port: 443}: {},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := tt.initial
			h.Add(tt.ip, tt.protocol, tt.port)

			assert.Equal(t, tt.expected, h, "HostPortInfo.Add() resulted in unexpected state")
		})
	}
}

func TestHostPortInfoRemove(t *testing.T) {
	tests := []struct {
		name     string
		initial  HostPortInfo
		ip       string
		protocol string
		port     int32
		expected HostPortInfo
	}{
		{
			name: "Remove existing entry",
			initial: HostPortInfo{
				"192.168.1.1": {
					ProtocolPort{Protocol: "TCP", Port: 80}: {},
					ProtocolPort{Protocol: "UDP", Port: 53}: {},
				},
			},
			ip:       "192.168.1.1",
			protocol: "TCP",
			port:     80,
			expected: HostPortInfo{
				"192.168.1.1": {
					ProtocolPort{Protocol: "UDP", Port: 53}: {},
				},
			},
		},
		{
			name: "Remove last entry for IP",
			initial: HostPortInfo{
				"192.168.1.1": {
					ProtocolPort{Protocol: "TCP", Port: 80}: {},
				},
			},
			ip:       "192.168.1.1",
			protocol: "TCP",
			port:     80,
			expected: HostPortInfo{},
		},
		{
			name: "Remove non-existent entry",
			initial: HostPortInfo{
				"192.168.1.1": {
					ProtocolPort{Protocol: "TCP", Port: 80}: {},
				},
			},
			ip:       "192.168.1.1",
			protocol: "UDP",
			port:     53,
			expected: HostPortInfo{
				"192.168.1.1": {
					ProtocolPort{Protocol: "TCP", Port: 80}: {},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := tt.initial
			h.Remove(tt.ip, tt.protocol, tt.port)

			assert.Equal(t, tt.expected, h, "HostPortInfo.Remove() resulted in unexpected state")
		})
	}
}

func TestHostPortInfoLen(t *testing.T) {
	tests := []struct {
		name     string
		info     HostPortInfo
		expected int
	}{
		{
			name:     "Empty HostPortInfo",
			info:     HostPortInfo{},
			expected: 0,
		},
		{
			name: "Single IP, single port",
			info: HostPortInfo{
				"192.168.1.1": {
					ProtocolPort{Protocol: "TCP", Port: 80}: {},
				},
			},
			expected: 1,
		},
		{
			name: "Single IP, multiple ports",
			info: HostPortInfo{
				"192.168.1.1": {
					ProtocolPort{Protocol: "TCP", Port: 80}:  {},
					ProtocolPort{Protocol: "UDP", Port: 53}:  {},
					ProtocolPort{Protocol: "TCP", Port: 443}: {},
				},
			},
			expected: 3,
		},
		{
			name: "Multiple IPs, multiple ports",
			info: HostPortInfo{
				"192.168.1.1": {
					ProtocolPort{Protocol: "TCP", Port: 80}: {},
					ProtocolPort{Protocol: "UDP", Port: 53}: {},
				},
				"10.0.0.1": {
					ProtocolPort{Protocol: "TCP", Port: 8080}: {},
				},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.info.Len()
			assert.Equal(t, tt.expected, result, "HostPortInfo.Len() returned unexpected result")
		})
	}
}

func TestHostPortInfoCheckConflict(t *testing.T) {
	tests := []struct {
		name     string
		hpi      HostPortInfo
		ip       string
		protocol string
		port     int32
		expected bool
	}{
		{
			name: "No conflict",
			hpi: HostPortInfo{
				"192.168.1.1": {ProtocolPort{Protocol: "TCP", Port: 80}: {}},
			},
			ip:       "192.168.1.1",
			protocol: "TCP",
			port:     8080,
			expected: false,
		},
		{
			name: "Conflict with same IP",
			hpi: HostPortInfo{
				"192.168.1.1": {ProtocolPort{Protocol: "TCP", Port: 80}: {}},
			},
			ip:       "192.168.1.1",
			protocol: "TCP",
			port:     80,
			expected: true,
		},
		{
			name: "Conflict with 0.0.0.0",
			hpi: HostPortInfo{
				"0.0.0.0": {ProtocolPort{Protocol: "TCP", Port: 80}: {}},
			},
			ip:       "192.168.1.1",
			protocol: "TCP",
			port:     80,
			expected: true,
		},
		{
			name: "No conflict with different protocol",
			hpi: HostPortInfo{
				"192.168.1.1": {ProtocolPort{Protocol: "TCP", Port: 80}: {}},
			},
			ip:       "192.168.1.1",
			protocol: "UDP",
			port:     80,
			expected: false,
		},
		{
			name: "Input IP is 0.0.0.0, conflict with any IP",
			hpi: HostPortInfo{
				"192.168.1.1": {ProtocolPort{Protocol: "TCP", Port: 80}: {}},
				"10.0.0.1":    {ProtocolPort{Protocol: "UDP", Port: 53}: {}},
			},
			ip:       "0.0.0.0",
			protocol: "TCP",
			port:     80,
			expected: true,
		},
		{
			name: "Input IP is 0.0.0.0, no conflict",
			hpi: HostPortInfo{
				"192.168.1.1": {ProtocolPort{Protocol: "TCP", Port: 80}: {}},
				"10.0.0.1":    {ProtocolPort{Protocol: "UDP", Port: 53}: {}},
			},
			ip:       "0.0.0.0",
			protocol: "TCP",
			port:     8080,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.hpi.CheckConflict(tt.ip, tt.protocol, tt.port)
			assert.Equal(t, tt.expected, result)
		})
	}
}
