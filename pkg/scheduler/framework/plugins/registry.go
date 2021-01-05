package plugins

import (
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/clusteraffinity"
)

// NewPlugins builds all the scheduling plugins.
func NewPlugins() map[string]framework.Plugin {
	return map[string]framework.Plugin{
		clusteraffinity.Name: clusteraffinity.New(),
	}
}
