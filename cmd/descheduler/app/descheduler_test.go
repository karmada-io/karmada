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

package app

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/karmada-io/karmada/cmd/descheduler/app/options"
	"github.com/karmada-io/karmada/pkg/util/names"
	testingutil "github.com/karmada-io/karmada/pkg/util/testing"
)

func TestNewDeschedulerCommand(t *testing.T) {
	ctx := context.Background()
	cmd := NewDeschedulerCommand(ctx)

	assert.NotNil(t, cmd)
	assert.Equal(t, names.KarmadaDeschedulerComponentName, cmd.Use)
	assert.NotEmpty(t, cmd.Long)
}

func TestDeschedulerCommandFlagParsing(t *testing.T) {
	testCases := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{"Default flags", []string{}, false},
		{"With custom health probe bind address", []string{"--health-probe-bind-address=127.0.0.1:8080"}, false},
		{"With custom metrics bind address", []string{"--metrics-bind-address=127.0.0.1:8081"}, false},
		{"With leader election enabled", []string{"--leader-elect=true"}, false},
		{"With invalid flag", []string{"--invalid-flag=value"}, true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cmd := NewDeschedulerCommand(ctx)
			cmd.SetArgs(tc.args)
			err := cmd.ParseFlags(tc.args)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServeHealthzAndMetrics(t *testing.T) {
	ports, err := testingutil.GetFreePorts("127.0.0.1", 2)
	require.NoError(t, err)
	healthAddress := fmt.Sprintf("127.0.0.1:%d", ports[0])
	metricsAddress := fmt.Sprintf("127.0.0.1:%d", ports[1])

	go serveHealthzAndMetrics(healthAddress, metricsAddress)

	// For servers to start
	time.Sleep(100 * time.Millisecond)

	t.Run("Healthz endpoint", func(t *testing.T) {
		resp, err := http.Get("http://" + healthAddress + "/healthz")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Metrics endpoint", func(t *testing.T) {
		resp, err := http.Get("http://" + metricsAddress + "/metrics")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestDeschedulerOptionsValidation(t *testing.T) {
	testCases := []struct {
		name        string
		setupOpts   func(*options.Options)
		expectError bool
	}{
		{
			name: "Default options",
			setupOpts: func(_ *options.Options) {
				// Default options are valid
			},
			expectError: false,
		},
		{
			name: "Invalid descheduling interval",
			setupOpts: func(o *options.Options) {
				o.DeschedulingInterval.Duration = -1 * time.Second
			},
			expectError: true,
		},
		{
			name: "Invalid unschedulable threshold",
			setupOpts: func(o *options.Options) {
				o.UnschedulableThreshold.Duration = -1 * time.Second
			},
			expectError: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := options.NewOptions()
			tc.setupOpts(opts)
			errs := opts.Validate()
			if tc.expectError {
				assert.NotEmpty(t, errs)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}
