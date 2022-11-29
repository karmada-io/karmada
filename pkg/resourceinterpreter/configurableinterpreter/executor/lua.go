package executor

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/configurableinterpreter/luavm"
)

// LuaExecutor can execute lua script.
type LuaExecutor struct {
	vm *luavm.VM
}

// NewLuaExecutor create an executor for lua script.
func NewLuaExecutor() Interface {
	return &LuaExecutor{
		// TODO: set appropriate size according workers.
		vm: luavm.New(false, 8),
	}
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica by lua script.
func (e *LuaExecutor) GetReplicas(r *configv1alpha1.ReplicaResourceRequirement, obj *unstructured.Unstructured) (replica int32, requires *workv1alpha2.ReplicaRequirements, enabled bool, err error) {
	enabled = r.LuaScript != ""
	if !enabled {
		return
	}

	rets, err := e.vm.RunScript(r.LuaScript, "GetReplicas", 2, obj)
	if err != nil {
		return
	}

	replica, err = luavm.ConvertLuaResultToInt(rets[0])
	if err != nil {
		return
	}

	requires = &workv1alpha2.ReplicaRequirements{}
	err = luavm.ConvertLuaResultInto(rets[1], requires)
	return
}

// ReviseReplica revises the replica of the given object.
func (e *LuaExecutor) ReviseReplica(r *configv1alpha1.ReplicaRevision, object *unstructured.Unstructured, replica int64) (revised *unstructured.Unstructured, enabled bool, err error) {
	enabled = r.LuaScript != ""
	if !enabled {
		return
	}

	rets, err := e.vm.RunScript(r.LuaScript, "ReviseReplica", 1, object, replica)
	if err != nil {
		return
	}

	revised = &unstructured.Unstructured{}
	err = luavm.ConvertLuaResultInto(rets[0], revised)
	return
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object.
func (e *LuaExecutor) Retain(r *configv1alpha1.LocalValueRetention, desired *unstructured.Unstructured, observed *unstructured.Unstructured) (retained *unstructured.Unstructured, enabled bool, err error) {
	enabled = r.LuaScript != ""
	if !enabled {
		return
	}

	rets, err := e.vm.RunScript(r.LuaScript, "Retain", 1, desired, observed)
	if err != nil {
		return
	}

	retained = &unstructured.Unstructured{}
	err = luavm.ConvertLuaResultInto(rets[0], retained)
	return
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated.
func (e *LuaExecutor) AggregateStatus(r *configv1alpha1.StatusAggregation, obj *unstructured.Unstructured, items []workv1alpha2.AggregatedStatusItem) (aggregated *unstructured.Unstructured, enabled bool, err error) {
	enabled = r.LuaScript != ""
	if !enabled {
		return
	}

	rets, err := e.vm.RunScript(r.LuaScript, "AggregateStatus", 1, obj, items)
	if err != nil {
		return
	}

	aggregated = &unstructured.Unstructured{}
	err = luavm.ConvertLuaResultInto(rets[0], aggregated)
	return
}

// InterpretHealth returns the health state of the object.
func (e *LuaExecutor) InterpretHealth(r *configv1alpha1.HealthInterpretation, obj *unstructured.Unstructured) (healthy bool, enabled bool, err error) {
	enabled = r.LuaScript != ""
	if !enabled {
		return
	}

	rets, err := e.vm.RunScript(r.LuaScript, "InterpretHealth", 1, obj)
	if err != nil {
		return
	}

	healthy, err = luavm.ConvertLuaResultToBool(rets[0])
	return
}

// ReflectStatus returns the status of the object.
func (e *LuaExecutor) ReflectStatus(r *configv1alpha1.StatusReflection, obj *unstructured.Unstructured) (status *runtime.RawExtension, enabled bool, err error) {
	enabled = r.LuaScript != ""
	if !enabled {
		return
	}

	rets, err := e.vm.RunScript(r.LuaScript, "ReflectStatus", 1, obj)
	if err != nil {
		return
	}
	status = &runtime.RawExtension{}
	err = luavm.ConvertLuaResultInto(rets[0], status)
	return
}

// GetDependencies returns the dependent resources of the given object.
func (e *LuaExecutor) GetDependencies(r *configv1alpha1.DependencyInterpretation, obj *unstructured.Unstructured) (dependencies []configv1alpha1.DependentObjectReference, enabled bool, err error) {
	enabled = r.LuaScript != ""
	if !enabled {
		return
	}

	rets, err := e.vm.RunScript(r.LuaScript, "GetDependencies", 1, obj)
	if err != nil {
		return
	}

	err = luavm.ConvertLuaResultInto(rets[0], &dependencies)
	return
}
