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

package core

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
)

func TestValidateGVK(t *testing.T) {
	tests := []struct {
		name      string
		reference *workv1alpha2.ObjectReference
		expected  bool
	}{
		{
			name: "supportedGVKs",
			reference: &workv1alpha2.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			expected: true,
		},
		{
			name: "unsupportedGVKs",
			reference: &workv1alpha2.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := validateGVK(tt.reference)
			if res != tt.expected {
				t.Errorf("validateGVK() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestValidatePlacement(t *testing.T) {
	fakePlacement1 := policyv1alpha1.Placement{
		ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
			ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
		},
	}
	marshaledBytes1, _ := json.Marshal(fakePlacement1)
	fakePlacement2 := policyv1alpha1.Placement{
		ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
			ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
			ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
		},
	}
	marshaledBytes2, _ := json.Marshal(fakePlacement2)
	fakePlacement3 := policyv1alpha1.Placement{
		ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
			ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
			ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			WeightPreference:          &policyv1alpha1.ClusterPreferences{},
		},
	}
	marshaledBytes3, _ := json.Marshal(fakePlacement3)
	fakePlacement4 := policyv1alpha1.Placement{
		ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
			ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
			ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			WeightPreference: &policyv1alpha1.ClusterPreferences{
				DynamicWeight: policyv1alpha1.DynamicWeightByAvailableReplicas,
			},
		},
	}
	marshaledBytes4, _ := json.Marshal(fakePlacement4)

	tests := []struct {
		name     string
		binding  *workv1alpha2.ResourceBinding
		expected bool
	}{
		{
			name: "no policyPlacementAnnotation",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			expected: false,
		},
		{
			name: "propagationPolicy schedules replicas as non-dynamic",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "foo",
					Namespace:   "bar",
					Annotations: map[string]string{util.PolicyPlacementAnnotation: string(marshaledBytes1)},
				},
			},
			expected: false,
		},
		{
			name: "propagationPolicy schedules replicas as dynamic: ReplicaDivisionPreference is Aggregated",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "foo",
					Namespace:   "bar",
					Annotations: map[string]string{util.PolicyPlacementAnnotation: string(marshaledBytes2)},
				},
			},
			expected: true,
		},
		{
			name: "propagationPolicy schedules replicas as dynamic: DynamicWeight is null",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "foo",
					Namespace:   "bar",
					Annotations: map[string]string{util.PolicyPlacementAnnotation: string(marshaledBytes3)},
				},
			},
			expected: false,
		},
		{
			name: "propagationPolicy schedules replicas as dynamic: DynamicWeight is not null",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "foo",
					Namespace:   "bar",
					Annotations: map[string]string{util.PolicyPlacementAnnotation: string(marshaledBytes4)},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := validatePlacement(tt.binding)
			if res != tt.expected {
				t.Errorf("validatePlacement() = %v, want %v", res, tt.expected)
			}
		})
	}
}
