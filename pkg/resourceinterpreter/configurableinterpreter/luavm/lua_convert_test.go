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
