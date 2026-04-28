package deep

import (
	"fmt"
	"reflect"
)

// Copy creates a deep copy of src. It returns the copy and a nil error in case
// of success and the zero value for the type and a non-nil error on failure.
func Copy[T any](src T) (T, error) {
	return copy(src, false)
}

// CopySkipUnsupported creates a deep copy of src. It returns the copy and a nil
// errorin case of success and the zero value for the type and a non-nil error
// on failure. Unsupported types are skipped (the copy will have the zero value
// for the type) instead of returning an error.
func CopySkipUnsupported[T any](src T) (T, error) {
	return copy(src, true)
}

// MustCopy creates a deep copy of src. It returns the copy on success or panics
// in case of any failure.
func MustCopy[T any](src T) T {
	dst, err := copy(src, false)
	if err != nil {
		panic(err)
	}

	return dst
}

type pointersMap map[uintptr]map[string]reflect.Value

func copy[T any](src T, skipUnsupported bool) (T, error) {
	v := reflect.ValueOf(src)

	// We might have a zero value, so we check for this here otherwise
	// calling interface below will panic.
	if v.Kind() == reflect.Invalid {
		// This amounts to returning the zero value for T.
		var t T
		return t, nil
	}

	dst, err := recursiveCopy(v, make(pointersMap),
		skipUnsupported)
	if err != nil {
		var t T
		return t, err
	}

	return dst.Interface().(T), nil
}

func recursiveCopy(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	switch v.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128, reflect.String:
		// Direct type, just copy it.
		return v, nil
	case reflect.Array:
		return recursiveCopyArray(v, pointers, skipUnsupported)
	case reflect.Interface:
		return recursiveCopyInterface(v, pointers, skipUnsupported)
	case reflect.Map:
		return recursiveCopyMap(v, pointers, skipUnsupported)
	case reflect.Ptr:
		return recursiveCopyPtr(v, pointers, skipUnsupported)
	case reflect.Slice:
		return recursiveCopySlice(v, pointers, skipUnsupported)
	case reflect.Struct:
		return recursiveCopyStruct(v, pointers, skipUnsupported)
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		if v.IsNil() {
			// If we have a nil function, unsafe pointer or channel, then we
			// can copy it.
			return v, nil
		} else {
			if skipUnsupported {
				return reflect.Zero(v.Type()), nil
			} else {
				return reflect.Value{}, fmt.Errorf("unsuported non-nil value for type: %s", v.Type())
			}
		}
	default:
		if skipUnsupported {
			return reflect.Zero(v.Type()), nil
		} else {
			return reflect.Value{}, fmt.Errorf("unsuported type: %s", v.Type())
		}
	}
}

func recursiveCopyArray(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	dst := reflect.New(v.Type()).Elem()

	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		elemDst, err := recursiveCopy(elem, pointers, skipUnsupported)
		if err != nil {
			return reflect.Value{}, err
		}

		dst.Index(i).Set(elemDst)
	}

	return dst, nil
}

func recursiveCopyInterface(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	if v.IsNil() {
		// If the interface is nil, just return it.
		return v, nil
	}

	return recursiveCopy(v.Elem(), pointers, skipUnsupported)
}

func recursiveCopyMap(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	if v.IsNil() {
		// If the slice is nil, just return it.
		return v, nil
	}

	dst := reflect.MakeMap(v.Type())

	for _, key := range v.MapKeys() {
		elem := v.MapIndex(key)
		elemDst, err := recursiveCopy(elem, pointers,
			skipUnsupported)
		if err != nil {
			return reflect.Value{}, err
		}

		dst.SetMapIndex(key, elemDst)
	}

	return dst, nil
}

func recursiveCopyPtr(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	// If the pointer is nil, just return it.
	if v.IsNil() {
		return v, nil
	}

	typeName := v.Type().String()

	// If the pointer is already in the pointers map, return it.
	ptr := v.Pointer()
	if dstMap, ok := pointers[ptr]; ok {
		if dst, ok := dstMap[typeName]; ok {
			return dst, nil
		}
	}

	// Otherwise, create a new pointer and add it to the pointers map.
	dst := reflect.New(v.Type().Elem())

	if pointers[ptr] == nil {
		pointers[ptr] = make(map[string]reflect.Value)
	}

	pointers[ptr][typeName] = dst

	// Proceed with the copy.
	elem := v.Elem()
	elemDst, err := recursiveCopy(elem, pointers, skipUnsupported)
	if err != nil {
		return reflect.Value{}, err
	}

	dst.Elem().Set(elemDst)

	return dst, nil
}

func recursiveCopySlice(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	if v.IsNil() {
		// If the slice is nil, just return it.
		return v, nil
	}

	dst := reflect.MakeSlice(v.Type(), v.Len(), v.Cap())

	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		elemDst, err := recursiveCopy(elem, pointers,
			skipUnsupported)
		if err != nil {
			return reflect.Value{}, err
		}

		dst.Index(i).Set(elemDst)
	}

	return dst, nil
}

func recursiveCopyStruct(v reflect.Value, pointers pointersMap,
	skipUnsupported bool) (reflect.Value, error) {
	dst := reflect.New(v.Type()).Elem()

	for i := 0; i < v.NumField(); i++ {
		elem := v.Field(i)

		// If the field is unexported, we need to disable read-only mode. If it
		// is exported, doing this changes nothing so we just do it. We need to
		// do this here not because we are writting to the field (this is the
		// source), but because Interface() does not work if the read-only bits
		// are set.
		disableRO(&elem)

		elemDst, err := recursiveCopy(elem, pointers,
			skipUnsupported)
		if err != nil {
			return reflect.Value{}, err
		}

		dstField := dst.Field(i)

		// If the field is unexported, we need to disable read-only mode so we
		// can actually write to it.
		disableRO(&dstField)

		dstField.Set(elemDst)
	}

	return dst, nil
}
