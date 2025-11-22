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
	"github.com/karmada-io/karmada/pkg/util/names"
)

// isMultiTemplateSchedulingApplicable checks if the given ResourceBindingSpec
// meets the criteria for multi-template scheduling:
//  1. The referenced resource holds multiple pod templates (multiple components).
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

	if len(spec.Components) < 2 {
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

// calculateMultiTemplateAvailableSets calculates available sets for multi-template scheduling.
// It uses MaxAvailableComponentSets to estimate capacity for workloads with multiple pod templates.
func calculateMultiTemplateAvailableSets(ctx context.Context, estimator estimatorclient.ReplicaEstimator, name string, clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec, availableTargetClusters []workv1alpha2.TargetCluster) ([]workv1alpha2.TargetCluster, error) {
	req := estimatorclient.ComponentSetEstimationRequest{
		Clusters:   clusters,
		Components: spec.Components,
		Namespace:  spec.Resource.Namespace,
	}

	namespacedKey := names.NamespacedKey(spec.Resource.Namespace, spec.Resource.Name)
	resp, err := estimator.MaxAvailableComponentSets(ctx, req)
	if err != nil {
		klog.Errorf("Failed to calculate available component set with estimator(%s) for workload(%s, kind=%s, %s): %v",
			name, spec.Resource.APIVersion, spec.Resource.Kind, namespacedKey, err)
		return availableTargetClusters, err
	}

	// Use a map to safely update replicas regardless of order.
	resMap := make(map[string]int32, len(resp))
	for i := range resp {
		if resp[i].Sets == estimatorclient.UnauthenticReplica {
			continue
		}
		resMap[resp[i].Name] = resp[i].Sets
	}
	for i := range availableTargetClusters {
		if newReplicas, ok := resMap[availableTargetClusters[i].Name]; ok {
			if availableTargetClusters[i].Replicas > newReplicas {
				availableTargetClusters[i].Replicas = newReplicas
			}
		} else {
			klog.Warningf("The estimator(%s) missed estimation from cluster(%s) when estimating for workload(%s, kind=%s, %s).",
				name, availableTargetClusters[i].Name, spec.Resource.APIVersion, spec.Resource.Kind, namespacedKey)
		}
	}

	return availableTargetClusters, nil
}
