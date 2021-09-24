package core

import (
	"context"
	"fmt"
	"math"
	"sort"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	lister "github.com/karmada-io/karmada/pkg/generated/listers/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/runtime"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// ScheduleAlgorithm is the interface that should be implemented to schedule a resource to the target clusters.
type ScheduleAlgorithm interface {
	Schedule(context.Context, *policyv1alpha1.Placement, *workv1alpha1.ResourceBindingSpec) (scheduleResult ScheduleResult, err error)
	ScaleSchedule(context.Context, *policyv1alpha1.Placement, *workv1alpha1.ResourceBindingSpec) (scheduleResult ScheduleResult, err error)
	FailoverSchedule(context.Context, *policyv1alpha1.Placement, *workv1alpha1.ResourceBindingSpec) (scheduleResult ScheduleResult, err error)
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

func (g *genericScheduler) Schedule(ctx context.Context, placement *policyv1alpha1.Placement, spec *workv1alpha1.ResourceBindingSpec) (result ScheduleResult, err error) {
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

func (g *genericScheduler) assignReplicas(clusters []*clusterv1alpha1.Cluster, replicaSchedulingStrategy *policyv1alpha1.ReplicaSchedulingStrategy, object *workv1alpha1.ResourceBindingSpec) ([]workv1alpha1.TargetCluster, error) {
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
		if replicaSchedulingStrategy.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided {
			if replicaSchedulingStrategy.ReplicaDivisionPreference == policyv1alpha1.ReplicaDivisionPreferenceWeighted {
				if replicaSchedulingStrategy.WeightPreference == nil {
					// if ReplicaDivisionPreference is set to "Weighted" and WeightPreference is not set, scheduler will weight all clusters the same.
					replicaSchedulingStrategy.WeightPreference = getDefaultWeightPreference(clusters)
				}
				return g.divideReplicasByStaticWeight(clusters, replicaSchedulingStrategy.WeightPreference.StaticWeightList, object.Replicas)
			}
			if replicaSchedulingStrategy.ReplicaDivisionPreference == policyv1alpha1.ReplicaDivisionPreferenceAggregated {
				return g.divideReplicasAggregatedWithResource(clusters, object)
			}

			// will never reach here, only "Aggregated" and "Weighted" are support
			return nil, nil
		}
	}

	for i, cluster := range clusters {
		targetClusters[i] = workv1alpha1.TargetCluster{Name: cluster.Name}
	}
	return targetClusters, nil
}

func getDefaultWeightPreference(clusters []*clusterv1alpha1.Cluster) *policyv1alpha1.ClusterPreferences {
	staticWeightLists := make([]policyv1alpha1.StaticClusterWeight, 0)
	for _, cluster := range clusters {
		staticWeightList := policyv1alpha1.StaticClusterWeight{
			TargetCluster: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
			},
			Weight: 1,
		}
		staticWeightLists = append(staticWeightLists, staticWeightList)
	}

	return &policyv1alpha1.ClusterPreferences{
		StaticWeightList: staticWeightLists,
	}
}

// divideReplicasByStaticWeight assigns a total number of replicas to the selected clusters by the weight list.
func (g *genericScheduler) divideReplicasByStaticWeight(clusters []*clusterv1alpha1.Cluster, staticWeightList []policyv1alpha1.StaticClusterWeight, replicas int32) ([]workv1alpha1.TargetCluster, error) {
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

	if weightSum == 0 {
		for _, cluster := range clusters {
			weightSum++
			matchClusters[cluster.Name] = 1
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

func (g *genericScheduler) divideReplicasAggregatedWithResource(clusters []*clusterv1alpha1.Cluster,
	spec *workv1alpha1.ResourceBindingSpec, preUsedClustersName ...string) ([]workv1alpha1.TargetCluster, error) {
	// make sure preUsedClusters are in front of the unUsedClusters in the list of clusterAvailableReplicas
	// so that we can assign new replicas to them preferentially when scale up.
	// preUsedClusters have none items during first scheduler
	preUsedClusters, unUsedClusters := g.getPreUsed(clusters, preUsedClustersName...)
	preUsedClustersAvailableReplicas := g.calAvailableReplicas(preUsedClusters, spec)
	unUsedClustersAvailableReplicas := g.calAvailableReplicas(unUsedClusters, spec)
	clusterAvailableReplicas := append(preUsedClustersAvailableReplicas, unUsedClustersAvailableReplicas...)
	return g.divideReplicasAggregatedWithClusterReplicas(clusterAvailableReplicas, spec.Replicas)
}

func (g *genericScheduler) calAvailableReplicas(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha1.ResourceBindingSpec) []workv1alpha1.TargetCluster {
	availableTargetClusters := make([]workv1alpha1.TargetCluster, len(clusters))

	// Set the boundary.
	for i := range availableTargetClusters {
		availableTargetClusters[i].Name = clusters[i].Name
		availableTargetClusters[i].Replicas = math.MaxInt32
	}

	// Get the minimum value of MaxAvailableReplicas in terms of all estimators.
	estimators := estimatorclient.GetReplicaEstimators()
	for _, estimator := range estimators {
		res, err := estimator.MaxAvailableReplicas(clusters, spec.ReplicaRequirements)
		if err != nil {
			klog.Errorf("Max cluster available replicas error: %v", err)
			continue
		}
		for i := range res {
			if res[i].Replicas == estimatorclient.UnauthenticReplica {
				continue
			}
			if availableTargetClusters[i].Name == res[i].Name && availableTargetClusters[i].Replicas > res[i].Replicas {
				availableTargetClusters[i].Replicas = res[i].Replicas
			}
		}
	}

	// In most cases, the target cluster max available replicas should not be MaxInt32 unless the workload is best-effort
	// and the scheduler-estimator has not been enabled. So we set the replicas to spec.Replicas for avoiding overflow.
	for i := range availableTargetClusters {
		if availableTargetClusters[i].Replicas == math.MaxInt32 {
			availableTargetClusters[i].Replicas = spec.Replicas
		}
	}

	sort.Sort(TargetClustersList(availableTargetClusters))
	klog.V(4).Infof("Target cluster: %v", availableTargetClusters)
	return availableTargetClusters
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
			continue
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

func (g *genericScheduler) ScaleSchedule(ctx context.Context, placement *policyv1alpha1.Placement,
	spec *workv1alpha1.ResourceBindingSpec) (result ScheduleResult, err error) {
	newTargetClusters := make([]workv1alpha1.TargetCluster, len(spec.Clusters))

	if spec.Replicas > 0 {
		if placement.ReplicaScheduling.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
			for i, cluster := range spec.Clusters {
				newTargetClusters[i] = workv1alpha1.TargetCluster{Name: cluster.Name, Replicas: spec.Replicas}
			}
			result.SuggestedClusters = newTargetClusters
			return result, nil
		}
		if placement.ReplicaScheduling.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided {
			if placement.ReplicaScheduling.ReplicaDivisionPreference == policyv1alpha1.ReplicaDivisionPreferenceWeighted {
				preSelectedClusters := g.getPreSelected(spec.Clusters)
				if placement.ReplicaScheduling.WeightPreference == nil {
					// if ReplicaDivisionPreference is set to "Weighted" and WeightPreference is not set, scheduler will weight all clusters the same.
					placement.ReplicaScheduling.WeightPreference = getDefaultWeightPreference(preSelectedClusters)
				}
				clustersWithReplicase, err := g.divideReplicasByStaticWeight(preSelectedClusters, placement.ReplicaScheduling.WeightPreference.StaticWeightList, spec.Replicas)
				if err != nil {
					return result, fmt.Errorf("failed to assignReplicas with Weight: %v", err)
				}
				result.SuggestedClusters = clustersWithReplicase
				return result, nil
			}
			if placement.ReplicaScheduling.ReplicaDivisionPreference == policyv1alpha1.ReplicaDivisionPreferenceAggregated {
				return g.scaleScheduleWithReplicaDivisionPreferenceAggregated(spec)
			}

			// will never reach here, only "Aggregated" and "Weighted" are support
			return result, nil
		}
	}

	for i, cluster := range spec.Clusters {
		newTargetClusters[i] = workv1alpha1.TargetCluster{Name: cluster.Name}
	}
	result.SuggestedClusters = newTargetClusters
	return result, nil
}

func (g *genericScheduler) scaleScheduleWithReplicaDivisionPreferenceAggregated(spec *workv1alpha1.ResourceBindingSpec) (result ScheduleResult, err error) {
	assignedReplicas := util.GetSumOfReplicas(spec.Clusters)
	if assignedReplicas > spec.Replicas {
		newTargetClusters, err := g.scaleDownScheduleWithReplicaDivisionPreferenceAggregated(spec)
		if err != nil {
			return result, fmt.Errorf("failed to scaleDown: %v", err)
		}
		result.SuggestedClusters = newTargetClusters
	} else if assignedReplicas < spec.Replicas {
		newTargetClusters, err := g.scaleUpScheduleWithReplicaDivisionPreferenceAggregated(spec)
		if err != nil {
			return result, fmt.Errorf("failed to scaleUp: %v", err)
		}
		result.SuggestedClusters = newTargetClusters
	} else {
		result.SuggestedClusters = spec.Clusters
	}
	return result, nil
}

func (g *genericScheduler) scaleDownScheduleWithReplicaDivisionPreferenceAggregated(spec *workv1alpha1.ResourceBindingSpec) ([]workv1alpha1.TargetCluster, error) {
	return g.divideReplicasAggregatedWithClusterReplicas(spec.Clusters, spec.Replicas)
}

func (g *genericScheduler) scaleUpScheduleWithReplicaDivisionPreferenceAggregated(spec *workv1alpha1.ResourceBindingSpec) ([]workv1alpha1.TargetCluster, error) {
	// find the clusters that have old replicas so we can assign new replicas to them preferentially
	// targetMap map of the result for the old replicas so that it can be merged with the new result easily
	targetMap := make(map[string]int32)
	usedTargetClusters := make([]string, 0)
	assignedReplicas := int32(0)
	for _, cluster := range spec.Clusters {
		targetMap[cluster.Name] = cluster.Replicas
		assignedReplicas += cluster.Replicas
		if cluster.Replicas > 0 {
			usedTargetClusters = append(usedTargetClusters, cluster.Name)
		}
	}
	preSelected := g.getPreSelected(spec.Clusters)
	// only the new replicas are considered during this scheduler, the old replicas will not be moved.
	// if not the old replicas may be recreated which is not expected during scaling up
	// use usedTargetClusters to make sure that we assign new replicas to them preferentially so that all the replicas are aggregated
	newObject := spec.DeepCopy()
	newObject.Replicas = spec.Replicas - assignedReplicas
	result, err := g.divideReplicasAggregatedWithResource(preSelected, newObject, usedTargetClusters...)
	if err != nil {
		return result, err
	}
	// merge the result of this scheduler for new replicas and the data of old replicas
	for i, cluster := range result {
		value, ok := targetMap[cluster.Name]
		if ok {
			result[i].Replicas = cluster.Replicas + value
			delete(targetMap, cluster.Name)
		}
	}
	for key, value := range targetMap {
		result = append(result, workv1alpha1.TargetCluster{Name: key, Replicas: value})
	}
	return result, nil
}

func (g *genericScheduler) getPreSelected(targetClusters []workv1alpha1.TargetCluster) []*clusterv1alpha1.Cluster {
	var preSelectedClusters []*clusterv1alpha1.Cluster
	clusterInfoSnapshot := g.schedulerCache.Snapshot()
	for _, targetCluster := range targetClusters {
		for _, cluster := range clusterInfoSnapshot.GetClusters() {
			if targetCluster.Name == cluster.Cluster().Name {
				preSelectedClusters = append(preSelectedClusters, cluster.Cluster())
				break
			}
		}
	}
	return preSelectedClusters
}

func (g *genericScheduler) getPreUsed(clusters []*clusterv1alpha1.Cluster, preUsedClustersName ...string) ([]*clusterv1alpha1.Cluster, []*clusterv1alpha1.Cluster) {
	if len(preUsedClustersName) == 0 {
		return clusters, nil
	}
	preUsedClusterSet := sets.NewString(preUsedClustersName...)
	var preUsedCluster []*clusterv1alpha1.Cluster
	var unUsedCluster []*clusterv1alpha1.Cluster
	for i := range clusters {
		if preUsedClusterSet.Has(clusters[i].Name) {
			preUsedCluster = append(preUsedCluster, clusters[i])
		} else {
			unUsedCluster = append(unUsedCluster, clusters[i])
		}
	}
	return preUsedCluster, unUsedCluster
}

func (g *genericScheduler) FailoverSchedule(ctx context.Context, placement *policyv1alpha1.Placement,
	spec *workv1alpha1.ResourceBindingSpec) (result ScheduleResult, err error) {
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

	var reScheduleResult []workv1alpha1.TargetCluster
	for cluster := range targetClusters {
		reScheduleResult = append(reScheduleResult, workv1alpha1.TargetCluster{Name: cluster})
	}

	return ScheduleResult{reScheduleResult}, nil
}

// calcReservedCluster eliminates the not-ready clusters from the 'bindClusters'.
func calcReservedCluster(bindClusters, readyClusters sets.String) sets.String {
	return bindClusters.Difference(bindClusters.Difference(readyClusters))
}

// calcAvailableCluster returns a list of ready clusters that not in 'bindClusters'.
func calcAvailableCluster(bindCluster, readyClusters sets.String) sets.String {
	return readyClusters.Difference(bindCluster)
}
