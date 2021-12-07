package options

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateKarmadaSchedulerEstimator(t *testing.T) {
	successCases := []Options{
		{
			ClusterName: "testCluster",
			BindAddress: "0.0.0.0",
			SecurePort:  10100,
			ServerPort:  8088,
		},
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
			opt: Options{
				ClusterName: "",
				BindAddress: "127.0.0.1",
				SecurePort:  10100,
				ServerPort:  8088,
			},
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterName"), "", "clusterName cannot be empty")},
		},
		"invalid BindAddress": {
			opt: Options{
				ClusterName: "testCluster",
				BindAddress: "127.0.0.1:8082",
				SecurePort:  10100,
				ServerPort:  8088,
			},
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("BindAddress"), "127.0.0.1:8082", "not a valid textual representation of an IP address")},
		},
		"invalid SecurePort": {
			opt: Options{
				ClusterName: "testCluster",
				BindAddress: "127.0.0.1",
				SecurePort:  908188,
				ServerPort:  8088,
			},
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("SecurePort"), 908188, "must be a valid port between 0 and 65535 inclusive")},
		},
		"invalid ServerPort": {
			opt: Options{
				ClusterName: "testCluster",
				BindAddress: "127.0.0.1",
				SecurePort:  9089,
				ServerPort:  80888,
			},
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
