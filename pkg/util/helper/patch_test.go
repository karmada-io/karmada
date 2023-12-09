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
	"encoding/json"
	"fmt"
	"math"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
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

func TestGenJSONPatch(t *testing.T) {
	type args struct {
		op    string
		from  string
		path  string
		value interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "add object field",
			args: args{
				op:    "add",
				path:  "/abc",
				value: 1,
			},
			want:    `[{"op":"add","path":"/abc","value":1}]`,
			wantErr: false,
		},
		{
			name: "replace object field",
			args: args{
				op:    "replace",
				path:  "/abc",
				value: 1,
			},
			want:    `[{"op":"replace","path":"/abc","value":1}]`,
			wantErr: false,
		},
		{
			name: "remove object field, redundant args will be ignored",
			args: args{
				op:    "remove",
				from:  "123",
				path:  "/abc",
				value: 1,
			},
			want:    `[{"op":"remove","path":"/abc"}]`,
			wantErr: false,
		},
		{
			name: "move object field",
			args: args{
				op:   "move",
				from: "/abc",
				path: "/123",
			},
			want:    `[{"op":"move","from":"/abc","path":"/123"}]`,
			wantErr: false,
		},
		{
			name: "copy object field, redundant array value will be ignored",
			args: args{
				op:    "copy",
				from:  "/123",
				path:  "/abc",
				value: []interface{}{1, "a", false, 4.5},
			},
			want:    `[{"op":"copy","from":"/123","path":"/abc"}]`,
			wantErr: false,
		},
		{
			name: "replace object field, input string typed number",
			args: args{
				op:    "replace",
				path:  "/abc",
				value: "1",
			},
			want:    `[{"op":"replace","path":"/abc","value":"1"}]`,
			wantErr: false,
		},
		{
			name: "replace object field, input invalid type",
			args: args{
				op:    "replace",
				path:  "/abc",
				value: make(chan int),
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "replace object field, input invalid value",
			args: args{
				op:    "replace",
				path:  "/abc",
				value: math.Inf(1),
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "replace object field, input struct value",
			args: args{
				op:   "replace",
				path: "/abc",
				value: struct {
					A string
					B int
					C float64
					D bool
				}{"a", 1, 1.2, true},
			},
			want:    `[{"op":"replace","path":"/abc","value":{"A":"a","B":1,"C":1.2,"D":true}}]`,
			wantErr: false,
		},
		{
			name: "test object field, input array value",
			args: args{
				op:    "test",
				path:  "/abc",
				value: []interface{}{1, "a", false, 4.5},
			},
			want:    `[{"op":"test","path":"/abc","value":[1,"a",false,4.5]}]`,
			wantErr: false,
		},
		{
			name: "move object field, input invalid path, but we won't verify it",
			args: args{
				op:   "move",
				from: "123",
				path: "abc",
			},
			want:    `[{"op":"move","from":"123","path":"abc"}]`,
			wantErr: false,
		},
		{
			name: "input invalid op",
			args: args{
				op:   "whatever",
				path: "/abc",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patchBytes, err := GenJSONPatch(tt.args.op, tt.args.from, tt.args.path, tt.args.value)
			if tt.wantErr != (err != nil) {
				t.Errorf("wantErr: %v, but got err: %v", tt.wantErr, err)
			}
			if tt.want != string(patchBytes) {
				t.Errorf("want: %s, but got: %s", tt.want, patchBytes)
			}
		})
	}
}

func TestGenReplaceFieldJSONPatch(t *testing.T) {
	originalObj := &appsv1.Deployment{Status: appsv1.DeploymentStatus{
		ObservedGeneration: 1,
		Replicas:           1,
		UpdatedReplicas:    1,
		ReadyReplicas:      1,
		AvailableReplicas:  1,
	}}
	newObj := originalObj.DeepCopy()
	newObj.Status = appsv1.DeploymentStatus{
		ObservedGeneration: 2,
		Replicas:           2,
		UpdatedReplicas:    2,
		ReadyReplicas:      2,
		AvailableReplicas:  2,
	}
	newStatusJSON, _ := json.Marshal(newObj.Status)
	pathStatus := "/status"
	type args struct {
		path               string
		originalFieldValue interface{}
		newFieldValue      interface{}
	}
	tests := []struct {
		name string
		args args
		want func() ([]byte, error)
	}{
		{
			name: "return nil when no patch is needed",
			args: args{
				path:               pathStatus,
				originalFieldValue: originalObj.Status,
				newFieldValue:      originalObj.Status,
			},
			want: func() ([]byte, error) {
				return nil, nil
			},
		},
		{
			name: "return add JSONPatch when field in original obj is nil",
			args: args{
				path:               pathStatus,
				originalFieldValue: nil,
				newFieldValue:      newObj.Status,
			},
			want: func() ([]byte, error) {
				return GenJSONPatch(JSONPatchOPAdd, "", pathStatus, newObj.Status)
			},
		},
		{
			name: "e2e return add JSONPatch when field in original obj is nil",
			args: args{
				path:               pathStatus,
				originalFieldValue: nil,
				newFieldValue:      newObj.Status,
			},
			want: func() ([]byte, error) {
				return []byte(fmt.Sprintf(`[{"op":"add","path":"%s","value":%s}]`, pathStatus, newStatusJSON)), nil
			},
		},
		{
			name: "return replace JSONPatch when field in original obj in non-nil, whatever what's in the original field",
			args: args{
				path:               pathStatus,
				originalFieldValue: originalObj.Status,
				newFieldValue:      newObj.Status,
			},
			want: func() ([]byte, error) {
				return GenJSONPatch(JSONPatchOPReplace, "", pathStatus, newObj.Status)
			},
		},
		{
			name: "e2e return replace JSONPatch when field in original obj in non-nil, whatever what's in the original field",
			args: args{
				path:               pathStatus,
				originalFieldValue: originalObj.Status,
				newFieldValue:      newObj.Status,
			},
			want: func() ([]byte, error) {
				return []byte(fmt.Sprintf(`[{"op":"replace","path":"%s","value":%s}]`, pathStatus, newStatusJSON)), nil
			},
		},
		{
			name: "return remove JSONPatch when field in new obj is nil",
			args: args{
				path:               pathStatus,
				originalFieldValue: originalObj.Status,
				newFieldValue:      nil,
			},
			want: func() ([]byte, error) {
				return GenJSONPatch(JSONPatchOPRemove, "", pathStatus, nil)
			},
		},
		{
			name: "e2e return remove JSONPatch when field in new obj is nil",
			args: args{
				path:               pathStatus,
				originalFieldValue: originalObj.Status,
				newFieldValue:      nil,
			},
			want: func() ([]byte, error) {
				return []byte(fmt.Sprintf(`[{"op":"remove","path":"%s"}]`, pathStatus)), nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenReplaceFieldJSONPatch(tt.args.path, tt.args.originalFieldValue, tt.args.newFieldValue)
			want, wantErr := tt.want()

			if fmt.Sprint(wantErr) != fmt.Sprint(err) {
				t.Errorf("wantErr: %s, but got err: %s", fmt.Sprint(wantErr), fmt.Sprint(err))
			}
			if string(want) != string(got) {
				t.Errorf("\nwant:    %s\nbut got: %s\n", want, got)
			}
		})
	}
}
