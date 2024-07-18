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
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/karmada-io/karmada/pkg/apis/cluster/validation"
)

// Validate checks Options and return a slice of found errs.
func (o *Options) Validate() field.ErrorList {
	errs := field.ErrorList{}

	newPath := field.NewPath("Options")
	if errMsgs := validation.ValidateClusterName(o.ClusterName); len(errMsgs) > 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterName"), o.ClusterName, strings.Join(errMsgs, ",")))
	}

	if o.ClusterStatusUpdateFrequency.Duration < 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterStatusUpdateFrequency"), o.ClusterStatusUpdateFrequency, "must be greater than or equal to 0"))
	}

	if o.ClusterLeaseDuration.Duration < 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterLeaseDuration"), o.ClusterLeaseDuration, "must be greater than or equal to 0"))
	}

	if o.ClusterLeaseRenewIntervalFraction <= 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterLeaseRenewIntervalFraction"), o.ClusterLeaseRenewIntervalFraction, "must be greater than 0"))
	}

	return errs
}
