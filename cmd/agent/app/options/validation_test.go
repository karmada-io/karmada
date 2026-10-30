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

	"github.com/spf13/pflag"
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
		ClusterName:                       "demo",
		ClusterStatusUpdateFrequency:      metav1.Duration{Duration: 10 * time.Second},
		ClusterLeaseDuration:              metav1.Duration{Duration: 40 * time.Second},
		ClusterLeaseRenewIntervalFraction: 0.25,
		EnableClusterRegistration:         true,
	}

	if modifyOptions != nil {
		modifyOptions(&option)
	}
	return option
}

func TestValidateKarmadaAgentConfiguration(t *testing.T) {
	successCases := []Options{
		New(nil),
		New(func(options *Options) {
			options.LeaderElection.LeaderElect = true
		}),
	}

	for _, successCase := range successCases {
		if errs := successCase.Validate(pflag.NewFlagSet("test", pflag.ContinueOnError)); len(errs) != 0 {
			t.Errorf("expected success: %v", errs)
		}
	}

	newPath := field.NewPath("Options")
	testCases := map[string]struct {
		opt          Options
		expectedErrs field.ErrorList
	}{
		"invalid ClusterName": {
			opt: New(func(options *Options) {
				options.ClusterName = ""
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterName"), "", "must be not empty")},
		},
		"invalid ClusterStatusUpdateFrequency": {
			opt: New(func(options *Options) {
				options.ClusterStatusUpdateFrequency = metav1.Duration{Duration: -10 * time.Second}
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterStatusUpdateFrequency"), metav1.Duration{Duration: -10 * time.Second}, "must be greater than or equal to 0")},
		},
		"invalid ClusterLeaseDuration": {
			opt: New(func(options *Options) {
				options.ClusterLeaseDuration = metav1.Duration{Duration: -40 * time.Second}
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterLeaseDuration"), metav1.Duration{Duration: -40 * time.Second}, "must be greater than or equal to 0")},
		},
		"invalid ClusterLeaseRenewIntervalFraction": {
			opt: New(func(options *Options) {
				options.ClusterLeaseRenewIntervalFraction = 1.2
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterLeaseRenewIntervalFraction"), float64(1.2), "must be greater than 0 and less than 1")},
		},
		"invalid ClusterLeaseRenewIntervalFraction (negative)": {
			opt: New(func(options *Options) {
				options.ClusterLeaseRenewIntervalFraction = -0.1
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterLeaseRenewIntervalFraction"), float64(-0.1), "must be greater than 0 and less than 1")},
		},
	}

	for _, testCase := range testCases {
		errs := testCase.opt.Validate(pflag.NewFlagSet("test", pflag.ContinueOnError))
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

func TestValidateClusterRegistrationFlags(t *testing.T) {
	newPath := field.NewPath("Options")
	forbidden := func(fieldName string) *field.Error {
		return field.Forbidden(newPath.Child(fieldName), "must not be set when --enable-cluster-registration=false")
	}
	registrationFlags := []string{
		"--cluster-api-endpoint=https://10.0.0.1:6443",
		"--proxy-server-address=http://10.0.0.2:8080",
		"--cluster-provider=aws",
		"--cluster-region=us-east-1",
		"--cluster-zones=us-east-1a,us-east-1b",
		"--report-secrets=None",
		"--cluster-namespace=karmada-cluster",
	}

	testCases := map[string]struct {
		args         []string
		expectedErrs field.ErrorList
	}{
		"registration enabled by default, registration flags allowed": {
			args: registrationFlags,
		},
		"registration disabled, no registration flags set": {
			args: []string{"--enable-cluster-registration=false"},
		},
		"registration disabled, registration flags set": {
			args: append([]string{"--enable-cluster-registration=false"}, registrationFlags...),
			expectedErrs: field.ErrorList{
				forbidden("ClusterAPIEndpoint"),
				forbidden("ProxyServerAddress"),
				forbidden("ClusterProvider"),
				forbidden("ClusterRegion"),
				forbidden("ClusterZones"),
				forbidden("ReportSecrets"),
				forbidden("ClusterNamespace"),
			},
		},
		"registration disabled, registration flag set to an empty value": {
			args:         []string{"--enable-cluster-registration=false", "--cluster-provider="},
			expectedErrs: field.ErrorList{forbidden("ClusterProvider")},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			opts := NewOptions()
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			opts.AddFlags(fs, nil)
			if err := fs.Parse(append([]string{"--cluster-name=member1"}, testCase.args...)); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			errs := opts.Validate(fs)
			if len(testCase.expectedErrs) != len(errs) {
				t.Fatalf("Expected %d errors, got %d errors: %v", len(testCase.expectedErrs), len(errs), errs)
			}
			for i, err := range errs {
				if err.Error() != testCase.expectedErrs[i].Error() {
					t.Fatalf("Expected error: %s, got %s", testCase.expectedErrs[i], err.Error())
				}
			}
		})
	}
}
