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
	estimators := estimatorclient.GetReplicaEstimators()
	ctx := context.WithValue(context.TODO(), util.ContextKeyObject,
		fmt.Sprintf("kind=%s, name=%s/%s", spec.Resource.Kind, spec.Resource.Namespace, spec.Resource.Name))

	// Collect results from all estimators, then merge.
	// When scheduler-estimator is available, prefer its result over general-estimator
	// because it has more accurate cluster-level information (e.g., potential capacity
	// from node autoscaler providers configured in NodeResourceEstimator).
	var allResults []estimatorResult

	for name, estimator := range estimators {
		if features.FeatureGate.Enabled(features.MultiplePodTemplatesScheduling) && isMultiTemplateSchedulingApplicable(spec) {
			var err error
			availableTargetClusters, err = calculateMultiTemplateAvailableSets(ctx, multiTemplateEstimationContext{
				estimator:               estimator,
				estimatorName:           name,
				clusters:                clusters,
				spec:                    spec,
				availableTargetClusters: availableTargetClusters,
				assigningCache:          assigningCache,
			})
			if err != nil {
				continue
			}
		} else {
			res, err := estimator.MaxAvailableReplicas(ctx, clusters, spec.ReplicaRequirements)
			if err != nil {
				klog.Errorf("Max cluster available replicas error: %v", err)
				continue
			}
			klog.V(4).Infof("Invoked MaxAvailableReplicas of estimator %s for workload(%s, kind=%s, %s): %v", name,
				spec.Resource.APIVersion, spec.Resource.Kind, namespacedKey, res)
			allResults = append(allResults, estimatorResult{name: name, clusters: res})
		}
	}

	// Merge results: skip general-estimator's value when scheduler-estimator reports higher
	mergeEstimatorResults(allResults, availableTargetClusters)

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

type estimatorResult struct {
	name     string
	clusters []workv1alpha2.TargetCluster
}

// mergeEstimatorResults applies estimator results to availableTargetClusters using min.
// When scheduler-estimator reports higher availability than general-estimator,
// the general-estimator value is skipped to avoid masking capacity from node
// autoscaler providers (e.g., Karpenter) configured in NodeResourceEstimator.
func mergeEstimatorResults(allResults []estimatorResult, availableTargetClusters []workv1alpha2.TargetCluster) {
	var schedulerEstimatorResult []workv1alpha2.TargetCluster
	for _, r := range allResults {
		if r.name == "scheduler-estimator" {
			schedulerEstimatorResult = r.clusters
			break
		}
	}

	for _, r := range allResults {
		for i := range r.clusters {
			if r.clusters[i].Replicas == estimatorclient.UnauthenticReplica {
				continue
			}
			if schedulerEstimatorResult != nil && r.name == "general-estimator" {
				if seReplicas := findClusterReplicas(schedulerEstimatorResult, r.clusters[i].Name); seReplicas > r.clusters[i].Replicas {
					continue
				}
			}
			if availableTargetClusters[i].Name == r.clusters[i].Name && availableTargetClusters[i].Replicas > r.clusters[i].Replicas {
				availableTargetClusters[i].Replicas = r.clusters[i].Replicas
			}
		}
	}
}

// findClusterReplicas returns the replica count for the named cluster, or -1 if not found.
func findClusterReplicas(clusters []workv1alpha2.TargetCluster, name string) int32 {
	for _, c := range clusters {
		if c.Name == name {
			return c.Replicas
		}
	}
	return -1
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
