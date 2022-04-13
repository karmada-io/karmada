package core

import (
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/core/spreadconstraint"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/runtime"
	"github.com/karmada-io/karmada/pkg/scheduler/metrics"
)

// ScheduleAlgorithm is the interface that should be implemented to schedule a resource to the target clusters.
type ScheduleAlgorithm interface {
	Schedule(context.Context, *policyv1alpha1.Placement, *workv1alpha2.ResourceBindingSpec) (scheduleResult ScheduleResult, err error)
}

// ScheduleResult includes the clusters selected.
type ScheduleResult struct {
	SuggestedClusters []workv1alpha2.TargetCluster
}

type genericScheduler struct {
	schedulerCache    cache.Cache
	scheduleFramework framework.Framework
}

// NewGenericScheduler creates a genericScheduler object.
func NewGenericScheduler(
	schedCache cache.Cache,
	plugins []string,
) ScheduleAlgorithm {
	return &genericScheduler{
		schedulerCache:    schedCache,
		scheduleFramework: runtime.NewFramework(plugins),
	}
}

func (g *genericScheduler) Schedule(ctx context.Context, placement *policyv1alpha1.Placement, spec *workv1alpha2.ResourceBindingSpec) (result ScheduleResult, err error) {
	clusterInfoSnapshot := g.schedulerCache.Snapshot()
	if clusterInfoSnapshot.NumOfClusters() == 0 {
		return result, fmt.Errorf("no clusters available to schedule")
	}

	feasibleClusters, err := g.findClustersThatFit(ctx, g.scheduleFramework, placement, &spec.Resource, clusterInfoSnapshot)
	if err != nil {
		return result, fmt.Errorf("failed to findClustersThatFit: %v", err)
	}
	if len(feasibleClusters) == 0 {
		return result, fmt.Errorf("no clusters fit")
	}
	klog.V(4).Infof("feasible clusters found: %v", feasibleClusters)

	clustersScore, err := g.prioritizeClusters(ctx, g.scheduleFramework, placement, spec, feasibleClusters)
	if err != nil {
		return result, fmt.Errorf("failed to prioritizeClusters: %v", err)
	}
	klog.V(4).Infof("feasible clusters scores: %v", clustersScore)

	clusters, err := g.selectClusters(clustersScore, placement, spec)
	if err != nil {
		return result, fmt.Errorf("failed to select clusters: %v", err)
	}
	klog.V(4).Infof("selected clusters: %v", clusters)

	clustersWithReplicas, err := g.assignReplicas(clusters, placement.ReplicaScheduling, spec)
	if err != nil {
		return result, fmt.Errorf("failed to assignReplicas: %v", err)
	}
	result.SuggestedClusters = clustersWithReplicas

	return result, nil
}

// findClustersThatFit finds the clusters that are fit for the placement based on running the filter plugins.
func (g *genericScheduler) findClustersThatFit(
	ctx context.Context,
	fwk framework.Framework,
	placement *policyv1alpha1.Placement,
	resource *workv1alpha2.ObjectReference,
	clusterInfo *cache.Snapshot) ([]*clusterv1alpha1.Cluster, error) {
	defer metrics.ScheduleStep(metrics.ScheduleStepFilter, time.Now())

	var out []*clusterv1alpha1.Cluster
	clusters := clusterInfo.GetReadyClusters()
	for _, c := range clusters {
		if result := fwk.RunFilterPlugins(ctx, placement, resource, c.Cluster()); !result.IsSuccess() {
			klog.V(4).Infof("cluster %q is not fit, reason: %v", c.Cluster().Name, result.AsError())
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
	placement *policyv1alpha1.Placement,
	spec *workv1alpha2.ResourceBindingSpec,
	clusters []*clusterv1alpha1.Cluster) (result framework.ClusterScoreList, err error) {
	defer metrics.ScheduleStep(metrics.ScheduleStepScore, time.Now())

	scoresMap, err := fwk.RunScorePlugins(ctx, placement, spec, clusters)
	if err != nil {
		return result, err
	}

	if klog.V(4).Enabled() {
		for plugin, nodeScoreList := range scoresMap {
			klog.Infof("Plugin %s scores on %v/%v => %v", plugin, spec.Resource.Namespace, spec.Resource.Name, nodeScoreList)
		}
	}

	result = make(framework.ClusterScoreList, len(clusters))
	for i := range clusters {
		result[i] = framework.ClusterScore{Cluster: clusters[i], Score: 0}
		for j := range scoresMap {
			result[i].Score += scoresMap[j][i].Score
		}
	}

	return result, nil
}

func (g *genericScheduler) selectClusters(clustersScore framework.ClusterScoreList,
	placement *policyv1alpha1.Placement, spec *workv1alpha2.ResourceBindingSpec) ([]*clusterv1alpha1.Cluster, error) {
	defer metrics.ScheduleStep(metrics.ScheduleStepSelect, time.Now())

	groupClustersInfo := spreadconstraint.GroupClustersWithScore(clustersScore, placement, spec)

	return spreadconstraint.SelectBestClusters(placement, groupClustersInfo)
}

func (g *genericScheduler) assignReplicas(
	clusters []*clusterv1alpha1.Cluster,
	replicaSchedulingStrategy *policyv1alpha1.ReplicaSchedulingStrategy,
	spec *workv1alpha2.ResourceBindingSpec,
) ([]workv1alpha2.TargetCluster, error) {
	defer metrics.ScheduleStep(metrics.ScheduleStepAssignReplicas, time.Now())

	if len(clusters) == 0 {
		return nil, fmt.Errorf("no clusters available to schedule")
	}

	if spec.Replicas <= 0 {
		targetClusters := make([]workv1alpha2.TargetCluster, len(clusters))
		for i, cluster := range clusters {
			targetClusters[i] = workv1alpha2.TargetCluster{Name: cluster.Name}
		}
		return targetClusters, nil
	}

	if assigner, ok := GetAssignReplica(replicaSchedulingStrategy); ok {
		return assigner.AssignReplica(spec, replicaSchedulingStrategy, clusters)
	}

	return nil, fmt.Errorf("undefined replica scheduling type")
}
