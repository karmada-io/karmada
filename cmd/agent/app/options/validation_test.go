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
		ClusterName:                       "demo",
		SecurePort:                        8090,
		ClusterStatusUpdateFrequency:      metav1.Duration{Duration: 10 * time.Second},
		ClusterLeaseDuration:              metav1.Duration{Duration: 40 * time.Second},
		ClusterLeaseRenewIntervalFraction: 0.25,
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
			opt: New(func(options *Options) {
				options.ClusterName = ""
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterName"), "", "must be not empty")},
		},
		"invalid SecurePort": {
			opt: New(func(options *Options) {
				options.SecurePort = -10
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("SecurePort"), -10, "must be between 0 and 65535 inclusive")},
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
				options.ClusterLeaseRenewIntervalFraction = 0
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterLeaseRenewIntervalFraction"), 0, "must be greater than 0")},
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
