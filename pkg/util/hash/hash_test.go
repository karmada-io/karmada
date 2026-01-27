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

	type nestedStruct struct {
		Inner testStruct
		Tags  []string
	}

	tests := []struct {
		name     string
		obj1     interface{}
		obj2     interface{}
		wantSame bool
	}{
		{
			name:     "identical structs should produce same hash",
			obj1:     testStruct{Name: "test", Value: 42},
			obj2:     testStruct{Name: "test", Value: 42},
			wantSame: true,
		},
		{
			name:     "different struct values should produce different hash",
			obj1:     testStruct{Name: "test1", Value: 42},
			obj2:     testStruct{Name: "test2", Value: 42},
			wantSame: false,
		},
		{
			name:     "different struct field values should produce different hash",
			obj1:     testStruct{Name: "test", Value: 1},
			obj2:     testStruct{Name: "test", Value: 2},
			wantSame: false,
		},
		{
			name:     "nil values should produce same hash",
			obj1:     nil,
			obj2:     nil,
			wantSame: true,
		},
		{
			name:     "empty structs should produce same hash",
			obj1:     testStruct{},
			obj2:     testStruct{},
			wantSame: true,
		},
		{
			name:     "nested structs with same values should produce same hash",
			obj1:     nestedStruct{Inner: testStruct{Name: "inner", Value: 1}, Tags: []string{"a", "b"}},
			obj2:     nestedStruct{Inner: testStruct{Name: "inner", Value: 1}, Tags: []string{"a", "b"}},
			wantSame: true,
		},
		{
			name:     "nested structs with different values should produce different hash",
			obj1:     nestedStruct{Inner: testStruct{Name: "inner1", Value: 1}, Tags: []string{"a", "b"}},
			obj2:     nestedStruct{Inner: testStruct{Name: "inner2", Value: 1}, Tags: []string{"a", "b"}},
			wantSame: false,
		},
		{
			name:     "identical maps should produce same hash",
			obj1:     map[string]int{"a": 1, "b": 2},
			obj2:     map[string]int{"a": 1, "b": 2},
			wantSame: true,
		},
		{
			name:     "maps with different values should produce different hash",
			obj1:     map[string]int{"a": 1, "b": 2},
			obj2:     map[string]int{"a": 1, "b": 3},
			wantSame: false,
		},
		{
			name:     "maps with different keys should produce different hash",
			obj1:     map[string]int{"a": 1, "b": 2},
			obj2:     map[string]int{"a": 1, "c": 2},
			wantSame: false,
		},
		{
			name:     "maps with same content but different insertion order should produce same hash",
			obj1:     map[string]int{"a": 1, "b": 2},
			obj2:     map[string]int{"b": 2, "a": 1},
			wantSame: true,
		},
		{
			name:     "identical slices should produce same hash",
			obj1:     []string{"a", "b", "c"},
			obj2:     []string{"a", "b", "c"},
			wantSame: true,
		},
		{
			name:     "slices with different order should produce different hash",
			obj1:     []string{"a", "b", "c"},
			obj2:     []string{"c", "b", "a"},
			wantSame: false,
		},
		{
			name:     "slices with different length should produce different hash",
			obj1:     []string{"a", "b"},
			obj2:     []string{"a", "b", "c"},
			wantSame: false,
		},
		{
			name:     "empty slices should produce same hash",
			obj1:     []string{},
			obj2:     []string{},
			wantSame: true,
		},
		{
			name:     "identical integers should produce same hash",
			obj1:     42,
			obj2:     42,
			wantSame: true,
		},
		{
			name:     "different integers should produce different hash",
			obj1:     42,
			obj2:     43,
			wantSame: false,
		},
		{
			name:     "identical strings should produce same hash",
			obj1:     "hello",
			obj2:     "hello",
			wantSame: true,
		},
		{
			name:     "different strings should produce different hash",
			obj1:     "hello",
			obj2:     "world",
			wantSame: false,
		},
		{
			name:     "empty string and non-empty string should produce different hash",
			obj1:     "",
			obj2:     "hello",
			wantSame: false,
		},
		{
			name:     "identical booleans should produce same hash",
			obj1:     true,
			obj2:     true,
			wantSame: true,
		},
		{
			name:     "different booleans should produce different hash",
			obj1:     true,
			obj2:     false,
			wantSame: false,
		},
		{
			name:     "identical float64 should produce same hash",
			obj1:     3.14159,
			obj2:     3.14159,
			wantSame: true,
		},
		{
			name:     "different float64 should produce different hash",
			obj1:     3.14159,
			obj2:     2.71828,
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

func TestDeepHashObject_HasherIsReset(t *testing.T) {
	hasher := fnv.New32a()
	obj := struct{ Name string }{Name: "test"}

	// First call
	DeepHashObject(hasher, obj)
	hash1 := hasher.Sum32()

	// Second call with same object - should produce same hash because hasher is reset
	DeepHashObject(hasher, obj)
	hash2 := hasher.Sum32()

	if hash1 != hash2 {
		t.Errorf("DeepHashObject() should reset hasher before writing, got different hashes: %v vs %v", hash1, hash2)
	}
}

func TestDeepHashObject_HasherResetClearsPreviousState(t *testing.T) {
	hasher := fnv.New32a()

	// Write something to hasher first
	DeepHashObject(hasher, "initial data")

	// Now hash a different object - should not be affected by previous state
	obj := struct{ Value int }{Value: 100}
	DeepHashObject(hasher, obj)
	hash1 := hasher.Sum32()

	// Create fresh hasher and hash same object
	freshHasher := fnv.New32a()
	DeepHashObject(freshHasher, obj)
	hash2 := freshHasher.Sum32()

	if hash1 != hash2 {
		t.Errorf("DeepHashObject() should produce same hash regardless of previous hasher state, got %v vs %v", hash1, hash2)
	}
}

func TestDeepHashObject_Deterministic(t *testing.T) {
	obj := struct {
		Name   string
		Values []int
		Meta   map[string]string
	}{
		Name:   "test-object",
		Values: []int{1, 2, 3},
		Meta:   map[string]string{"key": "value"},
	}

	// Hash the same object multiple times
	hashes := make([]uint32, 10)
	for i := 0; i < 10; i++ {
		hasher := fnv.New32a()
		DeepHashObject(hasher, obj)
		hashes[i] = hasher.Sum32()
	}

	// All hashes should be identical
	for i := 1; i < len(hashes); i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("DeepHashObject() should be deterministic, hash[%d]=%v differs from hash[0]=%v", i, hashes[i], hashes[0])
		}
	}
}

