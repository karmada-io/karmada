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

package features

import (
	"fmt"
	"testing"

	"k8s.io/component-base/featuregate"
)

func TestPriorityBasedPreemptiveSchedulingFeatureGate(t *testing.T) {
	spec, exists := FeatureGate.GetAll()[PriorityBasedPreemptiveScheduling]
	if !exists {
		t.Fatalf("feature gate %q is not registered", PriorityBasedPreemptiveScheduling)
	}
	if spec.Default {
		t.Errorf("feature gate %q should be disabled by default", PriorityBasedPreemptiveScheduling)
	}
	if spec.PreRelease != featuregate.Alpha {
		t.Errorf("feature gate %q pre-release stage = %q, want %q", PriorityBasedPreemptiveScheduling, spec.PreRelease, featuregate.Alpha)
	}
}

func TestPriorityBasedPreemptiveSchedulingEnabled(t *testing.T) {
	originalFeatureGate := FeatureGate.DeepCopy()
	t.Cleanup(func() {
		FeatureGate = originalFeatureGate
	})

	tests := []struct {
		name                     string
		priorityBasedScheduling  bool
		preemptiveScheduling     bool
		wantPreemptiveScheduling bool
	}{
		{
			name: "both feature gates disabled",
		},
		{
			name:                    "only priority based scheduling enabled",
			priorityBasedScheduling: true,
		},
		{
			name:                 "only preemptive scheduling enabled",
			preemptiveScheduling: true,
		},
		{
			name:                     "both feature gates enabled",
			priorityBasedScheduling:  true,
			preemptiveScheduling:     true,
			wantPreemptiveScheduling: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := FeatureGate.Set(fmt.Sprintf("%s=%t,%s=%t",
				PriorityBasedScheduling,
				test.priorityBasedScheduling,
				PriorityBasedPreemptiveScheduling,
				test.preemptiveScheduling,
			)); err != nil {
				t.Fatalf("failed to set feature gates: %v", err)
			}

			if got := PriorityBasedPreemptiveSchedulingEnabled(); got != test.wantPreemptiveScheduling {
				t.Errorf("PriorityBasedPreemptiveSchedulingEnabled() = %t, want %t", got, test.wantPreemptiveScheduling)
			}
		})
	}
}
