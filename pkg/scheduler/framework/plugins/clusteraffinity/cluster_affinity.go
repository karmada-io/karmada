package clusteraffinity

import (
	"context"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
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
func (p *ClusterAffinity) Filter(ctx context.Context,
	bindingSpec *workv1alpha2.ResourceBindingSpec, cluster *clusterv1alpha1.Cluster) *framework.Result {
	affinity := bindingSpec.Placement.ClusterAffinity
	if affinity != nil {
		if util.ClusterMatches(cluster, *affinity) {
			return framework.NewResult(framework.Success)
		}
		return framework.NewResult(framework.Unschedulable, "cluster(s) didn't match the placement cluster affinity constraint")
	}

	// If no clusters specified and it is not excluded, mark it matched
	return framework.NewResult(framework.Success)
}

// Score calculates the score on the candidate cluster.
func (p *ClusterAffinity) Score(ctx context.Context,
	spec *workv1alpha2.ResourceBindingSpec, cluster *clusterv1alpha1.Cluster) (int64, *framework.Result) {
	return framework.MinClusterScore, framework.NewResult(framework.Success)
}

// ScoreExtensions of the Score plugin.
func (p *ClusterAffinity) ScoreExtensions() framework.ScoreExtensions {
	return p
}

// NormalizeScore normalizes the score for each candidate cluster.
func (p *ClusterAffinity) NormalizeScore(ctx context.Context, scores framework.ClusterScoreList) *framework.Result {
	return framework.NewResult(framework.Success)
}
