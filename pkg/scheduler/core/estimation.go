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
	"fmt"

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
	return estimateMultiTemplateAvailableSets(ctx, estCtx, estCtx.clusters, estCtx.spec.Components)
}

// calculateMultiTemplateAvailableSetsForScale plans capacity for a component replica scale.
// Callers must ensure that only replica counts changed: component requirements and placement
// must be unchanged, and failure-safe result retention must already be in place.
func calculateMultiTemplateAvailableSetsForScale(ctx context.Context, estCtx multiTemplateEstimationContext) ([]workv1alpha2.TargetCluster, error) {
	plans, err := buildComponentScalePlans(estCtx)
	if err != nil {
		return nil, err
	}

	result := make([]workv1alpha2.TargetCluster, 0, len(estCtx.clusters))
	fullDesiredClusters := make([]*clusterv1alpha1.Cluster, 0, len(estCtx.clusters))
	for i := range plans {
		switch plans[i].direction {
		case componentScaleNewCandidate:
			fullDesiredClusters = append(fullDesiredClusters, plans[i].cluster)
		case componentScaleDown:
			result = append(result, workv1alpha2.TargetCluster{
				Name:     plans[i].cluster.Name,
				Replicas: scaleDownAvailableComponentSets,
			})
		case componentScaleUp:
			delta := positiveComponentDelta(estCtx.spec.Components, plans[i].accepted)
			estimated, err := estimateMultiTemplateAvailableSets(ctx, estCtx, []*clusterv1alpha1.Cluster{plans[i].cluster}, delta)
			if err != nil {
				return nil, err
			}
			result = append(result, estimated...)
		}
	}

	if len(fullDesiredClusters) > 0 {
		estimated, err := estimateMultiTemplateAvailableSets(ctx, estCtx, fullDesiredClusters, estCtx.spec.Components)
		if err != nil {
			return nil, err
		}
		result = append(result, estimated...)
	}

	resultByCluster := make(map[string]workv1alpha2.TargetCluster, len(result))
	for i := range result {
		resultByCluster[result[i].Name] = result[i]
	}
	ordered := make([]workv1alpha2.TargetCluster, 0, len(result))
	for _, cluster := range estCtx.clusters {
		if clusterResult, exists := resultByCluster[cluster.Name]; exists {
			ordered = append(ordered, clusterResult)
		}
	}
	return ordered, nil
}

type componentScalePlan struct {
	cluster   *clusterv1alpha1.Cluster
	direction componentScaleDirection
	accepted  []workv1alpha2.TargetComponent
}

func buildComponentScalePlans(estCtx multiTemplateEstimationContext) ([]componentScalePlan, error) {
	if !componentNamesAreValidAndUnique(estCtx.spec.Components) {
		return nil, fmt.Errorf("component scale planning requires desired components to have unique, non-empty names")
	}

	plans := make([]componentScalePlan, 0, len(estCtx.clusters))
	for _, cluster := range estCtx.clusters {
		accepted, found := acceptedComponentsForCluster(estCtx.spec.Clusters, cluster.Name)
		if !found {
			plans = append(plans, componentScalePlan{cluster: cluster, direction: componentScaleNewCandidate})
			continue
		}

		direction := componentReplicaScaleDirection(estCtx.spec.Components, accepted)
		switch direction {
		case componentScaleUnknown:
			return nil, fmt.Errorf("component scale planning for cluster %q requires a comparable accepted component snapshot", cluster.Name)
		case componentScaleEqual:
			return nil, fmt.Errorf("component scale planning for cluster %q requires a replica change", cluster.Name)
		case componentScaleMixed:
			return nil, fmt.Errorf("mixed component scaling is not supported for cluster %q", cluster.Name)
		}
		plans = append(plans, componentScalePlan{cluster: cluster, direction: direction, accepted: accepted})
	}
	return plans, nil
}

