/*
The MIT License (MIT)

Copyright (c) 2015 Yusuke Inuzuka

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package lua

// This code is directly lifted from the github.com/yuin/gopher-lua codebase in order to use the Lua interpreter as safely as possible.
// We do not allow users to load modules using the path patterns defined in package.path (for Lua files) and package.cpath (for C libraries) when interpreting resources with lua scripts, which may potentially grant providers access to modules loaded outside the sandbox.

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

/* load lib {{{ */

var loLoaders = []lua.LGFunction{loLoaderPreload}

func OpenPackage(L *lua.LState) int {
	packagemod := L.RegisterModule(lua.LoadLibName, loFuncs)

	L.SetField(packagemod, "preload", L.NewTable())

	loaders := L.CreateTable(len(loLoaders), 0)
	for i, loader := range loLoaders {
		L.RawSetInt(loaders, i+1, L.NewFunction(loader))
	}
	L.SetField(packagemod, "loaders", loaders)
	L.SetField(L.Get(lua.RegistryIndex), "_LOADERS", loaders)

	loaded := L.NewTable()
	L.SetField(packagemod, "loaded", loaded)
	L.SetField(L.Get(lua.RegistryIndex), "_LOADED", loaded)

	L.Push(packagemod)
	return 1
}

var loFuncs = map[string]lua.LGFunction{
	"loadlib": loLoadLib,
}

func loLoaderPreload(L *lua.LState) int {
	name := L.CheckString(1)
	preload := L.GetField(L.GetField(L.Get(lua.EnvironIndex), "package"), "preload")
	if _, ok := preload.(*lua.LTable); !ok {
		L.RaiseError("package.preload must be a table")
	}
	lv := L.GetField(preload, name)
	if lv == lua.LNil {
		L.Push(lua.LString(fmt.Sprintf("no field package.preload['%s']", name)))
		return 1
	}
	L.Push(lv)
	return 1
}

func loLoadLib(L *lua.LState) int {
	L.RaiseError("loadlib is not supported")
	return 0
}

/* }}} */

//
