package clusteraffinity

import (
	"context"

	cluster "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
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
func New() framework.Plugin {
	return &ClusterAffinity{}
}

// Name returns the plugin name.
func (p *ClusterAffinity) Name() string {
	return Name
}

// Filter checks if the cluster matched the placement cluster affinity constraint.
func (p *ClusterAffinity) Filter(ctx context.Context, placement *v1alpha1.Placement, cluster *cluster.Cluster) *framework.Result {
	affinity := placement.ClusterAffinity
	if affinity != nil {
		for _, clusterName := range affinity.ExcludeClusters {
			if clusterName == cluster.Name {
				return framework.NewResult(framework.Unschedulable, "cluster is excluded")
			}
		}

		if len(affinity.ClusterNames) > 0 {
			for _, clusterName := range affinity.ClusterNames {
				if clusterName == cluster.Name {
					return framework.NewResult(framework.Success)
				}
			}
			return framework.NewResult(framework.Unschedulable, "cluster is not specified")
		}
	}

	// If no clusters specified and it is not excluded, mark it matched
	return nil
}

// Score calculates the score on the candidate cluster.
func (p *ClusterAffinity) Score(ctx context.Context, placement *v1alpha1.Placement, cluster *cluster.Cluster) (float64, *framework.Result) {
	return 0, nil
}
