package configurableinterpreter

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/configurableinterpreter/configmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

// ConfigurableInterpreter interprets resources with resource interpreter customizations.
type ConfigurableInterpreter struct {
	// configManager caches all ResourceInterpreterCustomizations.
	configManager configmanager.ConfigManager
}

// NewConfigurableInterpreter builds a new interpreter by registering the
// event handler to the provided informer instance.
func NewConfigurableInterpreter(informer genericmanager.SingleClusterInformerManager) *ConfigurableInterpreter {
	return &ConfigurableInterpreter{
		configManager: configmanager.NewInterpreterConfigManager(informer),
	}
}

// HookEnabled tells if any hook exist for specific resource gvk and operation type.
func (c *ConfigurableInterpreter) HookEnabled(kind schema.GroupVersionKind, operationType configv1alpha1.InterpreterOperation) bool {
	if !c.configManager.HasSynced() {
		klog.Errorf("not yet ready to handle request")
		return false
	}
	accessors, exist := c.configManager.LuaScriptAccessors()[kind]
	if !exist {
		return false
	}
	switch operationType {
	case configv1alpha1.InterpreterOperationAggregateStatus:
		return len(accessors.GetStatusAggregationLuaScript()) > 0
	case configv1alpha1.InterpreterOperationInterpretHealth:
		return len(accessors.GetHealthInterpretationLuaScript()) > 0
	case configv1alpha1.InterpreterOperationInterpretDependency:
		return len(accessors.GetDependencyInterpretationLuaScript()) > 0
	case configv1alpha1.InterpreterOperationInterpretReplica:
		return len(accessors.GetReplicaResourceLuaScript()) > 0
	case configv1alpha1.InterpreterOperationInterpretStatus:
		return len(accessors.GetStatusReflectionLuaScript()) > 0
	case configv1alpha1.InterpreterOperationRetain:
		return len(accessors.GetRetentionLuaScript()) > 0
	case configv1alpha1.InterpreterOperationReviseReplica:
		return len(accessors.GetReplicaRevisionLuaScript()) > 0
	}
	return false
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (c *ConfigurableInterpreter) GetReplicas(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {
	klog.Infof("ConfigurableInterpreter Execute ReviseReplica")
	customAccessor := c.configManager.LuaScriptAccessors()[object.GroupVersionKind()]
	if len(customAccessor.GetReplicaResourceLuaScript()) == 0 {
		return 0, nil, fmt.Errorf("customized interpreter operation GetReplicas for %q not found", object.GroupVersionKind())
	}

	luaScript := customAccessor.GetReplicaResourceLuaScript
	klog.Infof("lua script %s", luaScript)

	return 0, nil, nil
}

// ReviseReplica revises the replica of the given object.
func (c *ConfigurableInterpreter) ReviseReplica(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error) {
	klog.Infof("ConfigurableInterpreter Execute ReviseReplica")
	customAccessor := c.configManager.LuaScriptAccessors()[object.GroupVersionKind()]
	if len(customAccessor.GetReplicaRevisionLuaScript()) == 0 {
		return nil, fmt.Errorf("customized interpreter operation ReviseReplica for %q not found", object.GroupVersionKind())
	}

	luaScript := customAccessor.GetReplicaRevisionLuaScript()
	klog.Infof("lua script %s", luaScript)

	return nil, nil
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (c *ConfigurableInterpreter) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, err error) {
	klog.Infof("ConfigurableInterpreter Execute Retain")
	customAccessor := c.configManager.LuaScriptAccessors()[desired.GroupVersionKind()]
	if len(customAccessor.GetRetentionLuaScript()) == 0 {
		return nil, fmt.Errorf("customized interpreter operation Retain for %q not found", desired.GroupVersionKind())
	}

	luaScript := customAccessor.GetRetentionLuaScript()
	klog.Infof("lua script %s", luaScript)

	return nil, err
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (c *ConfigurableInterpreter) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	klog.Infof("ConfigurableInterpreter Execute AggregateStatus")
	customAccessor := c.configManager.LuaScriptAccessors()[object.GroupVersionKind()]
	if len(customAccessor.GetStatusAggregationLuaScript()) == 0 {
		return nil, fmt.Errorf("customized interpreter AggregateStatus Retain for %q not found", object.GroupVersionKind())
	}

	luaScript := customAccessor.GetStatusAggregationLuaScript()
	klog.Infof("lua script %s", luaScript)

	return nil, nil
}

// GetDependencies returns the dependent resources of the given object.
func (c *ConfigurableInterpreter) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, err error) {
	klog.Infof("ConfigurableInterpreter Execute GetDependencies")

	customAccessor := c.configManager.LuaScriptAccessors()[object.GroupVersionKind()]
	if len(customAccessor.GetDependencyInterpretationLuaScript()) == 0 {
		return nil, fmt.Errorf("customized interpreter GetDependencies Retain for %q not found", object.GroupVersionKind())
	}

	luaScript := customAccessor.GetDependencyInterpretationLuaScript()
	klog.Infof("lua script %s", luaScript)

	return nil, err
}

// ReflectStatus returns the status of the object.
func (c *ConfigurableInterpreter) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, err error) {
	klog.Infof("ConfigurableInterpreter Execute ReflectStatus")
	customAccessor := c.configManager.LuaScriptAccessors()[object.GroupVersionKind()]
	if len(customAccessor.GetStatusAggregationLuaScript()) == 0 {
		return nil, fmt.Errorf("customized interpreter GetDependencies Retain for %q not found", object.GroupVersionKind())
	}

	luaScript := customAccessor.GetStatusAggregationLuaScript()
	klog.Infof("lua script %s", luaScript)

	return nil, err
}

// InterpretHealth returns the health state of the object.
func (c *ConfigurableInterpreter) InterpretHealth(object *unstructured.Unstructured) (bool, error) {
	klog.Infof("ConfigurableInterpreter Execute InterpretHealth")
	customAccessor := c.configManager.LuaScriptAccessors()[object.GroupVersionKind()]
	if len(customAccessor.GetHealthInterpretationLuaScript()) == 0 {
		return false, fmt.Errorf("customized interpreter GetHealthInterpretation for %q not found", object.GroupVersionKind())
	}

	luaScript := customAccessor.GetHealthInterpretationLuaScript()
	klog.Infof("lua script %s", luaScript)

	return false, nil
}
