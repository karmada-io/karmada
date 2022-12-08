package helper

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestConvertToTypedObject(t *testing.T) {
	testCases := []struct {
		name        string
		in          interface{}
		out         interface{}
		expectedErr bool
	}{
		{
			name:        "in object is nil",
			in:          nil,
			expectedErr: true,
		},
		{
			name:        "out object is nil",
			in:          &unstructured.Unstructured{},
			out:         nil,
			expectedErr: true,
		},
		{
			name:        "in object with invalid type",
			in:          "some",
			out:         &corev1.Pod{},
			expectedErr: true,
		},
		{
			name: "convert unstructured object success",
			in: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "some-pod",
					},
				},
			},
			out:         &corev1.Pod{},
			expectedErr: false,
		},
		{
			name: "convert map[string]interface{} object success",
			in: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "some-pod",
				},
			},
			out:         &corev1.Pod{},
			expectedErr: false,
		},
		{
			name: "convert map[string]interface{} object with invalid spec",
			in: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": 123,
				},
			},
			out:         &corev1.Pod{},
			expectedErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ConvertToTypedObject(tc.in, tc.out); (err != nil) != tc.expectedErr {
				t.Errorf("ConvertToTypedObject expected error %v, but got %v", tc.expectedErr, err)
			}
		})
	}
}

func TestApplyReplica(t *testing.T) {
	testCases := []struct {
		name             string
		workload         *unstructured.Unstructured
		replicas         int64
		field            string
		expectedReplicas int64
		expectedErr      bool
	}{
		{
			name: "replica with wrong type",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": "some",
					},
				},
			},
			replicas:    2,
			field:       "replicas",
			expectedErr: true,
		},
		{
			name: "replicas field is not existing",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{},
				},
			},
			replicas:         2,
			field:            "replicas",
			expectedReplicas: 0,
			expectedErr:      false,
		},
		{
			name: "apply replicas success",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			replicas:         2,
			field:            "replicas",
			expectedReplicas: 2,
			expectedErr:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ApplyReplica(tc.workload, tc.replicas, tc.field); (err != nil) != tc.expectedErr {
				t.Errorf("ApplyReplica expected error %v, but got %v", tc.expectedErr, err)
			}
			if !tc.expectedErr {
				gotReplicas, _, _ := unstructured.NestedInt64(tc.workload.Object, "spec", tc.field)
				if gotReplicas != tc.expectedReplicas {
					t.Errorf("ApplyReplica expected replicas %d, but got %d", tc.expectedReplicas, gotReplicas)
				}
			}
		})
	}
}

func TestToUnstructured(t *testing.T) {
	testCases := []struct {
		name        string
		object      interface{}
		expectedErr bool
	}{
		{
			name:        "convert invalid object",
			object:      []string{"some"},
			expectedErr: true,
		},
		{
			name: "convert unstructured object",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			expectedErr: false,
		},
		{
			name: "convert typed object",
			object: &corev1.Pod{
				Spec: corev1.PodSpec{
					NodeName: "some-node",
				},
			},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ToUnstructured(tc.object); (err != nil) != tc.expectedErr {
				t.Errorf("ApplyReplica expected error %v, but got %v", tc.expectedErr, err)
			}
		})
	}
}
