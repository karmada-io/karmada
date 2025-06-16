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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// Custom taint effects that extend the core Kubernetes taint effects
const (
	// TaintEffectSelectiveNoExecute is like TaintEffectNoExecute, but only evicts pods
	// that match the taint's key-value pair. Enforced by the taint manager.
	TaintEffectSelectiveNoExecute corev1.TaintEffect = "SelectiveNoExecute"
)

// IsValidTaintEffect checks if the given taint effect is valid
func IsValidTaintEffect(effect corev1.TaintEffect) bool {
	switch effect {
	case corev1.TaintEffectNoSchedule,
		corev1.TaintEffectPreferNoSchedule,
		corev1.TaintEffectNoExecute,
		TaintEffectSelectiveNoExecute:
		return true
	default:
		return false
	}
}
