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

package helper

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestGenMergePatch(t *testing.T) {
	testObj := &workv1alpha2.ResourceBinding{
		TypeMeta:   metav1.TypeMeta{Kind: "ResourceBinding", APIVersion: "work.karmada.io/v1alpha2"},
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec:       workv1alpha2.ResourceBindingSpec{Clusters: []workv1alpha2.TargetCluster{{Name: "foo", Replicas: 20}}},
		Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{{Type: "Dummy", Reason: "Dummy"}}},
	}

	tests := []struct {
		name          string
		modifyFunc    func() interface{}
		expectedPatch string
		expectErr     bool
	}{
		{
			name: "update spec",
			modifyFunc: func() interface{} {
				modified := testObj.DeepCopy()
				modified.Spec.Replicas = 10
				modified.Spec.Clusters = []workv1alpha2.TargetCluster{
					{
						Name:     "m1",
						Replicas: 5,
					},
					{
						Name:     "m2",
						Replicas: 5,
					},
				}
				return modified
			},
			expectedPatch: `{"spec":{"clusters":[{"name":"m1","replicas":5},{"name":"m2","replicas":5}],"replicas":10}}`,
			expectErr:     false,
		},
		{
			name: "update status",
			modifyFunc: func() interface{} {
				modified := testObj.DeepCopy()
				modified.Status.SchedulerObservedGeneration = 10
				modified.Status.Conditions = []metav1.Condition{
					{
						Type:   "Scheduled",
						Reason: "BindingScheduled",
					},
					{
						Type:   "Dummy",
						Reason: "Dummy",
					},
				}
				return modified
			},
			expectedPatch: `{"status":{"conditions":[{"lastTransitionTime":null,"message":"","reason":"BindingScheduled","status":"","type":"Scheduled"},{"lastTransitionTime":null,"message":"","reason":"Dummy","status":"","type":"Dummy"}],"schedulerObservedGeneration":10}}`,
			expectErr:     false,
		},
		{
			name: "no change",
			modifyFunc: func() interface{} {
				modified := testObj.DeepCopy()
				return modified
			},
			expectedPatch: "",
			expectErr:     false,
		},
		{
			name: "invalid input should arise error",
			modifyFunc: func() interface{} {
				var invalid = 0
				return invalid
			},
			expectedPatch: "",
			expectErr:     true,
		},
		{
			name: "update to empty annotations",
			modifyFunc: func() interface{} {
				modified := testObj.DeepCopy()
				modified.Annotations = make(map[string]string, 0)
				return modified
			},
			expectedPatch: "",
			expectErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := GenMergePatch(testObj, tt.modifyFunc())
			if err != nil && tt.expectErr == false {
				t.Fatalf("unexpect error, but got: %v", err)
			} else if err == nil && tt.expectErr == true {
				t.Fatalf("expect error, but got none")
			}
			if string(patch) != tt.expectedPatch {
				t.Fatalf("want patch: %s, but got :%s", tt.expectedPatch, string(patch))
			}
		})
	}
}
