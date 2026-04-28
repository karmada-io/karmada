/*
Copyright 2024 The Karmada Authors.

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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func TestIsLazyActivationEnabled(t *testing.T) {
	tests := []struct {
		name                 string
		activationPreference policyv1alpha1.ActivationPreference
		expected             bool
	}{
		{
			name:                 "empty activation preference",
			activationPreference: "",
			expected:             false,
		},
		{
			name:                 "lazy activation enabled",
			activationPreference: policyv1alpha1.LazyActivation,
			expected:             true,
		},
		{
			name:                 "different activation preference",
			activationPreference: "SomeOtherPreference",
			expected:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsLazyActivationEnabled(tt.activationPreference)
			assert.Equal(t, tt.expected, result, "unexpected result for activation preference: %s", tt.activationPreference)
		})
	}
}

func TestIsOverflowSupport(t *testing.T) {
	tests := []struct {
		name     string
		strategy *policyv1alpha1.ReplicaSchedulingStrategy
		expected bool
	}{
		{
			name:     "nil strategy",
			strategy: nil,
			expected: false,
		},
		{
			name: "Duplicated scheduling type",
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
			},
			expected: false,
		},
		{
			name: "Divided with Aggregated preference",
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
			expected: true,
		},
		{
			name: "Divided with Weighted and dynamic weight",
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				WeightPreference: &policyv1alpha1.ClusterPreferences{
					DynamicWeight: policyv1alpha1.DynamicWeightByAvailableReplicas,
				},
			},
			expected: true,
		},
		{
			name: "Divided with Weighted but nil WeightPreference",
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			},
			expected: false,
		},
		{
			name: "Divided with Weighted but empty DynamicWeight",
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				WeightPreference: &policyv1alpha1.ClusterPreferences{
					StaticWeightList: []policyv1alpha1.StaticClusterWeight{
						{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"member1"}}, Weight: 1},
					},
				},
			},
			expected: false,
		},
		{
			name: "Divided with empty ReplicaDivisionPreference",
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOverflowSchedulingAllowed(tt.strategy)
			assert.Equal(t, tt.expected, result)
		})
	}
}
