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
			name:             "no priority declared should defaults to zero",
			expectedPriority: 0,
		},
		{
			name:             "no priority declared should defaults to zero",
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
			name:             "no priority declared should defaults to zero",
			expectedPriority: 0,
		},
		{
			name:             "no priority declared should defaults to zero",
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
