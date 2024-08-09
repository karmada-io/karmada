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

package hash

import (
	"crypto/sha256"
	"testing"
)

func TestDeepHashObject(t *testing.T) {
	tests := []struct {
		name   string
		object interface{}
	}{
		{
			name:   "simple string",
			object: "test string",
		},
		{
			name: "struct",
			object: struct {
				Name  string
				Value int
			}{
				Name:  "Test",
				Value: 42,
			},
		},
		{
			name: "nested struct",
			object: struct {
				Inner struct {
					Name  string
					Value int
				}
			}{
				Inner: struct {
					Name  string
					Value int
				}{
					Name:  "InnerTest",
					Value: 100,
				},
			},
		},
		{
			name:   "slice",
			object: []int{1, 2, 3, 4, 5},
		},
		{
			name:   "map",
			object: map[string]int{"a": 1, "b": 2, "c": 3},
		},
		{
			name: "pointer to struct",
			object: &struct {
				Name  string
				Value int
			}{
				Name:  "PtrTest",
				Value: 42,
			},
		},
		{
			name:   "nil value",
			object: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasher := sha256.New()
			DeepHashObject(hasher, tt.object)
			hash1 := hasher.Sum(nil)
			hasher.Reset()
			DeepHashObject(hasher, tt.object)
			hash2 := hasher.Sum(nil)

			if string(hash1) != string(hash2) {
				t.Errorf("hashes do not match for %s: %x != %x", tt.name, hash1, hash2)
			}
		})
	}
}
