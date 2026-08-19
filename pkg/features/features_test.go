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
)

func TestPreemptionEnabled(t *testing.T) {
	originalFeatureGate := FeatureGate.DeepCopy()
	t.Cleanup(func() {
		FeatureGate = originalFeatureGate
	})

	tests := []struct {
		name                                    string
		enablePriorityBasedScheduling           bool
		enablePriorityBasedPreemptiveScheduling bool
		want                                    bool
	}{
		{
			name: "both feature gates disabled",
		},
		{
			name:                          "only priority based scheduling enabled",
			enablePriorityBasedScheduling: true,
		},
		{
			name:                                    "only priority based preemptive scheduling enabled",
			enablePriorityBasedPreemptiveScheduling: true,
		},
		{
			name:                                    "both feature gates enabled",
			enablePriorityBasedScheduling:           true,
			enablePriorityBasedPreemptiveScheduling: true,
			want:                                    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := FeatureGate.Set(fmt.Sprintf("%s=%t,%s=%t",
				PriorityBasedScheduling,
				tt.enablePriorityBasedScheduling,
				PriorityBasedPreemptiveScheduling,
				tt.enablePriorityBasedPreemptiveScheduling,
			)); err != nil {
				t.Fatalf("failed to set feature gates: %v", err)
			}

			if got := PreemptionEnabled(); got != tt.want {
				t.Fatalf("PreemptionEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
