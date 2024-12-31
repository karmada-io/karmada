/*
Copyright 2022 The Karmada Authors.

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

package luavm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/util/sets"
	luajson "layeh.com/gopher-json"
)

// ConvertLuaResultInto convert lua result to obj
// @param references: the method of encoding a lua table is flawed and needs to be referenced by the same Kind object.
// take `Retain(desired, observed)` for example, we have problem to convert empty fields, but empty fields is most likely
// from desired or observed object, so take {desired, observed} object as references to obtain more accurate encoding values.
func ConvertLuaResultInto(luaResult *lua.LTable, obj interface{}, references ...any) error {
	objReflect, err := conversion.EnforcePtr(obj)
	if err != nil {
		return fmt.Errorf("obj is not pointer")
	}

	// 1. convert lua.LTable to json bytes
	jsonBytes, err := luajson.Encode(luaResult)
	if err != nil {
		return fmt.Errorf("encode lua result to json failed: %+v", err)
	}

	// 2. the json bytes above step got may not be expected in some special case, and should be adjusted manually.
	// because as for lua.LTable type, if a field is non-empty struct, it can be correctly encoded into struct format json;
	// but if a field is empty struct, it will be encoded into empty slice format as '[]' (root cause is empty lua.LTable
	// can not be distinguished from empty slice or empty struct).
	//
	// Supposing an object contains empty fields, like following one has an empty slice field and an empty struct field.
	// e.g: struct{one-filed: {}, another-field: []}
	//
	// When it is converted to lua.LTable, empty slice and empty struct are all converted to lua.LTable{}, which can't be distinguished.
	// e.g: lua.LTable{one-filed: lua.LTable{}, another-field: lua.LTable{}}
	//
	// When the lua.LTable is encoded into json, the two empty field are all encoded into empty slice format as '[]'.
	// e.g: {one-filed: [], another-field: []}
	//
	// Actually, this json value is not the expected, so we need to do some extra processing.
	// we should convert `one-filed` back to {}, and keep `another-field` unchanged.
	// expected: {one-filed: {}, another-field: []}
	isStruct := objReflect.Kind() == reflect.Struct
	jsonBytes, err = convertEmptySliceToEmptyStructPartly(jsonBytes, isStruct, references)
	if err != nil {
		return fmt.Errorf("convert empty object back to empty slice failed: %+v", err)
	}

	// 3. convert the converted json bytes to target object
	err = json.Unmarshal(jsonBytes, obj)
	if err != nil {
		return fmt.Errorf("json unmarshal obj failed: %+v", err)
	}

	return nil
}

// convertEmptySliceToEmptyStructPartly convert certain json fields with `[]` value which originally is empty struct back to `{}`.
// However, given a json bytes like {one-filed:[],another-field:[]}, the way to judge `one-filed:[]` should convert to `one-filed:{}`,
// while `another-field:[]` should keep unchanged can be described as follows:
//
// Actually, empty struct/slice value are not generated out of thin air, take `Retain(desired, observed)` for example,
// empty struct/slice value may come from three sources:
//  1. originally from desired object
//  2. originally from observed object
//  3. user added in Custom ResourceInterpreter (the probability is low and not recommended)
//
// For the first two cases, we just need to iterate through the json bytes of `desired` and `observed` objects (called as references),
// and record the empty slice fields in map `fieldOfEmptySlice`, record the empty struct fields in map `fieldOfEmptyStruct`,
// finally iterate through the given json bytes:
//  1. if an empty field ever existed in `fieldOfEmptySlice`, then it originally is slice, should keep unchanged.
//  2. if an empty field ever existed in `fieldOfEmptyStruct`, then it originally is struct, should be change back to struct.
//  3. if an empty field not ever existed in either map, is user added in Custom ResourceInterpreter,
//     we still remove this redundant field (compatible with previous).
func convertEmptySliceToEmptyStructPartly(objBytes []byte, isStruct bool, references ...any) ([]byte, error) {
	// 1. if an empty lua object is originally struct type (not slice), just return {}; if slice, just return [].
	if bytes.Equal(objBytes, []byte("[]")) {
		if isStruct {
			return []byte("{}"), nil
		}
		return []byte("[]"), nil
	}

	// 2. iterate through the json bytes of referenced objects and record the empty value fields.
	fieldOfEmptySlice, fieldOfEmptyStruct := sets.New[string](), sets.New[string]()
	for _, reference := range references {
		jsonBytes, err := json.Marshal(reference)
		if err != nil {
			return objBytes, fmt.Errorf("json marshal reference failed: %+v", err)
		}
		jsonVal := gjson.Parse(string(jsonBytes))

		fieldOfEmptySliceTemp, fieldOfEmptyStructTemp := traverseToFindEmptyField(jsonVal, nil)
		fieldOfEmptySlice = fieldOfEmptySlice.Union(fieldOfEmptySliceTemp)
		fieldOfEmptyStruct = fieldOfEmptyStruct.Union(fieldOfEmptyStructTemp)
	}

	// 3. iterate through the given json bytes, and check an empty slice field whether ever existed in referenced objects.
	objJSONStr := string(objBytes)
	objJSON := gjson.Parse(objJSONStr)
	fieldOfEmptySliceToStruct, fieldOfEmptySliceToDelete := traverseToFindEmptyFieldNeededModify(objJSON, nil, nil, fieldOfEmptySlice, fieldOfEmptyStruct)

	// 4. if an empty slice field is originally a struct, change it back to {}; if originally not exist, delete the field.
	var err error
	for _, fieldPath := range fieldOfEmptySliceToStruct.UnsortedList() {
		objJSONStr, err = sjson.Set(objJSONStr, fieldPath, struct{}{})
		if err != nil {
			return objBytes, fmt.Errorf("sjson set empty slice to empty struct failed: %+v", err)
		}
	}
	for _, fieldPath := range fieldOfEmptySliceToDelete.UnsortedList() {
		objJSONStr, err = sjson.Delete(objJSONStr, fieldPath)
		if err != nil {
			return objBytes, fmt.Errorf("sjson delete empty slice field failed: %+v", err)
		}
	}

	return []byte(objJSONStr), nil
}

// traverseToFindEmptyField get the field path with empty values by traverse a gjson.Result
//
// e.g: root={"spec":{"aa":{},"bb":[],"cc":["x"],"dd":{"ee":{}}}}
// we traverse the root and record every level filed name into `fieldPath` variable.
//  1. when traverse to the field `spec.aa`, we got an empty struct, so add it into `fieldOfEmptyStruct` variable.
//  2. when traverse to the field `spec.bb`, we got an empty slice, so add it into `fieldOfEmptySlice` variable.
//  3. when traverse to the field `spec.dd.ee`, we got another empty struct, so add it into `fieldOfEmptyStruct` variable.
//
// So, finally, fieldOfEmptySlice={"spec.bb"}, fieldOfEmptyStruct={"spec.aa", "spec.dd.ee"}
func traverseToFindEmptyField(root gjson.Result, fieldPath []string) (sets.Set[string], sets.Set[string]) {
	rootIsNotArray := !root.IsArray()
	fieldOfEmptySlice, fieldOfEmptyStruct := sets.New[string](), sets.New[string]()

	root.ForEach(func(key, value gjson.Result) bool {
		curFieldPath := fieldPath
		// e.g: root={"rules":[{"ruleName":[]}}]}, when we traverse `rules` field, which is an array.
		// the first element is key=0, value={"ruleName":[]}
		// however, we expected to record the `ruleName` field path as `rules.ruleName`, rather than `rules.0.ruleName`.
		if rootIsNotArray {
			curFieldPath = append(fieldPath, key.String())
		}
		curFieldStr := strings.Join(curFieldPath, ".")

		if value.IsArray() && len(value.Array()) == 0 {
			fieldOfEmptySlice.Insert(curFieldStr)
		} else if value.IsObject() && len(value.Map()) == 0 {
			fieldOfEmptyStruct.Insert(curFieldStr)
		} else if value.IsArray() || value.IsObject() {
			childEmptySlice, childEmptyStruct := traverseToFindEmptyField(value, curFieldPath)
			fieldOfEmptySlice = fieldOfEmptySlice.Union(childEmptySlice)
			fieldOfEmptyStruct = fieldOfEmptyStruct.Union(childEmptyStruct)
		}

		return true // keep iterating
	})

	return fieldOfEmptySlice, fieldOfEmptyStruct
}

// traverseToFindEmptyFieldNeededModify find the field with empty values which needed to be modified by traverse a gjson.Result
//
// e.g: root={"spec":{"aa":[],"bb":[],"cc":["x"],"dd":{"ee":[],"ff":[]}}}
// fieldOfEmptySlice={"spec.bb"}, fieldOfEmptyStruct={"spec.aa", "spec.dd.ee"}
//
// we traverse the root and record every level filed name into `fieldPath`、`fieldPathWithArrayIndex` variable,
// then judge whether the field path exist in `fieldOfEmptySlice` or `fieldOfEmptyStruct` variable:
//  1. when traverse to the field `spec.aa`, we got an empty slice, but this filed exists in `fieldOfEmptyStruct`,
//     so, it originally is struct, which needed to be modified back, we add it into `fieldOfEmptySliceToStruct` variable.
//  2. when traverse to the field `spec.bb`, we got an empty slice, and it exists in `fieldOfEmptySlice`,
//     so, it originally is slice, which should keep unchanged.
//  3. when traverse to the field `spec.dd.ee`, we got an empty slice, but it also exists in `fieldOfEmptyStruct`,
//     so, it originally is struct too, we add it into `fieldOfEmptySliceToStruct` variable.
//  4. when traverse to the field `spec.dd.ff`, we got an empty slice, but it not exists in either map variable,
//     so, it originally not exist, we can't judge whether it is struct, so we add it into `fieldOfEmptySliceToDelete` variable to remove it.
//
// So, finally, fieldOfEmptySliceToStruct={"spec.aa", "spec.dd.ee"}, fieldOfEmptySliceToDelete={"spec.dd.ff"}
func traverseToFindEmptyFieldNeededModify(root gjson.Result, fieldPath, fieldPathWithArrayIndex []string, fieldOfEmptySlice, fieldOfEmptyStruct sets.Set[string]) (sets.Set[string], sets.Set[string]) {
	rootIsNotArray := !root.IsArray()
	fieldOfEmptySliceToStruct, fieldOfEmptySliceToDelete := sets.New[string](), sets.New[string]()

	root.ForEach(func(key, value gjson.Result) bool {
		curFieldPath := fieldPath
		// e.g: root={"rules":[{"ruleName":[]}}]}, when we traverse `rules` field, which is an array.
		// the first element is key=0, value={"ruleName":[]}
		// we record `rules.ruleName` into curFieldPath, and record `rules.0.ruleName` into curFieldPathWithArrayIndex.
		if rootIsNotArray {
			curFieldPath = append(fieldPath, key.String())
		}
		curFieldPathWithArrayIndex := append(fieldPathWithArrayIndex, key.String())

		if value.IsArray() && len(value.Array()) == 0 {
			curFieldPathStr := strings.Join(curFieldPath, ".")
			curFieldPathWithIndexStr := strings.Join(curFieldPathWithArrayIndex, ".")

			if fieldOfEmptyStruct.Has(curFieldPathStr) {
				// if an empty object filed is originally an empty struct, we need to modify it back to struct.
				fieldOfEmptySliceToStruct.Insert(curFieldPathWithIndexStr)
			} else if !fieldOfEmptySlice.Has(curFieldPathStr) {
				// if an empty object filed is originally not exist (neither slice nor struct), we need to delete it.
				fieldOfEmptySliceToDelete.Insert(curFieldPathWithIndexStr)
			}
		} else if value.IsArray() || value.IsObject() {
			childEmptySliceToStruct, childEmptySliceToDelete := traverseToFindEmptyFieldNeededModify(value, curFieldPath,
				curFieldPathWithArrayIndex, fieldOfEmptySlice, fieldOfEmptyStruct)
			fieldOfEmptySliceToStruct = fieldOfEmptySliceToStruct.Union(childEmptySliceToStruct)
			fieldOfEmptySliceToDelete = fieldOfEmptySliceToDelete.Union(childEmptySliceToDelete)
		}

		return true // keep iterating
	})

	return fieldOfEmptySliceToStruct, fieldOfEmptySliceToDelete
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
