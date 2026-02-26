/*
Copyright 2023 The Karmada Authors.

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

package lifted

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
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
		name       string
		desiredObj *unstructured.Unstructured
		clusterObj *unstructured.Unstructured
		expect     bool
	}{
		{
			name: "name not equal",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "a",
					},
				},
			},
			clusterObj: &unstructured.Unstructured{
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
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "a",
					},
				},
			},
			clusterObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "b",
					},
				},
			},
			expect: false,
		},
		{
			name: "clusterObj annotations have updated item with record",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels, "Updated"}, ","),
							workv1alpha2.ManagedLabels:     "",
							"Updated":                      "old",
						},
					},
				},
			},
			clusterObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels, "Updated"}, ","),
							workv1alpha2.ManagedLabels:     "",
							"Updated":                      "new",
						},
					},
				},
			},
			expect: false,
		},
		{
			name: "clusterObj annotations have deleted item with record",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels, "Deleted"}, ","),
							workv1alpha2.ManagedLabels:     "",
							"Deleted":                      "true",
						},
					},
				},
			},
			clusterObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels, "Deleted"}, ","),
							workv1alpha2.ManagedLabels:     "",
						},
					},
				},
			},
			expect: false,
		},
		{
			name: "clusterObj annotations have added item without record",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels}, ","),
							workv1alpha2.ManagedLabels:     "",
						},
					},
				},
			},
			clusterObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels}, ","),
							workv1alpha2.ManagedLabels:     "",
							"Added":                        "true",
						},
					},
				},
			},
			expect: true,
		},
		{
			name: "clusterObj labels have updated item with record",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels}, ","),
							workv1alpha2.ManagedLabels:     "A,Updated",
						},
						"labels": map[string]interface{}{
							"A":       "a",
							"Updated": "old",
						},
					},
				},
			},
			clusterObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels}, ","),
							workv1alpha2.ManagedLabels:     "A,Updated",
						},
						"labels": map[string]interface{}{
							"A":       "a",
							"Updated": "new",
						},
					},
				},
			},
			expect: false,
		},
		{
			name: "clusterObj labels have deleted item with record",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels}, ","),
							workv1alpha2.ManagedLabels:     "A,Deleted",
						},
						"labels": map[string]interface{}{
							"A":       "a",
							"Deleted": "true",
						},
					},
				},
			},
			clusterObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels}, ","),
							workv1alpha2.ManagedLabels:     "A,Deleted",
						},
						"labels": map[string]interface{}{
							"A": "a",
						},
					},
				},
			},
			expect: false,
		},
		{
			name: "clusterObj labels have added item without record",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels}, ","),
							workv1alpha2.ManagedLabels:     "A",
						},
						"labels": map[string]interface{}{
							"A": "a",
						},
					},
				},
			},
			clusterObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels}, ","),
							workv1alpha2.ManagedLabels:     "A",
						},
						"labels": map[string]interface{}{
							"A":     "a",
							"Added": "true",
						},
					},
				},
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := objectMetaObjEquivalent(tt.desiredObj, tt.clusterObj)
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

func Test_propagateAnnotationsChanged(t *testing.T) {
	tests := []struct {
		name        string
		desireMaps  map[string]string
		clusterMaps map[string]string
		want        bool
	}{
		{
			name:       "nil desire annotations",
			desireMaps: nil,
			clusterMaps: map[string]string{
				"A": "desiredObj",
			},
			want: false,
		},
		{
			name: "nil cluster annotations",
			desireMaps: map[string]string{
				workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation}, ","),
			},
			clusterMaps: nil,
			want:        true,
		},
		{
			name: "desire annotations have deleted item",
			desireMaps: map[string]string{
				workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation}, ","),
			},
			clusterMaps: map[string]string{
				workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, "Deleted"}, ","),
				"Deleted":                      "deleted in the control plane",
			},
			want: true,
		},
		{
			name: "desire annotations have item but not record in the record annotation",
			desireMaps: map[string]string{
				workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation}, ","),
				"only-exist-in-control-plane":  "true",
			},
			clusterMaps: map[string]string{
				workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation}, ","),
			},
			want: false,
		},
		{
			name: "cluster annotations have updated item",
			desireMaps: map[string]string{
				workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, "Updated"}, ","),
				"Updated":                      "control plane",
			},
			clusterMaps: map[string]string{
				workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, "Updated"}, ","),
				"Updated":                      "update in the member cluster",
			},
			want: true,
		},
		{
			name: "cluster annotations have added item",
			desireMaps: map[string]string{
				workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, "A"}, ","),
				"A":                            "desiredObj",
			},
			clusterMaps: map[string]string{
				workv1alpha2.ManagedAnnotation: strings.Join([]string{workv1alpha2.ManagedAnnotation, "A"}, ","),
				"A":                            "desiredObj",
				"Added":                        "add in the member cluster",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, propagateAnnotationsChanged(tt.desireMaps, tt.clusterMaps),
				"propagateAnnotationsChanged(%v, %v)", tt.desireMaps, tt.clusterMaps)
		})
	}
}

func Test_propagateLabelsChange(t *testing.T) {
	tests := []struct {
		name          string
		desireLabels  map[string]string
		clusterLabels map[string]string
		keys          []string
		want          bool
	}{
		{
			name:         "nil desire labels",
			desireLabels: nil,
			clusterLabels: map[string]string{
				"A": "desiredObj",
			},
			keys: nil,
			want: false,
		},
		{
			name:         "nil desire labels & nil keys",
			desireLabels: nil,
			clusterLabels: map[string]string{
				"A": "desiredObj",
			},
			keys: nil,
			want: false,
		},
		{
			name: "desire labels have deleted item",
			desireLabels: map[string]string{
				"A": "desiredObj",
			},
			clusterLabels: map[string]string{
				"A": "desiredObj",
				"B": "b",
			},
			keys: []string{"A", "B"},
			want: true,
		},
		{
			name: "desire labels have item but not record in the record annotation",
			desireLabels: map[string]string{
				"A": "desiredObj",
				"B": "b",
			},
			clusterLabels: map[string]string{
				"B": "b",
			},
			keys: []string{"B"},
			want: false,
		},
		{
			name: "cluster labels have updated item",
			desireLabels: map[string]string{
				"B": "b",
			},
			clusterLabels: map[string]string{
				"B": "b-update",
			},
			keys: []string{"B"},
			want: true,
		},
		{
			name: "cluster labels have added item",
			desireLabels: map[string]string{
				"B": "b",
			},
			clusterLabels: map[string]string{
				"B": "b",
				"C": "new-added",
			},
			keys: []string{"B"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, propagateLabelsChange(tt.desireLabels, tt.clusterLabels, tt.keys),
				"propagateLabelsChange(%v, %v, %v)", tt.desireLabels, tt.clusterLabels, tt.keys)
		})
	}
}
