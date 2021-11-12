/*
Copyright The Karmada Authors.

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
	"fmt"

	"github.com/karmada-io/karmada/pkg/util"
)

func validateSkippedResourceConfig(opts *Options) []error {
	var errs []error
	skippedResourceConfig := util.NewSkippedResourceConfig()
	if err := skippedResourceConfig.Parse(opts.SkippedPropagatingAPIs); err != nil {
		errs = append(errs, err)
	}

	return errs
}

func validateSecurePort(opts *Options) []error {
	var errs []error
	if opts.SecurePort < 0 || opts.SecurePort > 65535 {
		errs = append(errs, fmt.Errorf("--secure-port %v must be between 0 and 65535 inclusive. ", opts.SecurePort))
	}
	return errs
}

func validateClusterStatusUpdateDuration(opts *Options) []error {
	var errs []error

	if opts.ClusterStatusUpdateFrequency.Duration <= 0 {
		errs = append(errs, fmt.Errorf("--cluster-status-update-frequency %v must be greater than 0. ", opts.ClusterStatusUpdateFrequency.Duration))
	}
	return errs
}

func validateClusterLeaseDuration(opts *Options) []error {
	var errs []error

	if opts.ClusterLeaseDuration.Duration <= 0 {
		errs = append(errs, fmt.Errorf("--cluster-lease-duration %v must be greater than 0. ", opts.ClusterLeaseDuration.Duration))
	}
	return errs
}

func validateClusterMonitorPeriod(opts *Options) []error {
	var errs []error

	if opts.ClusterMonitorPeriod.Duration <= 0 {
		errs = append(errs, fmt.Errorf("--cluster-monitor-period %v must be greater than 0. ", opts.ClusterMonitorPeriod.Duration))
	}
	return errs
}

func validateClusterMonitorGracePeriod(opts *Options) []error {
	var errs []error

	if opts.ClusterMonitorGracePeriod.Duration <= 0 {
		errs = append(errs, fmt.Errorf("--cluster-monitor-grace-period %v must be greater than 0. ", opts.ClusterMonitorGracePeriod.Duration))
	}
	return errs
}

func validateClusterStartupGracePeriod(opts *Options) []error {
	var errs []error

	if opts.ClusterStartupGracePeriod.Duration <= 0 {
		errs = append(errs, fmt.Errorf("--cluster-startup-grace-period %v must be greater than 0. ", opts.ClusterStartupGracePeriod.Duration))
	}
	return errs
}

// Validate checks Options and return a slice of found errs.
func (o *Options) Validate() []error {
	var errs []error
	errs = append(errs, validateSkippedResourceConfig(o)...)
	errs = append(errs, validateSecurePort(o)...)
	errs = append(errs, validateClusterStatusUpdateDuration(o)...)
	errs = append(errs, validateClusterLeaseDuration(o)...)
	errs = append(errs, validateClusterMonitorPeriod(o)...)
	errs = append(errs, validateClusterMonitorGracePeriod(o)...)
	errs = append(errs, validateClusterStartupGracePeriod(o)...)

	return errs
}
