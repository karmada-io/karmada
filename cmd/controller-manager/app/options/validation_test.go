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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// a callback function to modify options
type ModifyOptions func(option *Options)

// New an Options with default parameters
func New(modifyOptions ModifyOptions) Options {
	option := Options{
		SkippedPropagatingAPIs:       "cluster.karmada.io;policy.karmada.io;work.karmada.io",
		ClusterStatusUpdateFrequency: metav1.Duration{Duration: 10 * time.Second},
		ClusterLeaseDuration:         metav1.Duration{Duration: 10 * time.Second},
		ClusterMonitorPeriod:         metav1.Duration{Duration: 10 * time.Second},
		ClusterMonitorGracePeriod:    metav1.Duration{Duration: 10 * time.Second},
		ClusterStartupGracePeriod:    metav1.Duration{Duration: 10 * time.Second},
		FederatedResourceQuotaOptions: FederatedResourceQuotaOptions{
			ResourceQuotaSyncPeriod: metav1.Duration{
				Duration: 10 * time.Second,
			},
		},
	}

	if modifyOptions != nil {
		modifyOptions(&option)
	}
	return option
}

func TestValidateControllerManagerConfiguration(t *testing.T) {
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
		"invalid SkippedPropagatingAPIs": {
			opt: New(func(options *Options) {
				options.SkippedPropagatingAPIs = "a/b/c/d?"
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("SkippedPropagatingAPIs"), "a/b/c/d?", "Invalid API string")},
		},
		"invalid ClusterStatusUpdateFrequency": {
			opt: New(func(options *Options) {
				options.ClusterStatusUpdateFrequency.Duration = -10 * time.Second
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterStatusUpdateFrequency"), metav1.Duration{Duration: -10 * time.Second}, "must be greater than 0")},
		},
		"invalid ClusterLeaseDuration": {
			opt: New(func(options *Options) {
				options.ClusterLeaseDuration.Duration = -40 * time.Second
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterLeaseDuration"), metav1.Duration{Duration: -40 * time.Second}, "must be greater than 0")},
		},
		"invalid ClusterMonitorPeriod": {
			opt: New(func(options *Options) {
				options.ClusterMonitorPeriod.Duration = -40 * time.Second
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterMonitorPeriod"), metav1.Duration{Duration: -40 * time.Second}, "must be greater than 0")},
		},
		"invalid ClusterMonitorGracePeriod": {
			opt: New(func(options *Options) {
				options.ClusterMonitorGracePeriod.Duration = -40 * time.Second
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterMonitorGracePeriod"), metav1.Duration{Duration: -40 * time.Second}, "must be greater than 0")},
		},
		"invalid ClusterStartupGracePeriod": {
			opt: New(func(options *Options) {
				options.ClusterStartupGracePeriod.Duration = 0 * time.Second
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterStartupGracePeriod"), metav1.Duration{Duration: 0 * time.Second}, "must be greater than 0")},
		},
		"invalid FailoverOptions": {
			opt: New(func(options *Options) {
				options.FailoverOptions.EnableNoExecuteTaintEviction = true
				options.FailoverOptions.NoExecuteTaintEvictionPurgeMode = ""
			}),
			expectedErrs: field.ErrorList{
				field.Invalid(field.NewPath("FailoverOptions").Child("NoExecuteTaintEvictionPurgeMode"), "", "Invalid mode"),
			},
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
