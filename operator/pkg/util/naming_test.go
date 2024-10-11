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

import "testing"

func TestGenerateResourceName(t *testing.T) {
	tests := []struct {
		name      string
		component string
		suffix    string
		want      string
	}{
		{
			name:      "GenerateResourceName_WithKarmada_SuffixImmediatelyAfter",
			component: "karmada-demo",
			suffix:    "search",
			want:      "karmada-demo-search",
		},
		{
			name:      "GenerateResourceName_WithoutKarmada_KarmadaInTheMiddle",
			component: "test-demo",
			suffix:    "search",
			want:      "test-demo-karmada-search",
		},
	}
	for _, test := range tests {
		if got := generateResourceName(test.component, test.suffix); got != test.want {
			t.Errorf("expected resource name generated to be %s, but got %s", test.want, got)
		}
	}
}
