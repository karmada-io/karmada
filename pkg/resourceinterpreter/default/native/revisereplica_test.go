package native

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
