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
			name: "object has annotations",
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
			name: "object has annotations and labels",
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
		{
			name: "object has recorded annotations and labels",
			object: &unstructured.Unstructured{
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

func TestDedupeAndMergeAnnotations(t *testing.T) {
	tests := []struct {
		name               string
		existAnnotation    map[string]string
		newAnnotation      map[string]string
		expectedAnnotation map[string]string
	}{
		{
			name:               "nil both annotation",
			existAnnotation:    nil,
			newAnnotation:      nil,
			expectedAnnotation: nil,
		},
		{
			name:            "nil existing annotation",
			existAnnotation: nil,
			newAnnotation: map[string]string{
				"newAnnotationKey": "newAnnotationValues",
			},
			expectedAnnotation: map[string]string{
				"newAnnotationKey": "newAnnotationValues",
			},
		},
		{
			name: "nil new annotation",
			existAnnotation: map[string]string{
				"existAnnotationKey": "existAnnotationValues",
			},
			newAnnotation: nil,
			expectedAnnotation: map[string]string{
				"existAnnotationKey": "existAnnotationValues",
			},
		},
		{
			name: "same annotation",
			existAnnotation: map[string]string{
				"existAnnotationKey": "existAnnotationValues",
			},
			newAnnotation: map[string]string{
				"existAnnotationKey": "existAnnotationValues",
			},
			expectedAnnotation: map[string]string{
				"existAnnotationKey": "existAnnotationValues",
			},
		},
		{
			name: "different annotation",
			existAnnotation: map[string]string{
				"existAnnotationKey": "existAnnotationValues",
			},
			newAnnotation: map[string]string{
				"newAnnotationKey": "newAnnotationValues",
			},
			expectedAnnotation: map[string]string{
				"existAnnotationKey": "existAnnotationValues",
				"newAnnotationKey":   "newAnnotationValues",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.existAnnotation = DedupeAndMergeAnnotations(tt.existAnnotation, tt.newAnnotation)
			if !reflect.DeepEqual(tt.existAnnotation, tt.expectedAnnotation) {
				t.Errorf("DedupeAndMergeAnnotations(), existAnnotation = %v, want %v", tt.existAnnotation, tt.expectedAnnotation)
			}
		})
	}
}

func TestRemoveAnnotations(t *testing.T) {
	type args struct {
		obj  *unstructured.Unstructured
		keys []string
	}
	tests := []struct {
		name     string
		args     args
		expected *unstructured.Unstructured
	}{
		{
			name: "empty keys",
			args: args{
				obj: &unstructured.Unstructured{
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
				keys: []string{},
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
			name: "nil object annotations",
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
						},
					},
				},
				keys: []string{"foo"},
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
			name: "same keys",
			args: args{
				obj: &unstructured.Unstructured{
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
				keys: []string{"foo"},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "demo-deployment",
						"annotations": map[string]interface{}{},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
		},
		{
			name: "different keys",
			args: args{
				obj: &unstructured.Unstructured{
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
				keys: []string{"foo1"},
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
			name: "same keys of different length",
			args: args{
				obj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":        "demo-deployment",
							"annotations": map[string]interface{}{"foo": "bar", "foo1": "bar1"},
						},
						"spec": map[string]interface{}{
							"replicas": 2,
						},
					},
				},
				keys: []string{"foo", "foo1"},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "demo-deployment",
						"annotations": map[string]interface{}{},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
		},
		{
			name: "different keys of different length",
			args: args{
				obj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":        "demo-deployment",
							"annotations": map[string]interface{}{"foo": "bar", "foo1": "bar1"},
						},
						"spec": map[string]interface{}{
							"replicas": 2,
						},
					},
				},
				keys: []string{"foo2", "foo3"},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "demo-deployment",
						"annotations": map[string]interface{}{"foo": "bar", "foo1": "bar1"},
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
			RemoveAnnotations(tt.args.obj, tt.args.keys...)
			if !reflect.DeepEqual(tt.args.obj, tt.expected) {
				t.Errorf("RemoveAnnotations(), tt.args.obj = %v, want %v", tt.args.obj, tt.expected)
			}
		})
	}
}
