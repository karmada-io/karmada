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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGenEventRef(t *testing.T) {
	tests := []struct {
		name    string
		obj     *unstructured.Unstructured
		want    *corev1.ObjectReference
		wantErr bool
	}{
		{
			name: "has metadata.uid",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment",
						"uid":  "9249d2e7-3169-4c5f-be82-163bd80aa3cf",
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			want: &corev1.ObjectReference{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
				Name:       "demo-deployment",
				UID:        "9249d2e7-3169-4c5f-be82-163bd80aa3cf",
			},
			wantErr: false,
		},
		{
			name: "missing metadata.uid but has resourcetemplate.karmada.io/uid annotation",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "demo-deployment",
						"annotations": map[string]interface{}{"resourcetemplate.karmada.io/uid": "9249d2e7-3169-4c5f-be82-163bd80aa3cf"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			want: &corev1.ObjectReference{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
				Name:       "demo-deployment",
				UID:        "9249d2e7-3169-4c5f-be82-163bd80aa3cf",
			},
			wantErr: false,
		},
		{
			name: "missing metadata.uid and metadata.annotations",
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
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := GenEventRef(tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenEventRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(actual, tt.want) {
				t.Errorf("GenEventRef() = %v, want %v", actual, tt.want)
			}
		})
	}
}
