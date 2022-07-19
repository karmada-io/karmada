package plugins

import (
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/apienablement"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/clusteraffinity"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/clusterlocality"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/spreadconstraint"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/plugins/tainttoleration"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/runtime"
)

// NewInTreeRegistry builds the registry with all the in-tree plugins.
func NewInTreeRegistry() runtime.Registry {
	return runtime.Registry{
		apienablement.Name:    apienablement.New,
		tainttoleration.Name:  tainttoleration.New,
		clusteraffinity.Name:  clusteraffinity.New,
		spreadconstraint.Name: spreadconstraint.New,
		clusterlocality.Name:  clusterlocality.New,
	}
}
