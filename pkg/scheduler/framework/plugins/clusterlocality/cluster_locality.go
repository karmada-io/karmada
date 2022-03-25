package clusterlocality

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
	Name = "ClusterLocality"
)

// ClusterLocality is a score plugin that favors cluster that already have requested.
type ClusterLocality struct{}

var _ framework.ScorePlugin = &ClusterLocality{}

// New instantiates the clusteraffinity plugin.
func New() framework.Plugin {
	return &ClusterLocality{}
}

// Name returns the plugin name.
func (p *ClusterLocality) Name() string {
	return Name
}

// Score calculates the score on the candidate cluster.
// if cluster object is exist in resourceBinding.Spec.Clusters, Score is 100, otherwise it is 0.
func (p *ClusterLocality) Score(ctx context.Context, placement *policyv1alpha1.Placement,
	spec *workv1alpha2.ResourceBindingSpec, cluster *clusterv1alpha1.Cluster) (int64, *framework.Result) {
	if len(spec.Clusters) == 0 {
		return framework.MinClusterScore, framework.NewResult(framework.Success)
	}

	replicas := util.GetSumOfReplicas(spec.Clusters)
	if replicas <= 0 {
		return framework.MinClusterScore, framework.NewResult(framework.Success)
	}

	if isClusterScheduled(cluster.Name, spec.Clusters) {
		return framework.MaxClusterScore, framework.NewResult(framework.Success)
	}

	return framework.MinClusterScore, framework.NewResult(framework.Success)
}

// ScoreExtensions of the Score plugin.
func (p *ClusterLocality) ScoreExtensions() framework.ScoreExtensions {
	return nil
}

func isClusterScheduled(candidate string, schedulerClusters []workv1alpha2.TargetCluster) bool {
	for _, cluster := range schedulerClusters {
		if candidate == cluster.Name {
			return true
		}
	}

	return false
}
