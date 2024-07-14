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

package configuration

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

func TestHasWildcard(t *testing.T) {
	testCases := []struct {
		name   string
		slices []string
		want   bool
	}{
		{
			name:   "TestHasWildcard(): nil input args",
			slices: nil,
			want:   false,
		},
		{
			name: "TestHasWildcard(): has wildcard",
			slices: []string{
				"*",
			},
			want: true,
		},
		{
			name: "TestHasWildcard(): not have wildcard",
			slices: []string{
				"test",
			},
			want: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if result := hasWildcard(testCase.slices); result != testCase.want {
				t.Errorf("Case: %s failed: want %v, but got %v", testCase.name, testCase.want, result)
			}
		})
	}
}

func TestValidateRule(t *testing.T) {
	fldPath := field.NewPath("webhooks")

	tests := []struct {
		name          string
		rule          *configv1alpha1.Rule
		expectedError string
	}{
		{
			name:          "APIGroups required",
			rule:          &configv1alpha1.Rule{},
			expectedError: "apiGroups: Required value",
		},
		{
			name: "APIGroups cannot specify both * and others",
			rule: &configv1alpha1.Rule{
				APIGroups: []string{"*", "group"},
			},
			expectedError: "if '*' is present, must not specify other API groups",
		},
		{
			name:          "APIVersions required",
			rule:          &configv1alpha1.Rule{},
			expectedError: "webhooks.apiVersions: Required value",
		},
		{
			name: "APIVersions cannot specify both * and others",
			rule: &configv1alpha1.Rule{
				APIVersions: []string{"*", "v1"},
			},
			expectedError: "if '*' is present, must not specify other API versions",
		},
		{
			name: "APIVersions empty is not allowed",
			rule: &configv1alpha1.Rule{
				APIVersions: []string{""},
			},
			expectedError: "apiVersions[0]: Required value",
		},
		{
			name:          "Kinds required",
			rule:          &configv1alpha1.Rule{},
			expectedError: "kinds: Required value",
		},
		{
			name: "Kinds cannot specify both * and others",
			rule: &configv1alpha1.Rule{
				Kinds: []string{"*", "v1"},
			},
			expectedError: "if '*' is present, must not specify other kinds",
		},
		{
			name: "Kinds empty is not allowed",
			rule: &configv1alpha1.Rule{
				Kinds: []string{""},
			},
			expectedError: "kinds[0]: Required value",
		},
	}
	for _, test := range tests {
		errs := validateRule(test.rule, fldPath)
		err := errs.ToAggregate()
		if err != nil {
			if e, a := test.expectedError, err.Error(); !strings.Contains(a, e) || e == "" {
				t.Errorf("Case: %s failed: expected to contain %s, got %s", test.name, e, a)
			}
		} else {
			if test.expectedError != "" {
				t.Errorf("unexpected no error, expected to contain %s", test.expectedError)
			}
		}
	}
}
