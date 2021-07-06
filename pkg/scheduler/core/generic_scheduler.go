package core

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	lister "github.com/karmada-io/karmada/pkg/generated/listers/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/runtime"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// ScheduleAlgorithm is the interface that should be implemented to schedule a resource to the target clusters.
type ScheduleAlgorithm interface {
	Schedule(context.Context, *policyv1alpha1.Placement, *workv1alpha1.ObjectReference) (scheduleResult ScheduleResult, err error)
	ScaleSchedule(context.Context, *policyv1alpha1.Placement, *workv1alpha1.ObjectReference, []workv1alpha1.TargetCluster) (scheduleResult ScheduleResult, err error)
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

	clustersWithReplicase, err := g.assignReplicas(clusters, placement.ReplicaScheduling, resource)
	if err != nil {
		return result, fmt.Errorf("failed to assignReplicas: %v", err)
	}
	result.SuggestedClusters = clustersWithReplicase

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
	if object.Replicas == 0 {
		targetClusters := make([]workv1alpha1.TargetCluster, len(clusters))
		for i, cluster := range clusters {
			targetClusters[i] = workv1alpha1.TargetCluster{Name: cluster.Name}
		}
		return targetClusters, nil
	}
	if replicaSchedulingStrategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided {
		if replicaSchedulingStrategy.ReplicaDivisionPreference == policyv1alpha1.ReplicaDivisionPreferenceWeighted {
			return g.divideReplicasWighted(clusters, replicaSchedulingStrategy.WeightPreference.StaticWeightList, object.Replicas)
		}
		return g.divideReplicasAggregated(clusters, object)
	}
	targetClusters := make([]workv1alpha1.TargetCluster, len(clusters))
	for i, cluster := range clusters {
		targetClusters[i] = workv1alpha1.TargetCluster{Name: cluster.Name, Replicas: object.Replicas}
	}
	return targetClusters, nil
}

func (g *genericScheduler) divideReplicasWighted(clusters []*clusterv1alpha1.Cluster, staticWeightList []policyv1alpha1.StaticClusterWeight, replicas int32) ([]workv1alpha1.TargetCluster, error) {
	weightSum := int64(0)
	matchClusters := make(map[string]int64)
	desireReplicaInfos := make(map[string]int64)

	for _, cluster := range clusters {
		for _, staticWeightRule := range staticWeightList {
			if util.ClusterMatches(cluster, staticWeightRule.TargetCluster) {
				weightSum += staticWeightRule.Weight
				matchClusters[cluster.Name] = staticWeightRule.Weight
				break
			}
		}
	}

	allocatedReplicas := int32(0)
	for clusterName, weight := range matchClusters {
		desireReplicaInfos[clusterName] = weight * int64(replicas) / weightSum
		allocatedReplicas += int32(desireReplicaInfos[clusterName])
	}

	if remainReplicas := replicas - allocatedReplicas; remainReplicas > 0 {
		sortedClusters := helper.SortClusterByWeight(matchClusters)
		for i := 0; remainReplicas > 0; i++ {
			desireReplicaInfos[sortedClusters[i].ClusterName]++
			remainReplicas--
			if i == len(desireReplicaInfos) {
				i = 0
			}
		}
	}

	for _, cluster := range clusters {
		if _, exist := matchClusters[cluster.Name]; !exist {
			desireReplicaInfos[cluster.Name] = 0
		}
	}

	targetClusters := make([]workv1alpha1.TargetCluster, len(desireReplicaInfos))
	i := 0
	for key, value := range desireReplicaInfos {
		targetClusters[i] = workv1alpha1.TargetCluster{Name: key, Replicas: int32(value)}
		i++
	}
	return targetClusters, nil
}

// TargetClustersList is a slice of TargetCluster that implements sort.Interface to sort by Value.
type TargetClustersList []workv1alpha1.TargetCluster

func (a TargetClustersList) Len() int           { return len(a) }
func (a TargetClustersList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a TargetClustersList) Less(i, j int) bool { return a[i].Replicas > a[j].Replicas }

func (g *genericScheduler) divideReplicasAggregated(clusters []*clusterv1alpha1.Cluster, object *workv1alpha1.ObjectReference) ([]workv1alpha1.TargetCluster, error) {
	for _, value := range object.ReplicaResourceRequirements {
		if value.Value() > 0 {
			return g.divideReplicasAggregatedWithResourceRequirements(clusters, object)
		}
	}
	return g.divideReplicasAggregatedWithoutResourceRequirements(clusters, object)
}

func (g *genericScheduler) divideReplicasAggregatedWithResourceRequirements(clusters []*clusterv1alpha1.Cluster,
	object *workv1alpha1.ObjectReference) ([]workv1alpha1.TargetCluster, error) {
	clusterAvailableReplicas := g.calAvailableReplicas(clusters, object.ReplicaResourceRequirements)
	return g.divideReplicasAggregatedWithClusterReplicas(clusterAvailableReplicas, object.Replicas)
}

