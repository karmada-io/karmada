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
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// a callback function to modify options
type ModifyOptions func(option *Options)

// New an Options with default parameters
func New(modifyOptions ModifyOptions) Options {
	option := Options{
		ClusterName: "testCluster",
		ServerPort:  8088,
	}

	if modifyOptions != nil {
		modifyOptions(&option)
	}
	return option
}

func TestValidateKarmadaSchedulerEstimator(t *testing.T) {
	successCases := []Options{
		New(nil),
	}
	for _, successCase := range successCases {
		if errs := successCase.Validate(); len(errs) != 0 {
			t.Errorf("expected success: %v", errs)
		}
	}

	newPath := field.NewPath("Options")
	testCases := map[string]struct {
		opt          Options
		expectedErrs field.ErrorList
	}{
		"invalid ClusterName": {
			opt: New(func(option *Options) {
				option.ClusterName = ""
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterName"), "", "clusterName cannot be empty")},
		},
		"invalid ServerPort": {
			opt: New(func(option *Options) {
				option.ServerPort = 80888
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ServerPort"), 80888, "must be a valid port between 0 and 65535 inclusive")},
		},
	}
	for _, testCase := range testCases {
		errs := testCase.opt.Validate()
		if len(testCase.expectedErrs) != len(errs) {
			t.Fatalf("Expected %d errors, got %d errors: %v", len(testCase.expectedErrs), len(errs), errs)
		}
		for i, err := range errs {
			if err.Error() != testCase.expectedErrs[i].Error() {
				t.Fatalf("Expected error: %s, got %s", testCase.expectedErrs[i], err.Error())
			}
		}
	}
}
