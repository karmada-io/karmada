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
	"encoding/json"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// RegisterEqualityCheckFunctions registers custom check functions to the equality checker.
// These functions help avoid performing deep-equality checks on workloads represented as byte slices.
func RegisterEqualityCheckFunctions(e *conversion.Equalities) error {
	return e.AddFuncs(
		func(a, b workv1alpha1.Manifest) bool {
			return rawExtensionDeepEqual(&a.RawExtension, &b.RawExtension, e)
		},
		func(a, b workv1alpha1.ManifestStatus) bool {
			return e.DeepEqual(a.Identifier, b.Identifier) &&
				rawExtensionDeepEqual(a.Status, b.Status, e) &&
				e.DeepEqual(a.Health, b.Health)
		},
		func(a, b workv1alpha1.AggregatedStatusItem) bool {
			return e.DeepEqual(a.ClusterName, b.ClusterName) &&
				rawExtensionDeepEqual(a.Status, b.Status, e) &&
				e.DeepEqual(a.Applied, b.Applied) &&
				e.DeepEqual(a.AppliedMessage, b.AppliedMessage)
		},
		func(a, b workv1alpha2.AggregatedStatusItem) bool {
			return e.DeepEqual(a.ClusterName, b.ClusterName) &&
				rawExtensionDeepEqual(a.Status, b.Status, e) &&
				e.DeepEqual(a.Applied, b.Applied) &&
				e.DeepEqual(a.AppliedMessage, b.AppliedMessage)
		},
	)
}

func rawExtensionDeepEqual(a, b *runtime.RawExtension, checker *conversion.Equalities) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	aObj, errA := parseRawExtension(*a)
	bObj, errB := parseRawExtension(*b)
	if errA != nil && errB != nil {
		return checker.DeepEqual(a, b) // fallback to directly compare the object
	}
	if errA != nil || errB != nil {
		return false
	}
	return checker.DeepEqual(aObj, bObj)
}

func parseRawExtension(e runtime.RawExtension) (map[string]any, error) {
	j, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	if string(j) == "null" {
		return nil, nil
	}

	obj := make(map[string]any)
	err = json.Unmarshal(j, &obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}
