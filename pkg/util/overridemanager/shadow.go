/*
Copyright 2021 The Karmada Authors.

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

package overridemanager

import (
	"encoding/json"
	"sort"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// OverridePolicyShadow is the condensed version of a OverridePolicy or ClusterOverridePolicy.
type OverridePolicyShadow struct {
	// PolicyName is the name of the referencing policy.
	PolicyName string `json:"policyName"`

	// Overriders is the overrider list of the referencing policy.
	Overriders policyv1alpha1.Overriders `json:"overriders"`
}

// AppliedOverrides is the list of applied overriders.
type AppliedOverrides struct {
	// AppliedItems is the list of applied overriders.
	AppliedItems []OverridePolicyShadow `json:"appliedItems,omitempty"`
}

// Add appends an item to AppliedItems.
func (ao *AppliedOverrides) Add(policyName string, overriders policyv1alpha1.Overriders) {
	ao.AppliedItems = append(ao.AppliedItems, OverridePolicyShadow{PolicyName: policyName, Overriders: overriders})
}

// AscendOrder sort the applied items in ascending order.
func (ao *AppliedOverrides) AscendOrder() {
	sort.Slice(ao.AppliedItems, func(i, j int) bool {
		return ao.AppliedItems[i].PolicyName < ao.AppliedItems[j].PolicyName
	})
}

// MarshalJSON returns the JSON encoding of applied overrides.
func (ao *AppliedOverrides) MarshalJSON() ([]byte, error) {
	if len(ao.AppliedItems) == 0 {
		return nil, nil
	}

	ao.AscendOrder()
	return json.Marshal(ao.AppliedItems)
}
