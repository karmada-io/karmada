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
// For security reasons, we made the following changes to restrict the string library functions used when users customize the Karmada Lua interpreter.
// 1. do not allow users to use string.gsub and string.rep when interpreting resources with lua scripts, which may be used to create overly long strings.
// 2. limit the length of the string type parameters of the function to 1000,000.
// 3. add timeout checks to the internal for loops within the functions.

import (
	"fmt"
	"strings"
	"unsafe"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/pm"
)

const emptyLString = lua.LString("")
const maxInputStringParamsLen = 1000000

// OpenSafeString open safe string
func OpenSafeString(L *lua.LState) int {
	var mod *lua.LTable

	mod = L.RegisterModule(lua.StringLibName, strFuncs).(*lua.LTable)
	gmatch := L.NewClosure(strGmatch, L.NewFunction(strGmatchIter))
	mod.RawSetString("gmatch", gmatch)
	mod.RawSetString("gfind", gmatch)
	mod.RawSetString("__index", mod)

	L.SetMetatable(emptyLString, mod)
	L.Push(mod)
	return 1
}

var strFuncs = map[string]lua.LGFunction{
	"byte":    strByte,
	"char":    strChar,
	"dump":    strDump,
	"find":    strFind,
	"format":  strFormat,
	"gsub":    strGsub,
	"len":     strLen,
	"lower":   strLower,
	"match":   strMatch,
	"rep":     strRep,
	"reverse": strReverse,
	"sub":     strSub,
	"upper":   strUpper,
}

func strByte(L *lua.LState) int {
	str := L.CheckString(1)
	validateStrParamsLen(L, str)
	start := L.OptInt(2, 1) - 1
	end := L.OptInt(3, -1)
	l := len(str)
	if start < 0 {
		start = l + start + 1
	}
	if end < 0 {
		end = l + end + 1
	}

	if L.GetTop() == 2 {
		if start < 0 || start >= l {
			return 0
		}
		L.Push(lua.LNumber(str[start]))
		return 1
	}

	start = intMax(start, 0)
	end = intMin(end, l)
	if end < 0 || end <= start || start >= l {
		return 0
	}

	for i := start; i < end; i++ {
		raiseErrorIfContextIsDone(L)
		L.Push(lua.LNumber(str[i]))
	}
	return end - start
}

func strChar(L *lua.LState) int {
	top := L.GetTop()
	bytes := make([]byte, L.GetTop())
	for i := 1; i <= top; i++ {
		raiseErrorIfContextIsDone(L)
		bytes[i-1] = uint8(L.CheckInt(i))
	}
	L.Push(lua.LString(string(bytes)))
	return 1
}

func strDump(L *lua.LState) int {
	L.RaiseError("The Lua function string.dump is not supported by Karmada. If you believe this function is essential for your use case, please submit an issue to let us know why you need it.")
	return 0
}

