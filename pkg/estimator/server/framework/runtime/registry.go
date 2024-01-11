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

package runtime

import (
	"fmt"

	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
	plfeature "github.com/karmada-io/karmada/pkg/estimator/server/framework/plugins/feature"
)

// PluginFactory is a function that builds a plugin.
type PluginFactory = func(f framework.Handle) (framework.Plugin, error)

// PluginFactoryWithFts is a function that builds a plugin with certain feature gates.
type PluginFactoryWithFts func(framework.Handle, plfeature.Features) (framework.Plugin, error)

// FactoryAdapter can be used to inject feature gates for a plugin that needs
// them when the caller expects the older PluginFactory method.
func FactoryAdapter(fts plfeature.Features, withFts PluginFactoryWithFts) PluginFactory {
	return func(fh framework.Handle) (framework.Plugin, error) {
		return withFts(fh, fts)
	}
}

// Registry is a collection of all available plugins. The framework uses a
// registry to enable and initialize configured plugins.
// All plugins must be in the registry before initializing the framework.
type Registry map[string]PluginFactory

// Register adds a new plugin to the registry. If a plugin with the same name
// exists, it returns an error.
func (r Registry) Register(name string, factory PluginFactory) error {
	if _, ok := r[name]; ok {
		return fmt.Errorf("a plugin named %v already exists", name)
	}
	r[name] = factory
	return nil
}

// Unregister removes an existing plugin from the registry. If no plugin with
// the provided name exists, it returns an error.
func (r Registry) Unregister(name string) error {
	if _, ok := r[name]; !ok {
		return fmt.Errorf("no plugin named %v exists", name)
	}
	delete(r, name)
	return nil
}
