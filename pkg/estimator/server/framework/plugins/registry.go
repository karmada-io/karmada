/*
Copyright 2020 The Karmada Authors.

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

package plugins

import (
	"k8s.io/apiserver/pkg/util/feature"

	plfeature "github.com/karmada-io/karmada/pkg/estimator/server/framework/plugins/feature"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework/plugins/resourcequota"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework/runtime"
)

// NewInTreeRegistry builds the registry with all the in-tree plugins.
func NewInTreeRegistry() runtime.Registry {
	fts := plfeature.Features{
		EnableResourceQuotaEstimate: feature.DefaultFeatureGate.Enabled(plfeature.ResourceQuotaEstimate),
	}
	registry := runtime.Registry{
		resourcequota.Name: runtime.FactoryAdapter(fts, resourcequota.New),
	}
	return registry
}
