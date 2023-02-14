package utils

import (
	"reflect"
	"testing"
)

func TestParseTemplate(t *testing.T) {
	type args struct {
		strTmpl string
		obj     interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "strTmpl and obj are not empty",
			args: args{
				strTmpl: "foo",
				obj:     "bar",
			},
			want:    []byte{'f', 'o', 'o'},
			wantErr: false,
		},
		{
			name: "strTmpl is empty",
			args: args{
				strTmpl: "",
				obj:     "bar",
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "obj is nil",
			args: args{
				strTmpl: "foo",
				obj:     nil,
			},
			want:    []byte{'f', 'o', 'o'},
			wantErr: false,
		},
		{
			name: "obj and strTmpl are empty",
			args: args{
				strTmpl: "",
				obj:     "",
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTemplate(tt.args.strTmpl, tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}
