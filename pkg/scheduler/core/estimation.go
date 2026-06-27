/*
Copyright 2025 The Karmada Authors.

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

	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	schedulercache "github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// isMultiTemplateSchedulingApplicable checks if the given ResourceBindingSpec
// meets the criteria for component-based scheduling:
//  1. The referenced resource has at least one component populated.
//  2. The placement configuration schedules the resource to exactly one cluster.
//     This is currently determined by checking if spread constraints is set and requires exactly one cluster.
//
// Returns true if both conditions are satisfied, false otherwise.
// Note: We do not infer required cluster number from placement.clusterAffinity and
// placement.clusterAffinities because it's impossible to determine without cluster metadata
// whether the affinity rule matches exactly one cluster in the current environment, and the
// only reliable way is spread constraints.
func isMultiTemplateSchedulingApplicable(spec *workv1alpha2.ResourceBindingSpec) bool {
	if spec == nil {
		return false
	}

	if len(spec.Components) == 0 {
		return false
	}

	// Check if placement targets exactly one cluster
	if spec.Placement == nil {
		return false
	}
	for i := range spec.Placement.SpreadConstraints {
		if spec.Placement.SpreadConstraints[i].SpreadByField == policyv1alpha1.SpreadByFieldCluster &&
			spec.Placement.SpreadConstraints[i].MinGroups == 1 &&
			spec.Placement.SpreadConstraints[i].MaxGroups == 1 {
			return true
		}
	}

	return false
}

type multiTemplateEstimationContext struct {
	estimator        estimatorclient.ReplicaEstimator
	estimatorName    string
	clusters         []*clusterv1alpha1.Cluster
	spec             *workv1alpha2.ResourceBindingSpec
	assumedWorkloads map[string][]estimatorclient.AssumedWorkload
}

// calculateMultiTemplateAvailableSets calculates available sets for multi-template scheduling.
// It uses MaxAvailableComponentSets to estimate capacity for workloads with multiple pod templates.
func calculateMultiTemplateAvailableSets(ctx context.Context, estCtx multiTemplateEstimationContext) ([]workv1alpha2.TargetCluster, error) {
	req := estimatorclient.ComponentSetEstimationRequest{
		Clusters:         estCtx.clusters,
		Components:       estCtx.spec.Components,
		Namespace:        estCtx.spec.Resource.Namespace,
		AssumedWorkloads: estCtx.assumedWorkloads,
	}

	namespacedKey := names.NamespacedKey(estCtx.spec.Resource.Namespace, estCtx.spec.Resource.Name)
	resp, err := estCtx.estimator.MaxAvailableComponentSets(ctx, req)
	if err != nil {
		// Don't return early: on a partial failure resp still carries the clusters that
		// succeeded, so fall through and build a result from what is available.
		klog.Errorf("Failed to calculate available component set with estimator(%s) for workload(%s, kind=%s, %s): %v",
			estCtx.estimatorName, estCtx.spec.Resource.APIVersion, estCtx.spec.Resource.Kind, namespacedKey, err)
	}

	// Use a map to safely update replicas regardless of order.
	resMap := make(map[string]int32, len(resp))
	for i := range resp {
		if resp[i].Sets == estimatorclient.UnauthenticReplica {
			continue
		}
		resMap[resp[i].Name] = resp[i].Sets
	}

	result := make([]workv1alpha2.TargetCluster, 0, len(estCtx.clusters))
	for _, cluster := range estCtx.clusters {
		sets, ok := resMap[cluster.Name]
		if !ok {
			klog.Warningf("The estimator(%s) missed estimation from cluster(%s) when estimating for workload(%s, kind=%s, %s).",
				estCtx.estimatorName, cluster.Name, estCtx.spec.Resource.APIVersion, estCtx.spec.Resource.Kind, namespacedKey)
			continue
		}
		result = append(result, workv1alpha2.TargetCluster{Name: cluster.Name, Replicas: sets})
	}
	return result, err
}

// buildAssumedWorkloadsByCluster builds a map of assumed workloads for each cluster based on the assigning cache.
func buildAssumedWorkloadsByCluster(clusters []*clusterv1alpha1.Cluster, assigningCache *schedulercache.AssigningResourceBindingCache) map[string][]estimatorclient.AssumedWorkload {
	assumedWorkloads := make(map[string][]estimatorclient.AssumedWorkload, len(clusters))
	if assigningCache == nil {
		return assumedWorkloads
	}

	for _, cluster := range clusters {
		clusterAssumptions := assigningCache.GetAssumedWorkloads(cluster.Name)
		if len(clusterAssumptions) == 0 {
			continue
		}

		assumed := make([]estimatorclient.AssumedWorkload, len(clusterAssumptions))
		for i := range clusterAssumptions {
			assumed[i] = estimatorclient.AssumedWorkload{
				Namespace:  clusterAssumptions[i].Namespace,
				Components: clusterAssumptions[i].Components,
			}
		}
		assumedWorkloads[cluster.Name] = assumed
	}

	return assumedWorkloads
}
