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
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/estimator"
	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/util"
	schedulerframework "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework"
)

// maxPodsCountPerNode defines the maximum pods count per node.
// This is the default and recommended value set by Kubernetes for nodes.
// More details can be found at: https://kubernetes.io/docs/setup/best-practices/cluster-large/
const maxPodsCountPerNode = 110

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
	// Note: resourceSummary must be deep-copied before using in the function to avoid modifying the original data structure.
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
func (ge *GeneralEstimator) MaxAvailableComponentSets(_ context.Context, req ComponentSetEstimationRequest) ([]ComponentSetEstimationResponse, error) {
	responses := make([]ComponentSetEstimationResponse, len(req.Clusters))
	for i, cluster := range req.Clusters {
		maxComponentSets := ge.maxAvailableComponentSets(cluster, req.Components)
		responses[i] = ComponentSetEstimationResponse{Name: cluster.Name, Sets: maxComponentSets}
	}
	return responses, nil
}

func (ge *GeneralEstimator) maxAvailableComponentSets(cluster *clusterv1alpha1.Cluster, components []workv1alpha2.Component) int32 {
	resourceSummary := cluster.Status.ResourceSummary.DeepCopy()
	if resourceSummary == nil {
		return 0
	}

	// Aggregate per-set resource requirements
	perSet := perSetRequirement(components)

	// Check pod constraint
	available := availableResourceMap(resourceSummary)
	allowedPods := getAllowedPodNumber(resourceSummary)
	if allowedPods <= 0 {
		return 0
	}

	podsPerSet := podsInSet(components)
	if podsPerSet <= 0 {
		// No components or resources are defined, return max pod allowance as estimate
		return int32(allowedPods) // #nosec G115: integer overflow conversion int64 -> int32
	}

	podBound := int32(allowedPods / podsPerSet) // #nosec G115: integer overflow conversion int64 -> int32
	if len(perSet) == 0 || allZero(perSet) {
		return podBound
	}

	// Find limiting resource requirement, which will bound maxSet calculation
	maxSets := podBound
	for resName, req := range perSet {
		if req <= 0 {
			continue
		}

		resAvail := available[resName]
		if resAvail <= 0 {
			return 0 // no capacity for this resource
		}

		resBound := int32(resAvail / req) // #nosec G115: integer overflow conversion int64 -> int32
		if resBound < maxSets {
			maxSets = resBound
		}
	}

	if features.FeatureGate.Enabled(features.CustomizedClusterResourceModeling) && len(cluster.Status.ResourceSummary.AllocatableModelings) > 0 {
		if num, err := getMaximumSetsBasedOnResourceModels(cluster, components, maxSets); err != nil {
			klog.Warningf("Failed to get maximum sets based on resource models, skipping: %v", err)
		} else if num < maxSets {
			maxSets = num
		}
	}

	return maxSets
}

// getMaximumSetsBasedOnResourceModels computes the maximum number of full sets that can be
// placed on a cluster using the cluster's ResourceModels. It expands one set into
// replica kinds (demand + count) and performs a first-fit-decreasing placement onto model-grade nodes.
// `upperBound` caps the search. We can set this using the podBound (allowedPods / podsPerSet)
func getMaximumSetsBasedOnResourceModels(cluster *clusterv1alpha1.Cluster, components []workv1alpha2.Component, upperBound int32) (int32, error) {
	nodes, err := buildModelNodes(cluster)
	if err != nil {
		return -1, err
	}

	pbComponents := make([]pb.Component, 0, len(components))
	for _, comp := range components {
		pbComponents = append(pbComponents, pb.Component{
			Name:                comp.Name,
			Replicas:            comp.Replicas,
			ReplicaRequirements: toPBReplicaRequirements(comp.ReplicaRequirements),
		})
	}

	return estimator.NewSchedulingSimulator(nodes).SimulateScheduling(pbComponents, upperBound), nil
}

