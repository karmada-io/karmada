/*
Copyright 2024 The Karmada Authors.

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

package lua

import (
	"testing"

	"github.com/stretchr/testify/assert"
	lua "github.com/yuin/gopher-lua"
)

func TestLoLoaderPreload(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	OpenPackage(L)

	t.Run("nonexistent module", func(t *testing.T) {
		L.Push(L.NewFunction(func(L *lua.LState) int {
			L.Push(lua.LString("nonexistent"))
			return loLoaderPreload(L)
		}))
		assert.NoError(t, L.PCall(0, 1, nil))
		assert.Equal(t, "no field package.preload['nonexistent']", L.ToString(-1))
	})

	t.Run("existing module", func(t *testing.T) {
		L.GetField(L.GetField(L.Get(lua.EnvironIndex), "package"), "preload").(*lua.LTable).RawSetString("testmod", L.NewFunction(func(_ *lua.LState) int { return 0 }))
		L.Push(L.NewFunction(func(L *lua.LState) int {
			L.Push(lua.LString("testmod"))
			return loLoaderPreload(L)
		}))
		assert.NoError(t, L.PCall(0, 1, nil))
		assert.Equal(t, lua.LTFunction, L.Get(-1).Type())
	})
}

func TestLoLoadLib(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	L.Push(L.NewFunction(func(L *lua.LState) int {
		return loLoadLib(L)
	}))
	err := L.PCall(0, 0, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loadlib is not supported")
}

func TestOpenPackage(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	assert.Equal(t, 1, OpenPackage(L))
	pkg := L.Get(-1)
	assert.Equal(t, lua.LTTable, pkg.Type())
	assert.Equal(t, lua.LTTable, L.GetField(pkg, "preload").Type())
	assert.Equal(t, lua.LTTable, L.GetField(pkg, "loaders").Type())
	assert.Equal(t, lua.LTTable, L.GetField(pkg, "loaded").Type())
}
