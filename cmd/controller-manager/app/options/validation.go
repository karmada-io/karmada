/*
Copyright 2021 The Karmada Authors.

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
	"regexp"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/karmada-io/karmada/pkg/util"
)

// Validate checks Options and return a slice of found errs.
func (o *Options) Validate() field.ErrorList {
	errs := field.ErrorList{}
	newPath := field.NewPath("Options")

	skippedResourceConfig := util.NewSkippedResourceConfig()
	if err := skippedResourceConfig.Parse(o.SkippedPropagatingAPIs); err != nil {
		errs = append(errs, field.Invalid(newPath.Child("SkippedPropagatingAPIs"), o.SkippedPropagatingAPIs, "Invalid API string"))
	}
	if o.ClusterStatusUpdateFrequency.Duration <= 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterStatusUpdateFrequency"), o.ClusterStatusUpdateFrequency, "must be greater than 0"))
	}
	if o.ClusterLeaseDuration.Duration <= 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterLeaseDuration"), o.ClusterLeaseDuration, "must be greater than 0"))
	}
	if o.ClusterMonitorPeriod.Duration <= 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterMonitorPeriod"), o.ClusterMonitorPeriod, "must be greater than 0"))
	}
	if o.ClusterMonitorGracePeriod.Duration <= 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterMonitorGracePeriod"), o.ClusterMonitorGracePeriod, "must be greater than 0"))
	}
	if o.ClusterStartupGracePeriod.Duration <= 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterStartupGracePeriod"), o.ClusterStartupGracePeriod, "must be greater than 0"))
	}
	for index, ns := range o.SkippedPropagatingNamespaces {
		if _, err := regexp.Compile(fmt.Sprintf("^%s$", ns)); err != nil {
			errs = append(errs, field.Invalid(newPath.Child("SkippedPropagatingNamespaces").Index(index), ns, "Invalid namespace regular expression"))
		}
	}

	errs = append(errs, o.FederatedResourceQuotaOptions.Validate()...)
	errs = append(errs, o.FailoverOptions.Validate()...)

	return errs
}
