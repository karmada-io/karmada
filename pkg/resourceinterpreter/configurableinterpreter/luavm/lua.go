package luavm

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	luajson "layeh.com/gopher-json"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/fixedpool"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

// VM Defines a struct that implements the luaVM.
type VM struct {
	// UseOpenLibs flag to enable open libraries. Libraries are disabled by default while running, but enabled during testing to allow the use of print statements.
	UseOpenLibs bool
	Pool        *fixedpool.FixedPool
}

// New creates a manager for lua VM
func New(useOpenLibs bool, poolSize int) *VM {
	vm := &VM{
		UseOpenLibs: useOpenLibs,
	}
	vm.Pool = fixedpool.New(
		"luavm",
		func() (any, error) { return vm.NewLuaState() },
		func(a any) { a.(*lua.LState).Close() },
		poolSize)
	return vm
}

// NewLuaState creates a new lua state.
func (vm *VM) NewLuaState() (*lua.LState, error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	// Opens table library to allow access to functions to manipulate tables
	err := vm.setLib(l)
	if err != nil {
		return nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, lifted.SafeOsLoader)
	// preload kube library. Allows the 'local kube = require("kube")' to work
	l.PreloadModule(KubeLibName, KubeLoader)
	return l, err
}

// RunScript got a lua vm from pool, and execute script with given arguments.
func (vm *VM) RunScript(script string, fnName string, nRets int, args ...interface{}) ([]lua.LValue, error) {
	a, err := vm.Pool.Get()
	if err != nil {
		return nil, err
	}
	defer vm.Pool.Put(a)

	l := a.(*lua.LState)
	l.Pop(l.GetTop())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	l.SetContext(ctx)

	err = l.DoString(script)
	if err != nil {
		return nil, err
	}

	vArgs := make([]lua.LValue, len(args))
	for i, arg := range args {
		vArgs[i], err = decodeValue(l, arg)
		if err != nil {
			return nil, err
		}
	}

	f := l.GetGlobal(fnName)
	if f.Type() == lua.LTNil {
		return nil, fmt.Errorf("not found function %v", fnName)
	}
	if f.Type() != lua.LTFunction {
		return nil, fmt.Errorf("%s is not a function: %s", fnName, f.Type())
	}

	err = l.CallByParam(lua.P{
		Fn:      f,
		NRet:    nRets,
		Protect: true,
	}, vArgs...)
	if err != nil {
		return nil, err
	}

	// get rets from stack: [ret1, ret2, ret3 ...]
	rets := make([]lua.LValue, nRets)
	for i := range rets {
		rets[i] = l.Get(i + 1)
	}
	// pop all the values in stack
	l.Pop(l.GetTop())
	return rets, nil
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica by lua script.
func (vm *VM) GetReplicas(obj *unstructured.Unstructured, script string) (replica int32, requires *workv1alpha2.ReplicaRequirements, err error) {
	results, err := vm.RunScript(script, "GetReplicas", 2, obj)
	if err != nil {
		return 0, nil, err
	}

	replica, err = ConvertLuaResultToInt(results[0])
	if err != nil {
		return 0, nil, err
	}

	replicaRequirementResult := results[1]
	requires = &workv1alpha2.ReplicaRequirements{}
	if replicaRequirementResult.Type() == lua.LTTable {
		err = ConvertLuaResultInto(replicaRequirementResult, requires)
		if err != nil {
			klog.Errorf("ConvertLuaResultToReplicaRequirements err %v", err.Error())
			return 0, nil, err
		}
	} else if replicaRequirementResult.Type() == lua.LTNil {
		requires = nil
	} else {
		return 0, nil, fmt.Errorf("expect the returned requires type is table but got %s", replicaRequirementResult.Type())
	}

	return
}

// ReviseReplica revises the replica of the given object by lua.
func (vm *VM) ReviseReplica(object *unstructured.Unstructured, replica int64, script string) (*unstructured.Unstructured, error) {
	results, err := vm.RunScript(script, "ReviseReplica", 1, object, replica)
	if err != nil {
		return nil, err
	}

	luaResult := results[0]
	reviseReplicaResult := &unstructured.Unstructured{}
	if luaResult.Type() == lua.LTTable {
		err := ConvertLuaResultInto(luaResult, reviseReplicaResult)
		if err != nil {
			return nil, err
		}
		return reviseReplicaResult, nil
	}

	return nil, fmt.Errorf("expect the returned requires type is table but got %s", luaResult.Type())
}

func (vm *VM) setLib(l *lua.LState) error {
	for _, pair := range []struct {
		n string
		f lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage},
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		// load our 'safe' version of the OS library
		{lua.OsLibName, lifted.OpenSafeOs},
	} {
		if err := l.CallByParam(lua.P{
			Fn:      l.NewFunction(pair.f),
			NRet:    0,
			Protect: true,
		}, lua.LString(pair.n)); err != nil {
			return err
		}
	}
	return nil
}

