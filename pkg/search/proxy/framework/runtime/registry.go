package runtime

import (
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"

	"github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	"github.com/karmada-io/karmada/pkg/search/proxy/framework"
	"github.com/karmada-io/karmada/pkg/search/proxy/store"
)

// PluginDependency holds dependency for plugins. It will be passed to PluginFactory when initializing Plugin.
type PluginDependency struct {
	RestConfig *rest.Config
	RestMapper meta.RESTMapper

	KubeFactory    informers.SharedInformerFactory
	KarmadaFactory externalversions.SharedInformerFactory

	MinRequestTimeout time.Duration

	Store store.Store
}

// PluginFactory is the function to create a plugin.
type PluginFactory func(dep PluginDependency) (framework.Plugin, error)

// Registry is a collection of all available plugins. The framework uses a
// registry to enable and initialize configured plugins.
// All plugins must be in the registry before initializing the framework.
type Registry []PluginFactory

// Register adds a new plugin to the registry.
func (r *Registry) Register(factory PluginFactory) {
	*r = append(*r, factory)
}

// Merge merges the provided registry to the current one.
func (r *Registry) Merge(in Registry) {
	*r = append(*r, in...)
}
