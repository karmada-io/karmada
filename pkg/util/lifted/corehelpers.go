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
// https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/helper/helpers.go

package lifted

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/helper/helpers.go#L57-L61

// IsQuotaHugePageResourceName returns true if the resource name has the quota
// related huge page resource prefix.
func IsQuotaHugePageResourceName(name corev1.ResourceName) bool {
	return strings.HasPrefix(string(name), corev1.ResourceHugePagesPrefix) || strings.HasPrefix(string(name), corev1.ResourceRequestsHugePagesPrefix)
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/helper/helpers.go#L212-L232
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

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/helper/helpers.go#L234-L238

// IsStandardQuotaResourceName returns true if the resource is known to
// the quota tracking system
func IsStandardQuotaResourceName(str string) bool {
	return standardQuotaResources.Has(str) || IsQuotaHugePageResourceName(corev1.ResourceName(str))
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/helper/helpers.go#L240-L261
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

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/helper/helpers.go#L263-L266

// IsStandardResourceName returns true if the resource is known to the system
func IsStandardResourceName(str string) bool {
	return standardResources.Has(str) || IsQuotaHugePageResourceName(corev1.ResourceName(str))
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/helper/helpers.go#L268-L278
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

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/helper/helpers.go#L280-L283

// IsIntegerResourceName returns true if the resource is measured in integer values
func IsIntegerResourceName(str string) bool {
	return integerResources.Has(str) || IsExtendedResourceName(corev1.ResourceName(str))
}