func TestDeepHashObject_NilPointerVsNonNil(t *testing.T) {
	type outer struct {
		Ptr *struct{ Value int }
	}

	obj1 := outer{Ptr: nil}
	obj2 := outer{Ptr: &struct{ Value int }{Value: 0}}

	hasher1 := fnv.New32a()
	hasher2 := fnv.New32a()

	DeepHashObject(hasher1, obj1)
	DeepHashObject(hasher2, obj2)

	hash1 := hasher1.Sum32()
	hash2 := hasher2.Sum32()

	// nil pointer should produce different hash than pointer to zero value
	if hash1 == hash2 {
		t.Errorf("DeepHashObject() nil pointer should produce different hash than pointer to zero value")
	}
}

func TestDeepHashObject_DifferentHasherTypes(t *testing.T) {
	obj := struct{ Name string }{Name: "test"}

	// Test with fnv.New32a
	hasher32a := fnv.New32a()
	DeepHashObject(hasher32a, obj)
	hash32a := hasher32a.Sum32()

	// Test with fnv.New64a
	hasher64a := fnv.New64a()
	DeepHashObject(hasher64a, obj)
	hash64a := hasher64a.Sum64()

	// Different hasher types should work correctly
	if hash32a == 0 {
		t.Errorf("fnv.New32a should produce non-zero hash")
	}
	if hash64a == 0 {
		t.Errorf("fnv.New64a should produce non-zero hash")
	}

	// Same hasher type should be consistent
	hasher32a2 := fnv.New32a()
	DeepHashObject(hasher32a2, obj)
	hash32a2 := hasher32a2.Sum32()

	if hash32a != hash32a2 {
		t.Errorf("Same hasher type should produce consistent results, got %v vs %v", hash32a, hash32a2)
	}
}

func TestDeepHashObject_ComplexNestedStructure(t *testing.T) {
	type Address struct {
		Street string
		City   string
	}
	type Person struct {
		Name    string
		Age     int
		Address Address
	}

	person1 := Person{
		Name: "John",
		Age:  30,
		Address: Address{
			Street: "123 Main St",
			City:   "Anytown",
		},
	}

	person2 := Person{
		Name: "John",
		Age:  30,
		Address: Address{
			Street: "123 Main St",
			City:   "Anytown",
		},
	}

	person3 := Person{
		Name: "John",
		Age:  30,
		Address: Address{
			Street: "456 Oak Ave",
			City:   "Anytown",
		},
	}

	hasher1 := fnv.New32a()
	hasher2 := fnv.New32a()
	hasher3 := fnv.New32a()

	DeepHashObject(hasher1, person1)
	DeepHashObject(hasher2, person2)
	DeepHashObject(hasher3, person3)

	hash1 := hasher1.Sum32()
	hash2 := hasher2.Sum32()
	hash3 := hasher3.Sum32()

	if hash1 != hash2 {
		t.Errorf("Identical nested structs should produce same hash, got %v vs %v", hash1, hash2)
	}

	if hash1 == hash3 {
		t.Errorf("Different nested structs should produce different hash, both got %v", hash1)
	}
}
