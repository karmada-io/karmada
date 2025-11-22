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

package validation

import (
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// VerifyDependencies verifies dependencies.
func VerifyDependencies(dependencies []configv1alpha1.DependentObjectReference) error {
	allErrs := field.ErrorList{}
	fldPath := field.NewPath("dependencies")
	for i, dependency := range dependencies {
		fldPath := fldPath.Index(i)
		if len(dependency.APIVersion) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("apiVersion"), dependency.APIVersion, "missing required apiVersion"))
		}
		if len(dependency.Kind) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("kind"), dependency.Kind, "missing required kind"))
		}
		if len(dependency.Name) == 0 && dependency.LabelSelector == nil {
			allErrs = append(allErrs, field.Invalid(fldPath, dependencies[i], "dependency can not leave name and labelSelector all empty"))
		}
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(dependency.LabelSelector, metav1validation.LabelSelectorValidationOptions{
			AllowInvalidLabelValueInSelector: false,
		}, fldPath.Child("labelSelector"))...)
	}
	return allErrs.ToAggregate()
}
