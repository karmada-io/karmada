package options

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestOptions_Validate(t *testing.T) {
	rootPath := field.NewPath("Options")
	tests := []struct {
		name     string
		opts     *Options
		expected field.ErrorList
	}{
		{
			name: "valid option",
			opts: &Options{
				BindAddress: "0.0.0.0",
				SecurePort:  80,
			},
			expected: field.ErrorList{},
		},
		{
			name: "invalid BindAddress",
			opts: &Options{
				BindAddress: "xxx.xxx.xxx.xxx",
				SecurePort:  80,
			},
			expected: field.ErrorList{
				field.Invalid(rootPath.Child("BindAddress"), "xxx.xxx.xxx.xxx",
					"not a valid textual representation of an IP address"),
			},
		},
		{
			name: "invalid SecurePort",
			opts: &Options{
				BindAddress: "0.0.0.0",
				SecurePort:  -1,
			},
			expected: field.ErrorList{
				field.Invalid(rootPath.Child("SecurePort"), -1,
					"must be a valid port between 0 and 65535 inclusive"),
			},
		},
	}

	for _, test := range tests {
		if err := test.opts.Validate(); !reflect.DeepEqual(err, test.expected) {
			t.Errorf("Test %s failed: expected %v, but got %v", test.name, test.expected, err)
		}
	}
}
