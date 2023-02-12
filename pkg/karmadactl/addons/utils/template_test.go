package utils

import (
	"testing"
)

func TestParseTemplate(t *testing.T) {
	type args struct {
		strTmpl string
		obj     interface{}
	}
	tempargs := args{
		strTmpl: "foo",
		obj:     "bar",
	}

	got, err := ParseTemplate(tempargs.strTmpl, tempargs.obj)
	if err != nil {
		t.Errorf("ParseTemplate() error = %v, wantErr false", err)
		return
	}
	if got == nil {
		t.Errorf("ParseTemplate() want return not nil, but return nil")
	}
}
