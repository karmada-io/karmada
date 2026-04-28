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

package runtime

import (
	"testing"

	"github.com/google/uuid"

	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
)

type mockNoopPlugin struct {
	uuid string
}

func (p *mockNoopPlugin) Name() string {
	return p.uuid
}

func NewMockNoopPluginFactory() PluginFactory {
	uuid := uuid.New().String()
	return func(_ framework.Handle) (framework.Plugin, error) {
		return &mockNoopPlugin{uuid}, nil
	}
}

// isRegistryEqual compares two registries for equality. This function is used in place of
// reflect.DeepEqual() and cmp() as they don't compare function values.
func isRegistryEqual(registryX, registryY Registry) bool {
	for name, pluginFactory := range registryY {
		if val, ok := registryX[name]; ok {
			p1, _ := pluginFactory(nil)
			p2, _ := val(nil)
			if p1.Name() != p2.Name() {
				// pluginFactory functions are not the same.
				return false
			}
		} else {
			// registryY contains an entry that is not present in registryX
			return false
		}
	}
	for name := range registryX {
		if _, ok := registryY[name]; !ok {
			// registryX contains an entry that is not present in registryY
			return false
		}
	}
	return true
}

// TestRegistry_Register is same as https://github.com/kubernetes/kubernetes/blob/master/pkg/scheduler/framework/runtime/registry_test.go#L174-L223
// because the framework registry design is same as kubernetes framework register
func TestRegistry_Register(t *testing.T) {
	m1 := NewMockNoopPluginFactory()
	m2 := NewMockNoopPluginFactory()
	tests := []struct {
		name              string
		registry          Registry
		nameToRegister    string
		factoryToRegister PluginFactory
		expected          Registry
		shouldError       bool
	}{
		{
			name:              "valid Register",
			registry:          Registry{},
			nameToRegister:    "pluginFactory1",
			factoryToRegister: m1,
			expected: Registry{
				"pluginFactory1": m1,
			},
			shouldError: false,
		},
		{
			name: "Register duplicate factories",
			registry: Registry{
				"pluginFactory1": m1,
			},
			nameToRegister:    "pluginFactory1",
			factoryToRegister: m2,
			expected: Registry{
				"pluginFactory1": m1,
			},
			shouldError: true,
		},
	}

	for _, scenario := range tests {
		t.Run(scenario.name, func(t *testing.T) {
			err := scenario.registry.Register(scenario.nameToRegister, scenario.factoryToRegister)

			if (err == nil) == scenario.shouldError {
				t.Errorf("Register() shouldError is: %v however err is: %v.", scenario.shouldError, err)
				return
			}

			if !isRegistryEqual(scenario.expected, scenario.registry) {
				t.Errorf("Register(). Expected %v. Got %v instead.", scenario.expected, scenario.registry)
			}
		})
	}
}

// TestRegistry_Unregister is same as https://github.com/kubernetes/kubernetes/blob/master/pkg/scheduler/framework/runtime/registry_test.go#L225-L270
// because the framework registry design is same as kubernetes framework register
func TestRegistry_Unregister(t *testing.T) {
	m1 := NewMockNoopPluginFactory()
	m2 := NewMockNoopPluginFactory()
	tests := []struct {
		name             string
		registry         Registry
		nameToUnregister string
		expected         Registry
		shouldError      bool
	}{
		{
			name: "valid Unregister",
			registry: Registry{
				"pluginFactory1": m1,
				"pluginFactory2": m2,
			},
			nameToUnregister: "pluginFactory1",
			expected: Registry{
				"pluginFactory2": m2,
			},
			shouldError: false,
		},
		{
			name:             "Unregister non-existent plugin factory",
			registry:         Registry{},
			nameToUnregister: "pluginFactory1",
			expected:         Registry{},
			shouldError:      true,
		},
	}

	for _, scenario := range tests {
		t.Run(scenario.name, func(t *testing.T) {
			err := scenario.registry.Unregister(scenario.nameToUnregister)

			if (err == nil) == scenario.shouldError {
				t.Errorf("Unregister() shouldError is: %v however err is: %v.", scenario.shouldError, err)
				return
			}

			if !isRegistryEqual(scenario.expected, scenario.registry) {
				t.Errorf("Unregister(). Expected %v. Got %v instead.", scenario.expected, scenario.registry)
			}
		})
	}
}

// TestRegistry_Merge is same as https://github.com/kubernetes/kubernetes/blob/master/pkg/scheduler/framework/runtime/registry_test.go#L119-L172
// because the framework registry design is same as kubernetes framework register
func TestRegistry_Merge(t *testing.T) {
	m1 := NewMockNoopPluginFactory()
	m2 := NewMockNoopPluginFactory()
	tests := []struct {
		name            string
		primaryRegistry Registry
		registryToMerge Registry
		expected        Registry
		shouldError     bool
	}{
		{
			name: "valid Merge",
			primaryRegistry: Registry{
				"pluginFactory1": m1,
			},
			registryToMerge: Registry{
				"pluginFactory2": m2,
			},
			expected: Registry{
				"pluginFactory1": m1,
				"pluginFactory2": m2,
			},
			shouldError: false,
		},
		{
			name: "Merge duplicate factories",
			primaryRegistry: Registry{
				"pluginFactory1": m1,
			},
			registryToMerge: Registry{
				"pluginFactory1": m2,
			},
			expected: Registry{
				"pluginFactory1": m1,
			},
			shouldError: true,
		},
	}

	for _, scenario := range tests {
		t.Run(scenario.name, func(t *testing.T) {
			err := scenario.primaryRegistry.Merge(scenario.registryToMerge)

			if (err == nil) == scenario.shouldError {
				t.Errorf("Merge() shouldError is: %v, however err is: %v.", scenario.shouldError, err)
				return
			}

			if !isRegistryEqual(scenario.expected, scenario.primaryRegistry) {
				t.Errorf("Merge(). Expected %v. Got %v instead.", scenario.expected, scenario.primaryRegistry)
			}
		})
	}
}
