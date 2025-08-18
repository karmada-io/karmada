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

package declarative

import (
	"errors"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative/configmanager"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative/luavm"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/interpreter/validation"
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
func (c *ConfigurableInterpreter) HookEnabled(kind schema.GroupVersionKind, operationType configv1alpha1.InterpreterOperation) (bool, error) {
	accessor, enabled, err := c.getCustomAccessor(kind)
	if err != nil || !enabled {
		return enabled, err
	}

	if operationType == configv1alpha1.InterpreterOperationInterpretDependency {
		scripts := accessor.GetDependencyInterpretationLuaScripts()
		return scripts != nil, nil
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
	return len(script) > 0, nil
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (c *ConfigurableInterpreter) GetReplicas(object *unstructured.Unstructured) (replicas int32, requires *workv1alpha2.ReplicaRequirements, enabled bool, err error) {
	klog.V(4).Infof("Get replicas for object: %v %s/%s with configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	accessor, enabled, err := c.getCustomAccessor(object.GroupVersionKind())
	if err != nil {
		return 0, nil, false, err
	}
	if !enabled {
		return 0, nil, false, nil
	}

	script := accessor.GetReplicaResourceLuaScript()
	if len(script) == 0 {
		return 0, nil, false, nil
	}

	replicas, requires, err = c.luaVM.GetReplicas(object, script)
	if err != nil {
		return 0, nil, true, err
	}
	return replicas, requires, true, nil
}

// ReviseReplica revises the replica of the given object.
func (c *ConfigurableInterpreter) ReviseReplica(object *unstructured.Unstructured, replica int64) (revised *unstructured.Unstructured, enabled bool, err error) {
	klog.V(4).Infof("Revise replicas for object: %v %s/%s with configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	accessor, enabled, err := c.getCustomAccessor(object.GroupVersionKind())
	if err != nil {
		return nil, false, err
	}
	if !enabled {
		return nil, false, nil
	}

	script := accessor.GetReplicaRevisionLuaScript()
	if len(script) == 0 {
		return nil, false, nil
	}

	revised, err = c.luaVM.ReviseReplica(object, replica, script)
	if err != nil {
		return nil, true, err
	}
	return revised, true, nil
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (c *ConfigurableInterpreter) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, enabled bool, err error) {
	klog.V(4).Infof("Retain object: %v %s/%s with configurable interpreter.", desired.GroupVersionKind(), desired.GetNamespace(), desired.GetName())

	accessor, enabled, err := c.getCustomAccessor(desired.GroupVersionKind())
	if err != nil {
		return nil, false, err
	}
	if !enabled {
		return nil, false, nil
	}

	script := accessor.GetRetentionLuaScript()
	if len(script) == 0 {
		return nil, false, nil
	}

	retained, err = c.luaVM.Retain(desired, observed, script)
	if err != nil {
		return nil, true, err
	}
	return retained, true, nil
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (c *ConfigurableInterpreter) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (status *unstructured.Unstructured, enabled bool, err error) {
	klog.V(4).Infof("Aggregate status of object: %v %s/%s with configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())
	accessor, enabled, err := c.getCustomAccessor(object.GroupVersionKind())
	if err != nil {
		return nil, false, err
	}
	if !enabled {
		return nil, false, nil
	}

	script := accessor.GetStatusAggregationLuaScript()
	if len(script) == 0 {
		return nil, false, nil
	}

	status, err = c.luaVM.AggregateStatus(object, aggregatedStatusItems, script)
	if err != nil {
		return nil, true, err
	}
	return status, true, nil
}

// GetDependencies returns the dependent resources of the given object.
func (c *ConfigurableInterpreter) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, enabled bool, err error) {
	klog.V(4).Infof("Get dependencies of object: %v %s/%s with configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	accessor, enabled, err := c.getCustomAccessor(object.GroupVersionKind())
	if err != nil {
		return nil, false, err
	}
	if !enabled {
		return nil, false, nil
	}

	scripts := accessor.GetDependencyInterpretationLuaScripts()
	if scripts == nil {
		return nil, false, nil
	}

	refs := sets.New[configv1alpha1.DependentObjectReference]()
	for _, luaScript := range scripts {
		references, err := c.luaVM.GetDependencies(object, luaScript)
		if err != nil {
			klog.Errorf("Failed to get DependentObjectReferences from object: %v %s/%s, error: %v",
				object.GroupVersionKind(), object.GetNamespace(), object.GetName(), err)
			return nil, true, err
		}
		err = validation.VerifyDependencies(references)
		if err != nil {
			return nil, true, err
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
	return dependencies, true, nil
}

// ReflectStatus returns the status of the object.
func (c *ConfigurableInterpreter) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, enabled bool, err error) {
	klog.V(4).Infof("Reflect status of object: %v %s/%s with configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	accessor, enabled, err := c.getCustomAccessor(object.GroupVersionKind())
	if err != nil {
		return nil, false, err
	}
	if !enabled {
		return nil, false, nil
	}

	script := accessor.GetStatusReflectionLuaScript()
	if len(script) == 0 {
		return nil, false, nil
	}

	status, err = c.luaVM.ReflectStatus(object, script)
	if err != nil {
		return nil, true, err
	}
	return status, true, nil
}

// InterpretHealth returns the health state of the object.
func (c *ConfigurableInterpreter) InterpretHealth(object *unstructured.Unstructured) (health bool, enabled bool, err error) {
	klog.V(4).Infof("Get health status of object: %v %s/%s with configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	accessor, enabled, err := c.getCustomAccessor(object.GroupVersionKind())
	if err != nil {
		return false, false, err
	}
	if !enabled {
		return false, false, nil
	}

	script := accessor.GetHealthInterpretationLuaScript()
	if len(script) == 0 {
		return false, false, nil
	}

	health, err = c.luaVM.InterpretHealth(object, script)
	if err != nil {
		return false, true, err
	}
	return health, true, nil
}

func (c *ConfigurableInterpreter) getCustomAccessor(kind schema.GroupVersionKind) (configmanager.CustomAccessor, bool, error) {
	if !c.configManager.HasSynced() {
		err := errors.New("not yet ready to handle request")
		klog.Errorf("getCustomAccessor failed: %v", err)
		return nil, false, err
	}

	accessor, exist := c.configManager.CustomAccessors()[kind]
	return accessor, exist, nil
}

// LoadConfig loads and stores rules from customizations
func (c *ConfigurableInterpreter) LoadConfig(customizations []*configv1alpha1.ResourceInterpreterCustomization) {
	c.configManager.LoadConfig(customizations)
}
