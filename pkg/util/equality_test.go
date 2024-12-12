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
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

var unstructuredDeployment = &unstructured.Unstructured{
	Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"labels": map[string]any{
				"app": "nginx",
			},
			"name":      "nginx",
			"namespace": "default",
		},
		"spec": map[string]any{
			"selector": map[string]any{
				"app": "nginx",
			},
			"template": map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{
						"app": "nginx",
					},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "nginx",
							"image": "nginx:latest",
						},
					},
				},
			},
		},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{
					"lastTransitionTime": "2024-10-21T08:24:46Z",
					"lastUpdateTime":     "2024-10-21T08:24:46Z",
					"message":            `ReplicaSet "nginx-649577f8c7" has successfully progressed.`,
					"reason":             "NewReplicaSetAvailable",
					"status":             "True",
					"type":               "Progressing",
				},
				map[string]any{
					"lastTransitionTime": "2024-10-24T02:55:32Z",
					"lastUpdateTime":     "2024-10-24T02:55:32Z",
					"message":            "Deployment has minimum availability.",
					"reason":             "MinimumReplicasAvailable",
					"status":             "True",
					"type":               "Available",
				},
			},
			"availableReplicas":  int64(1),
			"observedGeneration": int64(1),
			"readyReplicas":      int64(1),
			"replicas":           int64(1),
			"updatedReplicas":    int64(1),
		},
	},
}

func TestRegisterEqualityCheckFunctions(t *testing.T) {
	obj := unstructuredDeployment.DeepCopy()
	tests := []struct {
		name         string
		objFn1       func() (runtime.Object, error)
		objFn2       func() (runtime.Object, error)
		addCheckFunc bool
		wantEqual    bool
		wantErr      bool
	}{
		{
			name: "without custom check functions",
			objFn1: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				unstructured.RemoveNestedField(obj.Object, "status")
				j, err := json.Marshal(obj)
				return &workv1alpha1.Work{
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: j,
									},
								},
							},
						},
					},
				}, err
			},
			objFn2: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				unstructured.RemoveNestedField(obj.Object, "status")
				j, err := obj.MarshalJSON() // will have extra '\n' at the end
				return &workv1alpha1.Work{
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: j,
									},
								},
							},
						},
					},
				}, err
			},
			addCheckFunc: false,
			wantEqual:    false,
			wantErr:      false,
		},
		{
			name: "with custom check functions",
			objFn1: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				unstructured.RemoveNestedField(obj.Object, "status")
				j, err := json.Marshal(obj)
				return &workv1alpha1.Work{
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: j,
									},
								},
							},
						},
					},
				}, err
			},
			objFn2: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				unstructured.RemoveNestedField(obj.Object, "status")
				j, err := obj.MarshalJSON() // will have extra '\n' at the end
				return &workv1alpha1.Work{
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: j,
									},
								},
							},
						},
					},
				}, err
			},
			addCheckFunc: true,
			wantEqual:    true,
			wantErr:      false,
		},
		{
			name: "return true if specs are same",
			objFn1: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				unstructured.RemoveNestedField(obj.Object, "status")
				j, err := json.Marshal(obj)
				return &workv1alpha1.Work{
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: j,
									},
								},
							},
						},
					},
				}, err
			},
			objFn2: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				unstructured.RemoveNestedField(obj.Object, "status")
				j, err := json.Marshal(obj)
				return &workv1alpha1.Work{
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: j,
									},
								},
							},
						},
					},
				}, err
			},
			addCheckFunc: true,
			wantEqual:    true,
			wantErr:      false,
		},
		{
			name: "able to notice the work spec difference",
			objFn1: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				unstructured.RemoveNestedField(obj.Object, "status")
				j, err := json.Marshal(obj)
				return &workv1alpha1.Work{
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: j,
									},
								},
							},
						},
					},
				}, err
			},
			objFn2: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				unstructured.RemoveNestedField(obj.Object, "status")
				err := unstructured.SetNestedField(obj.Object, "a", "metadata", "labels", "a")
				if err != nil {
					return nil, err
				}
				j, err := json.Marshal(obj)
				return &workv1alpha1.Work{
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: j,
									},
								},
							},
						},
					},
				}, err
			},
			addCheckFunc: true,
			wantEqual:    false,
			wantErr:      false,
		},
		{
			name: "able to notice the work status difference",
			objFn1: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				j, err := json.Marshal(obj.Object["status"])
				return &workv1alpha1.Work{
					Status: workv1alpha1.WorkStatus{
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Status: &runtime.RawExtension{
									Raw: j,
								},
							},
						},
					},
				}, err
			},
			objFn2: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				err := unstructured.SetNestedField(obj.Object, int64(5), "status", "observedGeneration")
				if err != nil {
					return nil, err
				}
				j, err := json.Marshal(obj.Object["status"])
				return &workv1alpha1.Work{
					Status: workv1alpha1.WorkStatus{
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Status: &runtime.RawExtension{
									Raw: j,
								},
							},
						},
					},
				}, err
			},
			addCheckFunc: true,
			wantEqual:    false,
			wantErr:      false,
		},
		{
			name: "able to notice the rb status difference",
			objFn1: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				j, err := json.Marshal(obj.Object["status"])
				return &workv1alpha2.ResourceBinding{
					Status: workv1alpha2.ResourceBindingStatus{
						AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
							{
								Status: &runtime.RawExtension{
									Raw: j,
								},
							},
						},
					},
				}, err
			},
			objFn2: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				err := unstructured.SetNestedField(obj.Object, int64(5), "status", "observedGeneration")
				if err != nil {
					return nil, err
				}
				j, err := json.Marshal(obj.Object["status"])
				return &workv1alpha2.ResourceBinding{
					Status: workv1alpha2.ResourceBindingStatus{
						AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
							{
								Status: &runtime.RawExtension{
									Raw: j,
								},
							},
						},
					},
				}, err
			},
			addCheckFunc: true,
			wantEqual:    false,
			wantErr:      false,
		},
		{
			name: "return true when status fields are equal",
			objFn1: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				j, err := json.Marshal(obj.Object["status"])
				return &workv1alpha2.ResourceBinding{
					Status: workv1alpha2.ResourceBindingStatus{
						AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
							{
								Status: &runtime.RawExtension{
									Raw: j,
								},
							},
						},
					},
				}, err
			},
			objFn2: func() (runtime.Object, error) {
				obj := obj.DeepCopy()
				j, err := json.Marshal(obj.Object["status"])
				return &workv1alpha2.ResourceBinding{
					Status: workv1alpha2.ResourceBindingStatus{
						AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
							{
								Status: &runtime.RawExtension{
									Raw: j,
								},
							},
						},
					},
				}, err
			},
			addCheckFunc: true,
			wantEqual:    true,
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := equality.Semantic.Copy()
			if tt.addCheckFunc {
				if err := RegisterEqualityCheckFunctions(&checker); (err != nil) != tt.wantErr {
					t.Fatalf("RegisterEqualityCheckFunctions() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
			obj1, err1 := tt.objFn1()
			obj2, err2 := tt.objFn2()
			if (err1 != nil || err2 != nil) != tt.wantErr {
				t.Fatalf("TestRegisterEqualityCheckFunctions err1: %v, err2: %v, wantErr: %v", err1, err2, tt.wantErr)
			}
			if equal := checker.DeepEqual(obj1, obj2); equal != tt.wantEqual {
				t.Errorf("TestRegisterEqualityCheckFunctions DeepEqual() = %v, want %v", equal, tt.wantEqual)
			}
		})
	}
}

