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
	"reflect"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// hasSignificantSpecFieldChanged checks if any significant spec fields have changed
func hasSignificantSpecFieldChanged(oldObj, newObj interface{}) bool {
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

// newBindingPredicate returns an inline predicate for ResourceBinding and ClusterResourceBinding
// to optimize event filtering and reduce unnecessary reconciliations
func newBindingPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Trigger reconciliation when deletion starts to ensure timely finalizer/cleanup handling
			if e.ObjectOld.GetDeletionTimestamp() != e.ObjectNew.GetDeletionTimestamp() {
				return true
			}
			// Check if any critical spec fields have changed
			return hasSignificantSpecFieldChanged(e.ObjectOld, e.ObjectNew)
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			// Ignore delete events; reconciliation is driven by generation bump and finalizers.
			// Cleanup is handled by controller logic (e.g., work deletion).
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			// Process generic events (rarely used)
			return true
		},
	}
}
