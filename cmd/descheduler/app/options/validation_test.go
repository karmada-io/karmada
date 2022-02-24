package options

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfig "k8s.io/component-base/config"
)

func TestValidateKarmadaDescheduler(t *testing.T) {
	successCases := []Options{
		{
			LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
				LeaderElect: false,
			},
			BindAddress:               "127.0.0.1",
			SecurePort:                9000,
			KubeAPIQPS:                40,
			KubeAPIBurst:              30,
			SchedulerEstimatorTimeout: metav1.Duration{Duration: 1 * time.Second},
			SchedulerEstimatorPort:    9001,
			DeschedulingInterval:      metav1.Duration{Duration: 1 * time.Second},
			UnschedulableThreshold:    metav1.Duration{Duration: 1 * time.Second},
		},
		{
			LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
				LeaderElect: true,
			},
			BindAddress:               "127.0.0.1",
			SecurePort:                9000,
			KubeAPIQPS:                40,
			KubeAPIBurst:              30,
			SchedulerEstimatorTimeout: metav1.Duration{Duration: 1 * time.Second},
			SchedulerEstimatorPort:    9001,
			DeschedulingInterval:      metav1.Duration{Duration: 1 * time.Second},
			UnschedulableThreshold:    metav1.Duration{Duration: 1 * time.Second},
		}, {
			LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
				LeaderElect: false,
			},
			BindAddress:  "127.0.0.1",
			SecurePort:   9000,
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
		"invalid BindAddress": {
			opt: Options{
				LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
					LeaderElect: false,
				},
				BindAddress:               "127.0.0.1:8080",
				SecurePort:                9000,
				KubeAPIQPS:                40,
				KubeAPIBurst:              30,
				SchedulerEstimatorTimeout: metav1.Duration{Duration: 1 * time.Second},
				SchedulerEstimatorPort:    9001,
				DeschedulingInterval:      metav1.Duration{Duration: 1 * time.Second},
				UnschedulableThreshold:    metav1.Duration{Duration: 1 * time.Second},
			},
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("BindAddress"), "127.0.0.1:8080", "not a valid textual representation of an IP address")},
		},
		"invalid SecurePort": {
			opt: Options{
				LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
					LeaderElect: false,
				},
				BindAddress:               "127.0.0.1",
				SecurePort:                90000,
				KubeAPIQPS:                40,
				KubeAPIBurst:              30,
				SchedulerEstimatorTimeout: metav1.Duration{Duration: 1 * time.Second},
				SchedulerEstimatorPort:    9001,
				DeschedulingInterval:      metav1.Duration{Duration: 1 * time.Second},
				UnschedulableThreshold:    metav1.Duration{Duration: 1 * time.Second},
			},
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("SecurePort"), 90000, "must be a valid port between 0 and 65535 inclusive")},
		},
		"invalid SchedulerEstimatorPort": {
			opt: Options{
				LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
					LeaderElect: false,
				},
				BindAddress:               "127.0.0.1",
				SecurePort:                9000,
				KubeAPIQPS:                40,
				KubeAPIBurst:              30,
				SchedulerEstimatorTimeout: metav1.Duration{Duration: 1 * time.Second},
				SchedulerEstimatorPort:    90000,
				DeschedulingInterval:      metav1.Duration{Duration: 1 * time.Second},
				UnschedulableThreshold:    metav1.Duration{Duration: 1 * time.Second},
			},
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("SchedulerEstimatorPort"), 90000, "must be a valid port between 0 and 65535 inclusive")},
		},
		"invalid SchedulerEstimatorTimeout": {
			opt: Options{
				LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
					LeaderElect: false,
				},
				BindAddress:               "127.0.0.1",
				SecurePort:                9000,
				KubeAPIQPS:                40,
				KubeAPIBurst:              30,
				SchedulerEstimatorTimeout: metav1.Duration{Duration: -1 * time.Second},
				SchedulerEstimatorPort:    9000,
				DeschedulingInterval:      metav1.Duration{Duration: 1 * time.Second},
				UnschedulableThreshold:    metav1.Duration{Duration: 1 * time.Second},
			},
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("SchedulerEstimatorTimeout"), metav1.Duration{Duration: -1 * time.Second}, "must be greater than or equal to 0")},
		},
		"invalid DeschedulingInterval": {
			opt: Options{
				LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
					LeaderElect: false,
				},
				BindAddress:               "127.0.0.1",
				SecurePort:                9000,
				KubeAPIQPS:                40,
				KubeAPIBurst:              30,
				SchedulerEstimatorTimeout: metav1.Duration{Duration: 1 * time.Second},
				SchedulerEstimatorPort:    9000,
				DeschedulingInterval:      metav1.Duration{Duration: -1 * time.Second},
				UnschedulableThreshold:    metav1.Duration{Duration: 1 * time.Second},
			},
			expectedErrs: field.ErrorList{field.Invalid(newPath.Child("DeschedulingInterval"), metav1.Duration{Duration: -1 * time.Second}, "must be greater than or equal to 0")},
		},
		"invalid UnschedulableThreshold": {
			opt: Options{
				LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
					LeaderElect: false,
				},
				BindAddress:               "127.0.0.1",
				SecurePort:                9000,
				KubeAPIQPS:                40,
				KubeAPIBurst:              30,
				SchedulerEstimatorTimeout: metav1.Duration{Duration: 1 * time.Second},
				SchedulerEstimatorPort:    9000,
				DeschedulingInterval:      metav1.Duration{Duration: 1 * time.Second},
				UnschedulableThreshold:    metav1.Duration{Duration: -1 * time.Second},
			},
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
