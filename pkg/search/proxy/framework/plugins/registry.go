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
