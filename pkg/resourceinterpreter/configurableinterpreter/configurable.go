package configurableinterpreter

import (
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
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
	luaVM         *luavm.VM
}

// NewConfigurableInterpreter builds a new interpreter by registering the
// event handler to the provided informer instance.
func NewConfigurableInterpreter(informer genericmanager.SingleClusterInformerManager) *ConfigurableInterpreter {
	return &ConfigurableInterpreter{
		configManager: configmanager.NewInterpreterConfigManager(informer),
		// TODO: set an appropriate pool size.
		luaVM: luavm.New(false, 10),
	}
}

// HookEnabled tells if any hook exist for specific resource gvk and operation type.
func (c *ConfigurableInterpreter) HookEnabled(kind schema.GroupVersionKind, operationType configv1alpha1.InterpreterOperation) bool {
	accessor, exist := c.getCustomAccessor(kind)
	if !exist {
		return exist
	}

	if operationType == configv1alpha1.InterpreterOperationInterpretDependency {
		scripts := accessor.GetDependencyInterpretationLuaScripts()
		return scripts != nil
	}

	var script string
	switch operationType {
	case configv1alpha1.InterpreterOperationAggregateStatus:
		script = accessor.GetStatusAggregationLuaScript()
	case configv1alpha1.InterpreterOperationInterpretHealth:
		script = accessor.GetHealthInterpretationLuaScript()
	case configv1alpha1.InterpreterOperationInterpretReplica:
		script = accessor.GetReplicaResourceLuaScript()
	case configv1alpha1.InterpreterOperationInterpretStatus:
		script = accessor.GetStatusReflectionLuaScript()
	case configv1alpha1.InterpreterOperationRetain:
		script = accessor.GetRetentionLuaScript()
	case configv1alpha1.InterpreterOperationReviseReplica:
		script = accessor.GetReplicaRevisionLuaScript()
	}
	return len(script) > 0
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (c *ConfigurableInterpreter) GetReplicas(object *unstructured.Unstructured) (replicas int32, requires *workv1alpha2.ReplicaRequirements, enabled bool, err error) {
	klog.V(4).Infof("Get replicas for object: %v %s/%s with configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	accessor, enabled := c.getCustomAccessor(object.GroupVersionKind())
	if !enabled {
		return
	}

	script := accessor.GetReplicaResourceLuaScript()
	if len(script) == 0 {
		enabled = false
		return
	}

	replicas, requires, err = c.luaVM.GetReplicas(object, script)
	return
}

// ReviseReplica revises the replica of the given object.
func (c *ConfigurableInterpreter) ReviseReplica(object *unstructured.Unstructured, replica int64) (revised *unstructured.Unstructured, enabled bool, err error) {
	klog.V(4).Infof("Revise replicas for object: %v %s/%s with configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	accessor, enabled := c.getCustomAccessor(object.GroupVersionKind())
	if !enabled {
		return
	}

	script := accessor.GetReplicaRevisionLuaScript()
	if len(script) == 0 {
		enabled = false
		return
	}

	revised, err = c.luaVM.ReviseReplica(object, replica, script)
	return
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (c *ConfigurableInterpreter) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, enabled bool, err error) {
	klog.V(4).Infof("Retain object: %v %s/%s with configurable interpreter.", desired.GroupVersionKind(), desired.GetNamespace(), desired.GetName())

	accessor, enabled := c.getCustomAccessor(desired.GroupVersionKind())
	if !enabled {
		return
	}

	script := accessor.GetRetentionLuaScript()
	if len(script) == 0 {
		enabled = false
		return
	}

	retained, err = c.luaVM.Retain(desired, observed, script)
	return
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (c *ConfigurableInterpreter) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (status *unstructured.Unstructured, enabled bool, err error) {
	klog.V(4).Infof("Aggregate status of object: %v %s/%s with configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())
	accessor, enabled := c.getCustomAccessor(object.GroupVersionKind())
	if !enabled {
		return
	}

	script := accessor.GetStatusAggregationLuaScript()
	if len(script) == 0 {
		enabled = false
		return
	}

	status, err = c.luaVM.AggregateStatus(object, aggregatedStatusItems, script)
	return
}

// GetDependencies returns the dependent resources of the given object.
func (c *ConfigurableInterpreter) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, enabled bool, err error) {
	klog.V(4).Infof("Get dependencies of object: %v %s/%s with configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	accessor, enabled := c.getCustomAccessor(object.GroupVersionKind())
	if !enabled {
		return
	}

	scripts := accessor.GetDependencyInterpretationLuaScripts()
	if scripts == nil {
		enabled = false
		return
	}

	refs := sets.New[configv1alpha1.DependentObjectReference]()
	for _, luaScript := range scripts {
		var references []configv1alpha1.DependentObjectReference
		references, err = c.luaVM.GetDependencies(object, luaScript)
		if err != nil {
			klog.Errorf("Failed to get DependentObjectReferences from object: %v %s/%s, error: %v",
				object.GroupVersionKind(), object.GetNamespace(), object.GetName(), err)
			return
		}
		refs.Insert(references...)
	}
	dependencies = refs.UnsortedList()

	// keep returned items in the same order between each call.
	sort.Slice(dependencies, func(i, j int) bool {
		if dependencies[i].APIVersion != dependencies[j].APIVersion {
			return dependencies[i].APIVersion < dependencies[j].APIVersion
		}
		if dependencies[i].Kind != dependencies[j].Kind {
			return dependencies[i].Kind < dependencies[j].Kind
		}
		if dependencies[i].Namespace != dependencies[j].Namespace {
			return dependencies[i].Namespace < dependencies[j].Namespace
		}
		return dependencies[i].Name < dependencies[j].Name
	})
	return
}

// ReflectStatus returns the status of the object.
func (c *ConfigurableInterpreter) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, enabled bool, err error) {
	klog.V(4).Infof("Reflect status of object: %v %s/%s with configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	accessor, enabled := c.getCustomAccessor(object.GroupVersionKind())
	if !enabled {
		return
	}

	script := accessor.GetStatusReflectionLuaScript()
	if len(script) == 0 {
		enabled = false
		return
	}

	status, err = c.luaVM.ReflectStatus(object, script)
	return
}

// InterpretHealth returns the health state of the object.
func (c *ConfigurableInterpreter) InterpretHealth(object *unstructured.Unstructured) (health bool, enabled bool, err error) {
	klog.V(4).Infof("Get health status of object: %v %s/%s with configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	accessor, enabled := c.getCustomAccessor(object.GroupVersionKind())
	if !enabled {
		return
	}

	script := accessor.GetHealthInterpretationLuaScript()
	if len(script) == 0 {
		enabled = false
		return
	}

	health, err = c.luaVM.InterpretHealth(object, script)
	return
}

func (c *ConfigurableInterpreter) getCustomAccessor(kind schema.GroupVersionKind) (configmanager.CustomAccessor, bool) {
	if !c.configManager.HasSynced() {
		klog.Errorf("not yet ready to handle request")
		return nil, false
	}

	accessor, exist := c.configManager.CustomAccessors()[kind]
	return accessor, exist
}

// LoadConfig loads and stores rules from customizations
func (c *ConfigurableInterpreter) LoadConfig(customizations []*configv1alpha1.ResourceInterpreterCustomization) {
	c.configManager.LoadConfig(customizations)
}
