package core

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	lister "github.com/karmada-io/karmada/pkg/generated/listers/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/runtime"
	"github.com/karmada-io/karmada/pkg/util"
)

// ScheduleAlgorithm is the interface that should be implemented to schedule a resource to the target clusters.
type ScheduleAlgorithm interface {
	Schedule(context.Context, *policyv1alpha1.Placement, *workv1alpha1.ObjectReference) (scheduleResult ScheduleResult, err error)
}

// ScheduleResult includes the clusters selected.
type ScheduleResult struct {
	SuggestedClusters []workv1alpha1.TargetCluster
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

func (g *genericScheduler) Schedule(ctx context.Context, placement *policyv1alpha1.Placement, resource *workv1alpha1.ObjectReference) (result ScheduleResult, err error) {
	clusterInfoSnapshot := g.schedulerCache.Snapshot()
	if clusterInfoSnapshot.NumOfClusters() == 0 {
		return result, fmt.Errorf("no clusters available to schedule")
	}

	feasibleClusters, err := g.findClustersThatFit(ctx, g.scheduleFramework, placement, resource, clusterInfoSnapshot)
	if err != nil {
		return result, fmt.Errorf("failed to findClustersThatFit: %v", err)
	}
	if len(feasibleClusters) == 0 {
		return result, fmt.Errorf("no clusters fit")
	}
	klog.V(4).Infof("feasible clusters found: %v", feasibleClusters)

	clustersScore, err := g.prioritizeClusters(ctx, g.scheduleFramework, placement, feasibleClusters)
	if err != nil {
		return result, fmt.Errorf("failed to prioritizeClusters: %v", err)
	}
	klog.V(4).Infof("feasible clusters scores: %v", clustersScore)

	clusters := g.selectClusters(clustersScore, placement.SpreadConstraints, feasibleClusters)

	clustersWithReplicas, err := g.assignReplicas(clusters, placement.ReplicaScheduling, resource)
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
	resource *workv1alpha1.ObjectReference,
	clusterInfo *cache.Snapshot) ([]*clusterv1alpha1.Cluster, error) {
	var out []*clusterv1alpha1.Cluster
	clusters := clusterInfo.GetReadyClusters()
	for _, c := range clusters {
		resMap := fwk.RunFilterPlugins(ctx, placement, resource, c.Cluster())
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
	placement *policyv1alpha1.Placement,
	clusters []*clusterv1alpha1.Cluster) (result framework.ClusterScoreList, err error) {
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

func (g *genericScheduler) selectClusters(clustersScore framework.ClusterScoreList, spreadConstraints []policyv1alpha1.SpreadConstraint, clusters []*clusterv1alpha1.Cluster) []*clusterv1alpha1.Cluster {
	if len(spreadConstraints) != 0 {
		return g.matchSpreadConstraints(clusters, spreadConstraints)
	}

	return clusters
}

func (g *genericScheduler) matchSpreadConstraints(clusters []*clusterv1alpha1.Cluster, spreadConstraints []policyv1alpha1.SpreadConstraint) []*clusterv1alpha1.Cluster {
	state := util.NewSpreadGroup()
	g.runSpreadConstraintsFilter(clusters, spreadConstraints, state)
	return g.calSpreadResult(state)
}

// Now support spread by cluster. More rules will be implemented later.
func (g *genericScheduler) runSpreadConstraintsFilter(clusters []*clusterv1alpha1.Cluster, spreadConstraints []policyv1alpha1.SpreadConstraint, spreadGroup *util.SpreadGroup) {
	for _, spreadConstraint := range spreadConstraints {
		spreadGroup.InitialGroupRecord(spreadConstraint)
		if spreadConstraint.SpreadByField == policyv1alpha1.SpreadByFieldCluster {
			g.groupByFieldCluster(clusters, spreadConstraint, spreadGroup)
		}
	}
}

func (g *genericScheduler) groupByFieldCluster(clusters []*clusterv1alpha1.Cluster, spreadConstraint policyv1alpha1.SpreadConstraint, spreadGroup *util.SpreadGroup) {
	for _, cluster := range clusters {
		clusterGroup := cluster.Name
		spreadGroup.GroupRecord[spreadConstraint][clusterGroup] = append(spreadGroup.GroupRecord[spreadConstraint][clusterGroup], cluster)
	}
}

func (g *genericScheduler) calSpreadResult(spreadGroup *util.SpreadGroup) []*clusterv1alpha1.Cluster {
	// TODO: now support single spread constraint
	if len(spreadGroup.GroupRecord) > 1 {
		return nil
	}

	return g.chooseSpreadGroup(spreadGroup)
}

func (g *genericScheduler) chooseSpreadGroup(spreadGroup *util.SpreadGroup) []*clusterv1alpha1.Cluster {
	var feasibleClusters []*clusterv1alpha1.Cluster
	for spreadConstraint, clusterGroups := range spreadGroup.GroupRecord {
		if spreadConstraint.SpreadByField == policyv1alpha1.SpreadByFieldCluster {
			if len(clusterGroups) < spreadConstraint.MinGroups {
				return nil
			}

			if len(clusterGroups) <= spreadConstraint.MaxGroups {
				for _, v := range clusterGroups {
					feasibleClusters = append(feasibleClusters, v...)
				}
				break
			}

			if spreadConstraint.MaxGroups > 0 && len(clusterGroups) > spreadConstraint.MaxGroups {
				var groups []string
				for group := range clusterGroups {
					groups = append(groups, group)
				}

				for i := 0; i < spreadConstraint.MaxGroups; i++ {
					feasibleClusters = append(feasibleClusters, clusterGroups[groups[i]]...)
				}
			}
		}
	}
	return feasibleClusters
}

func (g *genericScheduler) assignReplicas(clusters []*clusterv1alpha1.Cluster, replicaSchedulingStrategy *policyv1alpha1.ReplicaSchedulingStrategy, object *workv1alpha1.ObjectReference) ([]workv1alpha1.TargetCluster, error) {
	if len(clusters) == 0 {
		return nil, fmt.Errorf("no clusters available to schedule")
	}
	targetClusters := make([]workv1alpha1.TargetCluster, len(clusters))

	if object.Replicas > 0 && replicaSchedulingStrategy != nil {
		if replicaSchedulingStrategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
			for i, cluster := range clusters {
				targetClusters[i] = workv1alpha1.TargetCluster{Name: cluster.Name, Replicas: object.Replicas}
			}
			return targetClusters, nil
		}
	}

	for i, cluster := range clusters {
		targetClusters[i] = workv1alpha1.TargetCluster{Name: cluster.Name}
	}
	return targetClusters, nil
}
