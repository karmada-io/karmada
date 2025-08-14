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
	"errors"
	"fmt"
	"sort"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// ConfigManager can list dynamic webhooks.
type ConfigManager interface {
	HookAccessors() []WebhookAccessor
	HasSynced() bool
	// LoadConfig is used to load ResourceInterpreterWebhookConfiguration into the cache,
	// it requires the provided webhookConfigurations to be a full list of objects.
	// It is recommended to be called during startup. After called, HasSynced() will always
	// return true, and HookAccessors() will return a list of WebhookAccessor containing
	// all ResourceInterpreterWebhookConfiguration configurations.
	LoadConfig(webhookConfigurations []*configv1alpha1.ResourceInterpreterWebhookConfiguration)
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

	err := m.updateConfiguration()
	if err != nil {
		klog.ErrorS(err, "error updating configuration")
		return false
	}
	return true
}

// NewExploreConfigManager return a new interpreterConfigManager with resourceinterpreterwebhookconfigurations handlers.
func NewExploreConfigManager(inform genericmanager.SingleClusterInformerManager) ConfigManager {
	manager := &interpreterConfigManager{
		lister: inform.Lister(util.ResourceInterpreterWebhookConfigurationsGVR),
	}

	manager.configuration.Store([]WebhookAccessor{})

	manager.informer = inform
	configHandlers := fedinformer.NewHandlerOnEvents(
		func(_ interface{}) { _ = manager.updateConfiguration() },
		func(_, _ interface{}) { _ = manager.updateConfiguration() },
		func(_ interface{}) { _ = manager.updateConfiguration() })
	inform.ForResource(util.ResourceInterpreterWebhookConfigurationsGVR, configHandlers)

	return manager
}

// updateConfiguration is used as the event handler for the ResourceInterpreterWebhookConfiguration resource.
// Any changes (add, update, delete) to these resources will trigger this method, which loads all
// ResourceInterpreterWebhookConfiguration resources and refreshes the internal cache accordingly.
// Note: During startup, some events may be missed if the informer has not yet synced. If all events
// are missed during startup, updateConfiguration will be called when HasSynced() is invoked for the
// first time, ensuring the cache is updated on first use.
func (m *interpreterConfigManager) updateConfiguration() error {
	if m.informer == nil {
		return errors.New("informer manager is not configured")
	}
	if !m.informer.IsInformerSynced(util.ResourceInterpreterWebhookConfigurationsGVR) {
		return errors.New("informer of ResourceInterpreterWebhookConfiguration not synced")
	}

	configurations, err := m.lister.List(labels.Everything())
	if err != nil {
		return err
	}

	configs := make([]*configv1alpha1.ResourceInterpreterWebhookConfiguration, len(configurations))
	for index, c := range configurations {
		config := &configv1alpha1.ResourceInterpreterWebhookConfiguration{}
		err = helper.ConvertToTypedObject(c, config)
		if err != nil {
			return err
		}
		configs[index] = config
	}

	m.LoadConfig(configs)
	return nil
}

// LoadConfig loads the webhook configurations and updates the initialSynced flag to true.
func (m *interpreterConfigManager) LoadConfig(webhookConfigurations []*configv1alpha1.ResourceInterpreterWebhookConfiguration) {
	m.configuration.Store(mergeResourceExploreWebhookConfigurations(webhookConfigurations))
	m.initialSynced.Store(true)
}

func mergeResourceExploreWebhookConfigurations(configurations []*configv1alpha1.ResourceInterpreterWebhookConfiguration) []WebhookAccessor {
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
	return accessors
}
