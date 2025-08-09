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
	"errors"
	"sort"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	informer      genericmanager.SingleClusterInformerManager
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

	err := configManager.updateConfiguration()
	if err != nil {
		klog.ErrorS(err, "error updating configuration")
		return false
	}
	return true
}

// NewInterpreterConfigManager watches ResourceInterpreterCustomization and organizes
// the configurations in the cache.
func NewInterpreterConfigManager(informer genericmanager.SingleClusterInformerManager) ConfigManager {
	manager := &interpreterConfigManager{}
	manager.configuration.Store(make(map[schema.GroupVersionKind]CustomAccessor))

	// In interpret command, rules are not loaded from server, so we don't start informer for it.
	if informer != nil {
		manager.informer = informer
		manager.lister = informer.Lister(resourceInterpreterCustomizationsGVR)
		configHandlers := fedinformer.NewHandlerOnEvents(
			func(_ interface{}) { _ = manager.updateConfiguration() },
			func(_, _ interface{}) { _ = manager.updateConfiguration() },
			func(_ interface{}) { _ = manager.updateConfiguration() })
		informer.ForResource(resourceInterpreterCustomizationsGVR, configHandlers)
	}

	return manager
}

// updateConfiguration is used as the event handler for the ResourceInterpreterCustomization resource.
// Any changes (add, update, delete) to these resources will trigger this method, which loads all
// ResourceInterpreterCustomization resources and refreshes the internal cache accordingly.
// Note: During startup, some events may be missed if the informer has not yet synced. If all events
// are missed during startup, updateConfiguration will be called when HasSynced() is invoked for the
// first time, ensuring the cache is updated on first use.
func (configManager *interpreterConfigManager) updateConfiguration() error {
	if configManager.informer == nil {
		return errors.New("informer manager is not configured")
	}
	if !configManager.informer.IsInformerSynced(resourceInterpreterCustomizationsGVR) {
		return errors.New("informer of ResourceInterpreterCustomization not synced")
	}

	configurations, err := configManager.lister.List(labels.Everything())
	if err != nil {
		return err
	}

	configs := make([]*configv1alpha1.ResourceInterpreterCustomization, len(configurations))
	for index, c := range configurations {
		config := &configv1alpha1.ResourceInterpreterCustomization{}
		if err = helper.ConvertToTypedObject(c, config); err != nil {
			return err
		}
		configs[index] = config
	}

	configManager.LoadConfig(configs)
	return nil
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
