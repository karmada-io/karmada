package configurableinterpreter

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/configurableinterpreter/configmanager"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/configurableinterpreter/luavm"
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
	_, exist := c.getInterpreter(kind, operationType)
	return exist
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (c *ConfigurableInterpreter) GetReplicas(object *unstructured.Unstructured) (replicas int32, requires *workv1alpha2.ReplicaRequirements, enabled bool, err error) {
	luaScript, enabled := c.getInterpreter(object.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretReplica)
	if !enabled {
		return
	}
	vm := luavm.VM{UseOpenLibs: false}
	replicas, requires, err = vm.GetReplicas(object, luaScript)
	return
}

// ReviseReplica revises the replica of the given object.
func (c *ConfigurableInterpreter) ReviseReplica(object *unstructured.Unstructured, replica int64) (revised *unstructured.Unstructured, enabled bool, err error) {
	luaScript, enabled := c.getInterpreter(object.GroupVersionKind(), configv1alpha1.InterpreterOperationReviseReplica)
	if !enabled {
		return
	}
	vm := luavm.VM{UseOpenLibs: false}
	revised, err = vm.ReviseReplica(object, replica, luaScript)
	return
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (c *ConfigurableInterpreter) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, enabled bool, err error) {
	luaScript, enabled := c.getInterpreter(desired.GroupVersionKind(), configv1alpha1.InterpreterOperationRetain)
	if !enabled {
		return
	}
	vm := luavm.VM{UseOpenLibs: false}
	retained, err = vm.Retain(desired, observed, luaScript)
	return
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (c *ConfigurableInterpreter) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (status *unstructured.Unstructured, enabled bool, err error) {
	luaScript, enabled := c.getInterpreter(object.GroupVersionKind(), configv1alpha1.InterpreterOperationAggregateStatus)
	if !enabled {
		return
	}
	vm := luavm.VM{UseOpenLibs: false}
	status, err = vm.AggregateStatus(object, aggregatedStatusItems, luaScript)
	return
}

// GetDependencies returns the dependent resources of the given object.
func (c *ConfigurableInterpreter) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, enabled bool, err error) {
	luaScript, enabled := c.getInterpreter(object.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretDependency)
	if !enabled {
		return
	}
	vm := luavm.VM{UseOpenLibs: false}
	dependencies, err = vm.GetDependencies(object, luaScript)
	return
}

// ReflectStatus returns the status of the object.
func (c *ConfigurableInterpreter) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, enabled bool, err error) {
	luaScript, enabled := c.getInterpreter(object.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretStatus)
	if !enabled {
		return
	}
	vm := luavm.VM{UseOpenLibs: false}
	status, err = vm.ReflectStatus(object, luaScript)
	return
}

// InterpretHealth returns the health state of the object.
func (c *ConfigurableInterpreter) InterpretHealth(object *unstructured.Unstructured) (health bool, enabled bool, err error) {
	luaScript, enabled := c.getInterpreter(object.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretHealth)
	if !enabled {
		return
	}
	vm := luavm.VM{UseOpenLibs: false}
	health, err = vm.InterpretHealth(object, luaScript)
	return
}

func (c *ConfigurableInterpreter) getInterpreter(kind schema.GroupVersionKind, operationType configv1alpha1.InterpreterOperation) (string, bool) {
	if !c.configManager.HasSynced() {
		klog.Errorf("not yet ready to handle request")
		return "", false
	}
	accessors, exist := c.configManager.LuaScriptAccessors()[kind]
	if !exist {
		return "", false
	}
	var script string
	switch operationType {
	case configv1alpha1.InterpreterOperationAggregateStatus:
		script = accessors.GetStatusAggregationLuaScript()
	case configv1alpha1.InterpreterOperationInterpretHealth:
		script = accessors.GetHealthInterpretationLuaScript()
	case configv1alpha1.InterpreterOperationInterpretDependency:
		script = accessors.GetDependencyInterpretationLuaScript()
	case configv1alpha1.InterpreterOperationInterpretReplica:
		script = accessors.GetReplicaResourceLuaScript()
	case configv1alpha1.InterpreterOperationInterpretStatus:
		script = accessors.GetStatusReflectionLuaScript()
	case configv1alpha1.InterpreterOperationRetain:
		script = accessors.GetRetentionLuaScript()
	case configv1alpha1.InterpreterOperationReviseReplica:
		script = accessors.GetReplicaRevisionLuaScript()
	}
	return script, len(script) > 0
}
