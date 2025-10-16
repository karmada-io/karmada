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

package dependenciesdistributor

import (
	"context"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// conflictDetectedForConflictResolution checks if there is a conflict in ConflictResolution from referencing ResourceBindings.
func (d *DependenciesDistributor) conflictDetectedForConflictResolution(requiredBy []workv1alpha2.BindingSnapshot) (hasConflict bool, err error) {
	hasOverwrite := false
	hasAbort := false

	for _, snap := range requiredBy {
		refRB := &workv1alpha2.ResourceBinding{}
		if err := d.Client.Get(context.TODO(), client.ObjectKey{Namespace: snap.Namespace, Name: snap.Name}, refRB); err != nil {
			return false, err
		}

		cr := effectiveConflictResolution(refRB.Spec.ConflictResolution)
		if cr == policyv1alpha1.ConflictOverwrite {
			hasOverwrite = true
		} else {
			hasAbort = true
		}
	}

	hasConflict = hasOverwrite && hasAbort
	return hasConflict, nil
}

// conflictDetectedForPreserveOnDeletion checks if there is a conflict in PreserveResourcesOnDeletion from referencing ResourceBindings.
func (d *DependenciesDistributor) conflictDetectedForPreserveOnDeletion(requiredBy []workv1alpha2.BindingSnapshot) (hasConflict bool, err error) {
	// seenTrue indicates at least one referencing ResourceBinding has PreserveResourcesOnDeletion set to true.
	// seenFalse indicates at least one referencing ResourceBinding has PreserveResourcesOnDeletion set to false.
	seenTrue := false
	seenFalse := false

	for _, snap := range requiredBy {
		refRB := &workv1alpha2.ResourceBinding{}
		if err := d.Client.Get(context.TODO(), client.ObjectKey{Namespace: snap.Namespace, Name: snap.Name}, refRB); err != nil {
			return false, err
		}

		pres := ptr.Deref(refRB.Spec.PreserveResourcesOnDeletion, false)
		if pres {
			seenTrue = true
		} else {
			seenFalse = true
		}
	}

	hasConflict = seenTrue && seenFalse
	return hasConflict, nil
}
