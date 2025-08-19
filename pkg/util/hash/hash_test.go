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
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeepHashObject(t *testing.T) {
	tests := []struct {
		name         string
		object1      any
		object2      any
		shouldBeSame bool
	}{
		{
			name: "Identical objects",
			object1: map[string]any{
				"key1": "value1",
				"key2": 42,
			},
			object2: map[string]any{
				"key1": "value1",
				"key2": 42,
			},
			shouldBeSame: true,
		},
		{
			name: "Different objects",
			object1: map[string]any{
				"key1": "value1",
				"key2": 42,
			},
			object2: map[string]any{
				"key1": "value1",
				"key2": 43,
			},
			shouldBeSame: false,
		},
		{
			name: "Different types",
			object1: map[string]any{
				"key1": "value1",
				"key2": 42,
			},
			object2:      []any{"value1", 42},
			shouldBeSame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasher1 := sha256.New()
			DeepHashObject(hasher1, tt.object1)
			hash1 := hex.EncodeToString(hasher1.Sum(nil))

			hasher2 := sha256.New()
			DeepHashObject(hasher2, tt.object2)
			hash2 := hex.EncodeToString(hasher2.Sum(nil))

			if tt.shouldBeSame {
				assert.Equal(t, hash1, hash2)
			} else {
				assert.NotEqual(t, hash1, hash2)
			}
		})
	}
}

// Test that calling with the same object multiple times
// results in the same hash with the same hasher
func TestDeepHashObjectMultipleTimes(t *testing.T) {
	object := map[string]any{
		"key1": "value1",
		"key2": 42,
	}

	hasher := sha256.New()
	DeepHashObject(hasher, object)
	hash1 := hex.EncodeToString(hasher.Sum(nil))

	DeepHashObject(hasher, object)
	hash2 := hex.EncodeToString(hasher.Sum(nil))

	assert.Equal(t, hash1, hash2)
}
