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

// calculateMultiTemplateAvailableSetsForScale calculates capacity for a component replica
// scale on the current accepted target. It does not define result retention on failure.
func calculateMultiTemplateAvailableSetsForScale(ctx context.Context, estCtx multiTemplateEstimationContext) ([]workv1alpha2.TargetCluster, error) {
	if !componentNamesAreValidAndUnique(estCtx.spec.Components) {
		return nil, fmt.Errorf("component scale planning requires desired components to have unique, non-empty names")
	}
	if len(estCtx.clusters) != 1 || len(estCtx.spec.Clusters) != 1 ||
		estCtx.clusters[0].Name != estCtx.spec.Clusters[0].Name {
		return nil, fmt.Errorf("component scale planning requires exactly one accepted target cluster")
	}

	cluster := estCtx.clusters[0]
	accepted := estCtx.spec.Clusters[0].Components
	direction := componentReplicaScaleDirection(estCtx.spec.Components, accepted)
	switch direction {
	case componentScaleUnknown:
		return nil, fmt.Errorf("component scale planning for cluster %q requires a comparable accepted component snapshot", cluster.Name)
	case componentScaleEqual:
		return nil, fmt.Errorf("component scale planning for cluster %q requires a replica change", cluster.Name)
	case componentScaleMixed:
		return nil, fmt.Errorf("mixed component scaling is not supported for cluster %q", cluster.Name)
	case componentScaleDown:
		return []workv1alpha2.TargetCluster{{Name: cluster.Name, Replicas: minimumAvailableComponentSets}}, nil
	case componentScaleUp:
		delta := positiveComponentDelta(estCtx.spec.Components, accepted)
		return estimateMultiTemplateAvailableSets(ctx, estCtx, estCtx.clusters, delta)
	default:
		return nil, fmt.Errorf("component scale planning for cluster %q has an unsupported transition", cluster.Name)
	}
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
	componentScaleEqual
	componentScaleUp
	componentScaleDown
	componentScaleMixed
)

// minimumAvailableComponentSets is capacity evidence that one atomic component
// set can use the current target. AssignReplicas constructs the final assignment.
const minimumAvailableComponentSets int32 = 1

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
