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
	"reflect"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestConvertLuaResultInto(t *testing.T) {
	type barStruct struct {
		Bar string
	}
	type fooStruct struct {
		Bar barStruct
	}

	type args struct {
		luaResult lua.LValue
		obj       interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    interface{}
	}{
		{
			name: "not a pointer",
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
				luaResult: func() lua.LValue {
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
			name: "table with empty table into slice",
			args: args{
				luaResult: func() lua.LValue {
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
				luaResult: func() lua.LValue {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ConvertLuaResultInto(tt.args.luaResult, tt.args.obj); (err != nil) != tt.wantErr {
				t.Errorf("ConvertLuaResultInto() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got := tt.args.obj; !reflect.DeepEqual(tt.want, got) {
				t.Errorf("ConvertLuaResultInto() got = %v, want %v", got, tt.want)
			}
		})
	}
}
