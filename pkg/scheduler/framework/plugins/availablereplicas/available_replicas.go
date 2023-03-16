package availablereplicas

import (
	"context"
	"math"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "AvailableReplicas"
)

// AvailableReplicas is a scoring plugin that favors well-resourced clusters.
type AvailableReplicas struct {
	availableReplicasCache []workv1alpha2.TargetCluster
}

var _ framework.ScorePlugin = &AvailableReplicas{}

// New instantiates the AvailableReplicas plugin.
func New() (framework.Plugin, error) {
	return &AvailableReplicas{}, nil
}

// Name returns the plugin name.
func (p *AvailableReplicas) Name() string {
	return Name
}

// Score calculates the score on the candidate cluster.
// Get AvailableReplicas according to RB's ReplicaRequirements, and normalize it to the range of 0-100 interval,
// and convert it into a score.
func (p *AvailableReplicas) Score(ctx context.Context,
	spec *workv1alpha2.ResourceBindingSpec, clusters []*clusterv1alpha1.Cluster, name string) (int64, *framework.Result) {
	strategy := spec.Placement.ReplicaScheduling

	// If the replica division preference is 'Duplicated', ignore the information about cluster available resource.
	if strategy == nil || strategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		return framework.MinClusterScore, framework.NewResult(framework.Success)
	}
	if p.availableReplicasCache == nil {
		p.availableReplicasCache = estimatorclient.CalcAvailableReplicas(clusters, spec)
	}
	/*
	  (1) First find the minimum value minReplicas and maximum value maxReplicas of the availableReplicas
	  (2) The calculation coefficient is: k = (100-0)/(maxReplicas-minReplicas)
	  (3) Get the score normalized to the [0,100] interval: score = 0 + k * (curtAvailableReplicas-minReplicas)
	*/
	var curtAvailableReplicas int32
	var maxReplicas, minReplicas = int32(0), int32(math.MaxInt32)
	for _, target := range p.availableReplicasCache {
		if target.Replicas < minReplicas {
			minReplicas = target.Replicas
		}
		if target.Replicas > maxReplicas {
			maxReplicas = target.Replicas
		}
		if target.Name == name {
			curtAvailableReplicas = target.Replicas
		}
	}
	if maxReplicas == minReplicas {
		return framework.MinClusterScore, framework.NewResult(framework.Success)
	}

	k := float64(framework.MaxClusterScore-framework.MinClusterScore) / float64(maxReplicas-minReplicas)
	return framework.MinClusterScore + int64(k*float64(curtAvailableReplicas-minReplicas)), framework.NewResult(framework.Success)
}

// ScoreExtensions of the Score plugin.
func (p *AvailableReplicas) ScoreExtensions() framework.ScoreExtensions {
	p.availableReplicasCache = nil
	return nil
}
