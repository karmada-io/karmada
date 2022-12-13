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
	Schedule(context.Context, *policyv1alpha1.Placement, *workv1alpha2.ResourceBindingSpec, *ScheduleAlgorithmOption) (scheduleResult ScheduleResult, err error)
}

// ScheduleAlgorithmOption represents the option for ScheduleAlgorithm.
type ScheduleAlgorithmOption struct {
	EnableEmptyWorkloadPropagation bool
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
	registry runtime.Registry,
) (ScheduleAlgorithm, error) {
	f, err := runtime.NewFramework(registry)
	if err != nil {
		return nil, err
	}
	return &genericScheduler{
		schedulerCache:    schedCache,
		scheduleFramework: f,
	}, nil
}

func (g *genericScheduler) Schedule(ctx context.Context, placement *policyv1alpha1.Placement, spec *workv1alpha2.ResourceBindingSpec, scheduleAlgorithmOption *ScheduleAlgorithmOption) (result ScheduleResult, err error) {
	clusterInfoSnapshot := g.schedulerCache.Snapshot()
	if clusterInfoSnapshot.NumOfClusters() == 0 {
		return result, fmt.Errorf("no clusters available to schedule")
	}

	feasibleClusters, diagnosis, err := g.findClustersThatFit(ctx, g.scheduleFramework, placement, spec, &clusterInfoSnapshot)
	if err != nil {
		return result, fmt.Errorf("failed to findClustersThatFit: %v", err)
	}

	// Short path for case no cluster fit.
	if len(feasibleClusters) == 0 {
		return result, &framework.FitError{
			NumAllClusters: clusterInfoSnapshot.NumOfClusters(),
			Diagnosis:      diagnosis,
		}
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
	if scheduleAlgorithmOption.EnableEmptyWorkloadPropagation {
		clustersWithReplicas = attachZeroReplicasCluster(clusters, clustersWithReplicas)
	}
	result.SuggestedClusters = clustersWithReplicas

	return result, nil
}

// findClustersThatFit finds the clusters that are fit for the placement based on running the filter plugins.
func (g *genericScheduler) findClustersThatFit(
	ctx context.Context,
	fwk framework.Framework,
	placement *policyv1alpha1.Placement,
	bindingSpec *workv1alpha2.ResourceBindingSpec,
	clusterInfo *cache.Snapshot,
) ([]*clusterv1alpha1.Cluster, framework.Diagnosis, error) {
	startTime := time.Now()
	defer metrics.ScheduleStep(metrics.ScheduleStepFilter, startTime)

	diagnosis := framework.Diagnosis{
		ClusterToResultMap: make(framework.ClusterToResultMap),
	}

	var out []*clusterv1alpha1.Cluster
	// DO NOT filter unhealthy cluster, let users make decisions by using ClusterTolerations of Placement.
	clusters := clusterInfo.GetClusters()
	for _, c := range clusters {
		if result := fwk.RunFilterPlugins(ctx, placement, bindingSpec, c.Cluster()); !result.IsSuccess() {
			klog.V(4).Infof("cluster %q is not fit, reason: %v", c.Cluster().Name, result.AsError())
			diagnosis.ClusterToResultMap[c.Cluster().Name] = result
		} else {
			out = append(out, c.Cluster())
		}
	}

	return out, diagnosis, nil
}

// prioritizeClusters prioritize the clusters by running the score plugins.
func (g *genericScheduler) prioritizeClusters(
	ctx context.Context,
	fwk framework.Framework,
	placement *policyv1alpha1.Placement,
	spec *workv1alpha2.ResourceBindingSpec,
	clusters []*clusterv1alpha1.Cluster) (result framework.ClusterScoreList, err error) {
	startTime := time.Now()
	defer metrics.ScheduleStep(metrics.ScheduleStepScore, startTime)

	scoresMap, runScorePluginsResult := fwk.RunScorePlugins(ctx, placement, spec, clusters)
	if runScorePluginsResult != nil {
		return result, runScorePluginsResult.AsError()
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
	startTime := time.Now()
	defer metrics.ScheduleStep(metrics.ScheduleStepSelect, startTime)

	groupClustersInfo := spreadconstraint.GroupClustersWithScore(clustersScore, placement, spec, calAvailableReplicas)
	return spreadconstraint.SelectBestClusters(placement, groupClustersInfo, spec.Replicas)
}

func (g *genericScheduler) assignReplicas(
	clusters []*clusterv1alpha1.Cluster,
	replicaSchedulingStrategy *policyv1alpha1.ReplicaSchedulingStrategy,
	object *workv1alpha2.ResourceBindingSpec,
) ([]workv1alpha2.TargetCluster, error) {
	startTime := time.Now()
	defer metrics.ScheduleStep(metrics.ScheduleStepAssignReplicas, startTime)

	if len(clusters) == 0 {
		return nil, fmt.Errorf("no clusters available to schedule")
	}

	if object.Replicas > 0 && replicaSchedulingStrategy != nil {
		var strategy string

		switch replicaSchedulingStrategy.ReplicaSchedulingType {
		case policyv1alpha1.ReplicaSchedulingTypeDuplicated:
			strategy = DuplicatedStrategy
		case policyv1alpha1.ReplicaSchedulingTypeDivided:
			switch replicaSchedulingStrategy.ReplicaDivisionPreference {
			case policyv1alpha1.ReplicaDivisionPreferenceAggregated:
				strategy = AggregatedStrategy
			case policyv1alpha1.ReplicaDivisionPreferenceWeighted:
				if replicaSchedulingStrategy.WeightPreference != nil && len(replicaSchedulingStrategy.WeightPreference.DynamicWeight) != 0 {
					strategy = DynamicWeightStrategy
				} else {
					strategy = StaticWeightStrategy
				}
			}
		}

		assign, ok := assignFuncMap[strategy]
		if !ok {
			// should never happen at present
			return nil, fmt.Errorf("unsupported replica scheduling strategy, replicaSchedulingType: %s, replicaDivisionPreference: %s, "+
				"please try another scheduling strategy", replicaSchedulingStrategy.ReplicaSchedulingType, replicaSchedulingStrategy.ReplicaDivisionPreference)
		}
		return assign(&assignState{
			candidates: clusters,
			strategy:   replicaSchedulingStrategy,
			object:     object,
		})
	}

	// If not workload, assign all clusters without considering replicas.
	targetClusters := make([]workv1alpha2.TargetCluster, len(clusters))
	for i, cluster := range clusters {
		targetClusters[i] = workv1alpha2.TargetCluster{Name: cluster.Name}
	}
	return targetClusters, nil
}
