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
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/helper/helpers.go

package lifted

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/helper/helpers.go#L57-L61
// +lifted:changed

// IsQuotaHugePageResourceName returns true if the resource name has the quota
// related huge page resource prefix.
func IsQuotaHugePageResourceName(name corev1.ResourceName) bool {
	return strings.HasPrefix(string(name), corev1.ResourceHugePagesPrefix) || strings.HasPrefix(string(name), corev1.ResourceRequestsHugePagesPrefix)
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/helper/helpers.go#L210-L230
// +lifted:changed

var standardQuotaResources = sets.NewString(
	string(corev1.ResourceCPU),
	string(corev1.ResourceMemory),
	string(corev1.ResourceEphemeralStorage),
	string(corev1.ResourceRequestsCPU),
	string(corev1.ResourceRequestsMemory),
	string(corev1.ResourceRequestsStorage),
	string(corev1.ResourceRequestsEphemeralStorage),
	string(corev1.ResourceLimitsCPU),
	string(corev1.ResourceLimitsMemory),
	string(corev1.ResourceLimitsEphemeralStorage),
	string(corev1.ResourcePods),
	string(corev1.ResourceQuotas),
	string(corev1.ResourceServices),
	string(corev1.ResourceReplicationControllers),
	string(corev1.ResourceSecrets),
	string(corev1.ResourcePersistentVolumeClaims),
	string(corev1.ResourceConfigMaps),
	string(corev1.ResourceServicesNodePorts),
	string(corev1.ResourceServicesLoadBalancers),
)

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/helper/helpers.go#L232-L236

// IsStandardQuotaResourceName returns true if the resource is known to
// the quota tracking system
func IsStandardQuotaResourceName(str string) bool {
	return standardQuotaResources.Has(str) || IsQuotaHugePageResourceName(corev1.ResourceName(str))
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/helper/helpers.go#L238-L259
// +lifted:changed

var standardResources = sets.NewString(
	string(corev1.ResourceCPU),
	string(corev1.ResourceMemory),
	string(corev1.ResourceEphemeralStorage),
	string(corev1.ResourceRequestsCPU),
	string(corev1.ResourceRequestsMemory),
	string(corev1.ResourceRequestsEphemeralStorage),
	string(corev1.ResourceLimitsCPU),
	string(corev1.ResourceLimitsMemory),
	string(corev1.ResourceLimitsEphemeralStorage),
	string(corev1.ResourcePods),
	string(corev1.ResourceQuotas),
	string(corev1.ResourceServices),
	string(corev1.ResourceReplicationControllers),
	string(corev1.ResourceSecrets),
	string(corev1.ResourceConfigMaps),
	string(corev1.ResourcePersistentVolumeClaims),
	string(corev1.ResourceStorage),
	string(corev1.ResourceRequestsStorage),
	string(corev1.ResourceServicesNodePorts),
	string(corev1.ResourceServicesLoadBalancers),
)

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/helper/helpers.go#L261-L264

// IsStandardResourceName returns true if the resource is known to the system
func IsStandardResourceName(str string) bool {
	return standardResources.Has(str) || IsQuotaHugePageResourceName(corev1.ResourceName(str))
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/helper/helpers.go#LL266-L276
// +lifted:changed

var integerResources = sets.NewString(
	string(corev1.ResourcePods),
	string(corev1.ResourceQuotas),
	string(corev1.ResourceServices),
	string(corev1.ResourceReplicationControllers),
	string(corev1.ResourceSecrets),
	string(corev1.ResourceConfigMaps),
	string(corev1.ResourcePersistentVolumeClaims),
	string(corev1.ResourceServicesNodePorts),
	string(corev1.ResourceServicesLoadBalancers),
)

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/apis/core/helper/helpers.go#L278-L281

// IsIntegerResourceName returns true if the resource is measured in integer values
func IsIntegerResourceName(str string) bool {
	return integerResources.Has(str) || IsExtendedResourceName(corev1.ResourceName(str))
}

// ValidateContainerResourceName checks the name of resource specified for a container
func ValidateContainerResourceName(value string, fldPath *field.Path) field.ErrorList {
	allErrs := validateResourceName(value, fldPath)
	if len(strings.Split(value, "/")) == 1 {
		if !IsStandardContainerResourceName(value) {
			return append(allErrs, field.Invalid(fldPath, value, "must be a standard resource for containers"))
		}
	} else if !IsNativeResource(corev1.ResourceName(value)) {
		if !IsExtendedResourceName(corev1.ResourceName(value)) {
			return append(allErrs, field.Invalid(fldPath, value, "doesn't follow extended resource name standard"))
		}
	}
	return allErrs
}

var standardContainerResources = sets.NewString(
	string(corev1.ResourceCPU),
	string(corev1.ResourceMemory),
	string(corev1.ResourceEphemeralStorage),
)

// IsStandardContainerResourceName returns true if the container can make a resource request
// for the specified resource
func IsStandardContainerResourceName(str string) bool {
	return standardContainerResources.Has(str) || IsHugePageResourceName(corev1.ResourceName(str))
}

func ValidateDNS1123Label(value string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for _, msg := range validation.IsDNS1123Label(value) {
		allErrs = append(allErrs, field.Invalid(fldPath, value, msg))
	}
	return allErrs
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/core/validation/validation.go

// ValidateNameFunc validates that the provided name is valid for a given resource type.
// Not all resources have the same validation rules for names. Prefix is true
// if the name will have a value appended to it.  If the name is not valid,
// this returns a list of descriptions of individual characteristics of the
// value that were not valid.  Otherwise this returns an empty list or nil.
type ValidateNameFunc apimachineryvalidation.ValidateNameFunc

// ValidateObjectMeta validates an object's metadata on creation. It expects that name generation has already
// been performed.
// It doesn't return an error for rootscoped resources with namespace, because namespace should already be cleared before.
// TODO: Remove calls to this method scattered in validations of specific resources, e.g., ValidatePodUpdate.
func ValidateObjectMeta(meta *metav1.ObjectMeta, requiresNamespace bool, nameFn ValidateNameFunc, fldPath *field.Path) field.ErrorList {
	allErrs := apimachineryvalidation.ValidateObjectMeta(meta, requiresNamespace, apimachineryvalidation.ValidateNameFunc(nameFn), fldPath)
	// run additional checks for the finalizer name
	for i := range meta.Finalizers {
		allErrs = append(allErrs, validateKubeFinalizerName(string(meta.Finalizers[i]), fldPath.Child("finalizers").Index(i))...)
	}
	return allErrs
}

// validateKubeFinalizerName checks for "standard" names of legacy finalizer
func validateKubeFinalizerName(stringValue string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(strings.Split(stringValue, "/")) == 1 {
		if IsStandardFinalizerName(stringValue) {
			return append(allErrs, field.Invalid(fldPath, stringValue, "name is neither a standard finalizer name nor is it fully qualified"))
		}
	}

	return allErrs
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/core/helper/helpers.go

var standardFinalizers = sets.NewString(
	string(corev1.FinalizerKubernetes),
	metav1.FinalizerOrphanDependents,
	metav1.FinalizerDeleteDependents,
)

// IsStandardFinalizerName checks if the input string is a standard finalizer name
func IsStandardFinalizerName(str string) bool {
	return standardFinalizers.Has(str)
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/staging/src/k8s.io/apimachinery/pkg/api/validation/generic.go

// NameIsDNSSubdomain is a ValidateNameFunc for names that must be a DNS subdomain.
func NameIsDNSSubdomain(name string, prefix bool) []string {
	if prefix {
		name = maskTrailingDash(name)
	}
	return validation.IsDNS1123Subdomain(name)
}

// maskTrailingDash replaces the final character of a string with a subdomain safe
// value if it is a dash and if the length of this string is greater than 1. Note that
// this is used when a value could be appended to the string, see ValidateNameFunc
// for more info.
func maskTrailingDash(name string) string {
	if len(name) > 1 && strings.HasSuffix(name, "-") {
		return name[:len(name)-2] + "a"
	}
	return name
}
