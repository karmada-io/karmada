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
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

func TestRegisterEqualityCheckFunctions(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
		},
	}
	workloadWithCR, _ := obj.MarshalJSON()
	workloadWithoutCR, _ := json.Marshal(obj)

	tests := []struct {
		name         string
		obj1         runtime.Object
		obj2         runtime.Object
		addCheckFunc bool
		wantEqual    bool
		wantErr      bool
	}{
		{
			name: "without custom check functions",
			obj1: &workv1alpha1.Work{
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{Manifests: []workv1alpha1.Manifest{{RawExtension: runtime.RawExtension{Raw: workloadWithCR}}}},
				},
			},
			obj2: &workv1alpha1.Work{
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{Manifests: []workv1alpha1.Manifest{{RawExtension: runtime.RawExtension{Raw: workloadWithoutCR}}}},
				},
			},
			addCheckFunc: false,
			wantEqual:    false,
			wantErr:      false,
		},
		{
			name: "with custom check functions",
			obj1: &workv1alpha1.Work{
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{Manifests: []workv1alpha1.Manifest{{RawExtension: runtime.RawExtension{Raw: workloadWithCR}}}},
				},
			},
			obj2: &workv1alpha1.Work{
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{Manifests: []workv1alpha1.Manifest{{RawExtension: runtime.RawExtension{Raw: workloadWithoutCR}}}},
				},
			},
			addCheckFunc: true,
			wantEqual:    true,
			wantErr:      false,
		},
		{
			name: "custom check functions should be able to notice the difference",
			obj1: &workv1alpha1.Work{
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{Manifests: []workv1alpha1.Manifest{{RawExtension: runtime.RawExtension{Raw: workloadWithCR}}}},
				},
			},
			obj2: &workv1alpha1.Work{
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{Manifests: []workv1alpha1.Manifest{{RawExtension: runtime.RawExtension{Raw: nil}}}},
				},
			},
			addCheckFunc: true,
			wantEqual:    false,
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := equality.Semantic.Copy()
			if tt.addCheckFunc {
				if err := RegisterEqualityCheckFunctions(&checker); (err != nil) != tt.wantErr {
					t.Errorf("RegisterEqualityCheckFunctions() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
			if equal := checker.DeepEqual(tt.obj1, tt.obj2); equal != tt.wantEqual {
				t.Errorf("TestRegisterEqualityCheckFunctions DeepEqual() = %v, want %v", equal, tt.wantEqual)
			}
		})
	}
}
