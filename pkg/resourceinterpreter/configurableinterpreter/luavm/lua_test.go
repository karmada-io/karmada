package luavm

import (
	"encoding/json"
	"reflect"
	"testing"

	lua "github.com/yuin/gopher-lua"
	"k8s.io/utils/pointer"
)

func Test_decodeValue(t *testing.T) {
	L := lua.NewState()

	type args struct {
		value interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    lua.LValue
		wantErr bool
	}{
		{
			name: "nil",
			args: args{
				value: nil,
			},
			want: lua.LNil,
		},
		{
			name: "nil pointer",
			args: args{
				value: (*struct{})(nil),
			},
			want: lua.LNil,
		},
		{
			name: "int pointer",
			args: args{
				value: pointer.Int(1),
			},
			want: lua.LNumber(1),
		},
		{
			name: "int",
			args: args{
				value: 1,
			},
			want: lua.LNumber(1),
		},
		{
			name: "uint",
			args: args{
				value: uint(1),
			},
			want: lua.LNumber(1),
		},
		{
			name: "float",
			args: args{
				value: 1.0,
			},
			want: lua.LNumber(1),
		},
		{
			name: "bool",
			args: args{
				value: true,
			},
			want: lua.LBool(true),
		},
		{
			name: "string",
			args: args{
				value: "foo",
			},
			want: lua.LString("foo"),
		},
		{
			name: "json number",
			args: args{
				value: json.Number("1"),
			},
			want: lua.LString("1"),
		},
		{
			name: "slice",
			args: args{
				value: []string{"foo", "bar"},
			},
			want: func() lua.LValue {
				v := L.CreateTable(2, 0)
				v.Append(lua.LString("foo"))
				v.Append(lua.LString("bar"))
				return v
			}(),
		},
		{
			name: "slice pointer",
			args: args{
				value: &[]string{"foo", "bar"},
			},
			want: func() lua.LValue {
				v := L.CreateTable(2, 0)
				v.Append(lua.LString("foo"))
				v.Append(lua.LString("bar"))
				return v
			}(),
		},
		{
			name: "struct",
			args: args{
				value: struct {
					Foo string
				}{
					Foo: "foo",
				},
			},
			want: func() lua.LValue {
				v := L.CreateTable(0, 1)
				v.RawSetString("Foo", lua.LString("foo"))
				return v
			}(),
		},
		{
			name: "struct pointer",
			args: args{
				value: &struct {
					Foo string
				}{
					Foo: "foo",
				},
			},
			want: func() lua.LValue {
				v := L.CreateTable(0, 1)
				v.RawSetString("Foo", lua.LString("foo"))
				return v
			}(),
		},
		{
			name: "[]interface{}",
			args: args{
				value: []interface{}{1, 2},
			},
			want: func() lua.LValue {
				v := L.CreateTable(2, 0)
				v.Append(lua.LNumber(1))
				v.Append(lua.LNumber(2))
				return v
			}(),
		},
		{
			name: "map[string]interface{}",
			args: args{
				value: map[string]interface{}{
					"foo": "foo1",
				},
			},
			want: func() lua.LValue {
				v := L.CreateTable(0, 1)
				v.RawSetString("foo", lua.LString("foo1"))
				return v
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeValue(L, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decodeValue() got = %v, want %v", got, tt.want)
			}
		})
	}
}
