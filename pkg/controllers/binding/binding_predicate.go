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

package binding

import (
    "reflect"

    "k8s.io/klog/v2"
    "sigs.k8s.io/controller-runtime/pkg/event"
    "sigs.k8s.io/controller-runtime/pkg/predicate"

    workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// ResourceBindingPredicate implements a custom predicate for ResourceBinding and ClusterResourceBinding
// to optimize event filtering and reduce unnecessary reconciliations.
//
// Optimization strategy:
// 1. Ignore Delete events - garbage collection and owner references handle cleanup
// 2. Always process Create events - new bindings must trigger reconciliation
// 3. Process Update events only when:
//    - Generation changes (normal behavior)
//    - DeletionTimestamp is set (for finalizer cleanup)
//    - Critical spec fields change: requiredBy, clusters, gracefulEvictionTasks, resource, suspension, preserveResourcesOnDeletion, conflictResolution
type ResourceBindingPredicate struct{}

var _ predicate.Predicate = &ResourceBindingPredicate{}

// Create implements CreateEvent filter
func (r *ResourceBindingPredicate) Create(_ event.CreateEvent) bool {
	// Always process create events for new ResourceBinding/ClusterResourceBinding objects
	return true
}

// Update implements UpdateEvent filter
func (r *ResourceBindingPredicate) Update(e event.UpdateEvent) bool {
	// Check if generation changed (normal behavior)
	if e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() {
		return true
	}

	// Check if deletion timestamp is set (for finalizer cleanup)
	if e.ObjectOld.GetDeletionTimestamp() == nil && e.ObjectNew.GetDeletionTimestamp() != nil {
		return true
	}

	// Check if critical spec fields changed
	return r.hasCriticalSpecFieldChanged(e.ObjectOld, e.ObjectNew)
}

// Delete implements DeleteEvent filter
func (r *ResourceBindingPredicate) Delete(_ event.DeleteEvent) bool {
	// Ignore delete events - garbage collection and owner references handle cleanup
	// The controller still reconciles properly using create/update and finalizer logic
	return false
}

// Generic implements GenericEvent filter
func (r *ResourceBindingPredicate) Generic(_ event.GenericEvent) bool {
	// Process generic events (rarely used)
	return true
}

// hasCriticalSpecFieldChanged checks if any critical spec fields have changed
func (r *ResourceBindingPredicate) hasCriticalSpecFieldChanged(oldObj, newObj interface{}) bool {
	var oldSpec, newSpec workv1alpha2.ResourceBindingSpec

	switch oldBinding := oldObj.(type) {
	case *workv1alpha2.ResourceBinding:
		oldSpec = oldBinding.Spec
	case *workv1alpha2.ClusterResourceBinding:
		oldSpec = oldBinding.Spec
	default:
		klog.V(4).InfoS("Unknown object type in predicate", "type", reflect.TypeOf(oldObj))
		return false
	}

	switch newBinding := newObj.(type) {
	case *workv1alpha2.ResourceBinding:
		newSpec = newBinding.Spec
	case *workv1alpha2.ClusterResourceBinding:
		newSpec = newBinding.Spec
	default:
		klog.V(4).InfoS("Unknown object type in predicate", "type", reflect.TypeOf(newObj))
		return false
	}

	// Check critical fields that require reconciliation
	return !reflect.DeepEqual(oldSpec.RequiredBy, newSpec.RequiredBy) ||
		!reflect.DeepEqual(oldSpec.Clusters, newSpec.Clusters) ||
		!reflect.DeepEqual(oldSpec.GracefulEvictionTasks, newSpec.GracefulEvictionTasks) ||
		!reflect.DeepEqual(oldSpec.Resource, newSpec.Resource) ||
		!reflect.DeepEqual(oldSpec.Suspension, newSpec.Suspension) ||
		!reflect.DeepEqual(oldSpec.PreserveResourcesOnDeletion, newSpec.PreserveResourcesOnDeletion) ||
		oldSpec.ConflictResolution != newSpec.ConflictResolution
}

// NewResourceBindingPredicate creates a new ResourceBindingPredicate instance
func NewResourceBindingPredicate() *ResourceBindingPredicate {
	return &ResourceBindingPredicate{}
}
