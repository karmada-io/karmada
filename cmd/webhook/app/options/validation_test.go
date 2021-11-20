package options

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestOptions_Validate(t *testing.T) {
	rootPath := field.NewPath("Options")
	testCases := []struct {
		name string
		opts *Options
		want field.ErrorList
	}{
		{
			name: "invalid BindAddress",
			opts: &Options{
				BindAddress: "XXX.XXX.XXX.XXX",
				SecurePort:  80,
			},
			want: field.ErrorList{
				field.Invalid(rootPath.Child("BindAddress"), "XXX.XXX.XXX.XXX", "not a valid textual representation of an IP address"),
			},
		},
		{
			name: "invalid SecurePort",
			opts: &Options{
				BindAddress: "0.0.0.0",
				SecurePort:  65536,
			},
			want: field.ErrorList{
				field.Invalid(rootPath.Child("SecurePort"), 65536, "must be a valid port between 0 and 65535 inclusive"),
			},
		},
	}

	for _, testCase := range testCases {
		result := testCase.opts.Validate()
		if !reflect.DeepEqual(result, testCase.want) {
			t.Errorf("Test %s failed: want %v, but got %v", testCase.name, testCase.want, result)
		}
	}
}
