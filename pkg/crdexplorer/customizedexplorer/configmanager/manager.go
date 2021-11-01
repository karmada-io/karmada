package configmanager

import (
	"fmt"
	"sort"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
)

var resourceExploringWebhookConfigurationsGVR = schema.GroupVersionResource{
	Group:    configv1alpha1.GroupVersion.Group,
	Version:  configv1alpha1.GroupVersion.Version,
	Resource: "resourceexploringwebhookconfigurations",
}

// ConfigManager can list dynamic webhooks.
type ConfigManager interface {
	HookAccessors() []WebhookAccessor
	HasSynced() bool
}

// exploreConfigManager collect the resource explore webhook configuration.
type exploreConfigManager struct {
	configuration              *atomic.Value
	lister                     cache.GenericLister
	hasSynced                  func() bool
	initialConfigurationSynced *atomic.Value
}

// HookAccessors return all configured resource explore webhook.
func (m *exploreConfigManager) HookAccessors() []WebhookAccessor {
	return m.configuration.Load().([]WebhookAccessor)
}

// HasSynced return true when the manager is synced with existing configuration.
func (m *exploreConfigManager) HasSynced() bool {
	return false
}

// NewExploreConfigManager return a new exploreConfigManager with resourceexploringwebhookconfigurations handlers.
func NewExploreConfigManager(inform informermanager.SingleClusterInformerManager) ConfigManager {
	manager := &exploreConfigManager{
		configuration:              &atomic.Value{},
		lister:                     inform.Lister(resourceExploringWebhookConfigurationsGVR),
		hasSynced:                  func() bool { return inform.IsInformerSynced(resourceExploringWebhookConfigurationsGVR) },
		initialConfigurationSynced: &atomic.Value{},
	}

	manager.configuration.Store([]WebhookAccessor{})
	manager.initialConfigurationSynced.Store(false)

	configHandlers := informermanager.NewHandlerOnEvents(
		func(_ interface{}) { manager.updateConfiguration() },
		func(_, _ interface{}) { manager.updateConfiguration() },
		func(_ interface{}) { manager.updateConfiguration() })
	inform.ForResource(resourceExploringWebhookConfigurationsGVR, configHandlers)

	return manager
}

func (m *exploreConfigManager) updateConfiguration() {
	//configurations, err := m.lister.List(labels.Everything())
	//if err != nil {
	//	utilruntime.HandleError(fmt.Errorf("error updating configuration: %v", err))
	//	return
	//}
	configurations := make([]configv1alpha1.ResourceExploringWebhookConfiguration, 0)

	m.configuration.Store(mergeResourceExploreWebhookConfigurations(configurations))
	m.initialConfigurationSynced.Store(true)
}

func mergeResourceExploreWebhookConfigurations(configurations []configv1alpha1.ResourceExploringWebhookConfiguration) []WebhookAccessor {
	sort.SliceStable(configurations, func(i, j int) bool {
		return configurations[i].Name < configurations[j].Name
	})

	accessors := make([]WebhookAccessor, len(configurations))
	for _, config := range configurations {
		names := map[string]int{}
		for index, hook := range config.Webhooks {
			uid := fmt.Sprintf("%s/%s/%d", config.Name, hook.Name, names[hook.Name])
			names[hook.Name]++
			accessors = append(accessors, NewResourceExploringAccessor(uid, config.Name, &config.Webhooks[index]))
		}
	}

	return accessors
}
