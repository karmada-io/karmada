package lifted

import (
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func TestOpenSafeOs(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	expect := 1

	actual := OpenSafeOs(L)

	if actual != expect {
		t.Errorf("OpenSafeOs returned %v, expected %v", actual, expect)
	}
}

func TestSafeOsLoader(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	expect := 1

	actual := SafeOsLoader(L)

	if actual != expect {
		t.Errorf("SafeOsLoader returned %v, expected %v", actual, expect)
	}
}

func TestOsTime(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	actual := osTime(L)
	if actual != 1 {
		t.Errorf("osTime returned %v, expected %v", actual, 1)
	}
}

func TestGetIntField(t *testing.T) {
	tb := &lua.LTable{}
	tb.RawSetString("min", lua.LNumber(15))
	tb.RawSetString("day", lua.LString("a"))

	// Test with valid key
	expected := 15
	if v := getIntField(tb, "min", 0); v != expected {
		t.Errorf("getIntField(tb, \"min\", 0) returned %d, expected %d", v, expected)
	}

	// Test with non-number value
	expected = 0
	if v := getIntField(tb, "day", 0); v != expected {
		t.Errorf("getIntField(tb, \"day\", 0) returned %d, expected %d", v, expected)
	}
}

func TestGetBoolField(t *testing.T) {
	tb := &lua.LTable{}
	tb.RawSetString("min", lua.LNumber(15))
	tb.RawSetString("isdst", lua.LBool(false))

	// Test with valid key
	if v := getBoolField(tb, "isdst", false); v {
		t.Errorf("getBoolField(tb, \"isdst\", false) returned %v, expected %v", v, false)
	}

	// Test with non-number value
	if v := getBoolField(tb, "min", true); !v {
		t.Errorf("getBoolField(tb, \"min\", true) returned %v, expected %v", v, true)
	}
}

func TestStrftime(t *testing.T) {
	tests := []struct {
		name string
		time time.Time
		cfmt string
		want string
	}{
		{
			name: "character in cDateFlagToGo",
			time: time.Date(2022, time.February, 16, 15, 45, 27, 0, time.UTC),
			cfmt: "%Y/%m/%d %H:%M:%S",
			want: "2022/02/16 15:45:27",
		},
		{
			name: "character not in cDateFlagToGo",
			time: time.Date(2022, time.February, 16, 15, 45, 27, 0, time.FixedZone("", -8*60*60)),
			cfmt: "%A, %w %B %Y %I:%M:%S %p %Z %e",
			want: "Wednesday, 3 February 2022 03:45:27 PM -0800 %e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strftime(tt.time, tt.cfmt); got != tt.want {
				t.Errorf("strftime(%v, %q) got %q, want %q", tt.time, tt.cfmt, got, tt.want)
			}
		})
	}
}
