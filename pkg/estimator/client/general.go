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

package client

import (
	"context"
	"fmt"
	"math"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
)

// GeneralEstimator is the default replica estimator.
func init() {
	replicaEstimators["general-estimator"] = NewGeneralEstimator()
}

// GeneralEstimator is a normal estimator in terms of cluster ResourceSummary.
type GeneralEstimator struct{}

// NewGeneralEstimator builds a new GeneralEstimator.
func NewGeneralEstimator() *GeneralEstimator {
	return &GeneralEstimator{}
}

// MaxAvailableReplicas estimates the maximum replicas that can be applied to the target cluster by cluster ResourceSummary.
func (ge *GeneralEstimator) MaxAvailableReplicas(_ context.Context, clusters []*clusterv1alpha1.Cluster, replicaRequirements *workv1alpha2.ReplicaRequirements) ([]workv1alpha2.TargetCluster, error) {
	availableTargetClusters := make([]workv1alpha2.TargetCluster, len(clusters))
	for i, cluster := range clusters {
		maxReplicas := ge.maxAvailableReplicas(cluster, replicaRequirements)
		availableTargetClusters[i] = workv1alpha2.TargetCluster{Name: cluster.Name, Replicas: maxReplicas}
	}
	return availableTargetClusters, nil
}

func (ge *GeneralEstimator) maxAvailableReplicas(cluster *clusterv1alpha1.Cluster, replicaRequirements *workv1alpha2.ReplicaRequirements) int32 {
	//Note: resourceSummary must be deep-copied before using in the function to avoid modifying the original data structure.
	resourceSummary := cluster.Status.ResourceSummary.DeepCopy()
	if resourceSummary == nil {
		return 0
	}

	maximumReplicas := getAllowedPodNumber(resourceSummary)
	if maximumReplicas <= 0 {
		return 0
	}

	if replicaRequirements == nil {
		return int32(maximumReplicas) // #nosec G115: integer overflow conversion int64 -> int32
	}

	// if the allocatableModelings from the cluster status are empty possibly due to
	// users have not set the models or the state has not been collected,
	// we consider to use another way to calculate the max replicas.
	if features.FeatureGate.Enabled(features.CustomizedClusterResourceModeling) && len(cluster.Status.ResourceSummary.AllocatableModelings) > 0 {
		num, err := getMaximumReplicasBasedOnResourceModels(cluster, replicaRequirements)
		if err == nil {
			klog.Infof("cluster %s has max available replicas: %d according to cluster resource models", cluster.GetName(), num)
			if num < maximumReplicas {
				maximumReplicas = num
			}

			return int32(maximumReplicas) // #nosec G115: integer overflow conversion int64 -> int32
		}
		klog.Info(err.Error())
	}

	num := getMaximumReplicasBasedOnClusterSummary(resourceSummary, replicaRequirements)
	if num < maximumReplicas {
		maximumReplicas = num
	}

	return int32(maximumReplicas) // #nosec G115: integer overflow conversion int64 -> int32
}

// MaxAvailableComponentSets (generic estimator) – resourceSummary only.
func (ge *GeneralEstimator) MaxAvailableComponentSets(
	ctx context.Context,
	req *ComponentSetEstimationRequest,
) ([]ComponentSetEstimationResponse, error) {
	responses := make([]ComponentSetEstimationResponse, len(req.Clusters))
	for i, cluster := range req.Clusters {
		maxComponentSets := ge.maxAvailableComponentSets(cluster, req.Components)
		responses[i] = ComponentSetEstimationResponse{Name: cluster.Name, Sets: maxComponentSets}
	}
	return responses, nil
}

func (ge *GeneralEstimator) maxAvailableComponentSets(cluster *clusterv1alpha1.Cluster, components []*workv1alpha2.Component) int32 {
	// Compute per-set aggregate requirement (e.g. CPU, Memory)
	setCPU, setMem := perSetRequirement(components)
	resourceSummary := cluster.Status.ResourceSummary.DeepCopy()
	if resourceSummary == nil {
		return 0
	}

	// Upper bound by pods
	maxSets := getAllowedPodNumber(resourceSummary)
	if maxSets <= 0 {
		return 0
	}

	// Fallback to allowedPodNumber if there is no resourceRequirement
	if setCPU == 0 && setMem == 0 {
		return int32(maxSets)
	}

	// Upper bound by aggregate CPU/Memory
	num := getMaximumSetsBasedOnClusterSummary(resourceSummary, setCPU, setMem)
	if num < maxSets {
		maxSets = num
	}

	// Optional refinement with ResourceModels (if feature gate enabled)
	if features.FeatureGate.Enabled(features.CustomizedClusterResourceModeling) &&
		len(cluster.Status.ResourceSummary.AllocatableModelings) > 0 {

		num, err := getMaximumSetsBasedOnResourceModels(cluster, components)
		if err == nil && num < maxSets {
			maxSets = num
		}
	}

	return int32(maxSets)
}

