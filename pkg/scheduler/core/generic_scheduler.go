package core

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"

	clusterapi "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	lister "github.com/karmada-io/karmada/pkg/generated/listers/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/runtime"
)

// ScheduleAlgorithm is the interface that should be implemented to schedule a resource to the target clusters.
type ScheduleAlgorithm interface {
	Schedule(context.Context, *v1alpha1.PropagationBinding) (scheduleResult ScheduleResult, err error)
}

// ScheduleResult includes the clusters selected.
type ScheduleResult struct {
	SuggestedClusters []string
}

type genericScheduler struct {
	schedulerCache cache.Cache
	// TODO: move it into schedulerCache
	policyLister      lister.PropagationPolicyLister
	scheduleFramework framework.Framework
}

// NewGenericScheduler creates a genericScheduler object.
func NewGenericScheduler(
	schedCache cache.Cache,
	policyLister lister.PropagationPolicyLister,
	plugins []string,
) ScheduleAlgorithm {
	return &genericScheduler{
		schedulerCache:    schedCache,
		policyLister:      policyLister,
		scheduleFramework: runtime.NewFramework(plugins),
	}
}

func (g *genericScheduler) Schedule(ctx context.Context, binding *v1alpha1.PropagationBinding) (result ScheduleResult, err error) {
	klog.V(4).Infof("Scheduling %s/%s", binding.Namespace, binding.Name)

	clusterInfoSnapshot := g.schedulerCache.Snapshot()

	if clusterInfoSnapshot.NumOfClusters() == 0 {
		return result, fmt.Errorf("no clusters available to schedule")
	}

	var policyName string
	if len(binding.OwnerReferences) > 0 {
		owner := binding.OwnerReferences[0]
		if owner.APIVersion == v1alpha1.SchemeGroupVersion.String() && owner.Kind == "PropagationPolicy" {
			policyName = owner.Name
		}
	}

	policy, err := g.policyLister.PropagationPolicies(binding.Namespace).Get(policyName)
	if err != nil {
		return result, fmt.Errorf("no propagation policy found for <%s/%s>: %v", binding.Namespace, binding.Name, err)
	}

	feasibleClusters, err := g.findClustersThatFit(ctx, g.scheduleFramework, &policy.Spec.Placement, clusterInfoSnapshot)
	if err != nil {
		return result, fmt.Errorf("failed findClustersThatFit for <%s/%s>: %v", binding.Namespace, binding.Name, err)
	}
	if len(feasibleClusters) == 0 {
		return result, fmt.Errorf("no clusters fit")
	}
	klog.V(4).Infof("feasible clusters found for <%s/%s>: %v", binding.Namespace, binding.Name, feasibleClusters)

	clustersScore, err := g.prioritizeClusters(ctx, g.scheduleFramework, &policy.Spec.Placement, feasibleClusters)
	if err != nil {
		return result, fmt.Errorf("failed prioritizeClusters for <%s/%s>: %v", binding.Namespace, binding.Name, err)
	}
	klog.V(4).Infof("feasible clusters scores for <%s/%s>: %v", binding.Namespace, binding.Name, clustersScore)

	clusters := g.selectClusters(clustersScore)
	result.SuggestedClusters = clusters

	return result, nil
}

// findClustersThatFit finds the clusters that are fit for the placement based on running the filter plugins.
func (g *genericScheduler) findClustersThatFit(
	ctx context.Context,
	fwk framework.Framework,
	placement *v1alpha1.Placement,
	clusterInfo *cache.Snapshot) ([]*clusterapi.Cluster, error) {
	var out []*clusterapi.Cluster
	clusters := clusterInfo.GetReadyClusters()
	for _, c := range clusters {
		resMap := fwk.RunFilterPlugins(ctx, placement, c.Cluster())
		res := resMap.Merge()
		if !res.IsSuccess() {
			klog.V(4).Infof("cluster %q is not fit", c.Cluster().Name)
		} else {
			out = append(out, c.Cluster())
		}
	}

	return out, nil
}

// prioritizeClusters prioritize the clusters by running the score plugins.
func (g *genericScheduler) prioritizeClusters(
	ctx context.Context,
	fwk framework.Framework,
	placement *v1alpha1.Placement,
	clusters []*clusterapi.Cluster) (result framework.ClusterScoreList, err error) {
	scoresMap, err := fwk.RunScorePlugins(ctx, placement, clusters)
	if err != nil {
		return result, err
	}

	result = make(framework.ClusterScoreList, len(clusters))
	for i := range clusters {
		result[i] = framework.ClusterScore{Name: clusters[i].Name, Score: 0}
		for j := range scoresMap {
			result[i].Score += scoresMap[j][i].Score
		}
	}

	return result, nil
}

// TODO: update the algorithms
func (g *genericScheduler) selectClusters(clustersScore framework.ClusterScoreList) []string {
	out := make([]string, len(clustersScore))
	for i := range clustersScore {
		out[i] = clustersScore[i].Name
	}
	return out
}
