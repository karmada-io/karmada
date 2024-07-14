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
	"encoding/json"
	"reflect"
	"testing"

	"github.com/tidwall/gjson"
	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

func TestConvertLuaResultToStruct(t *testing.T) {
	type barStruct struct {
		Bar string
	}
	type fooStruct struct {
		Bar barStruct
	}

	type specStruct struct {
		EmptyMap      map[string]string
		NonEmptyMap   map[string]string
		EmptySlice    []string
		NonEmptySlice []string
		OtherField    string
	}
	type foolStruct struct {
		Spec specStruct
	}

	type args struct {
		luaResult *lua.LTable
		obj       interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    interface{}
	}{
		{
			name: "obj is not a pointer",
			args: args{
				luaResult: &lua.LTable{},
				obj:       struct{}{},
			},
			wantErr: true,
			want:    struct{}{},
		},
		{
			name: "empty table into struct",
			args: args{
				luaResult: &lua.LTable{},
				obj:       &fooStruct{},
			},
			wantErr: false,
			want:    &fooStruct{},
		},
		{
			name: "table with empty table into struct",
			args: args{
				luaResult: func() *lua.LTable {
					v := &lua.LTable{}
					v.RawSetString("Bar", &lua.LTable{})
					return v
				}(),
				obj: &fooStruct{},
			},
			wantErr: false,
			want:    &fooStruct{},
		},
		{
			name: "struct is not empty, and convert successfully",
			args: args{
				luaResult: func() *lua.LTable {
					bar := &lua.LTable{}
					bar.RawSetString("Bar", lua.LString("bar"))

					v := &lua.LTable{}
					v.RawSetString("Bar", bar)
					return v
				}(),
				obj: &fooStruct{},
			},
			wantErr: false,
			want: &fooStruct{
				Bar: barStruct{Bar: "bar"},
			},
		},
		{
			name: "empty table into slice",
			args: args{
				luaResult: &lua.LTable{},
				obj:       &[]string{},
			},
			wantErr: false,
			want:    &[]string{},
		},
		{
			name: "non-empty table into slice",
			args: args{
				luaResult: func() *lua.LTable {
					v := &lua.LTable{}
					v.Append(lua.LString("foo"))
					v.Append(lua.LString("bar"))
					return v
				}(),
				obj: &[]string{},
			},
			wantErr: false,
			want:    &[]string{"foo", "bar"},
		},
		{
			name: "table into DependentObjectReference slice",
			args: args{
				luaResult: func() *lua.LTable {
					item := &lua.LTable{}
					item.RawSetString("apiVersion", lua.LString("demo"))
					item.RawSetString("kind", lua.LString("demo"))

					v := &lua.LTable{}
					v.Append(item)
					return v
				}(),
				obj: &[]configv1alpha1.DependentObjectReference{},
			},
			wantErr: false,
			want: &[]configv1alpha1.DependentObjectReference{{
				APIVersion: "demo",
				Kind:       "demo",
			}},
		},
		{
			name: "convert struct with empty field",
			args: args{
				luaResult: func() *lua.LTable {
					innerMap := &lua.LTable{}
					innerMap.RawSetString("test-key", lua.LString(`\"trap-string\":[]`))

					innerSlice := &lua.LTable{}
					innerSlice.Append(lua.LString(`\"trap-string\":[]`))

					spec := &lua.LTable{}
					spec.RawSetH(lua.LString("EmptyMap"), &lua.LTable{})
					spec.RawSetH(lua.LString("NonEmptyMap"), innerMap)
					spec.RawSetH(lua.LString("EmptySlice"), &lua.LTable{})
					spec.RawSetH(lua.LString("NonEmptySlice"), innerSlice)
					spec.RawSetString("OtherField", lua.LString(`\"trap-string\":[]`))

					v := &lua.LTable{}
					v.RawSetString("Spec", spec)
					return v
				}(),
				obj: &foolStruct{},
			},
			wantErr: false,
			want: &foolStruct{
				Spec: specStruct{
					NonEmptyMap:   map[string]string{"test-key": `\"trap-string\":[]`},
					NonEmptySlice: []string{`\"trap-string\":[]`},
					OtherField:    `\"trap-string\":[]`,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ConvertLuaResultInto(tt.args.luaResult, tt.args.obj); (err != nil) != tt.wantErr {
				t.Errorf("ConvertLuaResultToStruct() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got := tt.args.obj; !reflect.DeepEqual(tt.want, got) {
				t.Errorf("ConvertLuaResultToStruct() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertLuaResultToUnstruct(t *testing.T) {
	type args struct {
		luaResult  *lua.LTable
		references []*unstructured.Unstructured
	}
	tests := []struct {
		name    string
		args    args
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "convert unstructured with empty field not referring to references",
			args: args{
				luaResult: func() *lua.LTable {
					innerMap := &lua.LTable{}
					innerMap.RawSetString("test-key", lua.LString(`\"trap-string\":[]`))

					innerSlice := &lua.LTable{}
					innerSlice.Append(lua.LString(`\"trap-string\":[]`))

					spec := &lua.LTable{}
					spec.RawSetH(lua.LString("EmptyMap"), &lua.LTable{})
					spec.RawSetH(lua.LString("NonEmptyMap"), innerMap)
					spec.RawSetH(lua.LString("EmptySlice"), &lua.LTable{})
					spec.RawSetH(lua.LString("NonEmptySlice"), innerSlice)
					spec.RawSetString("OtherField", lua.LString(`\"trap-string\":[]`))

					v := &lua.LTable{}
					v.RawSetString("kind", lua.LString("demo"))
					v.RawSetString("spec", spec)
					return v
				}(),
				references: nil,
			},
			wantErr: false,
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "demo",
					"spec": map[string]interface{}{
						"NonEmptyMap":   map[string]interface{}{"test-key": `\"trap-string\":[]`},
						"NonEmptySlice": []interface{}{`\"trap-string\":[]`},
						"OtherField":    `\"trap-string\":[]`,
					},
				},
			},
		},
		{
			name: "convert unstructured with empty field referring to references",
			args: args{
				luaResult: func() *lua.LTable {
					innerMap := &lua.LTable{}
					innerMap.RawSetString("test-key", lua.LString(`\"trap-string\":[]`))

					innerSlice := &lua.LTable{}
					innerSlice.Append(lua.LString(`\"trap-string\":[]`))

					spec := &lua.LTable{}
					spec.RawSetH(lua.LString("EmptyMap"), &lua.LTable{})
					spec.RawSetH(lua.LString("NonEmptyMap"), innerMap)
					spec.RawSetH(lua.LString("EmptySlice"), &lua.LTable{})
					spec.RawSetH(lua.LString("NonEmptySlice"), innerSlice)
					spec.RawSetString("OtherField", lua.LString(`\"trap-string\":[]`))

					v := &lua.LTable{}
					v.RawSetString("kind", lua.LString("demo"))
					v.RawSetString("spec", spec)
					return v
				}(),
				references: []*unstructured.Unstructured{{
					Object: map[string]interface{}{
						"kind": "demo",
						"spec": map[string]interface{}{"EmptySlice": []interface{}{}, "EmptyMap": map[string]interface{}{}}},
				}},
			},
			wantErr: false,
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "demo",
					"spec": map[string]interface{}{
						"EmptyMap":      map[string]interface{}{},
						"NonEmptyMap":   map[string]interface{}{"test-key": `\"trap-string\":[]`},
						"EmptySlice":    []interface{}{},
						"NonEmptySlice": []interface{}{`\"trap-string\":[]`},
						"OtherField":    `\"trap-string\":[]`,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got = &unstructured.Unstructured{}
			err := ConvertLuaResultInto(tt.args.luaResult, got, tt.args.references)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertLuaResultInto() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConvertLuaResultInto() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_traverseToFindEmptyField(t *testing.T) {
	type args struct {
		root      gjson.Result
		fieldPath []string
	}
	type wants struct {
		fieldOfEmptySlice  sets.Set[string]
		fieldOfEmptyStruct sets.Set[string]
	}
	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "traverse to find empty field",
			args: args{
				root:      gjson.Parse(`{"spec":[{"aa":{},"bb":[],"cc":["x"],"dd":{"ee":{}}}]}`),
				fieldPath: nil,
			},
			wants: wants{
				fieldOfEmptySlice:  sets.New[string]("spec.bb"),
				fieldOfEmptyStruct: sets.New[string]("spec.aa", "spec.dd.ee"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fieldOfEmptySlice, fieldOfEmptyStruct := traverseToFindEmptyField(tt.args.root, tt.args.fieldPath)

			if got := fieldOfEmptySlice; !reflect.DeepEqual(tt.wants.fieldOfEmptySlice, got) {
				t.Errorf("traverseToFindEmptyField() got fieldOfEmptySlice = %v, want %v",
					got, tt.wants.fieldOfEmptySlice)
			}

			if got := fieldOfEmptyStruct; !reflect.DeepEqual(tt.wants.fieldOfEmptyStruct, got) {
				t.Errorf("traverseToFindEmptyField() got fieldOfEmptyStruct = %v, want %v",
					got, tt.wants.fieldOfEmptyStruct)
			}
		})
	}
}

func Test_traverseToFindEmptyFieldNeededModify(t *testing.T) {
	type args struct {
		root                    gjson.Result
		fieldPath               []string
		fieldPathWithArrayIndex []string
		fieldOfEmptySlice       sets.Set[string]
		fieldOfEmptyStruct      sets.Set[string]
	}
	type wants struct {
		fieldOfEmptySliceToStruct sets.Set[string]
		fieldOfEmptySliceToDelete sets.Set[string]
	}
	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "traverse to find empty field needed modify",
			args: args{
				root:                    gjson.Parse(`{"spec":[{"aa":[],"bb":[],"cc":["x"],"dd":{"ee":[],"ff":[]}}]}`),
				fieldPath:               nil,
				fieldPathWithArrayIndex: nil,
				fieldOfEmptySlice:       sets.New[string]("spec.bb"),
				fieldOfEmptyStruct:      sets.New[string]("spec.aa", "spec.dd.ee"),
			},
			wants: wants{
				fieldOfEmptySliceToStruct: sets.New[string]("spec.0.aa", "spec.0.dd.ee"),
				fieldOfEmptySliceToDelete: sets.New[string]("spec.0.dd.ff"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fieldOfEmptySliceToStruct, fieldOfEmptySliceToDelete := traverseToFindEmptyFieldNeededModify(tt.args.root,
				tt.args.fieldPath, tt.args.fieldPathWithArrayIndex, tt.args.fieldOfEmptySlice, tt.args.fieldOfEmptyStruct)

			if got := fieldOfEmptySliceToStruct; !reflect.DeepEqual(tt.wants.fieldOfEmptySliceToStruct, got) {
				t.Errorf("traverseToFindEmptyFieldNeededModify() got fieldOfEmptySliceToStruct = %v, want %v",
					got, tt.wants.fieldOfEmptySliceToStruct)
			}

			if got := fieldOfEmptySliceToDelete; !reflect.DeepEqual(tt.wants.fieldOfEmptySliceToDelete, got) {
				t.Errorf("traverseToFindEmptyFieldNeededModify() got fieldOfEmptySliceToDelete = %v, want %v",
					got, tt.wants.fieldOfEmptySliceToDelete)
			}
		})
	}
}

func Test_convertEmptyObjectBackToEmptySlice(t *testing.T) {
	type args struct {
		objBytes   []byte
		isStruct   bool
		references []*unstructured.Unstructured
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    []byte
	}{
		{
			name: "convert empty object back to empty slice",
			args: args{
				objBytes: []byte(`{"spec":[{"aa":[],"bb":[],"cc":["x"],"dd":{"ee":[],"ff":[]}}]}`),
				isStruct: true,
				references: func() []*unstructured.Unstructured {
					desired, observed := &unstructured.Unstructured{}, &unstructured.Unstructured{}
					_ = json.Unmarshal([]byte(`{"kind": "demo", "spec":[{"aa":{},"bb":[]}]}`), desired)
					_ = json.Unmarshal([]byte(`{"kind": "demo", "spec":[{"dd":{"ee":{}}}]}`), observed)
					return []*unstructured.Unstructured{desired, observed}
				}(),
			},
			wantErr: false,
			want:    []byte(`{"spec":[{"aa":{},"bb":[],"cc":["x"],"dd":{"ee":{}}}]}`),
		},
		{
			name: "obj is not struct while objBytes is []",
			args: args{
				objBytes: []byte(`[]`),
				isStruct: false,
				references: func() []*unstructured.Unstructured {
					desired, observed := &unstructured.Unstructured{}, &unstructured.Unstructured{}
					_ = json.Unmarshal([]byte(`{"kind": "demo", "spec":[{"aa":{},"bb":[]}]}`), desired)
					_ = json.Unmarshal([]byte(`{"kind": "demo", "spec":[{"dd":{"ee":{}}}]}`), observed)
					return []*unstructured.Unstructured{desired, observed}
				}(),
			},
			wantErr: false,
			want:    []byte(`[]`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertEmptySliceToEmptyStructPartly(tt.args.objBytes, tt.args.isStruct, tt.args.references)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertEmptySliceToEmptyStructPartly() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertEmptySliceToEmptyStructPartly() got = %s, want %s", got, tt.want)
			}
		})
	}
}
