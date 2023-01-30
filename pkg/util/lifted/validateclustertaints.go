/*
Copyright 2014 The Kubernetes Authors.

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

// This code is directly lifted from the Kubernetes codebase.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/validation/validation.go#L5001-L5033
// https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/validation/validation.go#L3305-L3326

package lifted

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	unversionedvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/validation/validation.go#L5001-L5033
// +lifted:changed

// ValidateClusterTaints tests if given taints have valid data.
func ValidateClusterTaints(taints []corev1.Taint, fldPath *field.Path) field.ErrorList {
	allErrors := field.ErrorList{}

	//nolint:staticcheck
	// disable `deprecation` check for lifted code.
	uniqueTaints := map[corev1.TaintEffect]sets.String{}

	for i, currTaint := range taints {
		idxPath := fldPath.Index(i)
		// validate the taint key
		allErrors = append(allErrors, unversionedvalidation.ValidateLabelName(currTaint.Key, idxPath.Child("key"))...)
		// validate the taint value
		if errs := validation.IsValidLabelValue(currTaint.Value); len(errs) != 0 {
			allErrors = append(allErrors, field.Invalid(idxPath.Child("value"), currTaint.Value, strings.Join(errs, ";")))
		}
		// validate the taint effect
		allErrors = append(allErrors, validateClusterTaintEffect(&currTaint.Effect, false, idxPath.Child("effect"))...)

		// validate if taint is unique by <key, effect>
		if len(uniqueTaints[currTaint.Effect]) > 0 && uniqueTaints[currTaint.Effect].Has(currTaint.Key) {
			duplicatedError := field.Duplicate(idxPath, currTaint)
			duplicatedError.Detail = "taints must be unique by key and effect pair"
			allErrors = append(allErrors, duplicatedError)
			continue
		}

		// add taint to existingTaints for uniqueness check
		if len(uniqueTaints[currTaint.Effect]) == 0 {
			//nolint:staticcheck
			// disable `deprecation` check for lifted code.
			uniqueTaints[currTaint.Effect] = sets.String{}
		}
		uniqueTaints[currTaint.Effect].Insert(currTaint.Key)
	}
	return allErrors
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/validation/validation.go#L3305-L3326
// +lifted:changed
func validateClusterTaintEffect(effect *corev1.TaintEffect, allowEmpty bool, fldPath *field.Path) field.ErrorList {
	if !allowEmpty && len(*effect) == 0 {
		return field.ErrorList{field.Required(fldPath, "")}
	}

	allErrors := field.ErrorList{}
	switch *effect {
	case corev1.TaintEffectNoSchedule, corev1.TaintEffectNoExecute:
	default:
		validValues := []string{
			string(corev1.TaintEffectNoSchedule),
			string(corev1.TaintEffectNoExecute),
		}
		allErrors = append(allErrors, field.NotSupported(fldPath, *effect, validValues))
	}
	return allErrors
}
