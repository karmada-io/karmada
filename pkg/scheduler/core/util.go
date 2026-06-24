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
	"math"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/features"
	schedulercache "github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/core/spreadconstraint"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

type calculator func([]spreadconstraint.ClusterDetailInfo, *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster

func getDefaultWeightPreference(clusters []spreadconstraint.ClusterDetailInfo) *policyv1alpha1.ClusterPreferences {
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

func calAvailableReplicas(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec, assigningCache *schedulercache.AssigningResourceBindingCache) []workv1alpha2.TargetCluster {
	availableTargetClusters := make([]workv1alpha2.TargetCluster, len(clusters))

	// Set the boundary.
	for i := range availableTargetClusters {
		availableTargetClusters[i].Name = clusters[i].Name
		availableTargetClusters[i].Replicas = math.MaxInt32
	}

	// For non-workload, like ServiceAccount, ConfigMap, Secret and etc, it's unnecessary to calculate available replicas in member clusters.
	// See issue: https://github.com/karmada-io/karmada/issues/3743.
	namespacedKey := names.NamespacedKey(spec.Resource.Namespace, spec.Resource.Name)
	if spec.Replicas == 0 && len(spec.Components) == 0 {
		klog.V(4).Infof("Do not calculate available replicas for non-workload(%s, kind=%s, %s).", spec.Resource.APIVersion,
			spec.Resource.Kind, namespacedKey)
		return availableTargetClusters
	}

	// Get the minimum value of MaxAvailableReplicas in terms of all estimators.
	ctx := context.WithValue(context.TODO(), util.ContextKeyObject,
		fmt.Sprintf("kind=%s, name=%s/%s", spec.Resource.Kind, spec.Resource.Namespace, spec.Resource.Name))
	// Build assumed workloads once outside the estimator loop as it only depends on
	// clusters and assigningCache, both of which are constant for the entire loop.
	// When SchedulingOvercommitProtection is disabled the map stays empty so the
	// estimator behaves as if no in-flight assumptions exist.
	var assumedWorkloads map[string][]estimatorclient.AssumedWorkload
	if features.FeatureGate.Enabled(features.SchedulingOvercommitProtection) {
		assumedWorkloads = buildAssumedWorkloadsByCluster(clusters, assigningCache)
	}
	for name, estimator := range estimatorclient.GetReplicaEstimators() {
		res, err := runReplicaEstimator(ctx, replicaEstimationContext{
			estimatorName:    name,
			estimator:        estimator,
			clusters:         clusters,
			spec:             spec,
			assumedWorkloads: assumedWorkloads,
		})
		if err != nil {
			// On a partial failure res still holds the clusters that succeeded; failed ones
			// are marked UnauthenticReplica and skipped by mergeReplicaResults. Keep merging
			// instead of discarding the estimator for every cluster. The estimator already
			// logged the error, so only trace it here to avoid a duplicate error log.
			klog.V(4).Infof("Estimator %s returned an error, merging any partial results: %v", name, err)
		}
		availableTargetClusters = mergeReplicaResults(availableTargetClusters, res)
	}

	// In most cases, the target cluster max available replicas should not be MaxInt32 unless the workload is best-effort
	// and the scheduler-estimator has not been enabled. So we set the replicas to spec.Replicas for avoiding overflow.
	for i := range availableTargetClusters {
		if availableTargetClusters[i].Replicas == math.MaxInt32 {
			availableTargetClusters[i].Replicas = spec.Replicas
		}
	}

	klog.V(4).Infof("Target cluster calculated by estimators (available cluster && maxAvailableReplicas): %v", availableTargetClusters)
	return availableTargetClusters
}

// runReplicaEstimator dispatches to the multi-template or single-template estimation path.
func runReplicaEstimator(ctx context.Context, in replicaEstimationContext) ([]workv1alpha2.TargetCluster, error) {
	if features.FeatureGate.Enabled(features.MultiplePodTemplatesScheduling) && isMultiTemplateSchedulingApplicable(in.spec) {
		return runMultiTemplateEstimator(ctx, in)
	}
	return runSingleTemplateEstimator(ctx, in)
}

// replicaEstimationContext bundles the per-estimator inputs for runReplicaEstimator.
type replicaEstimationContext struct {
	estimatorName    string
	estimator        estimatorclient.ReplicaEstimator
	clusters         []*clusterv1alpha1.Cluster
	spec             *workv1alpha2.ResourceBindingSpec
	assumedWorkloads map[string][]estimatorclient.AssumedWorkload
}

// runMultiTemplateEstimator handles estimation for workloads with multiple pod templates.
func runMultiTemplateEstimator(ctx context.Context, in replicaEstimationContext) ([]workv1alpha2.TargetCluster, error) {
	return calculateMultiTemplateAvailableSets(ctx, multiTemplateEstimationContext{
		estimator:        in.estimator,
		estimatorName:    in.estimatorName,
		clusters:         in.clusters,
		spec:             in.spec,
		assumedWorkloads: in.assumedWorkloads,
	})
}

// runSingleTemplateEstimator handles estimation for workloads with a single pod template.
func runSingleTemplateEstimator(ctx context.Context, in replicaEstimationContext) ([]workv1alpha2.TargetCluster, error) {
	res, err := in.estimator.MaxAvailableReplicas(ctx, estimatorclient.ReplicaEstimationRequest{
		Clusters:            in.clusters,
		ReplicaRequirements: in.spec.ReplicaRequirements,
		AssumedWorkloads:    in.assumedWorkloads,
	})
	if err != nil {
		// Return res alongside the error so the caller can still use the clusters that
		// succeeded; failed clusters are marked UnauthenticReplica.
		klog.Errorf("Max cluster available replicas error: %v", err)
		return res, err
	}
	klog.V(4).Infof("Invoked MaxAvailableReplicas of estimator %s for workload(%s, kind=%s, %s): %v", in.estimatorName,
		in.spec.Resource.APIVersion, in.spec.Resource.Kind, names.NamespacedKey(in.spec.Resource.Namespace, in.spec.Resource.Name), res)
	return res, nil
}

// mergeReplicaResults takes the minimum replicas across estimators for each cluster.
func mergeReplicaResults(available []workv1alpha2.TargetCluster, res []workv1alpha2.TargetCluster) []workv1alpha2.TargetCluster {
	resMap := make(map[string]int32, len(res))
	for i := range res {
		if res[i].Replicas != estimatorclient.UnauthenticReplica {
			resMap[res[i].Name] = res[i].Replicas
		}
	}
	for i := range available {
		if newReplicas, ok := resMap[available[i].Name]; ok && available[i].Replicas > newReplicas {
			available[i].Replicas = newReplicas
		}
	}
	return available
}

// attachZeroReplicasCluster  attach cluster in clusters into targetCluster
// The purpose is to avoid workload not appeared in rb's spec.clusters field
func attachZeroReplicasCluster(clusters []spreadconstraint.ClusterDetailInfo,
	targetClusters []workv1alpha2.TargetCluster) []workv1alpha2.TargetCluster {
	targetClusterSet := sets.NewString()
	for i := range targetClusters {
		targetClusterSet.Insert(targetClusters[i].Name)
	}
	for i := range clusters {
		if !targetClusterSet.Has(clusters[i].Name) {
			targetClusters = append(targetClusters, workv1alpha2.TargetCluster{Name: clusters[i].Name, Replicas: 0})
		}
	}
	return targetClusters
}

// removeZeroReplicasCLuster remove the cluster with 0 replicas in assignResults
func removeZeroReplicasCluster(assignResults []workv1alpha2.TargetCluster) []workv1alpha2.TargetCluster {
	targetClusters := make([]workv1alpha2.TargetCluster, 0, len(assignResults))
	for _, cluster := range assignResults {
		if cluster.Replicas > 0 {
			targetClusters = append(targetClusters, workv1alpha2.TargetCluster{Name: cluster.Name, Replicas: cluster.Replicas})
		}
	}
	return targetClusters
}
