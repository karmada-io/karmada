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

func TestFilterBindings(t *testing.T) {
	tests := []struct {
		name     string
		bindings []*workv1alpha2.ResourceBinding
		expected int
	}{
		{
			name: "No valid bindings",
			bindings: []*workv1alpha2.ResourceBinding{
				createBinding("binding1", "v1", "Pod", nil),
				createBinding("binding2", "v1", "Service", nil),
			},
			expected: 0,
		},
		{
			name: "Mix of valid and invalid bindings",
			bindings: []*workv1alpha2.ResourceBinding{
				createBinding("binding1", "apps/v1", "Deployment", createValidPlacement()),
				createBinding("binding2", "v1", "Pod", createValidPlacement()),
				createBinding("binding3", "apps/v1", "Deployment", createInvalidPlacement()),
			},
			expected: 1,
		},
		{
			name: "All valid bindings",
			bindings: []*workv1alpha2.ResourceBinding{
				createBinding("binding1", "apps/v1", "Deployment", createValidPlacement()),
				createBinding("binding2", "apps/v1", "Deployment", createValidPlacement()),
			},
			expected: 2,
		},
		{
			name: "Invalid placement annotation",
			bindings: []*workv1alpha2.ResourceBinding{
				createBindingWithInvalidPlacementAnnotation("binding1", "apps/v1", "Deployment"),
			},
			expected: 0,
		},
		{
			name: "Mix of valid and invalid annotations",
			bindings: []*workv1alpha2.ResourceBinding{
				createBindingWithInvalidPlacementAnnotation("binding1", "apps/v1", "Deployment"),
				createBinding("binding2", "apps/v1", "Deployment", createValidPlacement()),
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterBindings(tt.bindings)
			if len(filtered) != tt.expected {
				t.Errorf("FilterBindings() returned %d bindings, expected %d", len(filtered), tt.expected)
			}
		})
	}
}

func TestValidateGVK(t *testing.T) {
	tests := []struct {
		name      string
		reference *workv1alpha2.ObjectReference
		expected  bool
	}{
		{
			name: "Supported GVK - Deployment",
			reference: &workv1alpha2.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			expected: true,
		},
		{
			name: "Unsupported GVK - Pod",
			reference: &workv1alpha2.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			expected: false,
		},
		{
			name: "Unsupported GVK - Custom Resource",
			reference: &workv1alpha2.ObjectReference{
				APIVersion: "custom.example.com/v1",
				Kind:       "MyCustomResource",
			},
			expected: false,
		},
		{
			name: "Unsupported GVK - Different Version of Deployment",
			reference: &workv1alpha2.ObjectReference{
				APIVersion: "apps/v2",
				Kind:       "Deployment",
			},
			expected: false,
		},
		{
			name: "Empty APIVersion",
			reference: &workv1alpha2.ObjectReference{
				APIVersion: "",
				Kind:       "Deployment",
			},
			expected: false,
		},
		{
			name: "Empty Kind",
			reference: &workv1alpha2.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "",
			},
			expected: false,
		},
		{
			name: "Case-sensitive check",
			reference: &workv1alpha2.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "deployment",
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
	tests := []struct {
		name     string
		binding  *workv1alpha2.ResourceBinding
		expected bool
	}{
		{
			name:     "No policyPlacementAnnotation",
			binding:  createBinding("binding1", "apps/v1", "Deployment", nil),
			expected: false,
		},
		{
			name:     "ReplicaSchedulingType Duplicated",
			binding:  createBinding("binding2", "apps/v1", "Deployment", createPlacement(policyv1alpha1.ReplicaSchedulingTypeDuplicated, "", nil)),
			expected: false,
		},
		{
			name:     "ReplicaSchedulingType Divided, Preference Aggregated",
			binding:  createBinding("binding3", "apps/v1", "Deployment", createPlacement(policyv1alpha1.ReplicaSchedulingTypeDivided, policyv1alpha1.ReplicaDivisionPreferenceAggregated, nil)),
			expected: true,
		},
		{
			name:     "ReplicaSchedulingType Divided, Preference Weighted, No DynamicWeight",
			binding:  createBinding("binding4", "apps/v1", "Deployment", createPlacement(policyv1alpha1.ReplicaSchedulingTypeDivided, policyv1alpha1.ReplicaDivisionPreferenceWeighted, &policyv1alpha1.ClusterPreferences{})),
			expected: false,
		},
		{
			name:     "ReplicaSchedulingType Divided, Preference Weighted, With DynamicWeight",
			binding:  createBinding("binding5", "apps/v1", "Deployment", createPlacement(policyv1alpha1.ReplicaSchedulingTypeDivided, policyv1alpha1.ReplicaDivisionPreferenceWeighted, &policyv1alpha1.ClusterPreferences{DynamicWeight: policyv1alpha1.DynamicWeightByAvailableReplicas})),
			expected: true,
		},
		{
			name:     "Invalid JSON in placement annotation",
			binding:  createBindingWithInvalidPlacementAnnotation("binding6", "apps/v1", "Deployment"),
			expected: false,
		},
		{
			name: "Valid JSON but invalid placement structure",
			binding: func() *workv1alpha2.ResourceBinding {
				b := createBinding("binding7", "apps/v1", "Deployment", nil)
				b.Annotations = map[string]string{util.PolicyPlacementAnnotation: `{"invalidField": "value"}`}
				return b
			}(),
			expected: false,
		},
		{
			name: "Nil ReplicaScheduling",
			binding: func() *workv1alpha2.ResourceBinding {
				p := &policyv1alpha1.Placement{
					ReplicaScheduling: nil,
				}
				return createBinding("binding8", "apps/v1", "Deployment", p)
			}(),
			expected: false,
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

func createBindingWithInvalidPlacementAnnotation(name, apiVersion, kind string) *workv1alpha2.ResourceBinding {
	binding := createBinding(name, apiVersion, kind, nil)
	binding.Annotations = map[string]string{util.PolicyPlacementAnnotation: "invalid json"}
	return binding
}

func createBinding(name, apiVersion, kind string, placement *policyv1alpha1.Placement) *workv1alpha2.ResourceBinding {
	binding := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Resource: workv1alpha2.ObjectReference{
				APIVersion: apiVersion,
				Kind:       kind,
			},
		},
	}

	if placement != nil {
		marshaledBytes, _ := json.Marshal(placement)
		binding.Annotations = map[string]string{util.PolicyPlacementAnnotation: string(marshaledBytes)}
	}

	return binding
}

func createPlacement(schedulingType policyv1alpha1.ReplicaSchedulingType, divisionPreference policyv1alpha1.ReplicaDivisionPreference, weightPreference *policyv1alpha1.ClusterPreferences) *policyv1alpha1.Placement {
	return &policyv1alpha1.Placement{
		ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
			ReplicaSchedulingType:     schedulingType,
			ReplicaDivisionPreference: divisionPreference,
			WeightPreference:          weightPreference,
		},
	}
}

func createValidPlacement() *policyv1alpha1.Placement {
	return createPlacement(policyv1alpha1.ReplicaSchedulingTypeDivided, policyv1alpha1.ReplicaDivisionPreferenceAggregated, nil)
}

func createInvalidPlacement() *policyv1alpha1.Placement {
	return createPlacement(policyv1alpha1.ReplicaSchedulingTypeDuplicated, "", nil)
}
