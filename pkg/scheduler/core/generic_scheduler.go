package core

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/runtime"
	"github.com/karmada-io/karmada/pkg/scheduler/metrics"
	"github.com/karmada-io/karmada/pkg/util"
)

// ScheduleAlgorithm is the interface that should be implemented to schedule a resource to the target clusters.
type ScheduleAlgorithm interface {
	Schedule(context.Context, *policyv1alpha1.Placement, *workv1alpha2.ResourceBindingSpec) (scheduleResult ScheduleResult, err error)
	ReSchedule(context.Context, *policyv1alpha1.Placement, *workv1alpha2.ResourceBindingSpec) (scheduleResult ScheduleResult, err error)
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

	clustersScore, err := g.prioritizeClusters(ctx, g.scheduleFramework, placement, feasibleClusters)
	if err != nil {
		return result, fmt.Errorf("failed to prioritizeClusters: %v", err)
	}
	klog.V(4).Infof("feasible clusters scores: %v", clustersScore)

	clusters := g.selectClusters(clustersScore, placement.SpreadConstraints, feasibleClusters)

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
	defer metrics.ScheduleStep(metrics.ScheduleStepScore, time.Now())

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
	defer metrics.ScheduleStep(metrics.ScheduleStepSelect, time.Now())

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

func (g *genericScheduler) assignReplicas(
	clusters []*clusterv1alpha1.Cluster,
	replicaSchedulingStrategy *policyv1alpha1.ReplicaSchedulingStrategy,
	object *workv1alpha2.ResourceBindingSpec,
) ([]workv1alpha2.TargetCluster, error) {
	defer metrics.ScheduleStep(metrics.ScheduleStepAssignReplicas, time.Now())
	if len(clusters) == 0 {
		return nil, fmt.Errorf("no clusters available to schedule")
	}
	targetClusters := make([]workv1alpha2.TargetCluster, len(clusters))

	if object.Replicas > 0 && replicaSchedulingStrategy != nil {
		switch replicaSchedulingStrategy.ReplicaSchedulingType {
		// 1. Duplicated Scheduling
		case policyv1alpha1.ReplicaSchedulingTypeDuplicated:
			for i, cluster := range clusters {
				targetClusters[i] = workv1alpha2.TargetCluster{Name: cluster.Name, Replicas: object.Replicas}
			}
			return targetClusters, nil
		// 2. Divided Scheduling
		case policyv1alpha1.ReplicaSchedulingTypeDivided:
			switch replicaSchedulingStrategy.ReplicaDivisionPreference {
			// 2.1 Weighted Scheduling
			case policyv1alpha1.ReplicaDivisionPreferenceWeighted:
				// If ReplicaDivisionPreference is set to "Weighted" and WeightPreference is not set,
				// scheduler will weight all clusters averagely.
				if replicaSchedulingStrategy.WeightPreference == nil {
					replicaSchedulingStrategy.WeightPreference = getDefaultWeightPreference(clusters)
				}
				// 2.1.1 Dynamic Weighted Scheduling (by resource)
				if len(replicaSchedulingStrategy.WeightPreference.DynamicWeight) != 0 {
					return divideReplicasByDynamicWeight(clusters, replicaSchedulingStrategy.WeightPreference.DynamicWeight, object)
				}
				// 2.1.2 Static Weighted Scheduling
				return divideReplicasByStaticWeight(clusters, replicaSchedulingStrategy.WeightPreference.StaticWeightList, object.Replicas)
			// 2.2 Aggregated scheduling (by resource)
			case policyv1alpha1.ReplicaDivisionPreferenceAggregated:
				return divideReplicasByResource(clusters, object, policyv1alpha1.ReplicaDivisionPreferenceAggregated)
			default:
				return nil, fmt.Errorf("undefined replica division preference: %s", replicaSchedulingStrategy.ReplicaDivisionPreference)
			}
		default:
			return nil, fmt.Errorf("undefined replica scheduling type: %s", replicaSchedulingStrategy.ReplicaSchedulingType)
		}
	}

	for i, cluster := range clusters {
		targetClusters[i] = workv1alpha2.TargetCluster{Name: cluster.Name}
	}
	return targetClusters, nil
}

func (g *genericScheduler) ReSchedule(ctx context.Context, placement *policyv1alpha1.Placement,
	spec *workv1alpha2.ResourceBindingSpec) (result ScheduleResult, err error) {
	readyClusters := g.schedulerCache.Snapshot().GetReadyClusterNames()
	totalClusters := util.ConvertToClusterNames(spec.Clusters)

	reservedClusters := calcReservedCluster(totalClusters, readyClusters)
	availableClusters := calcAvailableCluster(totalClusters, readyClusters)

	candidateClusters := sets.NewString()
	for clusterName := range availableClusters {
		clusterObj := g.schedulerCache.Snapshot().GetCluster(clusterName)
		if clusterObj == nil {
			return result, fmt.Errorf("failed to get clusterObj by clusterName: %s", clusterName)
		}

		resMap := g.scheduleFramework.RunFilterPlugins(ctx, placement, &spec.Resource, clusterObj.Cluster())
		res := resMap.Merge()
		if !res.IsSuccess() {
			klog.V(4).Infof("cluster %q is not fit", clusterName)
		} else {
			candidateClusters.Insert(clusterName)
		}
	}

	klog.V(4).Infof("Reserved bindingClusters : %v", reservedClusters.List())
	klog.V(4).Infof("Candidate bindingClusters: %v", candidateClusters.List())

	// TODO: should schedule as much as possible?
	deltaLen := len(spec.Clusters) - len(reservedClusters)
	if len(candidateClusters) < deltaLen {
		// for ReplicaSchedulingTypeDivided, we will try to migrate replicas to the other health clusters
		if placement.ReplicaScheduling == nil || placement.ReplicaScheduling.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
			klog.Warningf("ignore reschedule binding as insufficient available cluster")
			return ScheduleResult{}, nil
		}
	}

	// TODO: check if the final result meets the spread constraints.
	targetClusters := reservedClusters
	clusterList := candidateClusters.List()
	for i := 0; i < deltaLen && i < len(candidateClusters); i++ {
		targetClusters.Insert(clusterList[i])
	}

	var reScheduleResult []workv1alpha2.TargetCluster
	for cluster := range targetClusters {
		reScheduleResult = append(reScheduleResult, workv1alpha2.TargetCluster{Name: cluster})
	}

	return ScheduleResult{reScheduleResult}, nil
}
