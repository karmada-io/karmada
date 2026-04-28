/*
Copyright 2024 The Kubernetes Authors.

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

package config

import (
	"testing"

	"github.com/spf13/pflag"
)

// TestHPAControllerConfiguration_AddFlags tests that AddFlags adds all expected flags
func TestHPAControllerConfiguration_AddFlags(t *testing.T) {
	config := &HPAControllerConfiguration{}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	config.AddFlags(fs)

	expectedFlags := []string{
		"horizontal-pod-autoscaler-sync-period",
		"horizontal-pod-autoscaler-upscale-delay",
		"horizontal-pod-autoscaler-downscale-stabilization",
		"horizontal-pod-autoscaler-downscale-delay",
		"horizontal-pod-autoscaler-tolerance",
		"horizontal-pod-autoscaler-cpu-initialization-period",
		"horizontal-pod-autoscaler-initial-readiness-delay",
	}

	for _, flagName := range expectedFlags {
		if fs.Lookup(flagName) == nil {
			t.Errorf("Expected flag %s not found", flagName)
		}
	}
}

// TestHPAControllerConfiguration_AddFlags_NilReceiver tests AddFlags with a nil receiver
func TestHPAControllerConfiguration_AddFlags_NilReceiver(t *testing.T) {
	var config *HPAControllerConfiguration
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	config.AddFlags(fs)

	if fs.HasFlags() {
		t.Error("Expected no flags to be added when receiver is nil, but flags were added")
	}
}
