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

package context

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

func TestContext_IsControllerEnabled(t *testing.T) {
	tests := []struct {
		name                         string
		controllerName               string
		controllers                  []string
		disabledByDefaultControllers []string
		expected                     bool
	}{
		{
			name:                         "on by name",
			controllerName:               "bravo",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"alpha", "bravo", "-charlie"}, // --controllers=alpha,bravo,-charlie
			expected:                     true,
		},
		{
			name:                         "off by name",
			controllerName:               "charlie",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"alpha", "bravo", "-charlie"}, // --controllers=alpha,bravo,-charlie
			expected:                     false,
		},
		{
			name:                         "on by default",
			controllerName:               "alpha",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"*"}, // --controllers=*
			expected:                     true,
		},
		{
			name:                         "off by default",
			controllerName:               "delta",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"*"}, // --controllers=*
			expected:                     false,
		},
		{
			name:                         "on by star, not off by name",
			controllerName:               "alpha",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"*", "-charlie"}, // --controllers=*,-charlie
			expected:                     true,
		},
		{
			name:                         "off by name with star",
			controllerName:               "charlie",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"*", "-charlie"}, // --controllers=*,-charlie
			expected:                     false,
		},
		{
			name:                         "off by default implicit, no star",
			controllerName:               "foxtrot",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"alpha", "bravo", "-charlie"}, // --controllers=alpha,bravo,-charlie
			expected:                     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Context{
				Opts: Options{
					Controllers: tt.controllers,
				},
			}
			if got := c.IsControllerEnabled(tt.controllerName, sets.New(tt.disabledByDefaultControllers...)); got != tt.expected {
				t.Errorf("expected %v, but got %v", tt.expected, got)
			}
		})
	}
}