func Test_parseRawExtension(t *testing.T) {
	obj := unstructuredDeployment.DeepCopy()

	tests := []struct {
		name     string
		argsFunc func() (param runtime.RawExtension, want map[string]any, err error)
		wantErr  bool
	}{
		{
			name: "should able to parse status",
			argsFunc: func() (runtime.RawExtension, map[string]any, error) {
				status, ok := obj.Object["status"].(map[string]any)
				if !ok {
					return runtime.RawExtension{}, nil, fmt.Errorf("failed to convert status to map[string]any")
				}
				j, err := json.Marshal(status)
				return runtime.RawExtension{Raw: j}, status, err
			},
			wantErr: false,
		},
		{
			name: "should able to parse normal object",
			argsFunc: func() (runtime.RawExtension, map[string]any, error) {
				j, err := json.Marshal(obj)
				return runtime.RawExtension{Raw: j}, obj.Object, err
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extension, want, err := tt.argsFunc()
			if err != nil {
				t.Fatalf("parseRawExtension() get args fail, error = %v,", err)
			}
			got, err := parseRawExtension(extension)
			if err != nil != tt.wantErr {
				t.Fatalf("parseRawExtension() error = %v, wantErr %v", err, tt.wantErr)
			}
			// unmarshalled JSON won't equal to the original object because the number type will be different
			// so we marshal them to JSON and compare
			gotJ, gotErr := json.Marshal(got)
			wantJ, wantErr := json.Marshal(want)
			if gotErr != nil || wantErr != nil {
				t.Fatalf("parseRawExtension() marshal JSON fail, gotErr = %v, wantErr %v", wantJ, wantErr)
			}
			if !reflect.DeepEqual(gotJ, wantJ) {
				t.Errorf("parseRawExtension() = %v, want %v", gotJ, wantJ)
			}
		})
	}
}
