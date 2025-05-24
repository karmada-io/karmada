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
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// FailoverOptions holds the Failover configurations.
type FailoverOptions struct {
	// EnablePassiveMigration determines if the controller should respect NoExecute taints
	// and enable passive pod migration.
	EnablePassiveMigration bool
	// NoExecutePurgeMode defines the behavior of pod eviction during passive migration.
	// Valid modes:
	// - "gracefully": the system will wait for workload become healthy on the new cluster.
	// - "immediately": the system will immediately evict the legacy workload.
	// Default: "gracefully".
	PassiveMigrationMode string
}

// NewFailoverOptions creates a new FailoverOptions object with default parameters.
func NewFailoverOptions() *FailoverOptions {
	return &FailoverOptions{}
}

func (o *FailoverOptions) AddFlags(flags *pflag.FlagSet) {
	if o == nil {
		return
	}

	flags.BoolVar(&o.EnablePassiveMigration, "enable-passive-migration", false, "It determines if the controller should respect NoExecute taints and enable passive pod migration.")
	flags.StringVar(&o.PassiveMigrationMode, "passive-migration-mode", "gracefully", "It defines the behavior of workload eviction during passive migration. Valid modes: \"gracefully\" and \"immediately\".")

}

// Validate checks FailoverOptions and return a slice of found errs.
func (o *FailoverOptions) Validate() field.ErrorList {
	errs := field.ErrorList{}
	return errs
}
