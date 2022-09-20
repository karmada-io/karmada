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
// https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/v1/helper/helpers.go

package lifted

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/v1/helper/helpers.go#L31-L46
// +lifted:changed

// IsExtendedResourceName returns true if:
// 1. the resource name is not in the default namespace;
// 2. resource name does not have "requests." prefix,
// to avoid confusion with the convention in quota
// 3. it satisfies the rules in IsQualifiedName() after converted into quota resource name
func IsExtendedResourceName(name corev1.ResourceName) bool {
	if IsNativeResource(name) || strings.HasPrefix(string(name), corev1.DefaultResourceRequestsPrefix) {
		return false
	}
	// Ensure it satisfies the rules in IsQualifiedName() after converted into quota resource name
	nameForQuota := fmt.Sprintf("%s%s", corev1.DefaultResourceRequestsPrefix, string(name))
	if errs := validation.IsQualifiedName(nameForQuota); len(errs) != 0 {
		return false
	}
	return true
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/v1/helper/helpers.go#L48-L51

// IsPrefixedNativeResource returns true if the resource name is in the
// *kubernetes.io/ namespace.
func IsPrefixedNativeResource(name corev1.ResourceName) bool {
	return strings.Contains(string(name), corev1.ResourceDefaultNamespacePrefix)
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/v1/helper/helpers.go#L54-L60

// IsNativeResource returns true if the resource name is in the
// *kubernetes.io/ namespace. Partially-qualified (unprefixed) names are
// implicitly in the kubernetes.io/ namespace.
func IsNativeResource(name corev1.ResourceName) bool {
	return !strings.Contains(string(name), "/") ||
		IsPrefixedNativeResource(name)
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/v1/helper/helpers.go#L62-L66

// IsHugePageResourceName returns true if the resource name has the huge page
// resource prefix.
func IsHugePageResourceName(name corev1.ResourceName) bool {
	return strings.HasPrefix(string(name), corev1.ResourceHugePagesPrefix)
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/v1/helper/helpers.go#L132-L135

// IsAttachableVolumeResourceName returns true when the resource name is prefixed in attachable volume
func IsAttachableVolumeResourceName(name corev1.ResourceName) bool {
	return strings.HasPrefix(string(name), corev1.ResourceAttachableVolumesPrefix)
}
