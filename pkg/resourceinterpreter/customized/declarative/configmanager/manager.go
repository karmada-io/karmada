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

package configmanager

import (
	"fmt"
	"sort"
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
	CustomAccessors() map[schema.GroupVersionKind]CustomAccessor
	HasSynced() bool
	LoadConfig(customizations []*configv1alpha1.ResourceInterpreterCustomization)
}

// interpreterConfigManager collects the resource interpreter customization.
type interpreterConfigManager struct {
	initialSynced atomic.Bool
	lister        cache.GenericLister
	configuration atomic.Value
}

// CustomAccessors returns all cached configurations.
func (configManager *interpreterConfigManager) CustomAccessors() map[schema.GroupVersionKind]CustomAccessor {
	return configManager.configuration.Load().(map[schema.GroupVersionKind]CustomAccessor)
}

// HasSynced returns true when the cache is synced.
func (configManager *interpreterConfigManager) HasSynced() bool {
	if configManager.initialSynced.Load() {
		return true
	}

	if configuration, err := configManager.lister.List(labels.Everything()); err == nil && len(configuration) == 0 {
		// the empty list we initially stored is valid to use.
		// Setting initialSynced to true, so subsequent checks
		// would be able to take the fast path on the atomic boolean in a
		// cluster without any customization configured.
		configManager.initialSynced.Store(true)
		// the informer has synced, and we don't have any items
		return true
	}
	return false
}

// NewInterpreterConfigManager watches ResourceInterpreterCustomization and organizes
// the configurations in the cache.
func NewInterpreterConfigManager(informer genericmanager.SingleClusterInformerManager) ConfigManager {
	manager := &interpreterConfigManager{}
	manager.configuration.Store(make(map[schema.GroupVersionKind]CustomAccessor))

	// In interpret command, rules are not loaded from server, so we don't start informer for it.
	if informer != nil {
		manager.lister = informer.Lister(resourceInterpreterCustomizationsGVR)
		configHandlers := fedinformer.NewHandlerOnEvents(
			func(_ interface{}) { manager.updateConfiguration() },
			func(_, _ interface{}) { manager.updateConfiguration() },
			func(_ interface{}) { manager.updateConfiguration() })
		informer.ForResource(resourceInterpreterCustomizationsGVR, configHandlers)
	}

	return manager
}

func (configManager *interpreterConfigManager) updateConfiguration() {
	configurations, err := configManager.lister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error updating configuration: %v", err))
		return
	}

	configs := make([]*configv1alpha1.ResourceInterpreterCustomization, len(configurations))
	for index, c := range configurations {
		config := &configv1alpha1.ResourceInterpreterCustomization{}
		if err = helper.ConvertToTypedObject(c, config); err != nil {
			klog.Errorf("Failed to transform ResourceInterpreterCustomization: %v", err)
			return
		}
		configs[index] = config
	}

	configManager.LoadConfig(configs)
}

func (configManager *interpreterConfigManager) LoadConfig(configs []*configv1alpha1.ResourceInterpreterCustomization) {
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].Name < configs[j].Name
	})

	accessors := make(map[schema.GroupVersionKind]CustomAccessor)
	for _, config := range configs {
		key := schema.FromAPIVersionAndKind(config.Spec.Target.APIVersion, config.Spec.Target.Kind)

		var accessor CustomAccessor
		var ok bool
		if accessor, ok = accessors[key]; !ok {
			accessor = NewResourceCustomAccessor()
		}
		accessor.Merge(config.Spec.Customizations)
		accessors[key] = accessor
	}

	configManager.configuration.Store(accessors)
	configManager.initialSynced.Store(true)
}
