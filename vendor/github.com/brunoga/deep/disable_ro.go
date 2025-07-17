package deep

import (
	"reflect"
	"unsafe"
)

func init() {
	// Try to make sure we can use the trick in this file. It is still possible
	// that things will break but we should be able to detect most of them.
	t := reflect.TypeOf(reflect.Value{})
	if t.Kind() != reflect.Struct {
		panic("deep: reflect.Value is not a struct")
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Name == "flag" {
			// Check that the field is a uintptr.
			if f.Type.Kind() != reflect.Uintptr {
				panic("deep: reflect.Value.flag is not a uintptr")
			}

			flagOffset = f.Offset

			return
		}
	}

	panic("deep: reflect.Value has no flag field")
}

var flagOffset uintptr

const (
	// Lifted from go/src/reflect/value.go.
	flagStickyRO uintptr = 1 << 5
	flagEmbedRO  uintptr = 1 << 6
	flagRO       uintptr = flagStickyRO | flagEmbedRO
)

// disableRO disables the read-only flag of a reflect.Value object. We use this
// to allow the reflect package to access the value of an unexported field as
// if it was exported (so we can copy it).
//
// DANGER! HERE BE DRAGONS! This is brittle and depends on internal state of a
// reflect.Value object. It is not guaranteed to work in future (or previous)
// versions of Go although we try to detect changes and panic immediatelly
// during initialization.
func disableRO(v *reflect.Value) {
	// Get pointer to flags.
	flags := (*uintptr)(unsafe.Pointer(uintptr(unsafe.Pointer(v)) + flagOffset))

	// Clear the read-only flags.
	*flags &^= flagRO
}
