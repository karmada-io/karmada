/*
Copyright 2024 The Kubernetes Authors.

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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGetPodsLabelSet(t *testing.T) {
	tests := []struct {
		name     string
		template *corev1.PodTemplateSpec
		expected labels.Set
	}{
		{
			name: "Empty labels",
			template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expected: labels.Set{},
		},
		{
			name: "With labels",
			template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
						"env": "prod",
					},
				},
			},
			expected: labels.Set{
				"app": "test",
				"env": "prod",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPodsLabelSet(tt.template)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPodsFinalizers(t *testing.T) {
	tests := []struct {
		name     string
		template *corev1.PodTemplateSpec
		expected []string
	}{
		{
			name: "No finalizers",
			template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expected: []string{},
		},
		{
			name: "With finalizers",
			template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"finalizer1", "finalizer2"},
				},
			},
			expected: []string{"finalizer1", "finalizer2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPodsFinalizers(tt.template)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPodsAnnotationSet(t *testing.T) {
	tests := []struct {
		name     string
		template *corev1.PodTemplateSpec
		expected labels.Set
	}{
		{
			name: "Empty annotations",
			template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expected: labels.Set{},
		},
		{
			name: "With annotations",
			template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				},
			},
			expected: labels.Set{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPodsAnnotationSet(tt.template)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPodsPrefix(t *testing.T) {
	tests := []struct {
		name           string
		controllerName string
		expected       string
	}{
		{
			name:           "Short name",
			controllerName: "test",
			expected:       "test-",
		},
		{
			name:           "Long name",
			controllerName: "very-long-controller-name-that-exceeds-the-limit",
			expected:       "very-long-controller-name-that-exceeds-the-limit-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPodsPrefix(tt.controllerName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPodFromTemplate(t *testing.T) {
	tests := []struct {
		name           string
		template       *corev1.PodTemplateSpec
		parentObject   runtime.Object
		controllerRef  *metav1.OwnerReference
		expectedError  bool
		expectedErrMsg string
		validateResult func(*testing.T, *corev1.Pod)
	}{
		{
			name: "Valid template",
			template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": "test"},
					Annotations: map[string]string{"key": "value"},
					Finalizers:  []string{"finalizer1"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test-container", Image: "test-image"},
					},
				},
			},
			parentObject: &corev1.ReplicationController{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent",
					Namespace: "default",
				},
			},
			controllerRef: &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "ReplicationController",
				Name:       "parent",
				UID:        "test-uid",
			},
			expectedError: false,
			validateResult: func(t *testing.T, pod *corev1.Pod) {
				assert.Equal(t, "default", pod.Namespace)
				assert.Equal(t, "parent-", pod.GenerateName)
				assert.Equal(t, map[string]string{"app": "test"}, pod.Labels)
				assert.Equal(t, map[string]string{"key": "value"}, pod.Annotations)
				assert.Equal(t, []string{"finalizer1"}, pod.Finalizers)
				assert.Len(t, pod.OwnerReferences, 1)
				assert.Equal(t, "parent", pod.OwnerReferences[0].Name)
				assert.Len(t, pod.Spec.Containers, 1)
				assert.Equal(t, "test-container", pod.Spec.Containers[0].Name)
			},
		},
		{
			name: "Parent object without name",
			template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test-container", Image: "test-image"},
					},
				},
			},
			parentObject: &corev1.ReplicationController{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
			},
			controllerRef: nil,
			expectedError: false,
			validateResult: func(t *testing.T, pod *corev1.Pod) {
				assert.Equal(t, "default", pod.Namespace)
				assert.Empty(t, pod.GenerateName)
				assert.Empty(t, pod.Labels)
				assert.Empty(t, pod.Annotations)
				assert.Empty(t, pod.Finalizers)
				assert.Empty(t, pod.OwnerReferences)
				assert.Len(t, pod.Spec.Containers, 1)
				assert.Equal(t, "test-container", pod.Spec.Containers[0].Name)
			},
		},
		{
			name:           "Parent object without ObjectMeta",
			template:       &corev1.PodTemplateSpec{},
			parentObject:   &struct{ runtime.Object }{},
			controllerRef:  nil,
			expectedError:  true,
			expectedErrMsg: "parentObject does not have ObjectMeta",
			validateResult: func(t *testing.T, pod *corev1.Pod) {
				assert.Nil(t, pod)
			},
		},
		{
			name:     "Empty template",
			template: &corev1.PodTemplateSpec{},
			parentObject: &corev1.ReplicationController{
				ObjectMeta: metav1.ObjectMeta{Name: "parent", Namespace: "default"},
			},
			controllerRef: nil,
			expectedError: false,
			validateResult: func(t *testing.T, pod *corev1.Pod) {
				assert.NotNil(t, pod)
				assert.Equal(t, "default", pod.Namespace)
				assert.Equal(t, "parent-", pod.GenerateName)
				assert.Empty(t, pod.Labels)
				assert.Empty(t, pod.Annotations)
				assert.Empty(t, pod.Finalizers)
				assert.Empty(t, pod.OwnerReferences)
				assert.Empty(t, pod.Spec.Containers)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetPodFromTemplate(tt.template, tt.parentObject, tt.controllerRef)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validateResult != nil {
					tt.validateResult(t, result)
				}
			}
		})
	}
}
