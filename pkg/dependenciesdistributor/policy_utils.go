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


// AggregateAndDetectConflictResolution aggregates the ConflictResolution across referenced ResourceBindings
// and reports if there is a conflict (both Overwrite and Abort present).
// Aggregation rule: any Overwrite -> Overwrite; else Abort. Empty/zero value counts as Abort.
func (d *DependenciesDistributor) AggregateAndDetectConflictResolution(ctx context.Context, requiredBy []workv1alpha2.BindingSnapshot) (agg policyv1alpha1.ConflictResolution, conflicted bool, err error) {
	if len(requiredBy) == 0 {
		return policyv1alpha1.ConflictAbort, false, nil
	}

	hasOverwrite := false
	hasAbort := false

	for _, snap := range requiredBy {
		refRB := &workv1alpha2.ResourceBinding{}
		if err := d.Client.Get(ctx, client.ObjectKey{Namespace: snap.Namespace, Name: snap.Name}, refRB); err != nil {
			return "", false, err
		}

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
		agg = policyv1alpha1.ConflictOverwrite
	} else {
		agg = policyv1alpha1.ConflictAbort
	}
	return agg, conflicted, nil
}


// AggregateAndDetectPreserveOnDeletion aggregates the PreserveResourcesOnDeletion across referenced ResourceBindings
// and reports if there is a conflict (both true and false present).
// Aggregation rule: any true -> true; else false. Empty(nil) counts as false.
func (d *DependenciesDistributor) AggregateAndDetectPreserveOnDeletion(ctx context.Context, requiredBy []workv1alpha2.BindingSnapshot) (agg bool, conflicted bool, err error) {
	if len(requiredBy) == 0 {
		return false, false, nil
	}

	// seenTrue indicates at least one referencing ResourceBinding has PreserveResourcesOnDeletion set to true.
	// seenFalse indicates at least one referencing ResourceBinding has PreserveResourcesOnDeletion set to false.
	seenTrue := false
	seenFalse := false

	for _, snap := range requiredBy {
		refRB := &workv1alpha2.ResourceBinding{}
		if err := d.Client.Get(ctx, client.ObjectKey{Namespace: snap.Namespace, Name: snap.Name}, refRB); err != nil {
			return false, false, err
		}

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
	agg = seenTrue
	return agg, conflicted, nil
}

// effectiveConflictResolution returns Abort for empty values.
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
		conflictReasons = append(conflictReasons, "ConflictResolution mismatch (Overwrite vs Abort)")
	}

	if preserveConflictDetected {
		conflictReasons = append(conflictReasons, "PreserveResourcesOnDeletion mismatch (true vs false)")
	}

	if len(conflictReasons) > 0 {
		message := "Dependency policy conflict detected: %s."
		d.EventRecorder.Eventf(existBinding, corev1.EventTypeWarning, events.EventReasonDependencyPolicyConflict, message, strings.Join(conflictReasons, "; "))
	}
}

