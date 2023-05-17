package lifted

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestObjectVersion(t *testing.T) {
	t.Run("have generation", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"generation": int64(1),
				},
			},
		}
		res := ObjectVersion(obj)
		expect := fmt.Sprintf("%s%d", generationPrefix, 1)
		if res != expect {
			t.Errorf("expect %v, but got %v", expect, res)
		}
	})

	t.Run("don't have generation", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"resourceVersion": "version",
				},
			},
		}
		res := ObjectVersion(obj)
		expect := fmt.Sprintf("%s%s", resourceVersionPrefix, "version")
		if res != expect {
			t.Errorf("expect %v, but got %v", expect, res)
		}
	})
}

func TestObjectMetaObjEquivalent(t *testing.T) {
	tests := []struct {
		name   string
		a      *unstructured.Unstructured
		b      *unstructured.Unstructured
		expect bool
	}{
		{
			name: "name not equal",
			a: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "a",
					},
				},
			},
			b: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "b",
					},
				},
			},
			expect: false,
		},
		{
			name: "namespace not equal",
			a: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "a",
					},
				},
			},
			b: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "b",
					},
				},
			},
			expect: false,
		},
		{
			name: "label not equal",
			a: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{"a": "b"},
					},
				},
			},
			b: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{"c": "d"},
					},
				},
			},
			expect: false,
		},
		{
			name: "everything is the same",
			a: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "a",
						"namespace": "a",
						"labels":    "a",
					},
				},
			},
			b: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "a",
						"namespace": "a",
						"labels":    "a",
					},
				},
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := objectMetaObjEquivalent(tt.a, tt.b)
			if actual != tt.expect {
				t.Errorf("expect %v but got %v", tt.expect, actual)
			}
		})
	}
}

func TestObjectNeedsUpdate(t *testing.T) {
	clusterObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"generation": int64(1),
			},
		},
	}
	desiredObj := clusterObj
	recordedVersion := fmt.Sprintf("%s%d", generationPrefix, 2)
	actual := ObjectNeedsUpdate(desiredObj, clusterObj, recordedVersion)
	if actual != true {
		t.Errorf("expect %v, but got %v", true, actual)
	}
}
