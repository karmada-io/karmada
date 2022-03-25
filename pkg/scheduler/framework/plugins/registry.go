package plugins

import (
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/apiinstalled"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/clusteraffinity"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/clusterlocality"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/tainttoleration"
)

// NewPlugins builds all the scheduling plugins.
func NewPlugins() map[string]framework.Plugin {
	return map[string]framework.Plugin{
		clusteraffinity.Name: clusteraffinity.New(),
		tainttoleration.Name: tainttoleration.New(),
		apiinstalled.Name:    apiinstalled.New(),
		clusterlocality.Name: clusterlocality.New(),
	}
}
