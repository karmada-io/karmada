/*
Copyright 2026 The Karmada Authors.

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

package pb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func marshalNodeSelector(t *testing.T, ns *corev1.NodeSelector) []byte {
	t.Helper()
	b, err := ns.Marshal()
	require.NoError(t, err)
	return b
}

func makeNodeSelector(key, value string) *corev1.NodeSelector {
	return &corev1.NodeSelector{
		NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{Key: key, Operator: corev1.NodeSelectorOpIn, Values: []string{value}},
				},
			},
		},
	}
}

func TestNodeClaim_UnmarshalNodeAffinity(t *testing.T) {
	nsA := makeNodeSelector("zone", "us-east-1")
	nsB := makeNodeSelector("zone", "eu-west-1")

	tests := []struct {
		name     string
		claim    *NodeClaim
		expected *corev1.NodeSelector
		wantErr  bool
	}{
		{
			name:     "nil receiver returns nil",
			claim:    nil,
			expected: nil,
		},
		{
			name:     "empty NodeClaim returns nil",
			claim:    &NodeClaim{},
			expected: nil,
		},
		{
			name:     "reads from NodeAffinityBytes when set",
			claim:    &NodeClaim{NodeAffinityBytes: marshalNodeSelector(t, nsA)},
			expected: nsA,
		},
		{
			name:     "fallback to deprecated NodeAffinity when bytes empty",
			claim:    &NodeClaim{NodeAffinity: nsB},
			expected: nsB,
		},
		{
			name: "bytes takes precedence when both fields disagree",
			claim: &NodeClaim{
				NodeAffinityBytes: marshalNodeSelector(t, nsA),
				NodeAffinity:      nsB,
			},
			expected: nsA,
		},
		{
			name:    "invalid bytes returns error",
			claim:   &NodeClaim{NodeAffinityBytes: []byte("invalid")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.claim.UnmarshalNodeAffinity()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func marshalToleration(t *testing.T, tol corev1.Toleration) []byte {
	t.Helper()
	b, err := tol.Marshal()
	require.NoError(t, err)
	return b
}

func TestNodeClaim_UnmarshalTolerations(t *testing.T) {
	tolA := corev1.Toleration{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "val1", Effect: corev1.TaintEffectNoSchedule}
	tolB := corev1.Toleration{Key: "key2", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute}

	tests := []struct {
		name     string
		claim    *NodeClaim
		expected []corev1.Toleration
		wantErr  bool
	}{
		{
			name:     "nil receiver returns nil",
			claim:    (*NodeClaim)(nil),
			expected: nil,
		},
		{
			name:     "empty NodeClaim returns nil",
			claim:    &NodeClaim{},
			expected: nil,
		},
		{
			name: "reads from TolerationsBytes when set",
			claim: &NodeClaim{
				TolerationsBytes: [][]byte{marshalToleration(t, tolA), marshalToleration(t, tolB)},
			},
			expected: []corev1.Toleration{tolA, tolB},
		},
		{
			name: "fallback to deprecated Tolerations when bytes empty",
			claim: &NodeClaim{
				Tolerations: []*corev1.Toleration{&tolA, &tolB},
			},
			expected: []corev1.Toleration{tolA, tolB},
		},
		{
			name: "bytes takes precedence when both fields disagree",
			claim: &NodeClaim{
				TolerationsBytes: [][]byte{marshalToleration(t, tolA)},
				Tolerations:      []*corev1.Toleration{&tolB},
			},
			expected: []corev1.Toleration{tolA},
		},
		{
			name: "ordering is preserved in round-trip",
			claim: &NodeClaim{
				TolerationsBytes: [][]byte{marshalToleration(t, tolB), marshalToleration(t, tolA)},
			},
			expected: []corev1.Toleration{tolB, tolA},
		},
		{
			name: "invalid bytes returns error",
			claim: &NodeClaim{
				TolerationsBytes: [][]byte{[]byte("invalid")},
			},
			wantErr: true,
		},
		{
			name: "nil pointer in deprecated field treated as zero value",
			claim: &NodeClaim{
				Tolerations: []*corev1.Toleration{nil, &tolA},
			},
			expected: []corev1.Toleration{{}, tolA},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.claim.UnmarshalTolerations()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func marshalQuantity(t *testing.T, q resource.Quantity) []byte {
	t.Helper()
	b, err := q.Marshal()
	require.NoError(t, err)
	return b
}

func TestReplicaRequirements_UnmarshalResourceRequest(t *testing.T) {
	cpu100m := resource.MustParse("100m")
	mem256Mi := resource.MustParse("256Mi")

	tests := []struct {
		name     string
		req      *ReplicaRequirements
		expected corev1.ResourceList
		wantErr  bool
	}{
		{
			name:     "nil receiver returns nil",
			req:      (*ReplicaRequirements)(nil),
			expected: nil,
		},
		{
			name:     "empty struct returns nil",
			req:      &ReplicaRequirements{},
			expected: nil,
		},
		{
			name: "reads from ResourceRequestBytes when set",
			req: &ReplicaRequirements{
				ResourceRequestBytes: map[string][]byte{
					"cpu":    marshalQuantity(t, cpu100m),
					"memory": marshalQuantity(t, mem256Mi),
				},
			},
			expected: corev1.ResourceList{
				corev1.ResourceCPU:    cpu100m,
				corev1.ResourceMemory: mem256Mi,
			},
		},
		{
			name: "fallback to deprecated ResourceRequest when bytes empty",
			req: &ReplicaRequirements{
				ResourceRequest: map[string]*resource.Quantity{
					"cpu": &cpu100m,
				},
			},
			expected: corev1.ResourceList{
				corev1.ResourceCPU: cpu100m,
			},
		},
		{
			name: "bytes takes precedence when both fields disagree",
			req: &ReplicaRequirements{
				ResourceRequestBytes: map[string][]byte{
					"cpu": marshalQuantity(t, cpu100m),
				},
				ResourceRequest: map[string]*resource.Quantity{
					"cpu":    &mem256Mi,
					"memory": &mem256Mi,
				},
			},
			expected: corev1.ResourceList{
				corev1.ResourceCPU: cpu100m,
			},
		},
		{
			name: "invalid bytes returns error",
			req: &ReplicaRequirements{
				ResourceRequestBytes: map[string][]byte{
					"cpu": []byte("invalid"),
				},
			},
			wantErr: true,
		},
		{
			name: "nil pointer in deprecated field is skipped",
			req: &ReplicaRequirements{
				ResourceRequest: map[string]*resource.Quantity{
					"cpu":    &cpu100m,
					"memory": nil,
				},
			},
			expected: corev1.ResourceList{
				corev1.ResourceCPU: cpu100m,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.req.UnmarshalResourceRequest()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestComponentReplicaRequirements_UnmarshalResourceRequest(t *testing.T) {
	cpu100m := resource.MustParse("100m")
	mem256Mi := resource.MustParse("256Mi")

	tests := []struct {
		name     string
		req      *ComponentReplicaRequirements
		expected corev1.ResourceList
		wantErr  bool
	}{
		{
			name:     "nil receiver returns nil",
			req:      (*ComponentReplicaRequirements)(nil),
			expected: nil,
		},
		{
			name:     "empty struct returns nil",
			req:      &ComponentReplicaRequirements{},
			expected: nil,
		},
		{
			name: "reads from ResourceRequestBytes when set",
			req: &ComponentReplicaRequirements{
				ResourceRequestBytes: map[string][]byte{
					"cpu":    marshalQuantity(t, cpu100m),
					"memory": marshalQuantity(t, mem256Mi),
				},
			},
			expected: corev1.ResourceList{
				corev1.ResourceCPU:    cpu100m,
				corev1.ResourceMemory: mem256Mi,
			},
		},
		{
			name: "fallback to deprecated ResourceRequest when bytes empty",
			req: &ComponentReplicaRequirements{
				ResourceRequest: map[string]*resource.Quantity{
					"cpu": &cpu100m,
				},
			},
			expected: corev1.ResourceList{
				corev1.ResourceCPU: cpu100m,
			},
		},
		{
			name: "bytes takes precedence when both fields disagree",
			req: &ComponentReplicaRequirements{
				ResourceRequestBytes: map[string][]byte{
					"cpu": marshalQuantity(t, cpu100m),
				},
				ResourceRequest: map[string]*resource.Quantity{
					"cpu":    &mem256Mi,
					"memory": &mem256Mi,
				},
			},
			expected: corev1.ResourceList{
				corev1.ResourceCPU: cpu100m,
			},
		},
		{
			name: "invalid bytes returns error",
			req: &ComponentReplicaRequirements{
				ResourceRequestBytes: map[string][]byte{
					"cpu": []byte("invalid"),
				},
			},
			wantErr: true,
		},
		{
			name: "nil pointer in deprecated field is skipped",
			req: &ComponentReplicaRequirements{
				ResourceRequest: map[string]*resource.Quantity{
					"cpu":    &cpu100m,
					"memory": nil,
				},
			},
			expected: corev1.ResourceList{
				corev1.ResourceCPU: cpu100m,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.req.UnmarshalResourceRequest()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
