/*
Copyright 2025 The Karmada Authors.

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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
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
		{
			name: "empty kind",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"metadata": map[string]interface{}{
						"name": "test-obj",
						"uid":  "test-uid",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty name",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"uid": "test-uid",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing uid but has annotation",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "test-pod",
						"annotations": map[string]interface{}{
							workv1alpha2.ResourceTemplateUIDAnnotation: "annotation-uid",
						},
					},
				},
			},
			want: &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "test-pod",
				UID:        "annotation-uid",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := GenEventRef(tt.obj)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, actual)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, actual)
			}
		})
	}
}

func TestIsWorkContains(t *testing.T) {
	deployment := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
		},
	}
	deploymentData, _ := deployment.MarshalJSON()

	service := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
		},
	}
	serviceData, _ := service.MarshalJSON()

	tests := []struct {
		name           string
		manifests      []workv1alpha1.Manifest
		targetResource schema.GroupVersionKind
		want           bool
	}{
		{
			name: "resource exists in manifests",
			manifests: []workv1alpha1.Manifest{
				{RawExtension: runtime.RawExtension{Raw: deploymentData}},
				{RawExtension: runtime.RawExtension{Raw: serviceData}},
			},
			targetResource: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			want:           true,
		},
		{
			name: "resource does not exist in manifests",
			manifests: []workv1alpha1.Manifest{
				{RawExtension: runtime.RawExtension{Raw: serviceData}},
			},
			targetResource: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWorkContains(tt.manifests, tt.targetResource)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsWorkSuspendDispatching(t *testing.T) {
	tests := []struct {
		name string
		work *workv1alpha1.Work
		want bool
	}{
		{
			name: "dispatching is suspended",
			work: &workv1alpha1.Work{
				Spec: workv1alpha1.WorkSpec{
					SuspendDispatching: ptr.To(true),
				},
			},
			want: true,
		},
		{
			name: "dispatching is not suspended",
			work: &workv1alpha1.Work{
				Spec: workv1alpha1.WorkSpec{
					SuspendDispatching: ptr.To(false),
				},
			},
			want: false,
		},
		{
			name: "suspend dispatching is nil",
			work: &workv1alpha1.Work{
				Spec: workv1alpha1.WorkSpec{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWorkSuspendDispatching(tt.work)
			assert.Equal(t, tt.want, got)
		})
	}
}
