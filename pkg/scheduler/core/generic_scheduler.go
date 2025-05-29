/*
Copyright 2021 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/scheduler/framework/runtime"
	"github.com/karmada-io/karmada/pkg/scheduler/metrics"
)

// ScheduleAlgorithm is the interface that should be implemented to schedule a resource to the target clusters.
type ScheduleAlgorithm interface {
	Schedule(context.Context, *workv1alpha2.ResourceBindingSpec, *workv1alpha2.ResourceBindingStatus, *ScheduleAlgorithmOption) (scheduleResult ScheduleResult, err error)
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

func (g *genericScheduler) Schedule(
	ctx context.Context,
	spec *workv1alpha2.ResourceBindingSpec,
	status *workv1alpha2.ResourceBindingStatus,
	scheduleAlgorithmOption *ScheduleAlgorithmOption,
) (result ScheduleResult, err error) {
	clusterInfoSnapshot := g.schedulerCache.Snapshot()
	feasibleClusters, diagnosis, err := g.findClustersThatFit(ctx, spec, status, &clusterInfoSnapshot)
	if err != nil {
		return result, fmt.Errorf("failed to find fit clusters: %w", err)
	}

	// Short path for case no cluster fit.
	if len(feasibleClusters) == 0 {
		return result, &framework.FitError{
			NumAllClusters: clusterInfoSnapshot.NumOfClusters(),
			Diagnosis:      diagnosis,
		}
	}
	klog.V(4).Infof("Feasible clusters found: %v", feasibleClusters)

	clustersScore, err := g.prioritizeClusters(ctx, g.scheduleFramework, spec, feasibleClusters)
	if err != nil {
		return result, fmt.Errorf("failed to prioritize clusters: %w", err)
	}
	klog.V(4).Infof("Feasible clusters scores: %v", clustersScore)

	clusters, err := g.selectClusters(clustersScore, spec.Placement, spec)
	if err != nil {
		return result, fmt.Errorf("failed to select clusters: %w", err)
	}
	klog.V(4).Infof("Selected clusters: %v", clusters)

	clustersWithReplicas, err := g.assignReplicas(clusters, spec, status)
	if err != nil {
		return result, fmt.Errorf("failed to assign replicas: %w", err)
	}
	klog.V(4).Infof("Assigned Replicas: %v", clustersWithReplicas)

	if scheduleAlgorithmOption.EnableEmptyWorkloadPropagation {
		clustersWithReplicas = attachZeroReplicasCluster(clusters, clustersWithReplicas)
	}
	result.SuggestedClusters = clustersWithReplicas

	return result, nil
}

// findClustersThatFit finds the clusters that are fit for the placement based on running the filter plugins.
func (g *genericScheduler) findClustersThatFit(
	ctx context.Context,
	bindingSpec *workv1alpha2.ResourceBindingSpec,
	bindingStatus *workv1alpha2.ResourceBindingStatus,
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
		// When cluster is deleting, we will clean up the scheduled results in the cluster.
		// So we should not schedule resource to the deleting cluster.
		if !c.Cluster().DeletionTimestamp.IsZero() {
			klog.V(4).Infof("Cluster %q is deleting, skip it", c.Cluster().Name)
			continue
		}

		if result := g.scheduleFramework.RunFilterPlugins(ctx, bindingSpec, bindingStatus, c.Cluster()); !result.IsSuccess() {
			klog.V(4).Infof("Cluster %q is not fit, reason: %v", c.Cluster().Name, result.AsError())
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
	spec *workv1alpha2.ResourceBindingSpec,
	clusters []*clusterv1alpha1.Cluster) (result framework.ClusterScoreList, err error) {
	startTime := time.Now()
	defer metrics.ScheduleStep(metrics.ScheduleStepScore, startTime)

	scoresMap, runScorePluginsResult := fwk.RunScorePlugins(ctx, spec, clusters)
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
	return SelectClusters(clustersScore, placement, spec)
}

func (g *genericScheduler) assignReplicas(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec,
	status *workv1alpha2.ResourceBindingStatus) ([]workv1alpha2.TargetCluster, error) {
	return AssignReplicas(clusters, spec, status)
}
