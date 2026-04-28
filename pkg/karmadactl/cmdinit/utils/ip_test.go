/*
Copyright 2022 The Karmada Authors.

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

package utils

import (
	"testing"
)

func TestParseIP(t *testing.T) {
	tests := []struct {
		name     string
		IP       string
		expected int
	}{
		{"v4", "127.0.0.1", 4},
		{"v6", "1030::C9B4:FF12:48AA:1A2B", 6},
		{"v4-error", "333.0.0.0", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, IPType, _ := ParseIP(tt.IP)
			if IPType != tt.expected {
				t.Errorf("parse ip %d, want %d", IPType, tt.expected)
			}
		})
	}
}
