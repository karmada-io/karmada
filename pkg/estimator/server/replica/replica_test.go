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

package replica

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	listappsv1 "k8s.io/client-go/listers/apps/v1"
	listcorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/ptr"
)

func TestGetUnschedulablePodsOfWorkload(t *testing.T) {
	tests := []struct {
		name        string
		workload    *unstructured.Unstructured
		threshold   time.Duration
		pods        []*corev1.Pod
		replicaSets []*appsv1.ReplicaSet
		expected    int32
		expectError bool
	}{
		{
			name: "deployment with no unschedulable pods",
			// Create a deployment with a single pod that is properly scheduled
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
						"uid":       "test-deployment-uid",
					},
					"spec": map[string]interface{}{
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"app": "test",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"app": "test",
								},
							},
						},
					},
				},
			},
			threshold: 5 * time.Minute,
			// Pod is running and scheduled
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: "default",
						Labels: map[string]string{
							"app":               "test",
							"pod-template-hash": "xyz123",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "ReplicaSet",
								Name:       "test-rs",
								UID:        "test-rs-uid",
								Controller: ptr.To[bool](true),
							},
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodScheduled,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			// ReplicaSet that owns the pod and is owned by deployment
			replicaSets: []*appsv1.ReplicaSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-rs",
						Namespace: "default",
						UID:       "test-rs-uid",
						Labels: map[string]string{
							"app":               "test",
							"pod-template-hash": "xyz123",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "test-deployment",
								UID:        "test-deployment-uid",
								Controller: ptr.To[bool](true),
							},
						},
					},
					Spec: appsv1.ReplicaSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app":               "test",
								"pod-template-hash": "xyz123",
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app":               "test",
									"pod-template-hash": "xyz123",
								},
							},
						},
					},
				},
			},
			expected:    0,
			expectError: false,
		},
		{
			name: "deployment with unschedulable pods beyond threshold",
			// Create a deployment with a single unschedulable pod
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
						"uid":       "test-deployment-uid",
					},
					"spec": map[string]interface{}{
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"app": "test",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"app": "test",
								},
							},
						},
					},
				},
			},
			threshold: 5 * time.Minute,
			// Pod is pending and unschedulable for longer than threshold
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: "default",
						Labels: map[string]string{
							"app":               "test",
							"pod-template-hash": "xyz123",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "ReplicaSet",
								Name:       "test-rs",
								UID:        "test-rs-uid",
								Controller: ptr.To[bool](true),
							},
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
						Conditions: []corev1.PodCondition{
							{
								Type:               corev1.PodScheduled,
								Status:             corev1.ConditionFalse,
								Reason:             corev1.PodReasonUnschedulable,
								LastTransitionTime: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
							},
						},
					},
				},
			},
			replicaSets: []*appsv1.ReplicaSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-rs",
						Namespace: "default",
						UID:       "test-rs-uid",
						Labels: map[string]string{
							"app":               "test",
							"pod-template-hash": "xyz123",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "test-deployment",
								UID:        "test-deployment-uid",
								Controller: ptr.To[bool](true),
							},
						},
					},
					Spec: appsv1.ReplicaSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app":               "test",
								"pod-template-hash": "xyz123",
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app":               "test",
									"pod-template-hash": "xyz123",
								},
							},
						},
					},
				},
			},
			expected:    1,
			expectError: false,
		},
		{
			name: "unsupported workload kind",
			// Create a StatefulSet workload which is not supported
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "test-statefulset",
						"namespace": "default",
					},
				},
			},
			threshold:   5 * time.Minute,
			expected:    0,
			expectError: true,
		},
		{
			name: "deployment with negative threshold",
			// Testing that negative threshold is handled correctly (should be set to 0)
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
						"uid":       "test-deployment-uid",
					},
					"spec": map[string]interface{}{
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"app": "test",
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"app": "test",
								},
							},
						},
					},
				},
			},
			threshold: -5 * time.Minute,
			// Add pods that are unschedulable for different durations
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-1",
						Namespace: "default",
						Labels: map[string]string{
							"app":               "test",
							"pod-template-hash": "xyz123",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "ReplicaSet",
								Name:       "test-rs",
								UID:        "test-rs-uid",
								Controller: ptr.To[bool](true),
							},
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
						Conditions: []corev1.PodCondition{
							{
								Type:               corev1.PodScheduled,
								Status:             corev1.ConditionFalse,
								Reason:             corev1.PodReasonUnschedulable,
								LastTransitionTime: metav1.Time{Time: time.Now().Add(-1 * time.Second)},
							},
						},
					},
				},
			},
			replicaSets: []*appsv1.ReplicaSet{
				// Include matching ReplicaSet configuration
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-rs",
						Namespace: "default",
						UID:       "test-rs-uid",
						Labels: map[string]string{
							"app":               "test",
							"pod-template-hash": "xyz123",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "test-deployment",
								UID:        "test-deployment-uid",
								Controller: ptr.To[bool](true),
							},
						},
					},
					Spec: appsv1.ReplicaSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app":               "test",
								"pod-template-hash": "xyz123",
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app":               "test",
									"pod-template-hash": "xyz123",
								},
							},
						},
					},
				},
			},
			expected:    1, // Count as unschedulable since negative threshold becomes 0
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock listers with test data
			podLister := &mockPodLister{
				pods: map[string][]*corev1.Pod{
					"default": tt.pods,
				},
			}
			rsLister := &mockReplicaSetLister{
				replicaSets: map[string][]*appsv1.ReplicaSet{
					"default": tt.replicaSets,
				},
			}

			listers := &ListerWrapper{
				PodLister:        podLister,
				ReplicaSetLister: rsLister,
			}

			result, err := GetUnschedulablePodsOfWorkload(tt.workload, tt.threshold, listers)

			if tt.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPodUnschedulable(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		pod       *corev1.Pod
		threshold time.Duration
		expected  bool
	}{
		{
			name: "pod is schedulable",
			// Test case for a normally scheduled pod
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			threshold: 5 * time.Minute,
			expected:  false,
		},
		{
			name: "pod is unschedulable beyond threshold",
			// Test case for pod that has been unschedulable longer than threshold
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:               corev1.PodScheduled,
							Status:             corev1.ConditionFalse,
							Reason:             corev1.PodReasonUnschedulable,
							LastTransitionTime: metav1.Time{Time: now.Add(-10 * time.Minute)},
						},
					},
				},
			},
			threshold: 5 * time.Minute,
			expected:  true,
		},
		{
			name: "pod is unschedulable within threshold",
			// Test case for pod that has been unschedulable less than threshold
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:               corev1.PodScheduled,
							Status:             corev1.ConditionFalse,
							Reason:             corev1.PodReasonUnschedulable,
							LastTransitionTime: metav1.Time{Time: now.Add(-2 * time.Minute)},
						},
					},
				},
			},
			threshold: 5 * time.Minute,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := podUnschedulable(tt.pod, tt.threshold)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock implementations of the Kubernetes listers
