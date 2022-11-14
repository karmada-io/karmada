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
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

// VM Defines a struct that implements the luaVM.
type VM struct {
	// UseOpenLibs flag to enable open libraries. Libraries are disabled by default while running, but enabled during testing to allow the use of print statements.
	UseOpenLibs bool
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica by lua script.
func (vm VM) GetReplicas(obj *unstructured.Unstructured, script string) (replica int32, requires *workv1alpha2.ReplicaRequirements, err error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err = vm.setLib(l)
	if err != nil {
		return 0, nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, lifted.SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)

	err = l.DoString(script)
	f := l.GetGlobal("GetReplicas")

	if f.Type() == lua.LTNil {
		return 0, nil, fmt.Errorf("can't get function ReviseReplica pleace check the function name")
	}

	args := make([]lua.LValue, 1)
	args[0], err = decodeValue(l, obj.Object)
	if err != nil {
		return
	}
	err = l.CallByParam(lua.P{Fn: f, NRet: 2, Protect: true}, args...)
	if err != nil {
		return 0, nil, err
	}

	replicaRequirementResult := l.Get(l.GetTop())
	l.Pop(1)

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

	luaReplica := l.Get(l.GetTop())
	replica, err = ConvertLuaResultToInt(luaReplica)
	if err != nil {
		return 0, nil, err
	}
	return
}

// ReviseReplica revises the replica of the given object by lua.
func (vm VM) ReviseReplica(object *unstructured.Unstructured, replica int64, script string) (*unstructured.Unstructured, error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err := vm.setLib(l)
	if err != nil {
		return nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, lifted.SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)

	err = l.DoString(script)
	if err != nil {
		return nil, err
	}
	reviseReplicaLuaFunc := l.GetGlobal("ReviseReplica")
	if reviseReplicaLuaFunc.Type() == lua.LTNil {
		return nil, fmt.Errorf("can't get function ReviseReplica pleace check the function name")
	}

	args := make([]lua.LValue, 2)
	args[0], err = decodeValue(l, object.Object)
	if err != nil {
		return nil, err
	}
	args[1], err = decodeValue(l, replica)
	if err != nil {
		return nil, err
	}
	err = l.CallByParam(lua.P{Fn: reviseReplicaLuaFunc, NRet: 1, Protect: true}, args...)
	if err != nil {
		return nil, err
	}

	luaResult := l.Get(l.GetTop())
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

func (vm VM) setLib(l *lua.LState) error {
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
func (vm VM) Retain(desired *unstructured.Unstructured, observed *unstructured.Unstructured, script string) (retained *unstructured.Unstructured, err error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err = vm.setLib(l)
	if err != nil {
		return nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, lifted.SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)

	err = l.DoString(script)
	if err != nil {
		return nil, err
	}
	retainLuaFunc := l.GetGlobal("Retain")
	if retainLuaFunc.Type() == lua.LTNil {
		return nil, fmt.Errorf("can't get function Retatin pleace check the function ")
	}

	args := make([]lua.LValue, 2)
	args[0], err = decodeValue(l, desired.Object)
	if err != nil {
		return
	}
	args[1], err = decodeValue(l, observed.Object)
	if err != nil {
		return
	}
	err = l.CallByParam(lua.P{Fn: retainLuaFunc, NRet: 1, Protect: true}, args...)
	if err != nil {
		return nil, err
	}

	luaResult := l.Get(l.GetTop())
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
func (vm VM) AggregateStatus(object *unstructured.Unstructured, item []map[string]interface{}, script string) (*unstructured.Unstructured, error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err := vm.setLib(l)
	if err != nil {
		return nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, lifted.SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)

	err = l.DoString(script)
	if err != nil {
		return nil, err
	}

	f := l.GetGlobal("AggregateStatus")
	if f.Type() == lua.LTNil {
		return nil, fmt.Errorf("can't get function AggregateStatus pleace check the function ")
	}
	args := make([]lua.LValue, 2)
	args[0], err = decodeValue(l, object.Object)
	if err != nil {
		return nil, err
	}
	args[1], err = decodeValue(l, item)
	if err != nil {
		return nil, err
	}
	err = l.CallByParam(lua.P{Fn: f, NRet: 1, Protect: true}, args...)
	if err != nil {
		return nil, err
	}

	luaResult := l.Get(l.GetTop())
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
func (vm VM) InterpretHealth(object *unstructured.Unstructured, script string) (bool, error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err := vm.setLib(l)
	if err != nil {
		return false, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, lifted.SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)

	err = l.DoString(script)
	if err != nil {
		return false, err
	}
	f := l.GetGlobal("InterpretHealth")
	if f.Type() == lua.LTNil {
		return false, fmt.Errorf("can't get function InterpretHealth pleace check the function ")
	}

	args := make([]lua.LValue, 1)
	args[0], err = decodeValue(l, object.Object)
	if err != nil {
		return false, err
	}
	err = l.CallByParam(lua.P{Fn: f, NRet: 1, Protect: true}, args...)
	if err != nil {
		return false, err
	}

	var health bool
	luaResult := l.Get(l.GetTop())
	health, err = ConvertLuaResultToBool(luaResult)
	if err != nil {
		return false, err
	}
	return health, nil
}

// ReflectStatus returns the status of the object by lua.
func (vm VM) ReflectStatus(object *unstructured.Unstructured, script string) (status *runtime.RawExtension, err error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err = vm.setLib(l)
	if err != nil {
		return nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, lifted.SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)

	err = l.DoString(script)
	if err != nil {
		return nil, err
	}
	f := l.GetGlobal("ReflectStatus")
	if f.Type() == lua.LTNil {
		return nil, fmt.Errorf("can't get function ReflectStatus pleace check the function ")
	}

	args := make([]lua.LValue, 1)
	args[0], err = decodeValue(l, object.Object)
	if err != nil {
		return
	}
	err = l.CallByParam(lua.P{Fn: f, NRet: 2, Protect: true}, args...)
	if err != nil {
		return nil, err
	}
	luaStatusResult := l.Get(l.GetTop())
	l.Pop(1)
	if luaStatusResult.Type() != lua.LTTable {
		return nil, fmt.Errorf("expect the returned replica type is table but got %s", luaStatusResult.Type())
	}

	luaExistResult := l.Get(l.GetTop())
	var exist bool
	exist, err = ConvertLuaResultToBool(luaExistResult)
	if err != nil {
		return nil, err
	}

	if exist {
		resultMap := make(map[string]interface{})
		jsonBytes, err := luajson.Encode(luaStatusResult)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonBytes, &resultMap)
		if err != nil {
			return nil, err
		}
		return helper.BuildStatusRawExtension(resultMap)
	}
	return nil, err
}

// GetDependencies returns the dependent resources of the given object by lua.
func (vm VM) GetDependencies(object *unstructured.Unstructured, script string) (dependencies []configv1alpha1.DependentObjectReference, err error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manipulate tables
	err = vm.setLib(l)
	if err != nil {
		return nil, err
	}
	// preload our 'safe' version of the OS library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, lifted.SafeOsLoader)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)

	err = l.DoString(script)
	if err != nil {
		return nil, err
	}
	f := l.GetGlobal("GetDependencies")
	if f.Type() == lua.LTNil {
		return nil, fmt.Errorf("can't get function Retatin pleace check the function ")
	}

	args := make([]lua.LValue, 1)
	args[0], err = decodeValue(l, object.Object)
	if err != nil {
		return
	}
	err = l.CallByParam(lua.P{Fn: f, NRet: 1, Protect: true}, args...)
	if err != nil {
		return nil, err
	}

	luaResult := l.Get(l.GetTop())
	if luaResult.Type() == lua.LTTable {
		jsonBytes, err := luajson.Encode(luaResult)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonBytes, &dependencies)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("expect the returned requires type is table but got %s", luaResult.Type())
	}
	return
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
