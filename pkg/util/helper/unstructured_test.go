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

package helper

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
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
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
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
