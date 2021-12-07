package options

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateKarmadaWebhookConfiguration(t *testing.T) {
	successCases := []Options{
		{
			BindAddress:  "127.0.0.1",
			SecurePort:   9000,
			KubeAPIQPS:   40,
			KubeAPIBurst: 30,
		},
	}
	for _, successCases := range successCases {
		if errs := successCases.Validate(); len(errs) != 0 {
			t.Errorf("expected success: %v", errs)
		}
	}
	newPath := field.NewPath("Options")
	testCases := map[string]struct {
		opt          Options
		expectedErrs field.ErrorList
	}{
		"invalid BindAddress": {
			opt: Options{
				BindAddress:  "127.0.0.1:8080",
				SecurePort:   9000,
				KubeAPIQPS:   40,
				KubeAPIBurst: 30,
			},
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("BindAddress"), "127.0.0.1:8080", "not a valid textual representation of an IP address")},
		},
		"invalid SecurePort": {
			opt: Options{
				BindAddress:  "127.0.0.1",
				SecurePort:   900000,
				KubeAPIQPS:   40,
				KubeAPIBurst: 30,
			},
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("SecurePort"), 900000, "must be a valid port between 0 and 65535 inclusive")},
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
