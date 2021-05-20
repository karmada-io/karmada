package tainttoleration

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"

	cluster "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "TaintToleration"
)

// TaintToleration is a plugin that checks if a propagation policy tolerates a cluster's taints.
type TaintToleration struct{}

var _ framework.FilterPlugin = &TaintToleration{}
var _ framework.ScorePlugin = &TaintToleration{}

// New instantiates the TaintToleration plugin.
func New() framework.Plugin {
	return &TaintToleration{}
}

// Name returns the plugin name.
func (p *TaintToleration) Name() string {
	return Name
}

// Filter checks if the given tolerations in placement tolerate cluster's taints.
func (p *TaintToleration) Filter(ctx context.Context, placement *v1alpha1.Placement, cluster *cluster.Cluster) *framework.Result {
	filterPredicate := func(t *v1.Taint) bool {
		// now only interested in NoSchedule taint which means do not allow new resource to schedule onto the cluster unless they tolerate the taint
		// todo: supprot NoExecute taint
		return t.Effect == v1.TaintEffectNoSchedule
	}

	taint, isUntolerated := v1helper.FindMatchingUntoleratedTaint(cluster.Spec.Taints, placement.ClusterTolerations, filterPredicate)
	if !isUntolerated {
		return framework.NewResult(framework.Success)
	}

	return framework.NewResult(framework.Unschedulable, fmt.Sprintf("cluster had taint {%s: %s}, that the propagation policy didn't tolerate",
		taint.Key, taint.Value))
}

// Score calculates the score on the candidate cluster.
func (p *TaintToleration) Score(ctx context.Context, placement *v1alpha1.Placement, cluster *cluster.Cluster) (float64, *framework.Result) {
	return 0, framework.NewResult(framework.Success)
}