func estimateMultiTemplateAvailableSets(ctx context.Context, estCtx multiTemplateEstimationContext, clusters []*clusterv1alpha1.Cluster, components []workv1alpha2.Component) ([]workv1alpha2.TargetCluster, error) {
	req := estimatorclient.ComponentSetEstimationRequest{
		Clusters:         clusters,
		Components:       components,
		Namespace:        estCtx.spec.Resource.Namespace,
		AssumedWorkloads: estCtx.assumedWorkloads,
	}

	namespacedKey := names.NamespacedKey(estCtx.spec.Resource.Namespace, estCtx.spec.Resource.Name)
	resp, err := estCtx.estimator.MaxAvailableComponentSets(ctx, req)
	if err != nil {
		klog.Errorf("Failed to calculate available component set with estimator(%s) for workload(%s, kind=%s, %s): %v",
			estCtx.estimatorName, estCtx.spec.Resource.APIVersion, estCtx.spec.Resource.Kind, namespacedKey, err)
		return nil, err
	}

	// Use a map to safely update replicas regardless of order.
	resMap := make(map[string]int32, len(resp))
	for i := range resp {
		if resp[i].Sets == estimatorclient.UnauthenticReplica {
			continue
		}
		resMap[resp[i].Name] = resp[i].Sets
	}

	result := make([]workv1alpha2.TargetCluster, 0, len(clusters))
	for _, cluster := range clusters {
		sets, ok := resMap[cluster.Name]
		if !ok {
			klog.Warningf("The estimator(%s) missed estimation from cluster(%s) when estimating for workload(%s, kind=%s, %s).",
				estCtx.estimatorName, cluster.Name, estCtx.spec.Resource.APIVersion, estCtx.spec.Resource.Kind, namespacedKey)
			continue
		}
		result = append(result, workv1alpha2.TargetCluster{Name: cluster.Name, Replicas: sets})
	}
	return result, nil
}

func acceptedComponentsForCluster(clusters []workv1alpha2.TargetCluster, name string) ([]workv1alpha2.TargetComponent, bool) {
	for i := range clusters {
		if clusters[i].Name == name {
			return clusters[i].Components, true
		}
	}
	return nil, false
}

func positiveComponentDelta(desired []workv1alpha2.Component, accepted []workv1alpha2.TargetComponent) []workv1alpha2.Component {
	acceptedReplicas := make(map[string]int32, len(accepted))
	for i := range accepted {
		acceptedReplicas[accepted[i].Name] = accepted[i].Replicas
	}

	delta := make([]workv1alpha2.Component, 0, len(desired))
	for i := range desired {
		replicas := desired[i].Replicas - acceptedReplicas[desired[i].Name]
		if replicas <= 0 {
			continue
		}
		component := desired[i]
		component.Replicas = replicas
		delta = append(delta, component)
	}
	return delta
}

type componentScaleDirection int

const (
	componentScaleUnknown componentScaleDirection = iota
	componentScaleNewCandidate
	componentScaleEqual
	componentScaleUp
	componentScaleDown
	componentScaleMixed
)

// A scale-down requires no additional capacity. One available set keeps the
// current target eligible; AssignReplicas constructs the final assignment.
const scaleDownAvailableComponentSets int32 = 1

func componentReplicaScaleDirection(desired []workv1alpha2.Component, accepted []workv1alpha2.TargetComponent) componentScaleDirection {
	if len(desired) != len(accepted) || !componentNamesAreValidAndUnique(desired) {
		return componentScaleUnknown
	}

	acceptedReplicas := make(map[string]int32, len(accepted))
	for i := range accepted {
		if _, exists := acceptedReplicas[accepted[i].Name]; exists {
			return componentScaleUnknown
		}
		acceptedReplicas[accepted[i].Name] = accepted[i].Replicas
	}
	var scaleUp, scaleDown bool
	for i := range desired {
		replicas, exists := acceptedReplicas[desired[i].Name]
		if !exists {
			return componentScaleUnknown
		}
		switch {
		case desired[i].Replicas > replicas:
			scaleUp = true
		case desired[i].Replicas < replicas:
			scaleDown = true
		}
	}
	switch {
	case scaleUp && scaleDown:
		return componentScaleMixed
	case scaleUp:
		return componentScaleUp
	case scaleDown:
		return componentScaleDown
	default:
		return componentScaleEqual
	}
}

func componentNamesAreValidAndUnique(components []workv1alpha2.Component) bool {
	if len(components) == 0 {
		return false
	}

	names := make(map[string]struct{}, len(components))
	for i := range components {
		if components[i].Name == "" {
			return false
		}
		if _, exists := names[components[i].Name]; exists {
			return false
		}
		names[components[i].Name] = struct{}{}
	}
	return true
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
