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
	"errors"
	"reflect"
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
			name:                         "on by star, not in disabled list",
			controllerName:               "foxtrot",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{"*"}, // --controllers=*
			expected:                     true,
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
		{
			name:                         "empty controllers list",
			controllerName:               "alpha",
			disabledByDefaultControllers: []string{"delta", "echo"},
			controllers:                  []string{}, // No controllers
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

func TestInitializers_ControllerNames(t *testing.T) {
	initializers := Initializers{
		"controller1": func(_ Context) (bool, error) { return true, nil },
		"controller2": func(_ Context) (bool, error) { return true, nil },
		"controller3": func(_ Context) (bool, error) { return true, nil },
	}

	expected := []string{"controller1", "controller2", "controller3"}
	result := initializers.ControllerNames()

	if !reflect.DeepEqual(sets.New(result...), sets.New(expected...)) {
		t.Errorf("expected %v, but got %v", expected, result)
	}
}

func TestInitializers_StartControllers(t *testing.T) {
	tests := []struct {
		name                         string
		initializers                 Initializers
		enabledControllers           []string
		disabledByDefaultControllers []string
		expectedError                bool
	}{
		{
			name: "all controllers enabled and started successfully",
			initializers: Initializers{
				"controller1": func(_ Context) (bool, error) { return true, nil },
				"controller2": func(_ Context) (bool, error) { return true, nil },
			},
			enabledControllers:           []string{"*"},
			disabledByDefaultControllers: []string{},
			expectedError:                false,
		},
		{
			name: "some controllers disabled",
			initializers: Initializers{
				"controller1": func(_ Context) (bool, error) { return true, nil },
				"controller2": func(_ Context) (bool, error) { return true, nil },
				"controller3": func(_ Context) (bool, error) { return true, nil },
			},
			enabledControllers:           []string{"controller1", "controller2"},
			disabledByDefaultControllers: []string{"controller3"},
			expectedError:                false,
		},
		{
			name: "controller returns error",
			initializers: Initializers{
				"controller1": func(_ Context) (bool, error) { return true, nil },
				"controller2": func(_ Context) (bool, error) { return false, errors.New("test error") },
			},
			enabledControllers:           []string{"*"},
			disabledByDefaultControllers: []string{},
			expectedError:                true,
		},
		{
			name: "controller not started",
			initializers: Initializers{
				"controller1": func(_ Context) (bool, error) { return true, nil },
				"controller2": func(_ Context) (bool, error) { return false, nil },
			},
			enabledControllers:           []string{"*"},
			disabledByDefaultControllers: []string{},
			expectedError:                false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := Context{
				Opts: Options{
					Controllers: tt.enabledControllers,
				},
			}
			err := tt.initializers.StartControllers(ctx, sets.New(tt.disabledByDefaultControllers...))
			if (err != nil) != tt.expectedError {
				t.Errorf("expected error: %v, but got: %v", tt.expectedError, err)
			}
		})
	}
}
