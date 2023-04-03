package v1alpha1

import (
	"testing"

	"k8s.io/utils/pointer"
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
			declaredPriority: pointer.Int32(20),
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
			declaredPriority: pointer.Int32(20),
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
