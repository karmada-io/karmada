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

package plugins

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// mockPlugin is a mock implementation of EvictorPlugin for testing
type mockPlugin struct {
	name          string
	canEvictRB    bool
	canEvictCRB   bool
	rbCalled      bool
	crbCalled     bool
	lastTask      *workv1alpha2.GracefulEvictionTask
	lastBinding   *workv1alpha2.ResourceBinding
	lastCRBinding *workv1alpha2.ClusterResourceBinding
}

func (m *mockPlugin) Name() string {
	return m.name
}

func (m *mockPlugin) CanEvictRB(ctx context.Context, task *workv1alpha2.GracefulEvictionTask, binding *workv1alpha2.ResourceBinding) bool {
	m.rbCalled = true
	m.lastTask = task
	m.lastBinding = binding
	return m.canEvictRB
}

func (m *mockPlugin) CanEvictCRB(ctx context.Context, task *workv1alpha2.GracefulEvictionTask, binding *workv1alpha2.ClusterResourceBinding) bool {
	m.crbCalled = true
	m.lastTask = task
	m.lastCRBinding = binding
	return m.canEvictCRB
}

func newMockPlugin(name string, canEvictRB, canEvictCRB bool) *mockPlugin {
	return &mockPlugin{
		name:        name,
		canEvictRB:  canEvictRB,
		canEvictCRB: canEvictCRB,
	}
}

func TestRegisterPlugin(t *testing.T) {
	// Clear the registry before test
	gPluginRegistryLock.Lock()
	gPluginRegistry = make(map[string]PluginFactory)
	gPluginRegistryLock.Unlock()

	tests := []struct {
		name     string
		plugin   string
		factory  PluginFactory
		expected bool
	}{
		{
			name:   "register valid plugin",
			plugin: "test-plugin",
			factory: func() (EvictorPlugin, error) {
				return newMockPlugin("test-plugin", true, true), nil
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterPlugin(tt.plugin, tt.factory)

			gPluginRegistryLock.RLock()
			_, exists := gPluginRegistry[tt.plugin]
			gPluginRegistryLock.RUnlock()

			if exists != tt.expected {
				t.Errorf("RegisterPlugin() = %v, want %v", exists, tt.expected)
			}
		})
	}
}

func TestNewManager(t *testing.T) {
	// Clear the registry and register test plugins
	gPluginRegistryLock.Lock()
	gPluginRegistry = make(map[string]PluginFactory)
	gPluginRegistry["plugin1"] = func() (EvictorPlugin, error) {
		return newMockPlugin("plugin1", true, false), nil
	}
	gPluginRegistry["plugin2"] = func() (EvictorPlugin, error) {
		return newMockPlugin("plugin2", false, true), nil
	}
	gPluginRegistry["plugin3"] = func() (EvictorPlugin, error) {
		return newMockPlugin("plugin3", true, true), nil
	}
	gPluginRegistryLock.Unlock()

	tests := []struct {
		name        string
		pluginNames []string
		wantErr     bool
		wantCount   int
	}{
		{
			name:        "create manager with valid plugins",
			pluginNames: []string{"plugin1", "plugin2"},
			wantErr:     false,
			wantCount:   2,
		},
		{
			name:        "create manager with non-existent plugin",
			pluginNames: []string{"plugin1", "non-existent"},
			wantErr:     true,
			wantCount:   0,
		},
		{
			name:        "create manager with wildcard",
			pluginNames: []string{"*"},
			wantErr:     false,
			wantCount:   3, // all registered plugins
		},
		{
			name:        "create manager with empty list",
			pluginNames: []string{},
			wantErr:     false,
			wantCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.pluginNames)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(manager.enabledPlugins) != tt.wantCount {
					t.Errorf("NewManager() enabled plugins count = %v, want %v", len(manager.enabledPlugins), tt.wantCount)
				}
			}
		})
	}
}

func TestManager_CanBeCleanedRB(t *testing.T) {
	tests := []struct {
		name        string
		plugins     []EvictorPlugin
		expectClean bool
	}{
		{
			name:        "no plugins enabled",
			plugins:     []EvictorPlugin{},
			expectClean: false,
		},
		{
			name: "all plugins return false",
			plugins: []EvictorPlugin{
				newMockPlugin("plugin1", false, false),
				newMockPlugin("plugin2", false, false),
			},
			expectClean: false,
		},
		{
			name: "one plugin returns true",
			plugins: []EvictorPlugin{
				newMockPlugin("plugin1", false, false),
				newMockPlugin("plugin2", true, false),
				newMockPlugin("plugin3", false, false),
			},
			expectClean: true,
		},
		{
			name: "all plugins return true",
			plugins: []EvictorPlugin{
				newMockPlugin("plugin1", true, false),
				newMockPlugin("plugin2", true, false),
			},
			expectClean: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				enabledPlugins: tt.plugins,
			}

			task := &workv1alpha2.GracefulEvictionTask{
				FromCluster: "test-cluster",
			}
			binding := &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "test-namespace",
				},
			}

			result := manager.CanBeCleanedRB(context.Background(), task, binding)

			if result != tt.expectClean {
				t.Errorf("CanBeCleanedRB() = %v, want %v", result, tt.expectClean)
			}

			// Verify that at least one plugin was called if we expect cleanup
			// or all plugins were called if we don't expect cleanup
			calledCount := 0
			for _, plugin := range tt.plugins {
				mockPlugin := plugin.(*mockPlugin)
				if mockPlugin.rbCalled {
					calledCount++
					if mockPlugin.lastTask != task {
						t.Errorf("Plugin %s received wrong task", mockPlugin.name)
					}
					if mockPlugin.lastBinding != binding {
						t.Errorf("Plugin %s received wrong binding", mockPlugin.name)
					}
				}
			}

			// Basic verification: if we expect cleanup, at least one plugin should be called
			// if we don't expect cleanup, either no plugins or all plugins should be called
			if tt.expectClean && calledCount == 0 {
				t.Errorf("Expected cleanup but no plugins were called")
			}
		})
	}
}

