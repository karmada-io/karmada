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

func TestRetainAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		desired  *unstructured.Unstructured
		observed *unstructured.Unstructured
		expected *unstructured.Unstructured
	}{
		{
			name: "observed has nil annotations",
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
			name: "observed has annotations",
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
						"name":        "demo-deployment-1",
						"annotations": map[string]interface{}{"foo": "bar"},
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
						"annotations": map[string]interface{}{"foo": "bar"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
		},
		{
			name: "observed and desired have the same annotation key",
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "demo-deployment",
						"annotations": map[string]interface{}{"foo": "foo"},
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
						"annotations": map[string]interface{}{"foo": "bar"},
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
						"annotations": map[string]interface{}{"foo": "foo"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
		},
		{
			name: "do not merge deleted annotations",
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "demo-deployment",
						"annotations": map[string]interface{}{workv1alpha2.ManagedAnnotation: "karmada.io/annotations-managed-keyset"},
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
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: "karmada.io/annotations-managed-keyset,deleted",
							"deleted":                      "deleted",
							"retain":                       "retain",
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
						"name": "demo-deployment",
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: "karmada.io/annotations-managed-keyset",
							"retain":                       "retain",
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
			RetainAnnotations(tt.desired, tt.observed)
			if !reflect.DeepEqual(tt.desired, tt.expected) {
				t.Errorf("RetainAnnotations() = %v, want %v", tt.desired, tt.expected)
			}
		})
	}
}

func TestGetAnnotationValue(t *testing.T) {
	tests := []struct {
		name          string
		annotations   map[string]string
		annotationKey string
		expected      string
	}{
		{
			name:          "nil annotations",
			annotationKey: "foo",
			expected:      "",
		},
		{
			name:          "annotationKey is not exist",
			annotations:   map[string]string{"foo": "bar"},
			annotationKey: "foo1",
			expected:      "",
		},
		{
			name:          "existed annotationKey",
			annotations:   map[string]string{"foo": "bar"},
			annotationKey: "foo",
			expected:      "bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := GetAnnotationValue(tt.annotations, tt.annotationKey)
			if res != tt.expected {
				t.Errorf("MergeAnnotations() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestRecordManagedAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		object   *unstructured.Unstructured
		expected *unstructured.Unstructured
	}{
		{
			name: "nil annotation",
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
							workv1alpha2.ManagedAnnotation: "resourcetemplate.karmada.io/managed-annotations,resourcetemplate.karmada.io/managed-labels",
						},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
		},
		{
			name: "object has has annotations",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment-1",
						"annotations": map[string]interface{}{
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
							workv1alpha2.ManagedAnnotation: "foo,resourcetemplate.karmada.io/managed-annotations,resourcetemplate.karmada.io/managed-labels",
							"foo":                          "foo",
						},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
		},
		{
			name: "object has has annotations and labels",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment-1",
						"annotations": map[string]interface{}{
							"foo": "foo",
						},
						"labels": map[string]interface{}{
							"bar": "bar",
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
							workv1alpha2.ManagedAnnotation: "foo,resourcetemplate.karmada.io/managed-annotations,resourcetemplate.karmada.io/managed-labels",
							"foo":                          "foo",
						},
						"labels": map[string]interface{}{
							"bar": "bar",
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
			RecordManagedAnnotations(tt.object)
			if !reflect.DeepEqual(tt.object, tt.expected) {
				t.Errorf("RecordManagedAnnotations() = %v, want %v", tt.object, tt.expected)
			}
		})
	}
}

func TestMergeAnnotation(t *testing.T) {
	workload := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "demo-deployment",
			},
		},
	}
	workloadExistKey := workload.DeepCopy()
	workloadExistKey.SetAnnotations(map[string]string{"testKey": "oldValue"})
	workloadNotExistKey := workload.DeepCopy()
	workloadNotExistKey.SetAnnotations(map[string]string{"anotherKey": "anotherValue"})

	tests := []struct {
		name            string
		obj             *unstructured.Unstructured
		annotationKey   string
		annotationValue string
		want            map[string]string
	}{
		{
			name:            "nil annotation",
			obj:             &workload,
			annotationKey:   "testKey",
			annotationValue: "newValue",
			want:            map[string]string{"testKey": "newValue"},
		},
		{
			name:            "exist key",
			obj:             workloadExistKey,
			annotationKey:   "testKey",
			annotationValue: "newValue",
			want:            map[string]string{"testKey": "newValue"},
		},
		{
			name:            "not exist key",
			obj:             workloadNotExistKey,
			annotationKey:   "testKey",
			annotationValue: "newValue",
			want:            map[string]string{"anotherKey": "anotherValue", "testKey": "newValue"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeAnnotation(tt.obj, tt.annotationKey, tt.annotationValue)
			if !reflect.DeepEqual(tt.obj.GetAnnotations(), tt.want) {
				t.Errorf("MergeAnnotation(), obj.GetAnnotations = %v, want %v", tt.obj.GetAnnotations(), tt.want)
			}
		})
	}
}
