/*
Copyright 2025 The Karmada Authors.

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

import "k8s.io/apimachinery/pkg/util/sets"

// MergeFinalizers merges the new finalizers into exist finalizers, and deduplicates the finalizers.
// The result is sorted.
func MergeFinalizers(existFinalizers, newFinalizers []string) []string {
	if existFinalizers == nil && newFinalizers == nil {
		return nil
	}

	finalizers := sets.New[string](existFinalizers...).Insert(newFinalizers...)
	return sets.List[string](finalizers)
}
