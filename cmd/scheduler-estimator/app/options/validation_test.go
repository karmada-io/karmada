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
		BindAddress: "0.0.0.0",
		SecurePort:  10100,
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
		"invalid BindAddress": {
			opt: New(func(option *Options) {
				option.BindAddress = "127.0.0.1:8082"
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("BindAddress"), "127.0.0.1:8082", "not a valid textual representation of an IP address")},
		},
		"invalid SecurePort": {
			opt: New(func(option *Options) {
				option.SecurePort = 908188
			}),
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("SecurePort"), 908188, "must be a valid port between 0 and 65535 inclusive")},
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
