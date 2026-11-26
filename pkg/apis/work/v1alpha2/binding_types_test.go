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

package v1alpha2

import (
	"encoding/json"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourceBindingSpecPodSelectorsJSONRoundTrip(t *testing.T) {
	want := ResourceBindingSpec{
		PodSelectors: []metav1.LabelSelector{
			{MatchLabels: map[string]string{"app": "web"}},
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "tier",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"frontend", "api"},
					},
				},
			},
		},
	}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("failed to marshal ResourceBindingSpec: %v", err)
	}

	var got ResourceBindingSpec
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal ResourceBindingSpec: %v", err)
	}

	if !reflect.DeepEqual(got.PodSelectors, want.PodSelectors) {
		t.Fatalf("PodSelectors after JSON round trip = %#v, want %#v", got.PodSelectors, want.PodSelectors)
	}
}

func TestResourceBindingSpecPodSelectorsDeepCopy(t *testing.T) {
	original := &ResourceBindingSpec{
		PodSelectors: []metav1.LabelSelector{
			{MatchLabels: map[string]string{"app": "web"}},
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "tier",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"frontend", "api"},
					},
				},
			},
		},
	}

	copied := original.DeepCopy()
	copied.PodSelectors[0].MatchLabels["app"] = "worker"
	copied.PodSelectors[1].MatchExpressions[0].Values[0] = "backend"

	if got := original.PodSelectors[0].MatchLabels["app"]; got != "web" {
		t.Fatalf("original MatchLabels value after mutating copy = %q, want %q", got, "web")
	}
	if got := original.PodSelectors[1].MatchExpressions[0].Values[0]; got != "frontend" {
		t.Fatalf("original MatchExpressions value after mutating copy = %q, want %q", got, "frontend")
	}
}
