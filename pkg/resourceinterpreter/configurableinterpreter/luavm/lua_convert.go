package luavm

import (
	"encoding/json"
	"fmt"
	"reflect"

	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/conversion"
	luajson "layeh.com/gopher-json"
)

// ConvertLuaResultInto convert lua result to obj
func ConvertLuaResultInto(luaResult lua.LValue, obj interface{}) error {
	t, err := conversion.EnforcePtr(obj)
	if err != nil {
		return fmt.Errorf("obj is not pointer")
	}
	jsonBytes, err := luajson.Encode(luaResult)
	if err != nil {
		return fmt.Errorf("json Encode obj eroor %v", err)
	}

	//  for lua an empty object by json encode be [] not {}
	if t.Kind() == reflect.Struct && len(jsonBytes) > 1 && jsonBytes[0] == '[' {
		jsonBytes[0], jsonBytes[len(jsonBytes)-1] = '{', '}'
	}

	err = json.Unmarshal(jsonBytes, obj)
	if err != nil {
		return fmt.Errorf("can not unmarshal %v to %#v", string(jsonBytes), obj)
	}
	return nil
}

// ConvertLuaResultToInt convert lua result to int.
func ConvertLuaResultToInt(luaResult lua.LValue) (int32, error) {
	if luaResult.Type() != lua.LTNumber {
		return 0, fmt.Errorf("result type %#v is not number", luaResult.Type())
	}
	return int32(luaResult.(lua.LNumber)), nil
}

// ConvertLuaResultToBool convert lua result to bool.
func ConvertLuaResultToBool(luaResult lua.LValue) (bool, error) {
	if luaResult.Type() != lua.LTBool {
		return false, fmt.Errorf("result type %#v is not bool", luaResult.Type())
	}
	return bool(luaResult.(lua.LBool)), nil
}