func TestManager_CanBeCleanedCRB(t *testing.T) {
	tests := []struct {
		name        string
		plugins     []EvictorPlugin
		expectClean bool
	}{
		{
			name:        "no plugins enabled",
			plugins:     []EvictorPlugin{},
			expectClean: false,
		},
		{
			name: "all plugins return false",
			plugins: []EvictorPlugin{
				newMockPlugin("plugin1", false, false),
				newMockPlugin("plugin2", false, false),
			},
			expectClean: false,
		},
		{
			name: "one plugin returns true",
			plugins: []EvictorPlugin{
				newMockPlugin("plugin1", false, false),
				newMockPlugin("plugin2", false, true),
				newMockPlugin("plugin3", false, false),
			},
			expectClean: true,
		},
		{
			name: "all plugins return true",
			plugins: []EvictorPlugin{
				newMockPlugin("plugin1", false, true),
				newMockPlugin("plugin2", false, true),
			},
			expectClean: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				enabledPlugins: tt.plugins,
			}

			task := &workv1alpha2.GracefulEvictionTask{
				FromCluster: "test-cluster",
			}
			binding := &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
				},
			}

			result := manager.CanBeCleanedCRB(context.Background(), task, binding)

			if result != tt.expectClean {
				t.Errorf("CanBeCleanedCRB() = %v, want %v", result, tt.expectClean)
			}

			// Verify that at least one plugin was called if we expect cleanup
			// or all plugins were called if we don't expect cleanup
			calledCount := 0
			for _, plugin := range tt.plugins {
				mockPlugin := plugin.(*mockPlugin)
				if mockPlugin.crbCalled {
					calledCount++
					if mockPlugin.lastTask != task {
						t.Errorf("Plugin %s received wrong task", mockPlugin.name)
					}
					if mockPlugin.lastCRBinding != binding {
						t.Errorf("Plugin %s received wrong binding", mockPlugin.name)
					}
				}
			}

			// Basic verification: if we expect cleanup, at least one plugin should be called
			// if we don't expect cleanup, either no plugins or all plugins should be called
			if tt.expectClean && calledCount == 0 {
				t.Errorf("Expected cleanup but no plugins were called")
			}
		})
	}
}

func TestRegisteredPlugins(t *testing.T) {
	// Clear the registry and register test plugins
	gPluginRegistryLock.Lock()
	gPluginRegistry = make(map[string]PluginFactory)
	gPluginRegistry["plugin1"] = func() (EvictorPlugin, error) {
		return newMockPlugin("plugin1", true, true), nil
	}
	gPluginRegistry["plugin2"] = func() (EvictorPlugin, error) {
		return newMockPlugin("plugin2", true, true), nil
	}
	gPluginRegistryLock.Unlock()

	plugins := RegisteredPlugins()

	expectedCount := 2
	if len(plugins) != expectedCount {
		t.Errorf("RegisteredPlugins() returned %d plugins, want %d", len(plugins), expectedCount)
	}

	// Check that both plugins are in the list
	pluginMap := make(map[string]bool)
	for _, plugin := range plugins {
		pluginMap[plugin] = true
	}

	if !pluginMap["plugin1"] || !pluginMap["plugin2"] {
		t.Errorf("RegisteredPlugins() missing expected plugins")
	}
}

func TestIsRegistered(t *testing.T) {
	// Clear the registry and register a test plugin
	gPluginRegistryLock.Lock()
	gPluginRegistry = make(map[string]PluginFactory)
	gPluginRegistry["test-plugin"] = func() (EvictorPlugin, error) {
		return newMockPlugin("test-plugin", true, true), nil
	}
	gPluginRegistryLock.Unlock()

	tests := []struct {
		name     string
		plugin   string
		expected bool
	}{
		{
			name:     "registered plugin",
			plugin:   "test-plugin",
			expected: true,
		},
		{
			name:     "non-registered plugin",
			plugin:   "non-existent",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRegistered(tt.plugin)
			if result != tt.expected {
				t.Errorf("IsRegistered() = %v, want %v", result, tt.expected)
			}
		})
	}
}
