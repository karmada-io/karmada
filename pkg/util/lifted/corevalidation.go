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

// This code is directly lifted from the Kubernetes codebase in order to avoid relying on the k8s.io/kubernetes package.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L59
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L61
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L237-L240
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L320-L327
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L5522-L5540
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L5559-L5570
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L6167-L6177

package lifted

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L58
const isNegativeErrorMsg string = apimachineryvalidation.IsNegativeErrorMsg

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L59
const isInvalidQuotaResource string = `must be a standard resource for quota`

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L61
const isNotIntegerErrorMsg string = `must be an integer`

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L237-L240

// ValidatePodName can be used to check whether the given pod name is valid.
// Prefix indicates this name will be used as part of generation, in which case
// trailing dashes are allowed.
var ValidatePodName = apimachineryvalidation.NameIsDNSSubdomain

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L320-L327

// ValidateNonnegativeQuantity Validates that a Quantity is not negative
func ValidateNonnegativeQuantity(value resource.Quantity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if value.Cmp(resource.Quantity{}) < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, value.String(), isNegativeErrorMsg))
	}
	return allErrs
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L5522-L5540
// +lifted:changed

// Validate compute resource typename.
// Refer to docs/design/resources.md for more details.
func validateResourceName(value string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for _, msg := range validation.IsQualifiedName(value) {
		allErrs = append(allErrs, field.Invalid(fldPath, value, msg))
	}
	if len(allErrs) != 0 {
		return allErrs
	}

	if len(strings.Split(value, "/")) == 1 {
		if !IsStandardResourceName(value) {
			return append(allErrs, field.Invalid(fldPath, value, "must be a standard resource type or fully qualified"))
		}
	}

	return allErrs
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L5559-L5570
// +lifted:changed

// ValidateResourceQuotaResourceName Validate resource names that can go in a resource quota
// Refer to docs/design/resources.md for more details.
func ValidateResourceQuotaResourceName(value string, fldPath *field.Path) field.ErrorList {
	allErrs := validateResourceName(value, fldPath)

	if len(strings.Split(value, "/")) == 1 {
		if !IsStandardQuotaResourceName(value) {
			return append(allErrs, field.Invalid(fldPath, value, isInvalidQuotaResource))
		}
	}
	return allErrs
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/validation/validation.go#L6167-L6177
// +lifted:changed

// ValidateResourceQuantityValue enforces that specified quantity is valid for specified resource
func ValidateResourceQuantityValue(resource string, value resource.Quantity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateNonnegativeQuantity(value, fldPath)...)
	if IsIntegerResourceName(resource) {
		if value.MilliValue()%int64(1000) != int64(0) {
			allErrs = append(allErrs, field.Invalid(fldPath, value, isNotIntegerErrorMsg))
		}
	}
	return allErrs
}
