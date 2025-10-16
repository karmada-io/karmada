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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
)

// resolveResourceBindingFromSnapshots resolves and returns the list of ResourceBindings
// referenced by the given BindingSnapshots.
func (d *DependenciesDistributor) resolveResourceBindingFromSnapshots(ctx context.Context, requiredBy []workv1alpha2.BindingSnapshot) ([]*workv1alpha2.ResourceBinding, error) {
	if len(requiredBy) == 0 {
		return nil, nil
	}

	rbs := make([]*workv1alpha2.ResourceBinding, 0, len(requiredBy))
	for _, snap := range requiredBy {
		refRB := &workv1alpha2.ResourceBinding{}
		if err := d.Client.Get(ctx, client.ObjectKey{Namespace: snap.Namespace, Name: snap.Name}, refRB); err != nil {
			return nil, err
		}
		rbs = append(rbs, refRB)
	}
	return rbs, nil
}

// detectAndResolveConflictResolution computes the effective ConflictResolution across referenced ResourceBindings
// and reports if there is a conflict (both Overwrite and Abort present).
// Resolution rule: any Overwrite -> Overwrite; else Abort. Empty/zero value counts as Abort.
func (d *DependenciesDistributor) detectAndResolveConflictResolution(rbs []*workv1alpha2.ResourceBinding) (conflicted bool, effective policyv1alpha1.ConflictResolution) {
	hasOverwrite := false
	hasAbort := false

	for _, refRB := range rbs {
		cr := effectiveConflictResolution(refRB.Spec.ConflictResolution)
		if cr == policyv1alpha1.ConflictOverwrite {
			hasOverwrite = true
		} else {
			hasAbort = true
		}

		if hasOverwrite && hasAbort {
			break
		}
	}

	conflicted = hasOverwrite && hasAbort
	if hasOverwrite {
		effective = policyv1alpha1.ConflictOverwrite
	} else {
		effective = policyv1alpha1.ConflictAbort
	}
	return conflicted, effective
}

// detectAndResolvePreserveOnDeletion computes the effective PreserveResourcesOnDeletion across referenced ResourceBindings
// and reports if there is a conflict (both true and false present).
// Resolution rule: any true -> true; else false. Empty(nil) counts as false.
func (d *DependenciesDistributor) detectAndResolvePreserveOnDeletion(rbs []*workv1alpha2.ResourceBinding) (conflicted bool, effective bool) {
	// seenTrue indicates at least one referencing ResourceBinding has PreserveResourcesOnDeletion set to true.
	// seenFalse indicates at least one referencing ResourceBinding has PreserveResourcesOnDeletion set to false.
	seenTrue := false
	seenFalse := false

	for _, refRB := range rbs {
		pres := ptr.Deref(refRB.Spec.PreserveResourcesOnDeletion, false)
		if pres {
			seenTrue = true
		} else {
			seenFalse = true
		}

		if seenTrue && seenFalse {
			break
		}
	}

	conflicted = seenTrue && seenFalse
	effective = seenTrue
	return conflicted, effective
}

// effectiveConflictResolution returns the effective ConflictResolution value,
// defaulting to ConflictAbort if the provided value is empty.
func effectiveConflictResolution(value policyv1alpha1.ConflictResolution) policyv1alpha1.ConflictResolution {
	if value == "" {
		return policyv1alpha1.ConflictAbort
	}

	return value
}

// recordEventIfPolicyConflict emits a warning event if any policy conflicts were detected.
func (d *DependenciesDistributor) recordEventIfPolicyConflict(existBinding *workv1alpha2.ResourceBinding, crConflictDetected, preserveConflictDetected bool) {
	var conflictReasons []string

	if crConflictDetected {
		conflictReasons = append(conflictReasons, "ConflictResolution conflicted (Overwrite vs Abort)")
	}

	if preserveConflictDetected {
		conflictReasons = append(conflictReasons, "PreserveResourcesOnDeletion conflicted (true vs false)")
	}

	if len(conflictReasons) > 0 {
		message := "Dependency policy conflict detected: %s."
		d.EventRecorder.Eventf(existBinding, corev1.EventTypeWarning, events.EventReasonDependencyPolicyConflict, message, strings.Join(conflictReasons, "; "))
	}
}
