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
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// CorednsOptions contains options for coredns detector.
type CorednsOptions struct {
	Period           time.Duration
	SuccessThreshold time.Duration
	FailureThreshold time.Duration
	StaleThreshold   time.Duration
}

// NewCorednsOptions return default options for coredns detector.
func NewCorednsOptions() *CorednsOptions {
	return &CorednsOptions{}
}

// AddFlags adds flags of coredns detector to the specified FlagSet.
func (o *CorednsOptions) AddFlags(fs *pflag.FlagSet) {
	fs.DurationVar(&o.Period, "coredns-detect-period", 5*time.Second,
		"Specifies how often detector detects coredns health status.")
	fs.DurationVar(&o.SuccessThreshold, "coredns-success-threshold", 30*time.Second,
		"The duration of successes for the coredns to be considered healthy after recovery.")
	fs.DurationVar(&o.FailureThreshold, "coredns-failure-threshold", 30*time.Second,
		"The duration of failure for the coredns to be considered unhealthy.")
	fs.DurationVar(&o.StaleThreshold, "coredns-stale-threshold", time.Minute,
		"If the node condition of coredns has not been updated for coredns-stale-threshold, it should be considered unknown.")
}

// Complete fills in fields required to have valid data.
func (o *CorednsOptions) Complete() error {
	return nil
}

// Validate checks options and return a slice of found errs.
func (o *CorednsOptions) Validate() field.ErrorList {
	return field.ErrorList{}
}