func (g *genericScheduler) divideReplicasAggregatedWithClusterReplicas(clusterAvailableReplicas []workv1alpha1.TargetCluster, replicas int32) ([]workv1alpha1.TargetCluster, error) {
	clustersNum := 0
	clustersMaxReplicas := int32(0)
	for _, clusterInfo := range clusterAvailableReplicas {
		clustersNum++
		clustersMaxReplicas += clusterInfo.Replicas
		if clustersMaxReplicas >= replicas {
			break
		}
	}
	if clustersMaxReplicas < replicas {
		return nil, fmt.Errorf("clusters resources are not enough to schedule, max %v replicas are support", clustersMaxReplicas)
	}

	desireReplicaInfos := make(map[string]int32)
	allocatedReplicas := int32(0)
	for i, clusterInfo := range clusterAvailableReplicas {
		if i >= clustersNum {
			desireReplicaInfos[clusterInfo.Name] = 0
		}
		desireReplicaInfos[clusterInfo.Name] = clusterInfo.Replicas * replicas / clustersMaxReplicas
		allocatedReplicas += desireReplicaInfos[clusterInfo.Name]
	}

	if remainReplicas := replicas - allocatedReplicas; remainReplicas > 0 {
		for i := 0; remainReplicas > 0; i++ {
			desireReplicaInfos[clusterAvailableReplicas[i].Name]++
			remainReplicas--
			if i == clustersNum {
				i = 0
			}
		}
	}

	targetClusters := make([]workv1alpha1.TargetCluster, len(clusterAvailableReplicas))
	i := 0
	for key, value := range desireReplicaInfos {
		targetClusters[i] = workv1alpha1.TargetCluster{Name: key, Replicas: value}
		i++
	}
	return targetClusters, nil
}

func (g *genericScheduler) calAvailableReplicas(clusters []*clusterv1alpha1.Cluster, replicaResourceRequirements corev1.ResourceList) []workv1alpha1.TargetCluster {
	availableTargetClusters := make([]workv1alpha1.TargetCluster, len(clusters))
	for i, cluster := range clusters {
		maxReplicas := g.calClusterAvailableReplicas(cluster, replicaResourceRequirements)
		availableTargetClusters[i] = workv1alpha1.TargetCluster{Name: cluster.Name, Replicas: maxReplicas}
	}
	sort.Sort(TargetClustersList(availableTargetClusters))
	return availableTargetClusters
}

func (g *genericScheduler) calClusterAvailableReplicas(cluster *clusterv1alpha1.Cluster, resourcePerReplicas corev1.ResourceList) int32 {
	ReplicasResult := int64(0)
	calFlag := false
	resourceSummary := cluster.Status.ResourceSummary
	for key, value := range resourcePerReplicas {
		allocatable, ok := resourceSummary.Allocatable[key]
		if !ok {
			return 0
		}
		allocated, ok := resourceSummary.Allocated[key]
		if ok {
			allocatable.Sub(allocated)
		}
		allocating, ok := resourceSummary.Allocating[key]
		if ok {
			allocatable.Sub(allocating)
		}
		requestInt := value.Value()
		freeInt := allocatable.Value()
		if freeInt <= 0 {
			return 0
		}
		if requestInt != 0 {
			maxReplicas := freeInt / requestInt
			if !calFlag {
				ReplicasResult = maxReplicas
				calFlag = true
			} else if ReplicasResult > maxReplicas {
				ReplicasResult = maxReplicas
			}
		}
	}
	return int32(ReplicasResult)
}

func (g *genericScheduler) divideReplicasAggregatedWithoutResourceRequirements(clusters []*clusterv1alpha1.Cluster,
	object *workv1alpha1.ObjectReference) ([]workv1alpha1.TargetCluster, error) {
	targetClusters := make([]workv1alpha1.TargetCluster, len(clusters))
	for i, clusterInfo := range clusters {
		targetClusters[i] = workv1alpha1.TargetCluster{Name: clusterInfo.Name, Replicas: 0}
	}
	targetClusters[0] = workv1alpha1.TargetCluster{Name: clusters[0].Name, Replicas: object.Replicas}
	return targetClusters, nil
}

func (g *genericScheduler) ScaleSchedule(ctx context.Context, placement *policyv1alpha1.Placement,
	object *workv1alpha1.ObjectReference, targetClusters []workv1alpha1.TargetCluster) (result ScheduleResult, err error) {
	if object.Replicas == 0 {
		newTargetClusters := make([]workv1alpha1.TargetCluster, len(targetClusters))
		for i, cluster := range targetClusters {
			newTargetClusters[i] = workv1alpha1.TargetCluster{Name: cluster.Name}
		}
		result.SuggestedClusters = newTargetClusters
		return result, nil
	}
	if placement.ReplicaScheduling.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided {
		preSelectedClusters := g.getPreSelected(targetClusters)
		if placement.ReplicaScheduling.ReplicaDivisionPreference == policyv1alpha1.ReplicaDivisionPreferenceWeighted {
			clustersWithReplicase, err := g.divideReplicasWighted(preSelectedClusters, placement.ReplicaScheduling.WeightPreference.StaticWeightList, object.Replicas)
			if err != nil {
				return result, fmt.Errorf("failed to assignReplicas with Weight: %v", err)
			}
			result.SuggestedClusters = clustersWithReplicase
			return result, nil
		}
	}
	newTargetClusters := make([]workv1alpha1.TargetCluster, len(targetClusters))
	for i, cluster := range targetClusters {
		newTargetClusters[i] = workv1alpha1.TargetCluster{Name: cluster.Name, Replicas: object.Replicas}
	}
	result.SuggestedClusters = newTargetClusters
	return result, nil
}

func (g *genericScheduler) getPreSelected(targetClusters []workv1alpha1.TargetCluster) []*clusterv1alpha1.Cluster {
	PreSelectedClusters := make([]*clusterv1alpha1.Cluster, len(targetClusters))
	clusterInfoSnapshot := g.schedulerCache.Snapshot()
	for i, targetCluster := range targetClusters {
		for _, cluster := range clusterInfoSnapshot.GetClusters() {
			if targetCluster.Name == cluster.Cluster().Name {
				PreSelectedClusters[i] = cluster.Cluster()
				break
			}
		}
	}
	return PreSelectedClusters
}
