package util

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestMergeAnnotation(t *testing.T) {
	tests := []struct {
		name            string
		obj             *unstructured.Unstructured
		annotationKey   string
		annotationValue string
		expected        *unstructured.Unstructured
	}{
		{
			name: "nil annotations",
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
						"name":        "demo-deployment",
						"annotations": map[string]interface{}{"foo": "bar"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			annotationKey:   "foo",
			annotationValue: "bar",
		},
		{
			name: "same annotationKey",
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
			annotationKey:   "foo",
			annotationValue: "bar1",
		},
		{
			name: "new labelKey",
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
			annotationKey:   "foo1",
			annotationValue: "bar1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeAnnotation(tt.obj, tt.annotationKey, tt.annotationValue)
			if !reflect.DeepEqual(tt.obj, tt.expected) {
				t.Errorf("MergeAnnotation() = %v, want %v", tt.obj, tt.expected)
			}
		})
	}
}

func TestMergeAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		dst      *unstructured.Unstructured
		src      *unstructured.Unstructured
		expected *unstructured.Unstructured
	}{
		{
			name: "src has nil annotations",
			dst: &unstructured.Unstructured{
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
			src: &unstructured.Unstructured{
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
			name: "src has annotations",
			dst: &unstructured.Unstructured{
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
			src: &unstructured.Unstructured{
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
			name: "src and dst have the same annotation key",
			dst: &unstructured.Unstructured{
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
			src: &unstructured.Unstructured{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeAnnotations(tt.dst, tt.src)
			if !reflect.DeepEqual(tt.dst, tt.expected) {
				t.Errorf("MergeAnnotations() = %v, want %v", tt.dst, tt.expected)
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
