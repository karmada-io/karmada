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

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/karmada-io/karmada/pkg/apis/cluster/validation"
)

// Validate checks Options and return a slice of found errs.
// fs is the flag set the Options were parsed from, used to detect flags that were set explicitly.
func (o *Options) Validate(fs *pflag.FlagSet) field.ErrorList {
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

	if o.ClusterLeaseRenewIntervalFraction <= 0 || o.ClusterLeaseRenewIntervalFraction >= 1 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterLeaseRenewIntervalFraction"), o.ClusterLeaseRenewIntervalFraction, "must be greater than 0 and less than 1"))
	}

	// Check whether registration flags were set rather than their values, since some have non-empty defaults.
	if !o.EnableClusterRegistration {
		// clusterRegistrationFlags are the flags that only take effect when cluster registration is enabled,
		// paired with the Options field each one sets.
		clusterRegistrationFlags := []struct {
			name  string
			field string
		}{
			{name: "cluster-api-endpoint", field: "ClusterAPIEndpoint"},
			{name: "proxy-server-address", field: "ProxyServerAddress"},
			{name: "cluster-provider", field: "ClusterProvider"},
			{name: "cluster-region", field: "ClusterRegion"},
			{name: "cluster-zones", field: "ClusterZones"},
			{name: "report-secrets", field: "ReportSecrets"},
			{name: "cluster-namespace", field: "ClusterNamespace"},
		}
		for _, f := range clusterRegistrationFlags {
			if fs.Changed(f.name) {
				errs = append(errs, field.Forbidden(newPath.Child(f.field), "must not be set when --enable-cluster-registration=false"))
			}
		}
	}

	return errs
}
