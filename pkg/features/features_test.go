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

package features

import (
	"fmt"
	"testing"

	"k8s.io/component-base/featuregate"
)

func TestFeatureGateInitialization(t *testing.T) {
	for feature, spec := range DefaultFeatureGates {
		if FeatureGate.Enabled(feature) != spec.Default {
			t.Errorf("Feature %v should be %v by default", feature, spec.Default)
		}
	}
}

func TestFeatureToggling(t *testing.T) {
	testCases := []struct {
		name    string
		feature featuregate.Feature
		enable  bool
	}{
		{"Enable Failover", Failover, true},
		{"Disable GracefulEviction", GracefulEviction, false},
		{"Enable PropagateDeps", PropagateDeps, true},
		{"Disable CustomizedClusterResourceModeling", CustomizedClusterResourceModeling, false},
		{"Enable PolicyPreemption", PolicyPreemption, true},
		{"Enable MultiClusterService", MultiClusterService, true},
		{"Enable ResourceQuotaEstimate", ResourceQuotaEstimate, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := FeatureGate.Set(fmt.Sprintf("%s=%t", tc.feature, tc.enable))
			if err != nil {
				t.Errorf("Failed to set feature gate: %v", err)
			}
			if FeatureGate.Enabled(tc.feature) != tc.enable {
				t.Errorf("Feature %v should be %v", tc.feature, tc.enable)
			}
		})
	}
}

func TestFeatureDependencies(t *testing.T) {
	for feature, spec := range DefaultFeatureGates {
		FeatureGate.Set(fmt.Sprintf("%s=%t", feature, spec.Default))
	}

	FeatureGate.Set(fmt.Sprintf("%s=true", Failover))
	FeatureGate.Set(fmt.Sprintf("%s=true", GracefulEviction))
	if !FeatureGate.Enabled(GracefulEviction) {
		t.Errorf("GracefulEviction should be able to be enabled when Failover is enabled")
	}

	FeatureGate.Set(fmt.Sprintf("%s=false", GracefulEviction))
	if FeatureGate.Enabled(GracefulEviction) {
		t.Errorf("GracefulEviction should be able to be disabled independently of Failover")
	}
}

func TestUnknownFeature(t *testing.T) {
	err := FeatureGate.Set("UnknownFeature=true")
	if err == nil {
		t.Error("Setting an unknown feature should return an error")
	}
}

func TestFeatureGateReset(t *testing.T) {
	FeatureGate.Set(fmt.Sprintf("%s=false", Failover))
	FeatureGate.Set(fmt.Sprintf("%s=true", PolicyPreemption))

	for feature, spec := range DefaultFeatureGates {
		FeatureGate.Set(fmt.Sprintf("%s=%t", feature, spec.Default))
	}

	for feature, spec := range DefaultFeatureGates {
		if FeatureGate.Enabled(feature) != spec.Default {
			t.Errorf("Feature %v should be reset to its default state of %v", feature, spec.Default)
		}
	}
}
