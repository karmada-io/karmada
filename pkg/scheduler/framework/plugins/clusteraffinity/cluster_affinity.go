package clusteraffinity

import (
	"context"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "ClusterAffinity"
)

// ClusterAffinity is a plugin that checks if a resource selector matches the cluster label.
type ClusterAffinity struct{}

var _ framework.FilterPlugin = &ClusterAffinity{}
var _ framework.ScorePlugin = &ClusterAffinity{}

// New instantiates the clusteraffinity plugin.
func New() (framework.Plugin, error) {
	return &ClusterAffinity{}, nil
}

// Name returns the plugin name.
func (p *ClusterAffinity) Name() string {
	return Name
}

// Filter checks if the cluster matched the placement cluster affinity constraint.
func (p *ClusterAffinity) Filter(
	_ context.Context,
	bindingSpec *workv1alpha2.ResourceBindingSpec,
	bindingStatus *workv1alpha2.ResourceBindingStatus,
	cluster *clusterv1alpha1.Cluster,
) *framework.Result {
	var affinity *policyv1alpha1.ClusterAffinity
	if bindingSpec.Placement.ClusterAffinity != nil {
		affinity = bindingSpec.Placement.ClusterAffinity
	} else {
		for index, term := range bindingSpec.Placement.ClusterAffinities {
			if term.AffinityName == bindingStatus.SchedulerObservedAffinityName {
				affinity = &bindingSpec.Placement.ClusterAffinities[index].ClusterAffinity
				break
			}
		}
	}

	if affinity != nil {
		if util.ClusterMatches(cluster, *affinity) {
			return framework.NewResult(framework.Success)
		}
		return framework.NewResult(framework.Unschedulable, "cluster(s) did not match the placement cluster affinity constraint")
	}

	// If no clusters specified and it is not excluded, mark it matched
	return framework.NewResult(framework.Success)
}

// Score calculates the score on the candidate cluster.
func (p *ClusterAffinity) Score(_ context.Context,
	_ *workv1alpha2.ResourceBindingSpec, _ *clusterv1alpha1.Cluster) (int64, *framework.Result) {
	return framework.MinClusterScore, framework.NewResult(framework.Success)
}

// ScoreExtensions of the Score plugin.
func (p *ClusterAffinity) ScoreExtensions() framework.ScoreExtensions {
	return p
}

// NormalizeScore normalizes the score for each candidate cluster.
func (p *ClusterAffinity) NormalizeScore(_ context.Context, _ framework.ClusterScoreList) *framework.Result {
	return framework.NewResult(framework.Success)
}