func strFind(L *lua.LState) int {
	str := L.CheckString(1)
	validateStrParamsLen(L, str)
	pattern := L.CheckString(2)
	validateStrParamsLen(L, pattern)
	if len(pattern) == 0 {
		L.Push(lua.LNumber(1))
		L.Push(lua.LNumber(0))
		return 2
	}
	init := luaIndex2StringIndex(str, L.OptInt(3, 1), true)
	plain := false
	if L.GetTop() == 4 {
		plain = lua.LVAsBool(L.Get(4))
	}

	if plain {
		pos := strings.Index(str[init:], pattern)
		if pos < 0 {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(lua.LNumber(init+pos) + 1)
		L.Push(lua.LNumber(init + pos + len(pattern)))
		return 2
	}

	mds, err := pm.Find(pattern, unsafeFastStringToReadOnlyBytes(str), init, 1)
	if err != nil {
		L.RaiseError("%s", err.Error())
	}
	if len(mds) == 0 {
		L.Push(lua.LNil)
		return 1
	}
	md := mds[0]
	L.Push(lua.LNumber(md.Capture(0) + 1))
	L.Push(lua.LNumber(md.Capture(1)))
	for i := 2; i < md.CaptureLength(); i += 2 {
		raiseErrorIfContextIsDone(L)
		if md.IsPosCapture(i) {
			L.Push(lua.LNumber(md.Capture(i)))
		} else {
			L.Push(lua.LString(str[md.Capture(i):md.Capture(i+1)]))
		}
	}
	return md.CaptureLength()/2 + 1
}

func strFormat(L *lua.LState) int {
	str := L.CheckString(1)
	validateStrParamsLen(L, str)
	args := make([]interface{}, L.GetTop()-1)
	top := L.GetTop()
	for i := 2; i <= top; i++ {
		raiseErrorIfContextIsDone(L)
		args[i-2] = L.Get(i)
	}
	npat := strings.Count(str, "%") - strings.Count(str, "%%")
	L.Push(lua.LString(fmt.Sprintf(str, args[:intMin(npat, len(args))]...)))
	return 1
}

func strGsub(L *lua.LState) int {
	L.RaiseError("The Lua function string.gsub is not supported by Karmada. If you believe this function is essential for your use case, please submit an issue to let us know why you need it.")
	return 0
}

type strMatchData struct {
	str     string
	pos     int
	matches []*pm.MatchData
}

func strGmatchIter(L *lua.LState) int {
	md := L.CheckUserData(1).Value.(*strMatchData)
	str := md.str
	matches := md.matches
	idx := md.pos
	md.pos += 1
	if idx == len(matches) {
		return 0
	}
	L.Push(L.Get(1))
	match := matches[idx]
	if match.CaptureLength() == 2 {
		L.Push(lua.LString(str[match.Capture(0):match.Capture(1)]))
		return 1
	}

	for i := 2; i < match.CaptureLength(); i += 2 {
		raiseErrorIfContextIsDone(L)
		if match.IsPosCapture(i) {
			L.Push(lua.LNumber(match.Capture(i)))
		} else {
			L.Push(lua.LString(str[match.Capture(i):match.Capture(i+1)]))
		}
	}
	return match.CaptureLength()/2 - 1
}

func strGmatch(L *lua.LState) int {
	str := L.CheckString(1)
	validateStrParamsLen(L, str)
	pattern := L.CheckString(2)
	validateStrParamsLen(L, pattern)
	mds, err := pm.Find(pattern, []byte(str), 0, -1)
	if err != nil {
		L.RaiseError("%s", err.Error())
	}
	L.Push(L.Get(lua.UpvalueIndex(1)))
	ud := L.NewUserData()
	ud.Value = &strMatchData{str, 0, mds}
	L.Push(ud)
	return 2
}

func strLen(L *lua.LState) int {
	str := L.CheckString(1)
	L.Push(lua.LNumber(len(str)))
	return 1
}

func strLower(L *lua.LState) int {
	str := L.CheckString(1)
	validateStrParamsLen(L, str)
	L.Push(lua.LString(strings.ToLower(str)))
	return 1
}

func strMatch(L *lua.LState) int {
	str := L.CheckString(1)
	validateStrParamsLen(L, str)
	pattern := L.CheckString(2)
	validateStrParamsLen(L, pattern)
	offset := L.OptInt(3, 1)
	l := len(str)
	if offset < 0 {
		offset = l + offset + 1
	}
	offset--
	if offset < 0 {
		offset = 0
	}

	mds, err := pm.Find(pattern, unsafeFastStringToReadOnlyBytes(str), offset, 1)
	if err != nil {
		L.RaiseError("%s", err.Error())
	}
	if len(mds) == 0 {
		L.Push(lua.LNil)
		return 0
	}
	md := mds[0]
	nsubs := md.CaptureLength() / 2
	switch nsubs {
	case 1:
		L.Push(lua.LString(str[md.Capture(0):md.Capture(1)]))
		return 1
	default:
		for i := 2; i < md.CaptureLength(); i += 2 {
			if md.IsPosCapture(i) {
				L.Push(lua.LNumber(md.Capture(i)))
			} else {
				L.Push(lua.LString(str[md.Capture(i):md.Capture(i+1)]))
			}
		}
		return nsubs - 1
	}
}

func strRep(L *lua.LState) int {
	L.RaiseError("The Lua function string.rep is not supported by Karmada. If you believe this function is essential for your use case, please submit an issue to let us know why you need it.")
	return 0
}

func strReverse(L *lua.LState) int {
	str := L.CheckString(1)
	validateStrParamsLen(L, str)
	bts := []byte(str)
	out := make([]byte, len(bts))
	for i, j := 0, len(bts)-1; j >= 0; i, j = i+1, j-1 {
		raiseErrorIfContextIsDone(L)
		out[i] = bts[j]
	}
	L.Push(lua.LString(string(out)))
	return 1
}

func strSub(L *lua.LState) int {
	str := L.CheckString(1)
	validateStrParamsLen(L, str)
	start := luaIndex2StringIndex(str, L.CheckInt(2), true)
	end := luaIndex2StringIndex(str, L.OptInt(3, -1), false)
	l := len(str)
	if start >= l || end < start {
		L.Push(emptyLString)
	} else {
		L.Push(lua.LString(str[start:end]))
	}
	return 1
}

func strUpper(L *lua.LState) int {
	str := L.CheckString(1)
	validateStrParamsLen(L, str)
	L.Push(lua.LString(strings.ToUpper(str)))
	return 1
}

func luaIndex2StringIndex(str string, i int, start bool) int {
	if start && i != 0 {
		i -= 1
	}
	l := len(str)
	if i < 0 {
		i = l + i + 1
	}
	i = intMax(0, i)
	if !start && i > l {
		i = l
	}
	return i
}

func unsafeFastStringToReadOnlyBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func intMin(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func intMax(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func validateStrParamsLen(L *lua.LState, str string) {
	strLength := len(str)
	if strLength > maxInputStringParamsLen {
		L.RaiseError("string length %d exceeds the maximum allowed input string length %d in Karmada lua string lib, please check your lua script.", strLength, maxInputStringParamsLen)
	}
}

func raiseErrorIfContextIsDone(L *lua.LState) {
	if L.Context() == nil {
		return
	}

	select {
	case <-L.Context().Done():
		L.RaiseError("%s", L.Context().Err().Error())
	default:
	}
}