// buildModelNodes constructs identical nodes for each model grade using its Min vector,
// repeated AllocatableModelings[grade].Count times. Grades are indexed directly.
func buildModelNodes(cluster *clusterv1alpha1.Cluster) ([]*schedulerframework.NodeInfo, error) {
	if cluster == nil {
		return nil, fmt.Errorf("nil cluster")
	}
	if cluster.Status.ResourceSummary == nil {
		return nil, fmt.Errorf("resource summary is nil")
	}
	spec := cluster.Spec.ResourceModels
	allocs := cluster.Status.ResourceSummary.AllocatableModelings
	if len(spec) == 0 {
		return nil, fmt.Errorf("no resource models defined")
	}

	// Build capacity template per grade
	capsByGrade := make(map[uint]corev1.ResourceList, len(spec))
	for _, m := range spec {
		tmpl := make(corev1.ResourceList, len(m.Ranges))
		for _, r := range m.Ranges {
			tmpl[r.Name] = r.Min
		}

		// The number of pods a node can accommodate is a critical metric in scheduling simulation. Each node must have
		// a pod capacity limit; otherwise, the scheduler cannot determine if a node can accommodate any pod. Kubernetes
		// sets a default pod limit of 110 per node, which we adopt here for consistency.
		//
		// Note: This virtual node uses a fixed pod capacity of 110 for each simulation. The actual pod count already
		// running on this virtual node is not considered in the calculation. This means the estimation may be inaccurate
		// and could potentially exceed 110 pods in practice. This is an inherent limitation of the ResourceModel mechanism,
		// as the estimation is approximate rather than precise.
		tmpl[corev1.ResourcePods] = *resource.NewQuantity(maxPodsCountPerNode, resource.DecimalSI)
		capsByGrade[m.Grade] = tmpl
	}

	// Accumulate counts by grade
	countByGrade := make(map[uint]int, len(allocs))
	for _, a := range allocs {
		if a.Count < 0 {
			return nil, fmt.Errorf("negative node count for grade %d", a.Grade)
		}
		countByGrade[a.Grade] += a.Count
	}

	// Collect grades and sort, so that order of nodes is
	grades := make([]int, 0, len(capsByGrade))
	for g := range capsByGrade {
		grades = append(grades, int(g)) // #nosec G115: integer overflow conversion uint -> int
	}
	sort.Ints(grades)

	// Emit nodes for grades present in both spec & status.
	var nodes []*schedulerframework.NodeInfo
	for _, grade := range grades {
		tmpl, cnt := capsByGrade[uint(grade)], countByGrade[uint(grade)] // #nosec G115: integer overflow conversion int -> uint
		if tmpl == nil || cnt == 0 {
			continue
		}

		for range cnt {
			node := &schedulerframework.NodeInfo{
				Allocatable: util.NewResource(tmpl),
			}
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

// podsInSet computes the total number of pods in the CRD
func podsInSet(components []workv1alpha2.Component) int64 {
	var sum int64
	for _, c := range components {
		sum += int64(c.Replicas)
	}
	return sum
}

// perSetRequirement computes the aggregate resource(such as CPU, Memory, GPU, etc) demand of one set of components.
func perSetRequirement(components []workv1alpha2.Component) map[corev1.ResourceName]int64 {
	resourceRequirements := map[corev1.ResourceName]int64{}
	for _, c := range components {
		if c.ReplicaRequirements == nil || c.ReplicaRequirements.ResourceRequest == nil {
			continue
		}
		replicas := int64(c.Replicas)
		for resName, qty := range c.ReplicaRequirements.ResourceRequest {
			baseAmount := quantityAsInt64(qty)
			resourceRequirements[resName] += baseAmount * replicas
		}
	}
	return resourceRequirements
}

// availableResourceMap parses the cluster resourceSummary and returns map of resourceName -> availableQuantity (int64)
func availableResourceMap(resourceSummary *clusterv1alpha1.ResourceSummary) map[corev1.ResourceName]int64 {
	available := make(map[corev1.ResourceName]int64, len(resourceSummary.Allocatable))
	for key, allocatable := range resourceSummary.Allocatable {
		a := allocatable.DeepCopy()
		if allocated, ok := resourceSummary.Allocated[key]; ok {
			a.Sub(allocated)
		}
		if allocating, ok := resourceSummary.Allocating[key]; ok {
			a.Sub(allocating)
		}
		available[key] = quantityAsInt64(a)
	}
	return available
}

// Converts quantity into an int representation depending on format
func quantityAsInt64(q resource.Quantity) int64 {
	switch q.Format {
	case resource.DecimalSI, resource.DecimalExponent:
		return q.MilliValue()
	case resource.BinarySI:
		return q.Value()
	default:
		return q.Value()
	}
}

func allZero(m map[corev1.ResourceName]int64) bool {
	for _, v := range m {
		if v != 0 {
			return false
		}
	}
	return true
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
