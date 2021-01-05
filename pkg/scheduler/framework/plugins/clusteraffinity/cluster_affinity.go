package clusteraffinity

import (
	"context"

	membercluster "github.com/karmada-io/karmada/pkg/apis/membercluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/propagationstrategy/v1alpha1"
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

func New() framework.Plugin {
	return &ClusterAffinity{}
}

func (p *ClusterAffinity) Name() string {
	return Name
}

// Filter checks if the cluster matched the placement cluster affinity constraint.
func (p *ClusterAffinity) Filter(ctx context.Context, placement *v1alpha1.Placement, cluster *membercluster.MemberCluster) *framework.Result {
	for _, clusterName := range placement.ClusterAffinity.ExcludeClusters {
		if clusterName == cluster.Name {
			return framework.NewResult(framework.Unschedulable, "cluster is excluded")
		}
	}

	if len(placement.ClusterAffinity.ClusterNames) > 0 {
		for _, clusterName := range placement.ClusterAffinity.ClusterNames {
			if clusterName == cluster.Name {
				return framework.NewResult(framework.Success)
			}
		}
		return framework.NewResult(framework.Unschedulable, "cluster is not specified")
	}

	// If no clusters specified and it is not excluded, mark it matched
	return nil
}

func (p *ClusterAffinity) Score(ctx context.Context, placement *v1alpha1.Placement, cluster *membercluster.MemberCluster) (float64, *framework.Result) {
	return 0, nil
}
