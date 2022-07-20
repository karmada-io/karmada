package runtime

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

// PluginFactory is a function that builds a plugin.
type PluginFactory = func() (framework.Plugin, error)

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

// Merge merges the provided registry to the current one.
func (r Registry) Merge(in Registry) error {
	for name, factory := range in {
		if err := r.Register(name, factory); err != nil {
			return err
		}
	}
	return nil
}

// FactoryNames returns all known plugin names
func (r Registry) FactoryNames() []string {
	return sets.StringKeySet(r).List()
}

// Filter out the disabled plugin
func (r Registry) Filter(names []string) Registry {
	var retRegistry = make(Registry)
	for _, name := range names {
		// --plugins=*
		if name == "*" {
			for factoryName, factory := range r {
				klog.Infof("Enable Scheduler plugin %q", factoryName)
				retRegistry[factoryName] = factory
			}
			break
		}
	}

	for _, name := range names {
		// --plugins=foo
		if factory, ok := r[name]; ok {
			retRegistry[name] = factory
			klog.Infof("Enable Scheduler plugin %q", name)
			continue
		}
		// --plugins=*,-foo
		// --plugins=-foo,*
		if strings.HasPrefix(name, "-") && len(retRegistry) > 0 {
			factoryName := strings.TrimLeft(name, "-")
			delete(retRegistry, factoryName)
			klog.Warningf("%q is disabled", factoryName)
		}
	}

	return retRegistry
}
