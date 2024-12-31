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

package lua

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	lua "github.com/yuin/gopher-lua"
)

func TestOpenSafeOs(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	assert.Equal(t, 1, OpenSafeOs(L))

	// Test registered functions
	osTable := L.GetGlobal(lua.OsLibName)
	assert.Equal(t, lua.LTTable, osTable.Type())
	assert.NotNil(t, L.GetField(osTable, "time"))
	assert.NotNil(t, L.GetField(osTable, "date"))
}

func TestSafeOsLoader(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	assert.Equal(t, 1, SafeOsLoader(L))

	// Verify module table is returned
	result := L.Get(-1)
	assert.Equal(t, lua.LTTable, result.Type())
}

func TestOsTime(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	t.Run("no args", func(t *testing.T) {
		assert.Equal(t, 1, osTime(L))
		result := L.Get(-1)
		assert.Equal(t, lua.LTNumber, result.Type())

		now := time.Now().Unix()
		timestamp := int64(result.(lua.LNumber))
		assert.InDelta(t, now, timestamp, 1.0)
	})

	t.Run("with table", func(t *testing.T) {
		L.Push(L.NewFunction(func(L *lua.LState) int {
			tbl := L.NewTable()
			tbl.RawSetString("year", lua.LNumber(2024))
			tbl.RawSetString("month", lua.LNumber(3))
			tbl.RawSetString("day", lua.LNumber(14))
			tbl.RawSetString("hour", lua.LNumber(12))
			tbl.RawSetString("min", lua.LNumber(30))
			tbl.RawSetString("sec", lua.LNumber(45))
			tbl.RawSetString("isdst", lua.LBool(false))
			L.Push(tbl)
			return osTime(L)
		}))

		assert.NoError(t, L.PCall(0, 1, nil))
		result := L.Get(-1)
		assert.Equal(t, lua.LTNumber, result.Type())

		expectedTime := time.Date(2024, 3, 14, 12, 30, 45, 0, time.Local)
		assert.Equal(t, expectedTime.Unix(), int64(result.(lua.LNumber)))
	})
}

func TestOsDate(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	t.Run("default format", func(t *testing.T) {
		L.Push(L.NewFunction(func(L *lua.LState) int {
			return osDate(L)
		}))
		assert.NoError(t, L.PCall(0, 1, nil))
		result := L.Get(-1)
		assert.Equal(t, lua.LTString, result.Type())
	})

	t.Run("UTC time", func(t *testing.T) {
		L.Push(L.NewFunction(func(L *lua.LState) int {
			L.Push(lua.LString("!%Y-%m-%d"))
			return osDate(L)
		}))
		assert.NoError(t, L.PCall(0, 1, nil))
		result := L.Get(-1)
		assert.Equal(t, lua.LTString, result.Type())

		now := time.Now().UTC()
		expected := now.Format("2006-01-02")
		assert.Equal(t, expected, string(result.(lua.LString)))
	})

	t.Run("custom timestamp", func(t *testing.T) {
		timestamp := time.Date(2024, 3, 14, 15, 30, 45, 0, time.Local)
		L.Push(L.NewFunction(func(L *lua.LState) int {
			L.Push(lua.LString("%Y-%m-%d %H:%M:%S"))
			L.Push(lua.LNumber(timestamp.Unix()))
			return osDate(L)
		}))
		assert.NoError(t, L.PCall(0, 1, nil))
		result := L.Get(-1)
		assert.Equal(t, "2024-03-14 15:30:45", string(result.(lua.LString)))
	})

	t.Run("table format", func(t *testing.T) {
		timestamp := time.Now().Unix()
		L.Push(L.NewFunction(func(L *lua.LState) int {
			L.Push(lua.LString("*t"))
			L.Push(lua.LNumber(timestamp))
			return osDate(L)
		}))
		assert.NoError(t, L.PCall(0, 1, nil))

		result := L.Get(-1).(*lua.LTable)
		tm := time.Unix(timestamp, 0)
		assert.Equal(t, float64(tm.Year()), float64(result.RawGetString("year").(lua.LNumber)))
		assert.Equal(t, float64(tm.Month()), float64(result.RawGetString("month").(lua.LNumber)))
		assert.Equal(t, float64(tm.Day()), float64(result.RawGetString("day").(lua.LNumber)))
		assert.Equal(t, float64(tm.Hour()), float64(result.RawGetString("hour").(lua.LNumber)))
		assert.Equal(t, float64(tm.Minute()), float64(result.RawGetString("min").(lua.LNumber)))
		assert.Equal(t, float64(tm.Second()), float64(result.RawGetString("sec").(lua.LNumber)))
		assert.Equal(t, float64(tm.Weekday()+1), float64(result.RawGetString("wday").(lua.LNumber)))
	})
}

