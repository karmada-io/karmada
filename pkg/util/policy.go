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

package util

import policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"

// IsLazyActivationEnabled judge whether lazy activation preference is enabled.
func IsLazyActivationEnabled(activationPreference policyv1alpha1.ActivationPreference) bool {
	if activationPreference == "" {
		return false
	}
	return activationPreference == policyv1alpha1.LazyActivation
}
