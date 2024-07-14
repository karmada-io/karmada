/*
Copyright 2022 The Karmada Authors.

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

package v1alpha1

import (
	"testing"

	"k8s.io/utils/ptr"
)

func TestPropagationPolicy_ExplicitPriority(t *testing.T) {
	var tests = []struct {
		name             string
		declaredPriority *int32
		expectedPriority int32
	}{
		{
			name:             "expected to be zero in pp if no priority declared",
			expectedPriority: 0,
		},
		{
			name:             "expected to be declared priority in pp",
			declaredPriority: ptr.To[int32](20),
			expectedPriority: 20,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := PropagationPolicy{Spec: PropagationSpec{Priority: test.declaredPriority}}
			got := policy.ExplicitPriority()
			if test.expectedPriority != got {
				t.Fatalf("Expected：%d, but got: %d", test.expectedPriority, got)
			}
		})
	}
}

func TestClusterPropagationPolicy_ExplicitPriority(t *testing.T) {
	var tests = []struct {
		name             string
		declaredPriority *int32
		expectedPriority int32
	}{
		{
			name:             "expected to be zero in cpp if no priority declared",
			expectedPriority: 0,
		},
		{
			name:             "expected to be declared priority in cpp",
			declaredPriority: ptr.To[int32](20),
			expectedPriority: 20,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := ClusterPropagationPolicy{Spec: PropagationSpec{Priority: test.declaredPriority}}
			got := policy.ExplicitPriority()
			if test.expectedPriority != got {
				t.Fatalf("Expected：%d, but got: %d", test.expectedPriority, got)
			}
		})
	}
}

func TestPlacement_ReplicaSchedulingType(t *testing.T) {
	var tests = []struct {
		name                          string
		declaredReplicaSchedulingType ReplicaSchedulingType
		expectedReplicaSchedulingType ReplicaSchedulingType
	}{
		{
			name:                          "no replica scheduling strategy declared",
			expectedReplicaSchedulingType: ReplicaSchedulingTypeDuplicated,
		},
		{
			name:                          "replica scheduling strategy is 'Divided'",
			declaredReplicaSchedulingType: ReplicaSchedulingTypeDivided,
			expectedReplicaSchedulingType: ReplicaSchedulingTypeDivided,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := &Placement{}
			if test.declaredReplicaSchedulingType != "" {
				p.ReplicaScheduling = &ReplicaSchedulingStrategy{ReplicaSchedulingType: test.declaredReplicaSchedulingType}
			}
			got := p.ReplicaSchedulingType()
			if test.expectedReplicaSchedulingType != got {
				t.Fatalf("Expected：%s, but got: %s", test.expectedReplicaSchedulingType, got)
			}
		})
	}
}
