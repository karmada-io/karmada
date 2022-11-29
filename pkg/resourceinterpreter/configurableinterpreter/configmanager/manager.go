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
	GetInterpreter(kind schema.GroupVersionKind) *configv1alpha1.CustomizationRules
	LoadConfig(customizations []*configv1alpha1.ResourceInterpreterCustomization)
}

// interpreterConfigManager collects the resource interpreter customization.
type interpreterConfigManager struct {
	lister        cache.GenericLister
	configuration *atomic.Value
}

// NewInterpreterConfigManager watches ResourceInterpreterCustomization and organizes
// the configurations in the cache.
func NewInterpreterConfigManager(informer genericmanager.SingleClusterInformerManager) ConfigManager {
	manager := &interpreterConfigManager{
		configuration: &atomic.Value{},
	}
	manager.configuration.Store(make(map[schema.GroupVersionKind]*configv1alpha1.CustomizationRules))

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

func (configManager *interpreterConfigManager) IsEnabled(operationType configv1alpha1.InterpreterOperation, kind schema.GroupVersionKind) bool {
	rules := configManager.GetInterpreter(kind)
	if rules == nil {
		return false
	}

	switch operationType {
	case configv1alpha1.InterpreterOperationAggregateStatus:
		return rules.StatusAggregation != nil
	case configv1alpha1.InterpreterOperationInterpretHealth:
		return rules.HealthInterpretation != nil
	case configv1alpha1.InterpreterOperationInterpretDependency:
		return rules.DependencyInterpretation != nil
	case configv1alpha1.InterpreterOperationInterpretReplica:
		return rules.ReplicaResource != nil
	case configv1alpha1.InterpreterOperationInterpretStatus:
		return rules.StatusReflection != nil
	case configv1alpha1.InterpreterOperationRetain:
		return rules.Retention != nil
	case configv1alpha1.InterpreterOperationReviseReplica:
		return rules.ReplicaRevision != nil
	}
	return false
}

func (configManager *interpreterConfigManager) GetInterpreter(kind schema.GroupVersionKind) *configv1alpha1.CustomizationRules {
	configs := configManager.configuration.Load().(map[schema.GroupVersionKind]*configv1alpha1.CustomizationRules)
	return configs[kind]
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
			klog.Errorf("Failed to transform ResourceInterpreterCustomization: %w", err)
			return
		}
		configs[index] = config
	}

	configManager.LoadConfig(configs)
}

func (configManager *interpreterConfigManager) LoadConfig(configurations []*configv1alpha1.ResourceInterpreterCustomization) {
	configs := make(map[schema.GroupVersionKind]*configv1alpha1.CustomizationRules, len(configurations))
	for _, config := range configurations {
		kind := schema.FromAPIVersionAndKind(config.Spec.Target.APIVersion, config.Spec.Target.Kind)

		rules, ok := configs[kind]
		if !ok {
			rules = &configv1alpha1.CustomizationRules{}
			configs[kind] = rules
		}
		mergeConfigInto(&config.Spec.Customizations, rules)
	}
	configManager.configuration.Store(configs)
}

func mergeConfigInto(from, to *configv1alpha1.CustomizationRules) {
	if r := from.Retention; r != nil {
		to.Retention = r.DeepCopy()
	}
	if r := from.ReplicaResource; r != nil {
		to.ReplicaResource = r.DeepCopy()
	}
	if r := from.ReplicaRevision; r != nil {
		to.ReplicaRevision = r.DeepCopy()
	}
	if r := from.StatusReflection; r != nil {
		to.StatusReflection = r.DeepCopy()
	}
	if r := from.StatusAggregation; r != nil {
		to.StatusAggregation = r.DeepCopy()
	}
	if r := from.HealthInterpretation; r != nil {
		to.HealthInterpretation = r.DeepCopy()
	}
	if r := from.DependencyInterpretation; r != nil {
		to.DependencyInterpretation = r.DeepCopy()
	}
}
