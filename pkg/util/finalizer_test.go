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

import (
	"reflect"
	"testing"
)

func TestMergeFinalizers(t *testing.T) {
	tests := []struct {
		name            string
		existFinalizers []string
		newFinalizers   []string
		expectedResult  []string
	}{
		{
			name:            "both nil",
			existFinalizers: nil,
			newFinalizers:   nil,
			expectedResult:  nil,
		},
		{
			name:            "exist finalizers is nil",
			existFinalizers: nil,
			newFinalizers:   []string{"finalizer1", "finalizer2"},
			expectedResult:  []string{"finalizer1", "finalizer2"},
		},
		{
			name:            "new finalizers is nil",
			existFinalizers: []string{"finalizer1", "finalizer2"},
			newFinalizers:   nil,
			expectedResult:  []string{"finalizer1", "finalizer2"},
		},
		{
			name:            "both empty",
			existFinalizers: []string{},
			newFinalizers:   []string{},
			expectedResult:  []string{},
		},
		{
			name:            "exist finalizers is empty",
			existFinalizers: []string{},
			newFinalizers:   []string{"finalizer1", "finalizer2"},
			expectedResult:  []string{"finalizer1", "finalizer2"},
		},
		{
			name:            "new finalizers is empty",
			existFinalizers: []string{"finalizer1", "finalizer2"},
			newFinalizers:   []string{},
			expectedResult:  []string{"finalizer1", "finalizer2"},
		},
		{
			name:            "no duplicates",
			existFinalizers: []string{"finalizer1", "finalizer2"},
			newFinalizers:   []string{"finalizer3", "finalizer4"},
			expectedResult:  []string{"finalizer1", "finalizer2", "finalizer3", "finalizer4"},
		},
		{
			name:            "with duplicates",
			existFinalizers: []string{"finalizer1", "finalizer2"},
			newFinalizers:   []string{"finalizer2", "finalizer3"},
			expectedResult:  []string{"finalizer1", "finalizer2", "finalizer3"},
		},
		{
			name:            "all duplicates",
			existFinalizers: []string{"finalizer1", "finalizer2"},
			newFinalizers:   []string{"finalizer1", "finalizer2"},
			expectedResult:  []string{"finalizer1", "finalizer2"},
		},
		{
			name:            "duplicates in exist finalizers",
			existFinalizers: []string{"finalizer1", "finalizer2", "finalizer1"},
			newFinalizers:   []string{"finalizer3"},
			expectedResult:  []string{"finalizer1", "finalizer2", "finalizer3"},
		},
		{
			name:            "duplicates in new finalizers",
			existFinalizers: []string{"finalizer1"},
			newFinalizers:   []string{"finalizer2", "finalizer3", "finalizer2"},
			expectedResult:  []string{"finalizer1", "finalizer2", "finalizer3"},
		},
		{
			name:            "duplicates in both",
			existFinalizers: []string{"finalizer1", "finalizer2", "finalizer1"},
			newFinalizers:   []string{"finalizer2", "finalizer3", "finalizer2"},
			expectedResult:  []string{"finalizer1", "finalizer2", "finalizer3"},
		},
		{
			name:            "single finalizer in exist",
			existFinalizers: []string{"finalizer1"},
			newFinalizers:   []string{"finalizer2"},
			expectedResult:  []string{"finalizer1", "finalizer2"},
		},
		{
			name:            "single duplicate finalizer",
			existFinalizers: []string{"finalizer1"},
			newFinalizers:   []string{"finalizer1"},
			expectedResult:  []string{"finalizer1"},
		},
		{
			name:            "sort with result",
			existFinalizers: []string{"finalizer3", "finalizer1", "finalizer2"},
			newFinalizers:   []string{"finalizer4", "finalizer5"},
			expectedResult:  []string{"finalizer1", "finalizer2", "finalizer3", "finalizer4", "finalizer5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeFinalizers(tt.existFinalizers, tt.newFinalizers)
			if !reflect.DeepEqual(result, tt.expectedResult) {
				t.Errorf("MergeFinalizers() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}
