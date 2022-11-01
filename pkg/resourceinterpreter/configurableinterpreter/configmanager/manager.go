package configmanager

import (
	"fmt"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

var resourceInterpreterCustomizationsGVR = schema.GroupVersionResource{
	Group:    configv1alpha1.GroupVersion.Group,
	Version:  configv1alpha1.GroupVersion.Version,
	Resource: "resourceinterpretercustomizations",
}

// ConfigManager can list custom resource interpreter.
type ConfigManager interface {
	LuaScriptAccessors() map[schema.GroupVersionKind]LuaScriptAccessor
	HasSynced() bool
}

// interpreterConfigManager collects the resource interpreter customization.
type interpreterConfigManager struct {
	initialSynced *atomic.Value
	lister        cache.GenericLister
	configuration *atomic.Value
}

// LuaScriptAccessors returns all cached configurations.
func (configManager *interpreterConfigManager) LuaScriptAccessors() map[schema.GroupVersionKind]LuaScriptAccessor {
	return configManager.configuration.Load().(map[schema.GroupVersionKind]LuaScriptAccessor)
}

// HasSynced returns true when the cache is synced.
func (configManager *interpreterConfigManager) HasSynced() bool {
	if configManager.initialSynced.Load().(bool) {
		return true
	}

	if configManager.HasSynced() {
		configManager.initialSynced.Store(true)
		return true
	}
	return false
}

// NewInterpreterConfigManager watches ResourceInterpreterCustomization and organizes
// the configurations in the cache.
func NewInterpreterConfigManager(inform genericmanager.SingleClusterInformerManager) ConfigManager {
	manager := &interpreterConfigManager{
		lister:        inform.Lister(resourceInterpreterCustomizationsGVR),
		initialSynced: &atomic.Value{},
		configuration: &atomic.Value{},
	}
	manager.configuration.Store(make(map[schema.GroupVersionKind]LuaScriptAccessor))
	manager.initialSynced.Store(false)
	configHandlers := fedinformer.NewHandlerOnEvents(
		func(_ interface{}) { manager.updateConfiguration() },
		func(_, _ interface{}) { manager.updateConfiguration() },
		func(_ interface{}) { manager.updateConfiguration() })
	inform.ForResource(resourceInterpreterCustomizationsGVR, configHandlers)
	return manager
}

func (configManager *interpreterConfigManager) updateConfiguration() {
	configurations, err := configManager.lister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error updating configuration: %v", err))
		return
	}
	configs := make(map[schema.GroupVersionKind]CustomAccessor, len(configurations))

	for _, c := range configurations {
		config := &configv1alpha1.ResourceInterpreterCustomization{}
		if err = helper.ConvertToTypedObject(c, config); err != nil {
			klog.Errorf("Failed to transform ResourceInterpreterCustomization: %w", err)
			return
		}
		key := schema.FromAPIVersionAndKind(config.Spec.Target.APIVersion, config.Spec.Target.Kind)
		configs[key] = NewResourceCustomAccessorAccessor(config)
	}

	configManager.configuration.Store(configs)
	configManager.initialSynced.Store(true)
}
