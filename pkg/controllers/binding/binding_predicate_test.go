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

package binding

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestBindingPredicate_Create(t *testing.T) {
	predicate := newBindingPredicate()

	// Test ResourceBinding create event
	rb := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rb",
			Namespace: "default",
		},
	}

	createEvent := event.CreateEvent{Object: rb}
	if !predicate.Create(createEvent) {
		t.Error("Expected Create event to return true")
	}

	// Test ClusterResourceBinding create event
	crb := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-crb",
		},
	}

	createEventCRB := event.CreateEvent{Object: crb}
	if !predicate.Create(createEventCRB) {
		t.Error("Expected Create event for ClusterResourceBinding to return true")
	}
}

func TestBindingPredicate_Delete(t *testing.T) {
	predicate := newBindingPredicate()

	// Test ResourceBinding delete event
	rb := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rb",
			Namespace: "default",
		},
	}

	deleteEvent := event.DeleteEvent{Object: rb}
	if predicate.Delete(deleteEvent) {
		t.Error("Expected Delete event to return false")
	}

	// Test ClusterResourceBinding delete event
	crb := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-crb",
		},
	}

	deleteEventCRB := event.DeleteEvent{Object: crb}
	if predicate.Delete(deleteEventCRB) {
		t.Error("Expected Delete event for ClusterResourceBinding to return false")
	}
}

func TestBindingPredicate_Update(t *testing.T) {
	predicate := newBindingPredicate()

	tests := []struct {
		name     string
		oldObj   *workv1alpha2.ResourceBinding
		newObj   *workv1alpha2.ResourceBinding
		expected bool
	}{
		{
			name: "deletion timestamp set",
			oldObj: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
			},
			newObj: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-rb",
					Namespace:         "default",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
			},
			expected: true,
		},
		{
			name: "clusters field change",
			oldObj: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
					},
				},
			},
			newObj: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
						{Name: "cluster2"},
					},
				},
			},
			expected: true,
		},
		{
			name: "resource field change",
			oldObj: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
						Namespace:  "default",
					},
				},
			},
			newObj: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment-v2",
						Namespace:  "default",
					},
				},
			},
			expected: true,
		},
		{
			name: "conflict resolution change",
			oldObj: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					ConflictResolution: "Abort",
				},
			},
			newObj: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					ConflictResolution: "Overwrite",
				},
			},
			expected: true,
		},
		{
			name: "no significant change",
			oldObj: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
						Namespace:  "default",
					},
				},
			},
			newObj: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rb",
					Namespace: "default",
					Labels:    map[string]string{"test": "label"},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
						Namespace:  "default",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateEvent := event.UpdateEvent{
				ObjectOld: tt.oldObj,
				ObjectNew: tt.newObj,
			}

			result := predicate.Update(updateEvent)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestResourceBindingPredicate_Generic(t *testing.T) {
	predicate := newBindingPredicate()

	genericEvent := event.GenericEvent{Object: &workv1alpha2.ResourceBinding{}}
	if !predicate.Generic(genericEvent) {
		t.Error("Expected Generic event to return true")
	}
}

func TestResourceBindingPredicate_ClusterResourceBinding(t *testing.T) {
	predicate := newBindingPredicate()

	// Test ClusterResourceBinding update with clusters change
	oldCRB := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-crb",
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: []workv1alpha2.TargetCluster{
				{Name: "cluster1"},
			},
		},
	}

	newCRB := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-crb",
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: []workv1alpha2.TargetCluster{
				{Name: "cluster1"},
				{Name: "cluster2"},
			},
		},
	}

	updateEvent := event.UpdateEvent{
		ObjectOld: oldCRB,
		ObjectNew: newCRB,
	}

	if !predicate.Update(updateEvent) {
		t.Error("Expected ClusterResourceBinding update with clusters change to return true")
	}
}
