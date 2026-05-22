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
	"slices"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/karmada-io/karmada/pkg/estimator/server/framework/plugins/noderesource"
)

// Validate checks Options and return a slice of found errs.
func (o *Options) Validate() field.ErrorList {
	errs := field.ErrorList{}

	newPath := field.NewPath("Options")
	if len(o.ClusterName) == 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterName"), o.ClusterName, "clusterName cannot be empty"))
	}

	if o.ServerPort < 0 || o.ServerPort > 65535 {
		errs = append(errs, field.Invalid(newPath.Child("ServerPort"), o.ServerPort, "must be a valid port between 0 and 65535 inclusive"))
	}

	// Validate node capacity provider names: no unknowns, no duplicates.
	seen := make(map[string]bool, len(o.NodeCapacityProviders))
	for _, name := range o.NodeCapacityProviders {
		if seen[name] {
			errs = append(errs, field.Invalid(newPath.Child("NodeCapacityProviders"), name,
				fmt.Sprintf("duplicate node capacity provider %q", name)))
			continue
		}
		seen[name] = true
		if _, ok := noderesource.LookupProvider(name); !ok {
			errs = append(errs, field.Invalid(newPath.Child("NodeCapacityProviders"), name,
				fmt.Sprintf("unknown node capacity provider %q (available: %v)", name, noderesource.RegisteredProviders())))
		}
	}

	// --node-capacity-providers requires NodeResourceEstimator to be enabled.
	if len(o.NodeCapacityProviders) > 0 && len(o.Plugins) > 0 {
		if !slices.Contains(o.Plugins, "NodeResourceEstimator") {
			errs = append(errs, field.Invalid(newPath.Child("NodeCapacityProviders"), o.NodeCapacityProviders,
				"--node-capacity-providers requires NodeResourceEstimator to be enabled in --plugins"))
		}
	}

	return errs
}
