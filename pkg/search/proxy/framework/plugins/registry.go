/*
Copyright 2022 The Karmada Authors.

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
	cacheplugin "github.com/karmada-io/karmada/pkg/search/proxy/framework/plugins/cache"
	clusterplugin "github.com/karmada-io/karmada/pkg/search/proxy/framework/plugins/cluster"
	karmadaplugin "github.com/karmada-io/karmada/pkg/search/proxy/framework/plugins/karmada"
	pluginruntime "github.com/karmada-io/karmada/pkg/search/proxy/framework/runtime"
)

// For detailed information of in tree plugins' execution order, please see:
// https://github.com/karmada-io/karmada/tree/master/docs/proposals/resource-aggregation-proxy#request-routing

// NewInTreeRegistry builds the registry with all the in-tree plugins.
func NewInTreeRegistry() pluginruntime.Registry {
	return pluginruntime.Registry{
		cacheplugin.New,
		clusterplugin.New,
		karmadaplugin.New,
	}
}
