/*
Copyright 2021 The Karmada Authors.

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
