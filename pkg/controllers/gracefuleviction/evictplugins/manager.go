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
	"fmt"
	"sync"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"k8s.io/klog/v2"
)

var (
	// gPluginRegistry stores all registered plugin factories.
	gPluginRegistry = make(map[string]PluginFactory)
	// gPluginRegistryLock is a lock for gPluginRegistry.
	gPluginRegistryLock sync.RWMutex
)

// RegisterPlugin registers a plugin factory. It is expected to be called during plugin initialization.
func RegisterPlugin(name string, factory PluginFactory) {
	gPluginRegistryLock.Lock()
	defer gPluginRegistryLock.Unlock()
	gPluginRegistry[name] = factory
}

// Manager holds all enabled plugins and runs them.
type Manager struct {
	enabledPlugins []EvictorPlugin
}

// NewManager creates a new plugin manager with a set of enabled plugins.
func NewManager(pluginNames []string) (*Manager, error) {
	manager := &Manager{}
	gPluginRegistryLock.RLock()
	defer gPluginRegistryLock.RUnlock()

	// Handle the wildcard case to enable all registered plugins.
	if len(pluginNames) == 1 && pluginNames[0] == "*" {
		pluginNames = make([]string, 0, len(gPluginRegistry))
		for name := range gPluginRegistry {
			pluginNames = append(pluginNames, name)
		}
	}

	for _, name := range pluginNames {
		factory, ok := gPluginRegistry[name]
		if !ok {
			return nil, fmt.Errorf("plugin %q not found", name)
		}
		plugin, err := factory()
		if err != nil {
			return nil, fmt.Errorf("failed to create plugin %q: %v", name, err)
		}
		manager.enabledPlugins = append(manager.enabledPlugins, plugin)
		klog.Infof("Graceful eviction plugin %q enabled.", name)
	}
	return manager, nil
}

// CanBeCleanedRB runs all enabled plugins for a ResourceBinding.
// If any of them returns true, this function returns true.
func (m *Manager) CanBeCleanedRB(ctx context.Context, task *workv1alpha2.GracefulEvictionTask, binding *workv1alpha2.ResourceBinding) bool {
	for _, plugin := range m.enabledPlugins {
		if plugin.CanEvictRB(ctx, task, binding) {
			klog.V(4).Infof("Eviction task for cluster %s on ResourceBinding %s/%s can be cleaned, passed by plugin %s.", task.FromCluster, binding.Namespace, binding.Name, plugin.Name())
			return true
		}
	}
	return false
}

// CanBeCleanedCRB runs all enabled plugins for a ClusterResourceBinding.
// If any of them returns true, this function returns true.
func (m *Manager) CanBeCleanedCRB(ctx context.Context, task *workv1alpha2.GracefulEvictionTask, binding *workv1alpha2.ClusterResourceBinding) bool {
	for _, plugin := range m.enabledPlugins {
		if plugin.CanEvictCRB(ctx, task, binding) {
			klog.V(4).Infof("Eviction task for cluster %s on ClusterResourceBinding %s can be cleaned, passed by plugin %s.", task.FromCluster, binding.Name, plugin.Name())
			return true
		}
	}
	return false
}

// RegisteredPlugins returns a list of all registered plugin names.
func RegisteredPlugins() []string {
	gPluginRegistryLock.RLock()
	defer gPluginRegistryLock.RUnlock()

	names := make([]string, 0, len(gPluginRegistry))
	for name := range gPluginRegistry {
		names = append(names, name)
	}
	return names
}

// IsRegistered checks if a plugin with the given name is registered.
func IsRegistered(name string) bool {
	gPluginRegistryLock.RLock()
	defer gPluginRegistryLock.RUnlock()

	_, ok := gPluginRegistry[name]
	return ok
}
