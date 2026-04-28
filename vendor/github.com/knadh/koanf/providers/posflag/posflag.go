// Package posflag implements a koanf.Provider that reads commandline
// parameters as conf maps using spf13/pflag, a POSIX compliant
// alternative to Go's stdlib flag package.
package posflag

import (
	"errors"

	"github.com/knadh/koanf/maps"
	"github.com/spf13/pflag"
)

// KoanfIntf is an interface that represents a small subset of methods
// used by this package from Koanf{}. When using this package, a live
// instance of Koanf{} should be passed.
type KoanfIntf interface {
	Exists(string) bool
}

// Posflag implements a pflag command line provider.
type Posflag struct {
	delim   string
	flagset *pflag.FlagSet
	ko      KoanfIntf
	cb      func(key string, value string) (string, interface{})
	flagCB  func(f *pflag.Flag) (string, interface{})
}

// Provider returns a commandline flags provider that returns
// a nested map[string]interface{} of environment variable where the
// nesting hierarchy of keys are defined by delim. For instance, the
// delim "." will convert the key `parent.child.key: 1`
// to `{parent: {child: {key: 1}}}`.
//
// It takes an optional (but recommended) Koanf instance to see if the
// the flags defined have been set from other providers, for instance,
// a config file. If they are not, then the default values of the flags
// are merged. If they do exist, the flag values are not merged but only
// the values that have been explicitly set in the command line are merged.
func Provider(f *pflag.FlagSet, delim string, ko KoanfIntf) *Posflag {
	return &Posflag{
		flagset: f,
		delim:   delim,
		ko:      ko,
	}
}

// ProviderWithValue works exactly the same as Provider except the callback
// takes a (key, value) with the variable name and value and allows their modification.
// This is useful for cases where complex types like slices separated by
// custom separators.
// Returning "" for the key causes the particular flag to be disregarded.
func ProviderWithValue(f *pflag.FlagSet, delim string, ko KoanfIntf, cb func(key string, value string) (string, interface{})) *Posflag {
	return &Posflag{
		flagset: f,
		delim:   delim,
		ko:      ko,
		cb:      cb,
	}
}

// ProviderWithFlag takes pflag.FlagSet and a callback that takes *pflag.Flag
// and applies the callback to all items in the flagset. It does not parse
// pflag.Flag values and expects the callback to process the keys and values
// from *pflag.Flag however. FlagVal() can be used in the callbakc to avoid
// repeating the type-switch block for parsing values.
// Returning "" for the key causes the particular flag to be disregarded.
//
// Example:
//
//	p := posflag.ProviderWithFlag(flagset, ".", ko, func(f *pflag.Flag) (string, interface{}) {
//	   // Transform the key in whatever manner.
//	   key := f.Name
//
//	   // Use FlagVal() and then transform the value, or don't use it at all
//	   // and add custom logic to parse the value.
//	   val := posflag.FlagVal(flagset, f)
//
//	   return key, val
//	})
func ProviderWithFlag(f *pflag.FlagSet, delim string, ko KoanfIntf, cb func(f *pflag.Flag) (string, interface{})) *Posflag {
	return &Posflag{
		flagset: f,
		delim:   delim,
		ko:      ko,
		flagCB:  cb,
	}
}

// Read reads the flag variables and returns a nested conf map.
func (p *Posflag) Read() (map[string]interface{}, error) {
	mp := make(map[string]interface{})
	p.flagset.VisitAll(func(f *pflag.Flag) {
		var (
			key = f.Name
			val interface{}
		)

		// If there is a (key, value) callback set, pass the key and string
		// value from the flagset to it and use the results.
		if p.cb != nil {
			key, val = p.cb(key, f.Value.String())
		} else if p.flagCB != nil {
			// If there is a pflag.Flag callback set, pass the flag as-is
			// to it and use the results from the callback.
			key, val = p.flagCB(f)
		} else {
			// There are no callbacks set. Use the in-built flag value parser.
			val = FlagVal(p.flagset, f)
		}

		if key == "" {
			return
		}

		// If the default value of the flag was never changed by the user,
		// it should not override the value in the conf map (if it exists in the first place).
		if !f.Changed {
			if p.ko != nil {
				if p.ko.Exists(key) {
					return
				}
			} else {
				return
			}
		}

		// No callback. Use the key and value as-is.
		mp[key] = val
	})

	return maps.Unflatten(mp, p.delim), nil
}

