/*
Copyright 2024 The Karmada Authors.

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

package hpascaletargetmarker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// mockAsyncWorker simulates the behavior of an AsyncWorker for testing
type mockAsyncWorker struct {
	addCalled      bool
	addAfterCalled bool
	enqueueCalled  bool
	addFunc        func(interface{})
	addAfterFunc   func(interface{}, time.Duration)
	enqueueFunc    func(interface{})
}

func (m *mockAsyncWorker) Add(item interface{}) {
	m.addCalled = true
	if m.addFunc != nil {
		m.addFunc(item)
	}
}

func (m *mockAsyncWorker) AddAfter(item interface{}, duration time.Duration) {
	m.addAfterCalled = true
	if m.addAfterFunc != nil {
		m.addAfterFunc(item, duration)
	}
}

func (m *mockAsyncWorker) Enqueue(obj interface{}) {
	m.enqueueCalled = true
	if m.enqueueFunc != nil {
		m.enqueueFunc(obj)
	}
}

func (m *mockAsyncWorker) Run(_ int, _ <-chan struct{}) {}

func TestHpaScaleTargetMarkerCreate(t *testing.T) {
	tests := []struct {
		name           string
		object         client.Object
		expectAdded    bool
		expectReturned bool
	}{
		{
			name: "HPA with propagation policy",
			object: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						policyv1alpha1.PropagationPolicyPermanentIDLabel: "test-policy",
					},
				},
			},
			expectAdded:    true,
			expectReturned: false,
		},
		{
			name: "HPA without propagation policy",
			object: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expectAdded:    false,
			expectReturned: false,
		},
		{
			name:           "Non-HPA object",
			object:         &corev1.Pod{},
			expectAdded:    false,
			expectReturned: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorker := &mockAsyncWorker{}
			marker := &HpaScaleTargetMarker{
				scaleTargetWorker: mockWorker,
			}

			result := marker.Create(event.CreateEvent{Object: tt.object})

			assert.Equal(t, tt.expectReturned, result, "Unexpected return value")
			assert.Equal(t, tt.expectAdded, mockWorker.addCalled, "Unexpected worker.Add() call")
		})
	}
}

func TestHpaScaleTargetMarkerUpdate(t *testing.T) {
	tests := []struct {
		name           string
		oldObject      client.Object
		newObject      client.Object
		expectAdded    bool
		expectReturned bool
	}{
		{
			name: "Valid HPA update",
			oldObject: &autoscalingv2.HorizontalPodAutoscaler{
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Kind: "Deployment",
						Name: "old-deployment",
					},
				},
			},
			newObject: &autoscalingv2.HorizontalPodAutoscaler{
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Kind: "Deployment",
						Name: "new-deployment",
					},
				},
			},
			expectAdded:    true,
			expectReturned: false,
		},
		{
			name: "DesiredReplicas changed",
			oldObject: &autoscalingv2.HorizontalPodAutoscaler{
				Status: autoscalingv2.HorizontalPodAutoscalerStatus{
					DesiredReplicas: 1,
				},
			},
			newObject: &autoscalingv2.HorizontalPodAutoscaler{
				Status: autoscalingv2.HorizontalPodAutoscalerStatus{
					DesiredReplicas: 2,
				},
			},
			expectAdded:    false,
			expectReturned: true,
		},
		{
			name:           "Non-HPA old object",
			oldObject:      &corev1.Pod{},
			newObject:      &autoscalingv2.HorizontalPodAutoscaler{},
			expectAdded:    false,
			expectReturned: false,
		},
		{
			name:           "Non-HPA new object",
			oldObject:      &autoscalingv2.HorizontalPodAutoscaler{},
			newObject:      &corev1.Pod{},
			expectAdded:    false,
			expectReturned: false,
		},
		{
			name:           "Both objects non-HPA",
			oldObject:      &corev1.Pod{},
			newObject:      &corev1.Service{},
			expectAdded:    false,
			expectReturned: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorker := &mockAsyncWorker{}
			marker := &HpaScaleTargetMarker{
				scaleTargetWorker: mockWorker,
			}
			result := marker.Update(event.UpdateEvent{ObjectOld: tt.oldObject, ObjectNew: tt.newObject})
			assert.Equal(t, tt.expectReturned, result, "Unexpected return value")
			assert.Equal(t, tt.expectAdded, mockWorker.addCalled, "Unexpected worker.Add() call")
		})
	}
}

func TestHpaScaleTargetMarkerDelete(t *testing.T) {
	tests := []struct {
		name        string
		object      client.Object
		expectAdded bool
	}{
		{
			name: "Delete HPA",
			object: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expectAdded: true,
		},
		{
			name:        "Delete non-HPA object (error case)",
			object:      &corev1.Pod{},
			expectAdded: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorker := &mockAsyncWorker{}
			marker := &HpaScaleTargetMarker{
				scaleTargetWorker: mockWorker,
			}
			result := marker.Delete(event.DeleteEvent{Object: tt.object})
			assert.False(t, result, "Delete should always return false")
			assert.Equal(t, tt.expectAdded, mockWorker.addCalled, "Unexpected worker.Add() call")
		})
	}
}

func TestHpaScaleTargetMarkerGeneric(t *testing.T) {
	tests := []struct {
		name   string
		event  event.GenericEvent
		expect bool
	}{
		{
			name:   "Generic event with HPA object",
			event:  event.GenericEvent{Object: &autoscalingv2.HorizontalPodAutoscaler{}},
			expect: false,
		},
		{
			name:   "Generic event with non-HPA object",
			event:  event.GenericEvent{Object: &corev1.Pod{}},
			expect: false,
		},
		{
			name:   "Generic event with nil object",
			event:  event.GenericEvent{Object: nil},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marker := &HpaScaleTargetMarker{}
			result := marker.Generic(tt.event)
			assert.Equal(t, tt.expect, result, "Unexpected result for Generic event")
		})
	}
}

func TestHasBeenPropagated(t *testing.T) {
	tests := []struct {
		name     string
		hpa      *autoscalingv2.HorizontalPodAutoscaler
		expected bool
	}{
		{
			name: "HPA with PropagationPolicy",
			hpa: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						policyv1alpha1.PropagationPolicyPermanentIDLabel: "test-policy",
					},
				},
			},
			expected: true,
		},
		{
			name: "HPA with ClusterPropagationPolicy",
			hpa: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "test-cluster-policy",
					},
				},
			},
			expected: true,
		},
		{
			name: "HPA without propagation policy",
			hpa: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasBeenPropagated(tt.hpa)
			assert.Equal(t, tt.expected, result)
		})
	}
}
