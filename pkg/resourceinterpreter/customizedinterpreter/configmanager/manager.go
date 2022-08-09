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
	configuration *atomic.Value
	lister        cache.GenericLister
	initialSynced *atomic.Value
}

// HookAccessors return all configured resource interpreter webhook.
func (m *interpreterConfigManager) HookAccessors() []WebhookAccessor {
	return m.configuration.Load().([]WebhookAccessor)
}

// HasSynced return true when the manager is synced with existing configuration.
func (m *interpreterConfigManager) HasSynced() bool {
	if m.initialSynced.Load().(bool) {
		return true
	}

	if configuration, err := m.lister.List(labels.Everything()); err == nil && len(configuration) == 0 {
		// the empty list we initially stored is valid to use.
		// Setting initialSynced to true, so subsequent checks
		// would be able to take the fast path on the atomic boolean in a
		// cluster without any webhooks configured.
		m.initialSynced.Store(true)
		// the informer has synced, and we don't have any items
		return true
	}
	return false
}

// NewExploreConfigManager return a new interpreterConfigManager with resourceinterpreterwebhookconfigurations handlers.
func NewExploreConfigManager(inform genericmanager.SingleClusterInformerManager) ConfigManager {
	manager := &interpreterConfigManager{
		configuration: &atomic.Value{},
		lister:        inform.Lister(resourceExploringWebhookConfigurationsGVR),
		initialSynced: &atomic.Value{},
	}

	manager.configuration.Store([]WebhookAccessor{})
	manager.initialSynced.Store(false)

	configHandlers := fedinformer.NewHandlerOnEvents(
		func(_ interface{}) { manager.updateConfiguration() },
		func(_, _ interface{}) { manager.updateConfiguration() },
		func(_ interface{}) { manager.updateConfiguration() })
	inform.ForResource(resourceExploringWebhookConfigurationsGVR, configHandlers)

	return manager
}

func (m *interpreterConfigManager) updateConfiguration() {
	configurations, err := m.lister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error updating configuration: %v", err))
		return
	}

	configs := make([]*configv1alpha1.ResourceInterpreterWebhookConfiguration, 0)
	for _, c := range configurations {
		unstructuredConfig, err := helper.ToUnstructured(c)
		if err != nil {
			klog.Errorf("Failed to transform ResourceInterpreterWebhookConfiguration: %w", err)
			return
		}

		config := &configv1alpha1.ResourceInterpreterWebhookConfiguration{}
		err = helper.ConvertToTypedObject(unstructuredConfig, config)
		if err != nil {
			gvk := unstructuredConfig.GroupVersionKind().String()
			klog.Errorf("Failed to convert object(%s), err: %v", gvk, err)
			return
		}
		configs = append(configs, config)
	}

	m.configuration.Store(mergeResourceExploreWebhookConfigurations(configs))
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
