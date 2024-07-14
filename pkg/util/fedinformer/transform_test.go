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

package fedinformer

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

func TestStripUnusedFields(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want interface{}
	}{
		{
			name: "transform pods",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "foo",
					Name:        "bar",
					Labels:      map[string]string{"a": "b"},
					Annotations: map[string]string{"c": "d"},
					ManagedFields: []metav1.ManagedFieldsEntry{
						{
							Manager: "whatever",
						},
					},
				},
			},
			want: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "foo",
					Name:        "bar",
					Labels:      map[string]string{"a": "b"},
					Annotations: map[string]string{"c": "d"},
				},
			},
		},
		{
			name: "transform works",
			obj: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "foo",
					Name:        "bar",
					Labels:      map[string]string{"a": "b"},
					Annotations: map[string]string{"c": "d"},
					ManagedFields: []metav1.ManagedFieldsEntry{
						{
							Manager: "whatever",
						},
					},
				},
			},
			want: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "foo",
					Name:        "bar",
					Labels:      map[string]string{"a": "b"},
					Annotations: map[string]string{"c": "d"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := StripUnusedFields(tt.obj)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StripUnusedFields: got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeTransformFunc(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want interface{}
	}{
		{
			name: "transform nodes without status",
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "foo",
					Labels:      map[string]string{"a": "b"},
					Annotations: map[string]string{"c": "d"},
					ManagedFields: []metav1.ManagedFieldsEntry{
						{
							Manager: "whatever",
						},
					},
				},
			},
			want: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			},
		},
		{
			name: "transform nodes with status",
			obj: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:              *resource.NewMilliQuantity(1, resource.DecimalSI),
						corev1.ResourceMemory:           *resource.NewQuantity(1, resource.BinarySI),
						corev1.ResourcePods:             *resource.NewQuantity(1, resource.DecimalSI),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(1, resource.BinarySI),
					},
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						},
						{
							Type:   corev1.NodeMemoryPressure,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:              *resource.NewMilliQuantity(1, resource.DecimalSI),
						corev1.ResourceMemory:           *resource.NewQuantity(1, resource.BinarySI),
						corev1.ResourcePods:             *resource.NewQuantity(1, resource.DecimalSI),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(1, resource.BinarySI),
					},
					Conditions: []corev1.NodeCondition{
						{
							Type:   corev1.NodeReady,
							Status: corev1.ConditionTrue,
						},
						{
							Type:   corev1.NodeMemoryPressure,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := NodeTransformFunc(tt.obj)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NodeTransformFunc: got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodTransformFunc(t *testing.T) {
	timeNow := metav1.Now()
	tests := []struct {
		name string
		obj  interface{}
		want interface{}
	}{
		{
			name: "transform pods without status",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "foo",
					Name:        "bar",
					Labels:      map[string]string{"a": "b"},
					Annotations: map[string]string{"c": "d"},
					ManagedFields: []metav1.ManagedFieldsEntry{
						{
							Manager: "whatever",
						},
					},
					DeletionTimestamp: &timeNow,
				},
			},
			want: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "foo",
					Name:              "bar",
					Labels:            map[string]string{"a": "b"},
					DeletionTimestamp: &timeNow,
				},
			},
		},
		{
			name: "transform pods with status",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
				Spec: corev1.PodSpec{
					NodeName:       "test",
					InitContainers: []corev1.Container{{Name: "test"}},
					Containers:     []corev1.Container{{Name: "test"}},
					Overhead: corev1.ResourceList{
						corev1.ResourceCPU:              *resource.NewMilliQuantity(1, resource.DecimalSI),
						corev1.ResourceMemory:           *resource.NewQuantity(1, resource.BinarySI),
						corev1.ResourcePods:             *resource.NewQuantity(1, resource.DecimalSI),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(1, resource.BinarySI),
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type: corev1.PodReady,
						},
					},
					StartTime: &timeNow,
				},
			},
			want: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
				Spec: corev1.PodSpec{
					NodeName:       "test",
					InitContainers: []corev1.Container{{Name: "test"}},
					Containers:     []corev1.Container{{Name: "test"}},
					Overhead: corev1.ResourceList{
						corev1.ResourceCPU:              *resource.NewMilliQuantity(1, resource.DecimalSI),
						corev1.ResourceMemory:           *resource.NewQuantity(1, resource.BinarySI),
						corev1.ResourcePods:             *resource.NewQuantity(1, resource.DecimalSI),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(1, resource.BinarySI),
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type: corev1.PodReady,
						},
					},
					StartTime: &timeNow,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := PodTransformFunc(tt.obj)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PodTransformFunc: got %v, want %v", got, tt.want)
			}
		})
	}
}