// Retain returns the objects that based on the "desired" object but with values retained from the "observed" object by lua.
func (vm *VM) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured, script string) (retained *unstructured.Unstructured, err error) {
	results, err := vm.RunScript(script, "Retain", 1, desired, observed)
	if err != nil {
		return nil, err
	}

	luaResult := results[0]
	retainResult := &unstructured.Unstructured{}
	if luaResult.Type() == lua.LTTable {
		err := ConvertLuaResultInto(luaResult, retainResult)
		if err != nil {
			return nil, err
		}
		return retainResult, nil
	}
	return nil, fmt.Errorf("expect the returned requires type is table but got %s", luaResult.Type())
}

// AggregateStatus returns the objects that based on the 'object' but with status aggregated by lua.
func (vm *VM) AggregateStatus(object *unstructured.Unstructured, items []workv1alpha2.AggregatedStatusItem, script string) (*unstructured.Unstructured, error) {
	results, err := vm.RunScript(script, "AggregateStatus", 1, object, items)
	if err != nil {
		return nil, err
	}

	luaResult := results[0]
	aggregateStatus := &unstructured.Unstructured{}
	if luaResult.Type() == lua.LTTable {
		err := ConvertLuaResultInto(luaResult, aggregateStatus)
		if err != nil {
			return nil, err
		}
		return aggregateStatus, nil
	}
	return nil, fmt.Errorf("expect the returned requires type is table but got %s", luaResult.Type())
}

// InterpretHealth returns the health state of the object by lua.
func (vm *VM) InterpretHealth(object *unstructured.Unstructured, script string) (bool, error) {
	results, err := vm.RunScript(script, "InterpretHealth", 1, object)
	if err != nil {
		return false, err
	}

	var health bool
	health, err = ConvertLuaResultToBool(results[0])
	if err != nil {
		return false, err
	}
	return health, nil
}

// ReflectStatus returns the status of the object by lua.
func (vm *VM) ReflectStatus(object *unstructured.Unstructured, script string) (status *runtime.RawExtension, err error) {
	results, err := vm.RunScript(script, "ReflectStatus", 1, object)
	if err != nil {
		return nil, err
	}

	luaStatusResult := results[0]
	if luaStatusResult.Type() != lua.LTTable {
		return nil, fmt.Errorf("expect the returned replica type is table but got %s", luaStatusResult.Type())
	}

	status = &runtime.RawExtension{}
	err = ConvertLuaResultInto(luaStatusResult, status)
	return status, err
}

// GetDependencies returns the dependent resources of the given object by lua.
func (vm *VM) GetDependencies(object *unstructured.Unstructured, script string) (dependencies []configv1alpha1.DependentObjectReference, err error) {
	results, err := vm.RunScript(script, "GetDependencies", 1, object)
	if err != nil {
		return nil, err
	}

	luaResult := results[0]

	if luaResult.Type() != lua.LTTable {
		return nil, fmt.Errorf("expect the returned requires type is table but got %s", luaResult.Type())
	}
	err = ConvertLuaResultInto(luaResult, &dependencies)
	return
}

// NewWithContext creates a lua VM with the given context.
func NewWithContext(ctx context.Context) (*lua.LState, error) {
	vm := VM{}
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	// Opens table library to allow access to functions to manipulate tables
	err := vm.setLib(l)
	if err != nil {
		return nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, lifted.SafeOsLoader)
	// preload kube library. Allows the 'local kube = require("kube")' to work
	l.PreloadModule(KubeLibName, KubeLoader)
	if ctx != nil {
		l.SetContext(ctx)
	}
	return l, nil
}

// nolint:gocyclo
func decodeValue(L *lua.LState, value interface{}) (lua.LValue, error) {
	// We handle simple type without json for better performance.
	switch converted := value.(type) {
	case []interface{}:
		arr := L.CreateTable(len(converted), 0)
		for _, item := range converted {
			v, err := decodeValue(L, item)
			if err != nil {
				return nil, err
			}
			arr.Append(v)
		}
		return arr, nil
	case map[string]interface{}:
		tbl := L.CreateTable(0, len(converted))
		for key, item := range converted {
			v, err := decodeValue(L, item)
			if err != nil {
				return nil, err
			}
			tbl.RawSetString(key, v)
		}
		return tbl, nil
	case nil:
		return lua.LNil, nil
	}

	v := reflect.ValueOf(value)
	switch {
	case v.CanInt():
		return lua.LNumber(v.Int()), nil
	case v.CanUint():
		return lua.LNumber(v.Uint()), nil
	case v.CanFloat():
		return lua.LNumber(v.Float()), nil
	}

	switch t := v.Type(); t.Kind() {
	case reflect.String:
		return lua.LString(v.String()), nil
	case reflect.Bool:
		return lua.LBool(v.Bool()), nil
	case reflect.Pointer:
		if v.IsNil() {
			return lua.LNil, nil
		}
	}

	// Other types can't be handled, ask for help from json
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("json Marshal obj %#v error: %v", value, err)
	}

	lv, err := luajson.Decode(L, data)
	if err != nil {
		return nil, fmt.Errorf("lua Decode obj %#v error: %v", value, err)
	}
	return lv, nil
}
