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
	componentbaseconfig "k8s.io/component-base/config"
)

// a callback function to modify options
type ModifyOptions func(option *Options)

// New an Options with default parameters
func New(modifyOptions ModifyOptions) Options {
	option := Options{
		LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect: false,
		},
		KubeAPIQPS:                40,
		KubeAPIBurst:              30,
		SchedulerEstimatorTimeout: metav1.Duration{Duration: 1 * time.Second},
		SchedulerEstimatorPort:    9001,
		DeschedulingInterval:      metav1.Duration{Duration: 1 * time.Second},
		UnschedulableThreshold:    metav1.Duration{Duration: 1 * time.Second},
	}

	if modifyOptions != nil {
		modifyOptions(&option)
	}
	return option
}

func TestValidateKarmadaDescheduler(t *testing.T) {
	successCases := []Options{
		New(nil),
		New(func(option *Options) {
			option.LeaderElection = componentbaseconfig.LeaderElectionConfiguration{
				LeaderElect: true,
			}
		}), {
			LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
				LeaderElect: false,
			},
			KubeAPIQPS:   40,
			KubeAPIBurst: 30,
		}}

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
		"invalid SchedulerEstimatorPort": {
			opt: New(func(option *Options) {
				option.SchedulerEstimatorPort = 90000
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("SchedulerEstimatorPort"), 90000, "must be a valid port between 0 and 65535 inclusive")},
		},
		"invalid SchedulerEstimatorTimeout": {
			opt: New(func(option *Options) {
				option.SchedulerEstimatorTimeout = metav1.Duration{Duration: -1 * time.Second}
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("SchedulerEstimatorTimeout"), metav1.Duration{Duration: -1 * time.Second}, "must be greater than or equal to 0")},
		},
		"invalid DeschedulingInterval": {
			opt: New(func(option *Options) {
				option.DeschedulingInterval = metav1.Duration{Duration: -1 * time.Second}
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("DeschedulingInterval"), metav1.Duration{Duration: -1 * time.Second}, "must be greater than or equal to 0")},
		},
		"invalid UnschedulableThreshold": {
			opt: New(func(option *Options) {
				option.UnschedulableThreshold = metav1.Duration{Duration: -1 * time.Second}
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("UnschedulableThreshold"), metav1.Duration{Duration: -1 * time.Second}, "must be greater than or equal to 0")},
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
