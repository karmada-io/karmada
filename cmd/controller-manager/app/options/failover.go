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
	plugins "github.com/karmada-io/karmada/pkg/controllers/gracefuleviction/evictplugins"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// FailoverOptions holds the Failover configurations.
type FailoverOptions struct {
	// EnableNoExecuteTaintEviction enables controller response to NoExecute taints on clusters,
	// which triggers eviction of workloads without explicit tolerations.
	EnableNoExecuteTaintEviction bool
	// NoExecuteTaintEvictionPurgeMode controls resource cleanup behavior for NoExecute-triggered
	// evictions (only active when --enable-no-execute-taint-eviction=true).
	// Valid modes:
	// - "Gracefully": first schedules workloads to new clusters and then cleans up original
	//                 workloads after successful startup elsewhere to ensure service continuity.
	// - "Directly": directly evicts workloads first (risking temporary service interruption)
	//               and then triggers rescheduling to other clusters.
	// Default: "Gracefully".
	NoExecuteTaintEvictionPurgeMode string
	// EvictionPlugins holds the names of eviction plugins to be enabled.
	EvictionPlugins []string
}

// AddFlags adds flags related to FailoverOptions for controller manager to the specified FlagSet.
func (o *FailoverOptions) AddFlags(flags *pflag.FlagSet) {
	if o == nil {
		return
	}

	flags.BoolVar(&o.EnableNoExecuteTaintEviction, "enable-no-execute-taint-eviction", false, "Enables controller response to NoExecute taints on clusters, which triggers eviction of workloads without explicit tolerations. Given the impact of eviction caused by NoExecute Taint, this parameter is designed to remain disabled by default and requires careful evaluation by administrators before being enabled.\n")
	flags.StringVar(&o.NoExecuteTaintEvictionPurgeMode, "no-execute-taint-eviction-purge-mode", "Gracefully", "Controls resource cleanup behavior for NoExecute-triggered evictions (only active when --enable-no-execute-taint-eviction=true). Supported values are \"Directly\", and \"Gracefully\". \"Directly\" mode directly evicts workloads first (risking temporary service interruption) and then triggers rescheduling to other clusters, while \"Gracefully\" mode first schedules workloads to new clusters and then cleans up original workloads after successful startup elsewhere to ensure service continuity.")
	flags.StringSliceVar(&o.EvictionPlugins, "eviction-plugins", []string{"ResourceHealth"}, "A comma-separated list of graceful eviction plugins to be enabled. '*' can be used to enable all registered plugins. The 'ResourceHealth' plugin is enabled by default.")
}

// Validate checks FailoverOptions and return a slice of found errs.
func (o *FailoverOptions) Validate() field.ErrorList {
	errs := field.ErrorList{}
	path := field.NewPath("FailoverOptions")

	if o.EnableNoExecuteTaintEviction &&
		o.NoExecuteTaintEvictionPurgeMode != "Gracefully" &&
		o.NoExecuteTaintEvictionPurgeMode != "Directly" {
		errs = append(errs, field.Invalid(path.Child("NoExecuteTaintEvictionPurgeMode"),
			o.NoExecuteTaintEvictionPurgeMode, "Invalid mode"))
	}

	for _, pluginName := range o.EvictionPlugins {
		if pluginName == "*" {
			continue
		}
		if !plugins.IsRegistered(pluginName) {
			errs = append(errs, field.NotSupported(path.Child("EvictionPlugins"), pluginName, plugins.RegisteredPlugins()))
		}
	}

	return errs
}
