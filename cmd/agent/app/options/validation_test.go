package options

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfig "k8s.io/component-base/config"
)

func TestValidateKarmadaAgentConfiguration(t *testing.T) {
	successCases := []Options{
		{
			LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
				LeaderElect: false,
			},
			ClusterName:                       "demo",
			ClusterStatusUpdateFrequency:      metav1.Duration{Duration: 10 * time.Second},
			ClusterLeaseDuration:              metav1.Duration{Duration: 40 * time.Second},
			ClusterLeaseRenewIntervalFraction: 0.25,
		},
		{
			LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
				LeaderElect: true,
			},
			ClusterName:                       "demo",
			ClusterStatusUpdateFrequency:      metav1.Duration{Duration: 10 * time.Second},
			ClusterLeaseDuration:              metav1.Duration{Duration: 40 * time.Second},
			ClusterLeaseRenewIntervalFraction: 0.25,
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
				LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
					LeaderElect: false,
				},
				ClusterName:                       "",
				ClusterStatusUpdateFrequency:      metav1.Duration{Duration: 10 * time.Second},
				ClusterLeaseDuration:              metav1.Duration{Duration: 40 * time.Second},
				ClusterLeaseRenewIntervalFraction: 0.25,
			},
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterName"), "", "must be not empty")},
		},
		"invalid ClusterStatusUpdateFrequency": {
			opt: Options{
				LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
					LeaderElect: false,
				},
				ClusterName:                       "demo",
				ClusterStatusUpdateFrequency:      metav1.Duration{Duration: -10 * time.Second},
				ClusterLeaseDuration:              metav1.Duration{Duration: 40 * time.Second},
				ClusterLeaseRenewIntervalFraction: 0.25,
			},
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterStatusUpdateFrequency"), metav1.Duration{Duration: -10 * time.Second}, "must be greater than or equal to 0")},
		},
		"invalid ClusterLeaseDuration": {
			opt: Options{
				LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
					LeaderElect: false,
				},
				ClusterName:                       "demo",
				ClusterStatusUpdateFrequency:      metav1.Duration{Duration: 10 * time.Second},
				ClusterLeaseDuration:              metav1.Duration{Duration: -40 * time.Second},
				ClusterLeaseRenewIntervalFraction: 0.25,
			},
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("ClusterLeaseDuration"), metav1.Duration{Duration: -40 * time.Second}, "must be greater than or equal to 0")},
		},
		"invalid ClusterLeaseRenewIntervalFraction": {
			opt: Options{
				LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
					LeaderElect: false,
				},
				ClusterName:                       "demo",
				ClusterStatusUpdateFrequency:      metav1.Duration{Duration: 10 * time.Second},
				ClusterLeaseDuration:              metav1.Duration{Duration: 40 * time.Second},
				ClusterLeaseRenewIntervalFraction: 0,
			},
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
