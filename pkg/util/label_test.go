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

package util

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestGetLabelValue(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		labelKey string
		expected string
	}{
		{
			name:     "nil labels",
			labels:   nil,
			expected: "",
		},
		{
			name:     "empty labelKey",
			labels:   map[string]string{"foo": "bar"},
			expected: "",
		},
		{
			name:     "no exist labelKey",
			labels:   map[string]string{"foo": "bar"},
			labelKey: "foo1",
			expected: "",
		},
		{
			name:     "exist labelKey",
			labels:   map[string]string{"foo": "bar"},
			labelKey: "foo",
			expected: "bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := GetLabelValue(tt.labels, tt.labelKey)
			if res != tt.expected {
				t.Errorf("GetLabelValue() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestMergeLabel(t *testing.T) {
	tests := []struct {
		name       string
		obj        *unstructured.Unstructured
		labelKey   string
		labelValue string
		expected   *unstructured.Unstructured
	}{
		{
			name: "nil labels",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":   "demo-deployment",
						"labels": map[string]interface{}{"foo": "bar"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			labelKey:   "foo",
			labelValue: "bar",
		},
		{
			name: "same labelKey",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":   "demo-deployment",
						"labels": map[string]interface{}{"foo": "bar"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":   "demo-deployment",
						"labels": map[string]interface{}{"foo": "bar1"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			labelKey:   "foo",
			labelValue: "bar1",
		},
		{
			name: "new labelKey",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":   "demo-deployment",
						"labels": map[string]interface{}{"foo": "bar"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":   "demo-deployment",
						"labels": map[string]interface{}{"foo": "bar", "foo1": "bar1"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			labelKey:   "foo1",
			labelValue: "bar1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeLabel(tt.obj, tt.labelKey, tt.labelValue)
			if !reflect.DeepEqual(tt.obj, tt.expected) {
				t.Errorf("MergeLabel() = %v, want %v", tt.obj, tt.expected)
			}
		})
	}
}

func TestDedupeAndMergeLabels(t *testing.T) {
	tests := []struct {
		name       string
		existLabel map[string]string
		newLabel   map[string]string
		expected   map[string]string
	}{
		{
			name:       "two labels are nil",
			existLabel: nil,
			newLabel:   nil,
			expected:   nil,
		},
		{
			name:       "nil newLabel",
			existLabel: map[string]string{"foo": "bar"},
			newLabel:   nil,
			expected:   map[string]string{"foo": "bar"},
		},
		{
			name:       "nil existLabel",
			existLabel: nil,
			newLabel:   map[string]string{"foo": "bar"},
			expected:   map[string]string{"foo": "bar"},
		},
		{
			name:       "same labelKey",
			existLabel: map[string]string{"foo": "bar"},
			newLabel:   map[string]string{"foo": "bar"},
			expected:   map[string]string{"foo": "bar"},
		},
		{
			name:       "different labelKeys",
			existLabel: map[string]string{"foo": "bar"},
			newLabel:   map[string]string{"foo1": "bar1"},
			expected:   map[string]string{"foo": "bar", "foo1": "bar1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := DedupeAndMergeLabels(tt.existLabel, tt.newLabel)
			if !reflect.DeepEqual(res, tt.expected) {
				t.Errorf("DedupeAndMergeLabels() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestRemoveLabel(t *testing.T) {
	type args struct {
		obj      *unstructured.Unstructured
		labelKey string
	}
	tests := []struct {
		name     string
		args     args
		expected *unstructured.Unstructured
	}{
		{
			name: "nil object labels",
			args: args{
				obj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name": "demo-deployment",
						},
						"spec": map[string]interface{}{
							"replicas": 2,
						}}},
				labelKey: "foo",
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					}}},
		},
		{
			name: "same labelKey",
			args: args{
				obj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":   "demo-deployment",
							"labels": map[string]interface{}{"foo": "bar"},
						},
						"spec": map[string]interface{}{
							"replicas": 2,
						}}},
				labelKey: "foo",
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":   "demo-deployment",
						"labels": map[string]interface{}{},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					}}},
		},
		{
			name: "different labelKey",
			args: args{
				obj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":   "demo-deployment",
							"labels": map[string]interface{}{"foo": "bar"},
						},
						"spec": map[string]interface{}{
							"replicas": 2,
						}}},
				labelKey: "foo1",
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":   "demo-deployment",
						"labels": map[string]interface{}{"foo": "bar"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RemoveLabels(tt.args.obj, tt.args.labelKey)
			if !reflect.DeepEqual(tt.args.obj, tt.expected) {
				t.Errorf("RemoveLabel() = %v, want %v", tt.args.obj, tt.expected)
			}
		})
	}
}

func TestRetainLabels(t *testing.T) {
	tests := []struct {
		name     string
		desired  *unstructured.Unstructured
		observed *unstructured.Unstructured
		expected *unstructured.Unstructured
	}{
		{
			name: "observed has nil labels",
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			observed: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment-1",
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
		},
		{
			name: "observed has labels",
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			observed: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":   "demo-deployment-1",
						"labels": map[string]interface{}{"foo": "bar"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":   "demo-deployment",
						"labels": map[string]interface{}{"foo": "bar"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
		},
		{
			name: "observed and desired have the same label key",
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":   "demo-deployment",
						"labels": map[string]interface{}{"foo": "foo"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			observed: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":   "demo-deployment-1",
						"labels": map[string]interface{}{"foo": "bar"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":   "demo-deployment",
						"labels": map[string]interface{}{"foo": "foo"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
		},
		{
			name: "do not merge deleted labels",
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "demo-deployment",
						"annotations": map[string]interface{}{workv1alpha2.ManagedLabels: "foo"},
						"labels":      map[string]interface{}{"foo": "foo"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			observed: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "demo-deployment-1",
						"annotations": map[string]interface{}{workv1alpha2.ManagedLabels: "foo,deleted"},
						"labels": map[string]interface{}{
							"foo":     "bar",
							"deleted": "deleted",
							"retain":  "retain",
						},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "demo-deployment",
						"annotations": map[string]interface{}{workv1alpha2.ManagedLabels: "foo"},
						"labels": map[string]interface{}{
							"foo":    "foo",
							"retain": "retain",
						},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RetainLabels(tt.desired, tt.observed)
			if !reflect.DeepEqual(tt.desired, tt.expected) {
				t.Errorf("RetainLabels() = %v, want %v", tt.desired, tt.expected)
			}
		})
	}
}

func TestRecordManagedLabels(t *testing.T) {
	tests := []struct {
		name     string
		object   *unstructured.Unstructured
		expected *unstructured.Unstructured
	}{
		{
			name: "nil label",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment-1",
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment-1",
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedLabels: "",
						},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
		},
		{
			name: "object has has labels",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment-1",
						"labels": map[string]interface{}{
							"foo": "foo",
						},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment-1",
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedLabels: "foo",
						},
						"labels": map[string]interface{}{
							"foo": "foo",
						},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordManagedLabels(tt.object)
			if !reflect.DeepEqual(tt.object, tt.expected) {
				t.Errorf("RecordManagedLabels() = %v, want %v", tt.object, tt.expected)
			}
		})
	}
}
