package luavm

import (
	lua "github.com/yuin/gopher-lua"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/karmada-io/karmada/pkg/util/helper"
)

const (
	// KubeLibName is the name of the kube library.
	KubeLibName = "kube"
)

// KubeLoader loads the kube library. Import this library before your script by:
//
//	local kube = require("kube")
//
// Then you can call functions in this library by:
//
//	kube.xxx()
//
// This library Contains:
//   - function resourceAdd(r1, r2, ...)
//     accruing the quantity of resources. Example:
//     cpu = kube.resourceAdd(r1.cpu, r2.cpu, r3.cpu)
//   - function accuratePodRequirements(pod) requirements
//     accurate total resource requirements for pod. Example:
//     requirements = kube.accuratePodRequirements(pod)
func KubeLoader(ls *lua.LState) int {
	mod := ls.SetFuncs(ls.NewTable(), kubeFuncs)
	ls.Push(mod)
	return 1
}

var kubeFuncs = map[string]lua.LGFunction{
	"resourceAdd":             resourceAdd,
	"accuratePodRequirements": accuratePodRequirements,
}

func resourceAdd(ls *lua.LState) int {
	res := resource.Quantity{}
	n := ls.GetTop()
	for i := 1; i <= n; i++ {
		q := checkResourceQuantity(ls, i)
		res.Add(q)
	}

	s := res.String()
	ls.Push(lua.LString(s))
	return 1
}

func accuratePodRequirements(ls *lua.LState) int {
	n := ls.GetTop()
	if n != 1 {
		ls.RaiseError("getPodRequirements only accepts one argument")
		return 0
	}

	v := ls.CheckTable(1)
	pod := &corev1.PodTemplateSpec{}
	err := ConvertLuaResultInto(v, pod)
	if err != nil {
		ls.RaiseError("fail to convert lua value %#v to PodTemplateSpec: %v", v, err)
		return 0
	}

	requirements := helper.GenerateReplicaRequirements(pod)
	retValue, err := decodeValue(ls, requirements)
	if err != nil {
		ls.RaiseError("fail to convert %#v to Lua value: %v", requirements, err)
		return 0
	}

	ls.Push(retValue)
	return 1
}

func checkResourceQuantity(ls *lua.LState, n int) resource.Quantity {
	v := ls.Get(n)
	switch typ := v.Type(); typ {
	case lua.LTNil:
		return resource.Quantity{}
	case lua.LTString:
		s := string(v.(lua.LString))
		return resource.MustParse(s)
	default:
		ls.TypeError(n, lua.LTString)
		return resource.Quantity{}
	}
}
