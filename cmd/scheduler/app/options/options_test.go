/*
Copyright 2024 The Karmada Authors.

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
	"github.com/stretchr/testify/assert"
)

func TestNewOptions(t *testing.T) {
	opts := NewOptions()

	assert.True(t, opts.LeaderElection.LeaderElect, "Expected default LeaderElect to be true")
	assert.Equal(t, "karmada-system", opts.LeaderElection.ResourceNamespace, "Unexpected default ResourceNamespace")
	assert.Equal(t, 15*time.Second, opts.LeaderElection.LeaseDuration.Duration, "Unexpected default LeaseDuration")
	assert.Equal(t, "karmada-scheduler", opts.LeaderElection.ResourceName, "Unexpected default ResourceName")
}

func TestAddFlags(t *testing.T) {
	opts := NewOptions()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	opts.AddFlags(fs)

	testCases := []struct {
		name            string
		expectedType    string
		expectedDefault string
	}{
		{"kubeconfig", "string", ""},
		{"leader-elect", "bool", "true"},
		{"enable-scheduler-estimator", "bool", "false"},
		{"scheduler-estimator-port", "int", "10352"},
		{"plugins", "stringSlice", "[*]"},
		{"scheduler-name", "string", "default-scheduler"},
	}

	for _, tc := range testCases {
		flag := fs.Lookup(tc.name)
		assert.NotNil(t, flag, "Flag %s not found", tc.name)
		assert.Equal(t, tc.expectedType, flag.Value.Type(), "Unexpected type for flag %s", tc.name)
		assert.Equal(t, tc.expectedDefault, flag.DefValue, "Unexpected default value for flag %s", tc.name)
	}
}

func TestOptionsComplete(t *testing.T) {
	testCases := []struct {
		name            string
		bindAddress     string
		securePort      int
		expectedMetrics string
		expectedHealth  string
	}{
		{
			name:            "Default values",
			bindAddress:     defaultBindAddress,
			securePort:      defaultPort,
			expectedMetrics: "0.0.0.0:10351",
			expectedHealth:  "0.0.0.0:10351",
		},
		{
			name:            "Custom values",
			bindAddress:     "127.0.0.1",
			securePort:      8080,
			expectedMetrics: "127.0.0.1:8080",
			expectedHealth:  "127.0.0.1:8080",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := &Options{
				BindAddress: tc.bindAddress,
				SecurePort:  tc.securePort,
			}
			err := opts.Complete()
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedMetrics, opts.MetricsBindAddress)
			assert.Equal(t, tc.expectedHealth, opts.HealthProbeBindAddress)
		})
	}
}

func TestOptionsFlagParsing(t *testing.T) {
	opts := NewOptions()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	opts.AddFlags(fs)

	testArgs := []string{
		"--leader-elect=false",
		"--enable-scheduler-estimator=true",
		"--plugins=*,-foo,bar",
		"--scheduler-name=custom-scheduler",
	}

	err := fs.Parse(testArgs)
	assert.NoError(t, err)

	assert.False(t, opts.LeaderElection.LeaderElect)
	assert.True(t, opts.EnableSchedulerEstimator)
	assert.Equal(t, []string{"*", "-foo", "bar"}, opts.Plugins)
	assert.Equal(t, "custom-scheduler", opts.SchedulerName)
}
