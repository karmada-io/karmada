/*
Copyright 2025 The Karmada Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// FederatedResourceQuotaOptions holds the FederatedResourceQuota-related options.
type FederatedResourceQuotaOptions struct {
	// federatedResourceQuotaSyncPeriod is the period for syncing federated resource quota usage status
	// in the system.
	ResourceQuotaSyncPeriod metav1.Duration
}

// AddFlags adds flags related to FederatedResourceQuotaEnforcement for controller manager to the specified FlagSet.
func (o *FederatedResourceQuotaOptions) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}
	fs.DurationVar(&o.ResourceQuotaSyncPeriod.Duration, "federated-resource-quota-sync-period", time.Minute*5, "The interval for periodic full resynchronization of FederatedResourceQuota resources. This ensures quota recalculations occur at regular intervals to correct potential inaccuracies, particularly when webhook validation side effects.")
}

// Validate checks FederatedResourceQuotaOptions and return a slice of found errs.
func (o *FederatedResourceQuotaOptions) Validate() field.ErrorList {
	if o.ResourceQuotaSyncPeriod.Duration <= 0 {
		return field.ErrorList{field.Invalid(field.NewPath("federatedResourceQuotaSyncPeriod"), o.ResourceQuotaSyncPeriod, "must be greater than 0")}
	}
	return nil
}
