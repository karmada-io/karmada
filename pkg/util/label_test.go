package util

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
			RemoveLabel(tt.args.obj, tt.args.labelKey)
			if !reflect.DeepEqual(tt.args.obj, tt.expected) {
				t.Errorf("RemoveLabel() = %v, want %v", tt.args.obj, tt.expected)
			}
		})
	}
}

func TestMergeLabels(t *testing.T) {
	tests := []struct {
		name     string
		dst      *unstructured.Unstructured
		src      *unstructured.Unstructured
		expected *unstructured.Unstructured
	}{
		{
			name: "src has nil labels",
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
			name: "src has labels",
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
			name: "src and dst have the same label key",
			dst: &unstructured.Unstructured{
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
			src: &unstructured.Unstructured{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeLabels(tt.dst, tt.src)
			if !reflect.DeepEqual(tt.dst, tt.expected) {
				t.Errorf("MergeLabels() = %v, want %v", tt.dst, tt.expected)
			}
		})
	}
}