type mockPodLister struct {
	pods map[string][]*corev1.Pod // namespace -> pods mapping
}

func (m *mockPodLister) List(selector labels.Selector) (ret []*corev1.Pod, err error) {
	var pods []*corev1.Pod
	for _, nsPods := range m.pods {
		for _, pod := range nsPods {
			if selector == nil || selector.Matches(labels.Set(pod.Labels)) {
				pods = append(pods, pod)
			}
		}
	}
	return pods, nil
}

func (m *mockPodLister) Pods(namespace string) listcorev1.PodNamespaceLister {
	return &mockPodNamespaceLister{
		pods: m.pods[namespace],
	}
}

type mockPodNamespaceLister struct {
	pods []*corev1.Pod
}

func (m *mockPodNamespaceLister) List(selector labels.Selector) (ret []*corev1.Pod, err error) {
	var result []*corev1.Pod
	for _, pod := range m.pods {
		if selector == nil || selector.Matches(labels.Set(pod.Labels)) {
			result = append(result, pod)
		}
	}
	return result, nil
}

func (m *mockPodNamespaceLister) Get(name string) (*corev1.Pod, error) {
	for _, pod := range m.pods {
		if pod.Name == name {
			return pod, nil
		}
	}
	return nil, nil
}

type mockReplicaSetLister struct {
	replicaSets map[string][]*appsv1.ReplicaSet // namespace -> replicasets mapping
}

func (m *mockReplicaSetLister) List(selector labels.Selector) (ret []*appsv1.ReplicaSet, err error) {
	var replicaSets []*appsv1.ReplicaSet
	for _, nsRS := range m.replicaSets {
		for _, rs := range nsRS {
			if selector == nil || selector.Matches(labels.Set(rs.Labels)) {
				replicaSets = append(replicaSets, rs)
			}
		}
	}
	return replicaSets, nil
}

func (m *mockReplicaSetLister) GetPodReplicaSets(pod *corev1.Pod) ([]*appsv1.ReplicaSet, error) {
	var matchingRS []*appsv1.ReplicaSet
	for _, rsList := range m.replicaSets {
		for _, rs := range rsList {
			if rs.Namespace == pod.Namespace {
				selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
				if err != nil {
					continue
				}
				if selector.Matches(labels.Set(pod.Labels)) {
					matchingRS = append(matchingRS, rs)
				}
			}
		}
	}
	return matchingRS, nil
}

func (m *mockReplicaSetLister) ReplicaSets(namespace string) listappsv1.ReplicaSetNamespaceLister {
	return &mockReplicaSetNamespaceLister{
		replicaSets: m.replicaSets[namespace],
	}
}

type mockReplicaSetNamespaceLister struct {
	replicaSets []*appsv1.ReplicaSet
}

func (m *mockReplicaSetNamespaceLister) List(selector labels.Selector) (ret []*appsv1.ReplicaSet, err error) {
	var result []*appsv1.ReplicaSet
	for _, rs := range m.replicaSets {
		if selector == nil || selector.Matches(labels.Set(rs.Labels)) {
			result = append(result, rs)
		}
	}
	return result, nil
}

func (m *mockReplicaSetNamespaceLister) Get(name string) (*appsv1.ReplicaSet, error) {
	for _, rs := range m.replicaSets {
		if rs.Name == name {
			return rs, nil
		}
	}
	return nil, nil
}
