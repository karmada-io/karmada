/*
Copyright 2026 The Karmada Authors.

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

package helper

import (
	corev1 "k8s.io/api/core/v1"
	resourcehelper "k8s.io/component-helpers/resource"
)

// PodTemplateLimits returns the effective limits of one replica from its
// template. Status is intentionally excluded; pod-level limits and overhead
// follow the Kubernetes helper included in this module.
func PodTemplateLimits(template *corev1.PodTemplateSpec) corev1.ResourceList {
	if template == nil {
		return nil
	}
	pod := &corev1.Pod{Spec: *template.Spec.DeepCopy()}
	limits := resourcehelper.PodLimits(pod, resourcehelper.PodResourcesOptions{})
	if len(limits) == 0 {
		return nil
	}
	return limits.DeepCopy()
}
