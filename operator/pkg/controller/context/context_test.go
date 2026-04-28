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

package context

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

func TestIsControllerEnabled(t *testing.T) {
	tests := []struct {
		name                         string
		controllers                  []string
		controllerName               string
		disabledByDefaultControllers sets.Set[string]
		expected                     bool
	}{
		{
			name:                         "Explicitly enabled controller",
			controllers:                  []string{"foo", "bar"},
			controllerName:               "foo",
			disabledByDefaultControllers: sets.New[string](),
			expected:                     true,
		},
		{
			name:                         "Explicitly disabled controller",
			controllers:                  []string{"-foo", "bar"},
			controllerName:               "foo",
			disabledByDefaultControllers: sets.New[string](),
			expected:                     false,
		},
		{
			name:                         "Controller not mentioned, no star",
			controllers:                  []string{"bar", "baz"},
			controllerName:               "foo",
			disabledByDefaultControllers: sets.New[string](),
			expected:                     false,
		},
		{
			name:                         "Controller not mentioned, with star",
			controllers:                  []string{"*", "bar"},
			controllerName:               "foo",
			disabledByDefaultControllers: sets.New[string](),
			expected:                     true,
		},
		{
			name:                         "Controller not mentioned, with star, disabled by default",
			controllers:                  []string{"*", "bar"},
			controllerName:               "foo",
			disabledByDefaultControllers: sets.New("foo"),
			expected:                     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := Context{Controllers: tt.controllers}
			result := ctx.IsControllerEnabled(tt.controllerName, tt.disabledByDefaultControllers)
			if result != tt.expected {
				t.Errorf("IsControllerEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestInitializersControllerNames(t *testing.T) {
	tests := []struct {
		name         string
		initializers Initializers
		expected     []string
	}{
		{
			name: "Three controllers",
			initializers: Initializers{
				"controller1": func(_ Context) (bool, error) { return true, nil },
				"controller2": func(_ Context) (bool, error) { return true, nil },
				"controller3": func(_ Context) (bool, error) { return true, nil },
			},
			expected: []string{"controller1", "controller2", "controller3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.initializers.ControllerNames()
			if len(result) != len(tt.expected) {
				t.Errorf("ControllerNames() returned %d names, want %d", len(result), len(tt.expected))
			}

			expectedSet := sets.NewString(tt.expected...)
			for _, name := range result {
				if !expectedSet.Has(name) {
					t.Errorf("ControllerNames() returned unexpected name: %s", name)
				}
			}
		})
	}
}