// perSetRequirement computes aggregate CPU/Memory demand of one set of components.
func perSetRequirement(components []*workv1alpha2.Component) (cpuMilli int64, memBytes int64) {
	for _, c := range components {
		if c.ReplicaRequirements == nil || c.ReplicaRequirements.ResourceRequest == nil {
			continue
		}
		replicas := int64(c.Replicas)
		reqs := c.ReplicaRequirements.ResourceRequest

		if cpuQty, ok := reqs[corev1.ResourceCPU]; ok {
			cpuMilli += replicas * cpuQty.MilliValue()
		}
		if memQty, ok := reqs[corev1.ResourceMemory]; ok {
			memBytes += replicas * memQty.Value()
		}
	}
	return
}

// getMaximumSetsBasedOnClusterSummary checks aggregate resource limits.
func getMaximumSetsBasedOnClusterSummary(resourceSummary *clusterv1alpha1.ResourceSummary, setCPU, setMem int64) int64 {
	availCPU := availableQuantity(resourceSummary, corev1.ResourceCPU)
	availMem := availableQuantity(resourceSummary, corev1.ResourceMemory)

	if availCPU <= 0 || availMem <= 0 {
		return 0
	}

	switch {
	case setCPU > 0 && setMem > 0:
		return min64(availCPU/setCPU, availMem/setMem)
	case setCPU > 0:
		return availCPU / setCPU
	case setMem > 0:
		return availMem / setMem
	default:
		return math.MaxInt64
	}
}

// getMaximumSetsBasedOnResourceModels is a placeholder for future implementation.
// It should refine the maximum sets based on cluster resource models, similar
// to getMaximumReplicasBasedOnResourceModels but adapted to full component sets.
func getMaximumSetsBasedOnResourceModels(_ *clusterv1alpha1.Cluster, _ []*workv1alpha2.Component) (int64, error) {
	// TODO: implement logic based on cluster.Spec.ResourceModels
	// For now, just return MaxInt64 so it never reduces the upper bound.
	return math.MaxInt64, nil
}

