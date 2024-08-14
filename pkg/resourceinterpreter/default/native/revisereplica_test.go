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

package native

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestReviseDeploymentReplica(t *testing.T) {
	tests := []struct {
		name        string
		object      *unstructured.Unstructured
		replica     int64
		expected    *unstructured.Unstructured
		expectError bool
	}{
		{
			name: "Deployment .spec.replicas accessor error, expected int64",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": 1,
					},
				},
			},
			replica:     3,
			expectError: true,
		},
		{
			name: "revise deployment replica",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			replica: 3,
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			expectError: false,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res, err := reviseDeploymentReplica(tt.object, tt.replica)
			if err == nil && tt.expectError == true {
				t.Fatal("expect an error but got none")
			}
			if err != nil && tt.expectError != true {
				t.Fatalf("expect no error but got: %v", err)
			}
			if err == nil && tt.expectError == false {
				if !reflect.DeepEqual(res, tt.expected) {
					t.Errorf("reviseDeploymentReplica() = %v, want %v", res, tt.expected)
				}
			}
		})
	}
}

func TestReviseJobReplica(t *testing.T) {
	tests := []struct {
		name        string
		object      *unstructured.Unstructured
		replica     int64
		expected    *unstructured.Unstructured
		expectError bool
	}{
		{
			name: "Job .spec.parallelism accessor error, expected int64",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name": "fake-job",
					},
					"spec": map[string]interface{}{
						"parallelism": 1,
					},
				},
			},
			replica:     3,
			expectError: true,
		},
		{
			name: "revise job parallelism",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name": "fake-job",
					},
					"spec": map[string]interface{}{
						"parallelism": int64(1),
					},
				},
			},
			replica: 3,
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name": "fake-job",
					},
					"spec": map[string]interface{}{
						"parallelism": int64(3),
					},
				},
			},
			expectError: false,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res, err := reviseJobReplica(tt.object, tt.replica)
			if err == nil && tt.expectError == true {
				t.Fatal("expect an error but got none")
			}
			if err != nil && tt.expectError != true {
				t.Fatalf("expect no error but got: %v", err)
			}
			if err == nil && tt.expectError == false {
				if !reflect.DeepEqual(res, tt.expected) {
					t.Errorf("reviseJobReplica() = %v, want %v", res, tt.expected)
				}
			}
		})
	}
}

func TestReviseStatefulSetReplica(t *testing.T) {
	tests := []struct {
		name        string
		object      *unstructured.Unstructured
		replica     int64
		expected    *unstructured.Unstructured
		expectError bool
	}{
		{
			name: "StatefulSet .spec.replicas accessor error, expected int64",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name": "fake-statefulset",
					},
					"spec": map[string]interface{}{
						"replicas": 1,
					},
				},
			},
			replica:     3,
			expectError: true,
		},
		{
			name: "revise statefulset replica",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name": "fake-statefulset",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			replica: 3,
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name": "fake-statefulset",
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := reviseStatefulSetReplica(tt.object, tt.replica)
			if err == nil && tt.expectError == true {
				t.Fatal("expect an error but got none")
			}
			if err != nil && tt.expectError != true {
				t.Fatalf("expect no error but got: %v", err)
			}
			if err == nil && tt.expectError == false {
				if !reflect.DeepEqual(res, tt.expected) {
					t.Errorf("reviseStatefulSetReplica() = %v, want %v", res, tt.expected)
				}
			}
		})
	}
}

func TestGetAllDefaultReviseReplicaInterpreter(t *testing.T) {
	expectedKinds := []schema.GroupVersionKind{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "apps", Version: "v1", Kind: "StatefulSet"},
		{Group: "batch", Version: "v1", Kind: "Job"},
	}

	got := getAllDefaultReviseReplicaInterpreter()

	if len(got) != len(expectedKinds) {
		t.Errorf("getAllDefaultReviseReplicaInterpreter() returned map of length %v, want %v", len(got), len(expectedKinds))
	}

	for _, key := range expectedKinds {
		_, exists := got[key]
		if !exists {
			t.Errorf("getAllDefaultReviseReplicaInterpreter() missing key %v", key)
		}
	}
}
