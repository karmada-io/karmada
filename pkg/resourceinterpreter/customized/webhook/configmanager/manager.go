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

var resourceExploringWebhookConfigurationsGVR = schema.GroupVersionResource{
	Group:    configv1alpha1.GroupVersion.Group,
	Version:  configv1alpha1.GroupVersion.Version,
	Resource: "resourceinterpreterwebhookconfigurations",
}

// ConfigManager can list dynamic webhooks.
type ConfigManager interface {
	HookAccessors() []WebhookAccessor
	HasSynced() bool
}

// interpreterConfigManager collect the resource interpreter webhook configuration.
type interpreterConfigManager struct {
	configuration atomic.Value
	informer      genericmanager.SingleClusterInformerManager
	lister        cache.GenericLister
	initialSynced atomic.Bool
}

// HookAccessors return all configured resource interpreter webhook.
func (m *interpreterConfigManager) HookAccessors() []WebhookAccessor {
	return m.configuration.Load().([]WebhookAccessor)
}

// HasSynced return true when the manager is synced with existing configuration.
func (m *interpreterConfigManager) HasSynced() bool {
	if m.initialSynced.Load() {
		return true
	}

	if !m.informer.IsInformerSynced(resourceExploringWebhookConfigurationsGVR) {
		return false
	}

	configurations, err := m.lister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error listing configuration: %v", err))
		return false
	}

	if len(configurations) == 0 {
		// the empty list we initially stored is valid to use.
		// Setting initialSynced to true, so subsequent checks
		// would be able to take the fast path on the atomic boolean in a
		// cluster without any webhooks configured.
		m.initialSynced.Store(true)
		// the informer has synced, and we don't have any items
		return true
	}

	configs := make([]*configv1alpha1.ResourceInterpreterWebhookConfiguration, len(configurations))
	for index, c := range configurations {
		unstructuredConfig, err := helper.ToUnstructured(c)
		if err != nil {
			klog.Errorf("Failed to transform ResourceInterpreterWebhookConfiguration: %v", err)
			return false
		}

		config := &configv1alpha1.ResourceInterpreterWebhookConfiguration{}
		err = helper.ConvertToTypedObject(unstructuredConfig, config)
		if err != nil {
			klog.Errorf("Failed to transform ResourceInterpreterWebhookConfiguration: err: %v", err)
			return false
		}
		configs[index] = config
	}

	m.LoadConfig(configs)
	return true
}

// NewExploreConfigManager return a new interpreterConfigManager with resourceinterpreterwebhookconfigurations handlers.
func NewExploreConfigManager(informer genericmanager.SingleClusterInformerManager) ConfigManager {
	manager := &interpreterConfigManager{
		informer: informer,
		lister:   informer.Lister(resourceExploringWebhookConfigurationsGVR),
	}

	manager.configuration.Store([]WebhookAccessor{})

	configHandlers := fedinformer.NewHandlerOnEvents(
		func(_ interface{}) { manager.updateConfiguration() },
		func(_, _ interface{}) { manager.updateConfiguration() },
		func(_ interface{}) { manager.updateConfiguration() })
	informer.ForResource(resourceExploringWebhookConfigurationsGVR, configHandlers)

	return manager
}

func (m *interpreterConfigManager) updateConfiguration() {
	if !m.informer.IsInformerSynced(resourceExploringWebhookConfigurationsGVR) {
		klog.Info("ResourceInterpreterWebhookConfiguration informer not synced")
		return
	}

	configurations, err := m.lister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error updating configuration: %v", err))
		return
	}

	configs := make([]*configv1alpha1.ResourceInterpreterWebhookConfiguration, len(configurations))
	for index, c := range configurations {
		unstructuredConfig, err := helper.ToUnstructured(c)
		if err != nil {
			klog.Errorf("Failed to transform ResourceInterpreterWebhookConfiguration: %v", err)
			return
		}

		config := &configv1alpha1.ResourceInterpreterWebhookConfiguration{}
		err = helper.ConvertToTypedObject(unstructuredConfig, config)
		if err != nil {
			klog.Errorf("Failed to transform ResourceInterpreterWebhookConfiguration: err: %v", err)
			return
		}
		configs[index] = config
	}

	m.LoadConfig(configs)
}

func (m *interpreterConfigManager) LoadConfig(configurations []*configv1alpha1.ResourceInterpreterWebhookConfiguration) {
	sort.SliceStable(configurations, func(i, j int) bool {
		return configurations[i].Name < configurations[j].Name
	})

	var accessors []WebhookAccessor
	for ci, config := range configurations {
		for hi, hook := range config.Webhooks {
			uid := fmt.Sprintf("%s/%s", config.Name, hook.Name)
			accessors = append(accessors, NewResourceExploringAccessor(uid, config.Name, &configurations[ci].Webhooks[hi]))
		}
	}

	m.configuration.Store(accessors)
	m.initialSynced.Store(true)
}