func TestGetIntField(t *testing.T) {
	tb := &lua.LTable{}
	tb.RawSetString("min", lua.LNumber(15))
	tb.RawSetString("day", lua.LString("a"))

	t.Run("valid number", func(t *testing.T) {
		assert.Equal(t, 15, getIntField(tb, "min", 0))
	})

	t.Run("non-number value", func(t *testing.T) {
		assert.Equal(t, 0, getIntField(tb, "day", 0))
	})

	t.Run("missing key", func(t *testing.T) {
		assert.Equal(t, 42, getIntField(tb, "nonexistent", 42))
	})
}

func TestGetBoolField(t *testing.T) {
	tb := &lua.LTable{}
	tb.RawSetString("flag1", lua.LBool(true))
	tb.RawSetString("flag2", lua.LBool(false))
	tb.RawSetString("notbool", lua.LNumber(1))

	t.Run("true value", func(t *testing.T) {
		assert.True(t, getBoolField(tb, "flag1", false))
	})

	t.Run("false value", func(t *testing.T) {
		assert.False(t, getBoolField(tb, "flag2", true))
	})

	t.Run("non-bool value", func(t *testing.T) {
		assert.True(t, getBoolField(tb, "notbool", true))
	})

	t.Run("missing key", func(t *testing.T) {
		assert.True(t, getBoolField(tb, "nonexistent", true))
	})
}

func TestStrftime(t *testing.T) {
	tests := []struct {
		name string
		time time.Time
		cfmt string
		want string
	}{
		{
			name: "basic format",
			time: time.Date(2024, 3, 14, 15, 30, 45, 0, time.UTC),
			cfmt: "%Y-%m-%d %H:%M:%S",
			want: "2024-03-14 15:30:45",
		},
		{
			name: "escaped percent",
			time: time.Date(2024, 3, 14, 15, 30, 45, 0, time.UTC),
			cfmt: "%%Y",
			want: "%Y",
		},
		{
			name: "weekday",
			time: time.Date(2024, 3, 14, 15, 30, 45, 0, time.UTC),
			cfmt: "%w",
			want: "4",
		},
		{
			name: "month names",
			time: time.Date(2024, 3, 14, 15, 30, 45, 0, time.UTC),
			cfmt: "%B %b",
			want: "March Mar",
		},
		{
			name: "12-hour clock",
			time: time.Date(2024, 3, 14, 15, 30, 45, 0, time.UTC),
			cfmt: "%I %p",
			want: "03 PM",
		},
		{
			name: "timezone",
			time: time.Date(2024, 3, 14, 15, 30, 45, 0, time.UTC),
			cfmt: "%z %Z",
			want: "+0000 UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, strftime(tt.time, tt.cfmt))
		})
	}
}

func TestFlagScanner(t *testing.T) {
	t.Run("basic scanning", func(t *testing.T) {
		scanner := newFlagScanner('%', "", "", "test%%string")

		// Scan all characters before String() is valid
		for c, eos := scanner.Next(); !eos; c, eos = scanner.Next() {
			scanner.AppendChar(c)
		}

		assert.Equal(t, "test%string", scanner.String())
	})

	t.Run("empty string", func(t *testing.T) {
		scanner := newFlagScanner('%', "", "", "")
		c, eos := scanner.Next()
		assert.Equal(t, byte(0), c)
		assert.True(t, eos)
	})

	t.Run("single flag", func(t *testing.T) {
		scanner := newFlagScanner('%', "<", ">", "%d")
		scanner.Next() // Skip first %
		c, eos := scanner.Next()
		assert.Equal(t, byte('d'), c)
		assert.False(t, eos)
		assert.True(t, scanner.HasFlag)
	})
}