// ReadBytes is not supported by the pflag provider.
func (p *Posflag) ReadBytes() ([]byte, error) {
	return nil, errors.New("pflag provider does not support this method")
}

// Watch is not supported.
func (p *Posflag) Watch(cb func(event interface{}, err error)) error {
	return errors.New("posflag provider does not support this method")
}

// FlagVal examines a pflag.Flag and returns a typed value as an interface{}
// from the types that pflag supports. If it is of a type that isn't known
// for any reason, the value is returned as a string.
func FlagVal(fs *pflag.FlagSet, f *pflag.Flag) interface{} {
	var (
		key = f.Name
		val interface{}
	)

	switch f.Value.Type() {
	case "int":
		i, _ := fs.GetInt(key)
		val = int64(i)
	case "uint":
		i, _ := fs.GetUint(key)
		val = uint64(i)
	case "int8":
		i, _ := fs.GetInt8(key)
		val = int64(i)
	case "uint8":
		i, _ := fs.GetUint8(key)
		val = uint64(i)
	case "int16":
		i, _ := fs.GetInt16(key)
		val = int64(i)
	case "uint16":
		i, _ := fs.GetUint16(key)
		val = uint64(i)
	case "int32":
		i, _ := fs.GetInt32(key)
		val = int64(i)
	case "uint32":
		i, _ := fs.GetUint32(key)
		val = uint64(i)
	case "int64":
		val, _ = fs.GetInt64(key)
	case "uint64":
		val, _ = fs.GetUint64(key)
	case "float":
		val, _ = fs.GetFloat64(key)
	case "float32":
		val, _ = fs.GetFloat32(key)
	case "float64":
		val, _ = fs.GetFloat64(key)
	case "bool":
		val, _ = fs.GetBool(key)
	case "duration":
		val, _ = fs.GetDuration(key)
	case "ip":
		val, _ = fs.GetIP(key)
	case "ipMask":
		val, _ = fs.GetIPv4Mask(key)
	case "ipNet":
		val, _ = fs.GetIPNet(key)
	case "count":
		val, _ = fs.GetCount(key)
	case "bytesHex":
		val, _ = fs.GetBytesHex(key)
	case "bytesBase64":
		val, _ = fs.GetBytesBase64(key)
	case "string":
		val, _ = fs.GetString(key)
	case "stringSlice":
		val, _ = fs.GetStringSlice(key)
	case "intSlice":
		val, _ = fs.GetIntSlice(key)
	case "uintSlice":
		val, _ = fs.GetUintSlice(key)
	case "int32Slice":
		val, _ = fs.GetInt32Slice(key)
	case "int64Slice":
		val, _ = fs.GetInt64Slice(key)
	case "float32Slice":
		val, _ = fs.GetFloat32Slice(key)
	case "float64Slice":
		val, _ = fs.GetFloat64Slice(key)
	case "boolSlice":
		val, _ = fs.GetBoolSlice(key)
	case "durationSlice":
		val, _ = fs.GetDurationSlice(key)
	case "ipSlice":
		val, _ = fs.GetIPSlice(key)
	case "stringArray":
		val, _ = fs.GetStringArray(key)
	case "stringToString":
		val, _ = fs.GetStringToString(key)
	case "stringToInt":
		val, _ = fs.GetStringToInt(key)
	case "stringToInt64":
		val, _ = fs.GetStringToInt64(key)
	default:
		val = f.Value.String()
	}

	return val
}
