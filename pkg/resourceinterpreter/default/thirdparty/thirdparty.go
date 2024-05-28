/*
Copyright 2023 The Karmada Authors.

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

package thirdparty

import (
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
	"github.com/karmada-io/karmada/pkg/util/interpreter/validation"
)

// ConfigurableInterpreter interprets resources with third party resource interpreter.
type ConfigurableInterpreter struct {
	configManager configmanager.ConfigManager
	luaVM         *luavm.VM
}

// HookEnabled tells if any hook exist for specific resource gvk and operation type.
func (p *ConfigurableInterpreter) HookEnabled(kind schema.GroupVersionKind, operationType configv1alpha1.InterpreterOperation) bool {
	customAccessor, exist := p.getCustomAccessor(kind)
	if !exist {
		return exist
	}
	if operationType == configv1alpha1.InterpreterOperationInterpretDependency {
		scripts := customAccessor.GetDependencyInterpretationLuaScripts()
		return scripts != nil
	}
	var script string
	switch operationType {
	case configv1alpha1.InterpreterOperationAggregateStatus:
		script = customAccessor.GetStatusAggregationLuaScript()
	case configv1alpha1.InterpreterOperationInterpretHealth:
		script = customAccessor.GetHealthInterpretationLuaScript()
	case configv1alpha1.InterpreterOperationInterpretReplica:
		script = customAccessor.GetReplicaResourceLuaScript()
	case configv1alpha1.InterpreterOperationInterpretStatus:
		script = customAccessor.GetStatusReflectionLuaScript()
	case configv1alpha1.InterpreterOperationRetain:
		script = customAccessor.GetRetentionLuaScript()
	case configv1alpha1.InterpreterOperationReviseReplica:
		script = customAccessor.GetReplicaRevisionLuaScript()
	}
	return len(script) > 0
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (p *ConfigurableInterpreter) GetReplicas(object *unstructured.Unstructured) (replicas int32, requires *workv1alpha2.ReplicaRequirements, enabled bool, err error) {
	klog.V(4).Infof("Get replicas for object: %v %s/%s with thirdparty configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	customAccessor, enabled := p.getCustomAccessor(object.GroupVersionKind())
	if !enabled {
		return
	}

	script := customAccessor.GetReplicaResourceLuaScript()
	if len(script) == 0 {
		enabled = false
		return
	}

	replicas, requires, err = p.luaVM.GetReplicas(object, script)
	return
}

// ReviseReplica revises the replica of the given object.
func (p *ConfigurableInterpreter) ReviseReplica(object *unstructured.Unstructured, replica int64) (revised *unstructured.Unstructured, enabled bool, err error) {
	klog.V(4).Infof("Revise replicas for object: %v %s/%s with thirdparty configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	customAccessor, enabled := p.getCustomAccessor(object.GroupVersionKind())
	if !enabled {
		return
	}

	script := customAccessor.GetReplicaRevisionLuaScript()
	if len(script) == 0 {
		enabled = false
		return
	}

	revised, err = p.luaVM.ReviseReplica(object, replica, script)
	return
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (p *ConfigurableInterpreter) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, enabled bool, err error) {
	klog.V(4).Infof("Retain object: %v %s/%s with thirdparty configurable interpreter.", desired.GroupVersionKind(), desired.GetNamespace(), desired.GetName())

	customAccessor, enabled := p.getCustomAccessor(desired.GroupVersionKind())
	if !enabled {
		return
	}

	script := customAccessor.GetRetentionLuaScript()
	if len(script) == 0 {
		enabled = false
		return
	}

	retained, err = p.luaVM.Retain(desired, observed, script)
	return
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (p *ConfigurableInterpreter) AggregateStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (status *unstructured.Unstructured, enabled bool, err error) {
	klog.V(4).Infof("Aggregate status of object: %v %s/%s with thirdparty configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())
	customAccessor, enabled := p.getCustomAccessor(object.GroupVersionKind())
	if !enabled {
		return
	}

	script := customAccessor.GetStatusAggregationLuaScript()
	if len(script) == 0 {
		enabled = false
		return
	}

	status, err = p.luaVM.AggregateStatus(object, aggregatedStatusItems, script)
	return
}

// GetDependencies returns the dependent resources of the given object.
func (p *ConfigurableInterpreter) GetDependencies(object *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, enabled bool, err error) {
	klog.V(4).Infof("Get dependencies of object: %v %s/%s with thirdparty configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	customAccessor, enabled := p.getCustomAccessor(object.GroupVersionKind())
	if !enabled {
		return
	}

	scripts := customAccessor.GetDependencyInterpretationLuaScripts()
	if scripts == nil {
		enabled = false
		return
	}

	refs := sets.New[configv1alpha1.DependentObjectReference]()
	for _, luaScript := range scripts {
		var references []configv1alpha1.DependentObjectReference
		references, err = p.luaVM.GetDependencies(object, luaScript)
		if err != nil {
			klog.Errorf("Failed to get DependentObjectReferences from object: %v %s/%s, error: %v",
				object.GroupVersionKind(), object.GetNamespace(), object.GetName(), err)
			return
		}
		err = validation.VerifyDependencies(references)
		if err != nil {
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
func (p *ConfigurableInterpreter) ReflectStatus(object *unstructured.Unstructured) (status *runtime.RawExtension, enabled bool, err error) {
	klog.V(4).Infof("Reflect status of object: %v %s/%s with thirdparty configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	customAccessor, enabled := p.getCustomAccessor(object.GroupVersionKind())
	if !enabled {
		return
	}

	script := customAccessor.GetStatusReflectionLuaScript()
	if len(script) == 0 {
		enabled = false
		return
	}

	status, err = p.luaVM.ReflectStatus(object, script)
	return
}

// InterpretHealth returns the health state of the object.
func (p *ConfigurableInterpreter) InterpretHealth(object *unstructured.Unstructured) (health bool, enabled bool, err error) {
	klog.V(4).Infof("Get health status of object: %v %s/%s with thirdparty configurable interpreter.", object.GroupVersionKind(), object.GetNamespace(), object.GetName())

	customAccessor, enabled := p.getCustomAccessor(object.GroupVersionKind())
	if !enabled {
		return
	}

	script := customAccessor.GetHealthInterpretationLuaScript()
	if len(script) == 0 {
		enabled = false
		return
	}

	health, err = p.luaVM.InterpretHealth(object, script)
	return
}

func (p *ConfigurableInterpreter) getCustomAccessor(kind schema.GroupVersionKind) (configmanager.CustomAccessor, bool) {
	customAccessor, exist := p.configManager.CustomAccessors()[kind]
	return customAccessor, exist
}

// NewConfigurableInterpreter return a new ConfigurableInterpreter.
func NewConfigurableInterpreter() *ConfigurableInterpreter {
	return &ConfigurableInterpreter{
		configManager: NewThirdPartyConfigManager(),
		luaVM:         luavm.New(false, 10),
	}
}
