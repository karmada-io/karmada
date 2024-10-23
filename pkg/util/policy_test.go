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

import (
	"testing"

	"github.com/stretchr/testify/assert"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func TestIsLazyActivationEnabled(t *testing.T) {
	tests := []struct {
		name                 string
		activationPreference policyv1alpha1.ActivationPreference
		expected             bool
	}{
		{
			name:                 "empty activation preference",
			activationPreference: "",
			expected:             false,
		},
		{
			name:                 "lazy activation enabled",
			activationPreference: policyv1alpha1.LazyActivation,
			expected:             true,
		},
		{
			name:                 "different activation preference",
			activationPreference: "SomeOtherPreference",
			expected:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsLazyActivationEnabled(tt.activationPreference)
			assert.Equal(t, tt.expected, result, "unexpected result for activation preference: %s", tt.activationPreference)
		})
	}
}