// availableQuantity = allocatable - allocated - allocating
// Returns CPU in millicores, Memory in bytes.
func availableQuantity(rs *clusterv1alpha1.ResourceSummary, key corev1.ResourceName) int64 {
	allocatable, ok := rs.Allocatable[key]
	if !ok {
		return 0
	}
	allocated := rs.Allocated[key]
	allocating := rs.Allocating[key]

	allocatable.Sub(allocated)
	allocatable.Sub(allocating)

	if key == corev1.ResourceCPU {
		return allocatable.MilliValue() // millicores
	}
	return allocatable.Value() // bytes for memory, count for pods, etc.
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func getAllowedPodNumber(resourceSummary *clusterv1alpha1.ResourceSummary) int64 {
	var allocatable, allocated, allocating int64
	if resourceSummary.Allocatable != nil {
		allocatable = resourceSummary.Allocatable.Pods().Value()
	}
	if resourceSummary.Allocated != nil {
		allocated = resourceSummary.Allocated.Pods().Value()
	}
	if resourceSummary.Allocating != nil {
		allocating = resourceSummary.Allocating.Pods().Value()
	}
	allowedPodNumber := allocatable - allocated - allocating
	// When too many pods have been created, scheduling will fail so that the allocating pods number may be huge.
	// If allowedPodNumber is less than or equal to 0, we don't allow more pods to be created.
	if allowedPodNumber <= 0 {
		return 0
	}
	return allowedPodNumber
}

func convertToResourceModelsMinMap(models []clusterv1alpha1.ResourceModel) map[corev1.ResourceName][]resource.Quantity {
	resourceModelsMinMap := make(map[corev1.ResourceName][]resource.Quantity)
	for _, model := range models {
		for _, resourceModelRange := range model.Ranges {
			resourceModelsMinMap[resourceModelRange.Name] = append(resourceModelsMinMap[resourceModelRange.Name], resourceModelRange.Min)
		}
	}

	return resourceModelsMinMap
}

func getNodeAvailableReplicas(modelIndex int, replicaRequirements *workv1alpha2.ReplicaRequirements, resourceModelsMinMap map[corev1.ResourceName][]resource.Quantity) int64 {
	var maximumReplicasOneNode int64 = math.MaxInt64
	for key, value := range replicaRequirements.ResourceRequest {
		requestedQuantity := value.Value()
		if requestedQuantity <= 0 {
			continue
		}

		availableMinBoundary := resourceModelsMinMap[key][modelIndex]

		availableQuantity := availableMinBoundary.Value()
		if key == corev1.ResourceCPU {
			requestedQuantity = value.MilliValue()
			availableQuantity = availableMinBoundary.MilliValue()
		}

		maximumReplicasForResource := availableQuantity / requestedQuantity
		if maximumReplicasForResource < maximumReplicasOneNode {
			maximumReplicasOneNode = maximumReplicasForResource
		}
	}

	// if it is the first suitable model, we consider this case to be able to deploy a Pod.
	if maximumReplicasOneNode == 0 {
		return 1
	}
	return maximumReplicasOneNode
}

func getMaximumReplicasBasedOnClusterSummary(resourceSummary *clusterv1alpha1.ResourceSummary, replicaRequirements *workv1alpha2.ReplicaRequirements) int64 {
	var maximumReplicas int64 = math.MaxInt64
	for key, value := range replicaRequirements.ResourceRequest {
		requestedQuantity := value.Value()
		if requestedQuantity <= 0 {
			continue
		}

		// calculates available resource quantity
		// available = allocatable - allocated - allocating
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
		availableQuantity := allocatable.Value()
		// short path: no more resource left.
		if availableQuantity <= 0 {
			return 0
		}

		if key == corev1.ResourceCPU {
			requestedQuantity = value.MilliValue()
			availableQuantity = allocatable.MilliValue()
		}

		maximumReplicasForResource := availableQuantity / requestedQuantity
		if maximumReplicasForResource < maximumReplicas {
			maximumReplicas = maximumReplicasForResource
		}
	}

	return maximumReplicas
}

func getMaximumReplicasBasedOnResourceModels(cluster *clusterv1alpha1.Cluster, replicaRequirements *workv1alpha2.ReplicaRequirements) (int64, error) {
	resourceModelsMinMap := convertToResourceModelsMinMap(cluster.Spec.ResourceModels)

	minCompliantModelIndex := 0
	for key, value := range replicaRequirements.ResourceRequest {
		requestedQuantity := value.Value()
		if requestedQuantity <= 0 {
			continue
		}

		quantityArray, ok := resourceModelsMinMap[key]
		if !ok {
			return -1, fmt.Errorf("resource model is inapplicable as missing resource: %s", string(key))
		}

		// Find the minimum model grade for each type of resource quest, if no
		// suitable model is found indicates that there is no appropriate model
		// grade and return immediately.
		minCompliantModelIndexForResource := minimumModelIndex(quantityArray, value)
		if minCompliantModelIndexForResource == -1 {
			return 0, nil
		}
		if minCompliantModelIndex <= minCompliantModelIndexForResource {
			minCompliantModelIndex = minCompliantModelIndexForResource
		}
	}

	var maximumReplicasForResource int64
	for i := minCompliantModelIndex; i < len(cluster.Spec.ResourceModels); i++ {
		if cluster.Status.ResourceSummary.AllocatableModelings[i].Count == 0 {
			continue
		}
		maximumReplicasForResource += int64(cluster.Status.ResourceSummary.AllocatableModelings[i].Count) * getNodeAvailableReplicas(i, replicaRequirements, resourceModelsMinMap)
	}

	return maximumReplicasForResource, nil
}

func minimumModelIndex(minimumGrades []resource.Quantity, requestValue resource.Quantity) int {
	for index, minValue := range minimumGrades {
		// Suppose there is the following resource model:
		// Grade1: cpu [1C,2C)
		// Grade2: cpu [2C,3C)
		// If a Pod requests 1.5C of CPU, grade1 may not be able to provide sufficient resources,
		// so we will choose grade2.
		if minValue.Cmp(requestValue) >= 0 {
			return index
		}
	}

	return -1
}
