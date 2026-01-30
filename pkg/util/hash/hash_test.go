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

package hash

import (
	"hash/fnv"
	"testing"
)

func TestDeepHashObject(t *testing.T) {
	type testStruct struct {
		Name  string
		Value int
	}

	tests := []struct {
		name     string
		obj1     interface{}
		obj2     interface{}
		wantSame bool
	}{
		{
			name:     "identical structs produce same hash",
			obj1:     testStruct{Name: "test", Value: 42},
			obj2:     testStruct{Name: "test", Value: 42},
			wantSame: true,
		},
		{
			name:     "different structs produce different hash",
			obj1:     testStruct{Name: "test1", Value: 42},
			obj2:     testStruct{Name: "test2", Value: 42},
			wantSame: false,
		},
		{
			name:     "nil values produce same hash",
			obj1:     nil,
			obj2:     nil,
			wantSame: true,
		},
		{
			name:     "maps with same content produce same hash regardless of insertion order",
			obj1:     map[string]int{"a": 1, "b": 2},
			obj2:     map[string]int{"b": 2, "a": 1},
			wantSame: true,
		},
		{
			name:     "maps with different values produce different hash",
			obj1:     map[string]int{"a": 1, "b": 2},
			obj2:     map[string]int{"a": 1, "b": 3},
			wantSame: false,
		},
		{
			name:     "slices with same content produce same hash",
			obj1:     []string{"a", "b", "c"},
			obj2:     []string{"a", "b", "c"},
			wantSame: true,
		},
		{
			name:     "slices with different order produce different hash",
			obj1:     []string{"a", "b", "c"},
			obj2:     []string{"c", "b", "a"},
			wantSame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasher1 := fnv.New32a()
			hasher2 := fnv.New32a()

			DeepHashObject(hasher1, tt.obj1)
			DeepHashObject(hasher2, tt.obj2)

			hash1 := hasher1.Sum32()
			hash2 := hasher2.Sum32()

			if (hash1 == hash2) != tt.wantSame {
				t.Errorf("DeepHashObject() hash1 = %v, hash2 = %v, wantSame = %v", hash1, hash2, tt.wantSame)
			}
		})
	}
}

func TestDeepHashObject_Deterministic(t *testing.T) {
	obj := struct {
		Name   string
		Values []int
	}{
		Name:   "test",
		Values: []int{1, 2, 3},
	}

	hasher1 := fnv.New32a()
	hasher2 := fnv.New32a()

	DeepHashObject(hasher1, obj)
	DeepHashObject(hasher2, obj)

	if hasher1.Sum32() != hasher2.Sum32() {
		t.Errorf("DeepHashObject() should be deterministic")
	}
}

func TestDeepHashObject_HasherReset(t *testing.T) {
	hasher := fnv.New32a()

	// Hash something first
	DeepHashObject(hasher, "initial")

	// Hash a new object - should not be affected by previous state
	obj := struct{ Value int }{Value: 100}
	DeepHashObject(hasher, obj)
	hash1 := hasher.Sum32()

	// Fresh hasher with same object
	freshHasher := fnv.New32a()
	DeepHashObject(freshHasher, obj)
	hash2 := freshHasher.Sum32()

	if hash1 != hash2 {
		t.Errorf("DeepHashObject() should reset hasher, got %v vs %v", hash1, hash2)
	}
}
