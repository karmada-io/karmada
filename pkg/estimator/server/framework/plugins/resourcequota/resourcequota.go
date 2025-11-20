/*
Copyright 2024 The Karmada Authors.

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

package resourcequota

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/util"
	corev1helper "github.com/karmada-io/karmada/pkg/util/lifted"
	schedcache "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/cache"
)

const (
	// Name is the name of the plugin used in Registry and configurations.
	Name                   = "ResourceQuotaEstimator"
	resourceRequestsPrefix = "requests."
	resourceLimitsPrefix   = "limits."

	// noQuotaConstraint represents the value when there is no quota constraint.
	noQuotaConstraint = math.MaxInt32
)

// resourceQuotaEstimator estimates how many replicas are allowed by the ResourceQuota constraint for a given pb.ReplicaRequirements.
// Kubernetes ResourceQuota object provides constraints that limit aggregate resource consumption per namespace.
// It is categorized into 3 types:
// 1) compute resource including cpu, memory, hugepages-<size> and extended resource like gpu
// 2) storage including pvc, ephemeral-storage etc.
// 3) object count including count/services, count/secrets etc.
// ResourceQuota supports 6 scopes:
// 1) Terminating
// 2) NotTerminating
// 3) BestEffort
// 4) NotBestEffort
// 5) PriorityClass
// 6) CrossNamespacePodAffinity
// For now, we only support ComputeResource (cpu, memory and extended resource) and PriorityClass scope. The reasons are:
// 1) pb.ReplicaRequirements only contains requested resources currently, we cannot determine its QoS (BestEffort or not)
// 2) Storage and object count quotas do not support scope selector
// 3) Pod quota is not included in this iteration to keep the initial implementation focused on compute resources
// TODO (@wengyao04): we can extend the resourceQuotaEstimator support if further requirements are needed
type resourceQuotaEstimator struct {
	enabled  bool
	rqLister corelisters.ResourceQuotaLister
}

var _ framework.EstimateReplicasPlugin = &resourceQuotaEstimator{}
var _ framework.EstimateComponentsPlugin = &resourceQuotaEstimator{}

// New initializes a new plugin and returns it.
func New(fh framework.Handle) (framework.Plugin, error) {
	enabled := features.FeatureGate.Enabled(features.ResourceQuotaEstimate)
	if !enabled {
		// Disabled, won't do anything.
		return &resourceQuotaEstimator{}, nil
	}
	return &resourceQuotaEstimator{
		enabled:  enabled,
		rqLister: fh.SharedInformerFactory().Core().V1().ResourceQuotas().Lister(),
	}, nil
}

// Name returns name of the plugin. It is used in logs, etc.
func (pl *resourceQuotaEstimator) Name() string {
	return Name
}

// Estimate estimates the replicas allowed by the ResourceQuota constraints.
func (pl *resourceQuotaEstimator) Estimate(_ context.Context, _ *schedcache.Snapshot, replicaRequirements *pb.ReplicaRequirements) (int32, *framework.Result) {
	var replica int32 = noQuotaConstraint
	if !pl.enabled {
		klog.V(5).Info("Estimator Plugin", "name", Name, "enabled", pl.enabled)
		return replica, framework.NewResult(framework.Noopperation, fmt.Sprintf("%s is disabled", pl.Name()))
	}
	namespace := replicaRequirements.Namespace
	priorityClassName := replicaRequirements.PriorityClassName

	rqList, err := pl.rqLister.ResourceQuotas(namespace).List(labels.Everything())
	if err != nil {
		klog.Error(err, "failed to list resource quota", "namespace", namespace)
		// Conservative approach: return 0 replicas on error
		return 0, framework.AsResult(err)
	}
	for _, rq := range rqList {
		rqEvaluator := newResourceQuotaEvaluator(rq, priorityClassName)
		replicaFromRqEvaluator := rqEvaluator.evaluate(replicaRequirements)
		if replicaFromRqEvaluator < replica {
			replica = replicaFromRqEvaluator
		}
	}

	var result *framework.Result
	switch replica {
	case noQuotaConstraint:
		// No quota constraints found - resources are schedulable without restrictions
		result = framework.NewResult(framework.Success, fmt.Sprintf("%s found no quota constraints", pl.Name()))
	case 0:
		result = framework.NewResult(framework.Unschedulable, fmt.Sprintf("zero replica is estimated by %s", pl.Name()))
	default:
		result = framework.NewResult(framework.Success)
	}
	return replica, result
}

// EstimateComponents estimates the maximum component sets allowed by ResourceQuota constraints.
// For each ResourceQuota in the namespace, it filters the components that match the quota's scope
// selectors (e.g., priorityClassName), aggregates their resource requirements, and calculates how
// many complete component sets can fit within the quota. The function returns the minimum allowed
// sets across all ResourceQuotas to ensure all quota constraints are satisfied.
func (pl *resourceQuotaEstimator) EstimateComponents(_ context.Context, _ *schedcache.Snapshot, components []pb.Component, namespace string) (int32, *framework.Result) {
	if !pl.enabled {
		klog.V(5).Info("Estimator Plugin", "name", Name, "enabled", pl.enabled)
		return noQuotaConstraint, framework.NewResult(framework.Noopperation, fmt.Sprintf("%s is disabled", pl.Name()))
	}

	if len(components) == 0 {
		klog.V(5).Infof("%s: components list is empty, skipping resource quota check", pl.Name())
		return noQuotaConstraint, framework.NewResult(framework.Success, fmt.Sprintf("%s received empty components list", pl.Name()))
	}

	if namespace == "" {
		klog.V(5).Infof("%s: namespace is empty, skipping resource quota check", pl.Name())
		return noQuotaConstraint, framework.NewResult(framework.Success)
	}

	rqList, err := pl.rqLister.ResourceQuotas(namespace).List(labels.Everything())
	if err != nil {
		klog.Error(err, "failed to list resource quota", "namespace", namespace)
		// Conservative approach: return 0 component sets on error
		return 0, framework.AsResult(err)
	}

	if len(rqList) == 0 {
		klog.V(5).Infof("%s: no ResourceQuota found in namespace %s", pl.Name(), namespace)
		// No quotas exist - resources are schedulable without restrictions
		return noQuotaConstraint, framework.NewResult(framework.Success, fmt.Sprintf("%s found no quota constraints", pl.Name()))
	}

	// Evaluate all components together against each ResourceQuota.
	// Each ResourceQuota will filter components based on its scope selectors.
	var maxSets int32 = noQuotaConstraint
	for _, rq := range rqList {
		setsFromRq := pl.evaluateComponentsAgainstQuota(rq, components)

		klog.V(5).Infof("%s: ResourceQuota %s/%s allows %d component sets",
			pl.Name(), rq.Namespace, rq.Name, setsFromRq)

		if setsFromRq < maxSets {
			maxSets = setsFromRq
		}

		// Early exit if any quota allows zero sets.
		if maxSets == 0 {
			break
		}
	}

	// Determine the result based on the final maxSets value.
	var result *framework.Result
	switch maxSets {
	case noQuotaConstraint:
		// No quota constraints found - resources are schedulable without restrictions
		result = framework.NewResult(framework.Success, fmt.Sprintf("%s found no quota constraints", pl.Name()))
	case 0:
		result = framework.NewResult(framework.Unschedulable, fmt.Sprintf("zero component sets estimated by %s", pl.Name()))
	default:
		result = framework.NewResult(framework.Success)
	}

	klog.V(5).Infof("%s: final estimation result: %d component sets, status: %s",
		pl.Name(), maxSets, result.Code())

	return maxSets, result
}

// evaluateComponentsAgainstQuota evaluates all components against a single ResourceQuota.
//
// Steps:
//  1. Filter components that match this quota's scope selectors
//  2. If no components match → quota doesn't constrain workload → return noQuotaConstraint
//  3. Aggregate resource requirements for one complete component set
//  4. Calculate: sets = quota.available / aggregated_requirements
func (pl *resourceQuotaEstimator) evaluateComponentsAgainstQuota(rq *corev1.ResourceQuota,
	components []pb.Component) int32 {
	selectors := getScopeSelectorsFromQuota(rq)
	var matchedComponents []pb.Component

	if len(selectors) == 0 {
		matchedComponents = components
	} else {
		for _, component := range components {
			if quotaAppliesToPriority(selectors, component.ReplicaRequirements.PriorityClassName) {
				matchedComponents = append(matchedComponents, component)
			}
		}
	}

	if len(matchedComponents) == 0 {
		return noQuotaConstraint
	}

	availableResources := calculateFreeResources(rq, matchingResources(resourceNames(rq.Status.Hard)))
	if len(availableResources) == 0 {
		return noQuotaConstraint
	}

	perSetRequirements := pl.aggregateComponentRequirements(matchedComponents)
	return pl.evaluateResourcesAgainstQuota(availableResources, perSetRequirements)
}

func quotaAppliesToPriority(selectors []corev1.ScopedResourceSelectorRequirement, priorityClassName string) bool {
	for _, selector := range selectors {
		matchScope, err := matchesScope(selector, priorityClassName)
		if err != nil {
			klog.Error(err, "matchesScope failed")
			continue
		}
		if matchScope {
			return true
		}
	}
	return false
}

// aggregateComponentRequirements computes the total resource requirements for one complete
// component set by summing up each component's per-replica requirements multiplied by its replica count.
func (pl *resourceQuotaEstimator) aggregateComponentRequirements(components []pb.Component) corev1.ResourceList {
	resourceRequirements := map[corev1.ResourceName]int64{}

	for _, component := range components {
		if component.ReplicaRequirements.ResourceRequest == nil {
			continue
		}

		replicas := int64(component.Replicas)
		for resourceName, perReplicaQuantity := range component.ReplicaRequirements.ResourceRequest {
			// Only process supported compute resources (CPU, Memory) and extended resources (GPU, etc.)
			if !isSupportedResource(resourceName) {
				continue
			}

			// CPU uses MilliValue, all others use Value
			var perReplicaAmount int64
			if resourceName == corev1.ResourceCPU {
				perReplicaAmount = perReplicaQuantity.MilliValue()
			} else {
				perReplicaAmount = perReplicaQuantity.Value()
			}
			resourceRequirements[resourceName] += perReplicaAmount * replicas
		}
	}

	return convertToResourceList(resourceRequirements)
}

// evaluateResourcesAgainstQuota calculates how many complete sets of the required resources
// can fit within the available quota resources using resource-agnostic division.
func (pl *resourceQuotaEstimator) evaluateResourcesAgainstQuota(
	availableResources corev1.ResourceList,
	perSetRequirements corev1.ResourceList) int32 {
	// Filter to only include resources constrained by this quota
	filtered := filterConstrainedResources(availableResources, perSetRequirements)

	if len(filtered) == 0 {
		return noQuotaConstraint
	}

	// Create a Resource object with available quota
	availableResource := util.NewResource(availableResources)
	availableResource.AllowedPodNumber = math.MaxInt64 // Pod quota not supported yet

	// Calculate how many sets can fit using resource-agnostic division
	allowed := availableResource.MaxDivided(filtered)

	// Handle integer overflow: treat very large numbers as no constraint
	if allowed > math.MaxInt32 {
		return noQuotaConstraint
	}

	return int32(allowed) // #nosec G115: integer overflow conversion int64 -> int32
}

type resourceQuotaEvaluator struct {
	// key (string) is the name of the resource quota
	// value (ResourceList) is the free resources calculated by (hard - used) from the resource quota status
	resourceRequest map[string]corev1.ResourceList
}

func newResourceQuotaEvaluator(rq *corev1.ResourceQuota, priorityClassName string) *resourceQuotaEvaluator {
	selectors := getScopeSelectorsFromQuota(rq)
	resources := make(map[string]corev1.ResourceList)

	// Determine if this quota applies based on scope selectors
	quotaApplies := false
	if len(selectors) == 0 {
		// If there are no scope selectors, the quota applies to all pods
		quotaApplies = true
	} else {
		quotaApplies = quotaAppliesToPriority(selectors, priorityClassName)
	}

	// If the quota applies, extract the free resources
	if quotaApplies {
		matchResource := matchingResources(resourceNames(rq.Status.Hard))
		if len(matchResource) != 0 {
			freeResource := calculateFreeResources(rq, matchResource)
			resources[rq.Name] = freeResource
		}
	}

	return &resourceQuotaEvaluator{
		resourceRequest: resources,
	}
}

// evaluate evaluates resource requirements against all applicable ResourceQuotas
// and returns the most restrictive constraint (minimum allowed replicas).
func (e *resourceQuotaEvaluator) evaluate(replicaRequirements *pb.ReplicaRequirements) int32 {
	var result int32 = noQuotaConstraint

	for _, availableResources := range e.resourceRequest {
		// Filter to only include resources that are constrained by this ResourceQuota
		filteredRequirements := filterConstrainedResources(availableResources, replicaRequirements.ResourceRequest)

		// If no resources are constrained by this quota, skip it
		if len(filteredRequirements) == 0 {
			continue
		}

		// Create a Resource object with available quota
		availableResource := util.NewResource(availableResources)

		// Pod quota is not supported in the current implementation.
		// To add pod quota support in the future:
		// 1. Include pod count in aggregateComponentRequirements()
		// 2. Remove this line to let AllowedPodNumber be calculated from availableResources
		// 3. Add test cases for pod quota constraints
		availableResource.AllowedPodNumber = math.MaxInt64

		// Calculate how many replicas/sets can fit within the quota
		allowed := availableResource.MaxDivided(filteredRequirements)

		// Handle integer overflow: treat very large numbers as no constraint for this quota
		if allowed > math.MaxInt32 {
			continue
		}

		// Take the minimum across all ResourceQuotas (most restrictive)
		count := int32(allowed) // #nosec G115: integer overflow conversion int64 -> int32
		if count < result {
			result = count
		}
	}

	return result
}

// filterConstrainedResources returns only the resources from requirements that are actually
// constrained by the given ResourceQuota (i.e., present in availableResources).
func filterConstrainedResources(
	availableResources corev1.ResourceList,
	requirements corev1.ResourceList) corev1.ResourceList {
	filtered := corev1.ResourceList{}
	for resourceName, requirement := range requirements {
		if _, ok := availableResources[resourceName]; ok {
			filtered[resourceName] = requirement
		}
	}
	return filtered
}

// isSupportedResource checks if a resource is supported for quota evaluation.
// This function is called on resource names from component.ReplicaRequirements.ResourceRequest,
// which are expected to be unprefixed (e.g., "cpu", "memory", not "requests.cpu").
// Supported resources include:
// - CPU and Memory (unprefixed only)
// - Extended resources (GPU, custom devices, etc.)
// Unsupported resources (storage, object counts) are filtered out.
func isSupportedResource(resourceName corev1.ResourceName) bool {
	// Supported standard compute resources (unprefixed only).
	if resourceName == corev1.ResourceCPU || resourceName == corev1.ResourceMemory {
		return true
	}

	// Check if it's an extended resource (e.g., nvidia.com/gpu).
	if corev1helper.IsExtendedResourceName(resourceName) {
		return true
	}

	return false
}

// convertToResourceList converts a map of int64 values to a ResourceList with appropriate formats.
// Only handles supported compute resources: CPU (milli-units), Memory (bytes), and Extended Resources (GPU, etc.).
func convertToResourceList(resourceRequirements map[corev1.ResourceName]int64) corev1.ResourceList {
	result := corev1.ResourceList{}
	for resourceName, totalAmount := range resourceRequirements {
		switch resourceName {
		case corev1.ResourceCPU:
			// CPU is stored in millicores, use NewMilliQuantity
			result[resourceName] = *resource.NewMilliQuantity(totalAmount, resource.DecimalSI)
		case corev1.ResourceMemory:
			// Memory uses binary units (Ki, Mi, Gi)
			result[resourceName] = *resource.NewQuantity(totalAmount, resource.BinarySI)
		default:
			// All other supported resources are extended resources (GPU, etc.) using decimal format
			result[resourceName] = *resource.NewQuantity(totalAmount, resource.DecimalSI)
		}
	}
	return result
}

// calculateFreeResources calculates the free resources from the input ResourceQuota.
// It only calculates the free resources that are present in resourceNames.
func calculateFreeResources(rq *corev1.ResourceQuota, resourceNames []corev1.ResourceName) corev1.ResourceList {
	hardResourceList := corev1.ResourceList{}
	usedResourceList := corev1.ResourceList{}
	for _, resourceName := range resourceNames {
		rNameStr := string(resourceName)
		// skip limits because pb.ReplicaRequirements only supports requested resources
		if strings.HasPrefix(rNameStr, resourceLimitsPrefix) {
			continue
		}
		// requests.cpu is same as cpu
		// requests.memory is same as memory
		// we merge them together
		trimmedResourceName := corev1.ResourceName(strings.TrimPrefix(rNameStr, resourceRequestsPrefix))
		hardResource, hardResourceOk := rq.Status.Hard[resourceName]
		usedResource, usedResourceOk := rq.Status.Used[resourceName]
		if !hardResourceOk || !usedResourceOk {
			continue
		}
		hardResourceList[trimmedResourceName] = hardResource
		usedResourceList[trimmedResourceName] = usedResource
	}

	freeResourceList := corev1.ResourceList{}
	for resourceName, hard := range hardResourceList {
		if used, ok := usedResourceList[resourceName]; ok {
			hard.Sub(used)
			freeResourceList[resourceName] = hard
		}
	}
	return freeResourceList
}

// resourceNames returns a list of all resource names in the ResourceList
func resourceNames(resources corev1.ResourceList) []corev1.ResourceName {
	result := []corev1.ResourceName{}
	for resourceName := range resources {
		result = append(result, resourceName)
	}
	return result
}

func getScopeSelectorsFromQuota(quota *corev1.ResourceQuota) []corev1.ScopedResourceSelectorRequirement {
	selectors := []corev1.ScopedResourceSelectorRequirement{}
	for _, scope := range quota.Spec.Scopes {
		selectors = append(selectors, corev1.ScopedResourceSelectorRequirement{
			ScopeName: scope,
			Operator:  corev1.ScopeSelectorOpExists})
	}
	if quota.Spec.ScopeSelector != nil {
		selectors = append(selectors, quota.Spec.ScopeSelector.MatchExpressions...)
	}
	return selectors
}

// matchesScope evaluates whether the replica requirements match a ResourceQuota scope.
// Currently, only PriorityClass scope is supported.
func matchesScope(selector corev1.ScopedResourceSelectorRequirement, priorityClassName string) (bool, error) {
	switch selector.ScopeName {
	case corev1.ResourceQuotaScopeTerminating:
		return false, nil
	case corev1.ResourceQuotaScopeNotTerminating:
		return false, nil
	case corev1.ResourceQuotaScopeBestEffort:
		return false, nil
	case corev1.ResourceQuotaScopeNotBestEffort:
		return false, nil
	case corev1.ResourceQuotaScopePriorityClass:
		if selector.Operator == corev1.ScopeSelectorOpExists {
			// This is just checking for existence of a priorityClass,
			// no need to take the overhead of selector parsing/evaluation.
			return len(priorityClassName) != 0, nil
		}
		return matchesSelector(priorityClassName, selector)
	case corev1.ResourceQuotaScopeCrossNamespacePodAffinity:
		return false, nil
	default:
		// Unrecognized scope - this may indicate a new Kubernetes ResourceQuota scope was introduced
		klog.Warning("Unrecognized scope name of resource quota", "scope name", selector.ScopeName)
		return false, nil
	}
}

func matchesSelector(priorityClassName string, selector corev1.ScopedResourceSelectorRequirement) (bool, error) {
	labelSelector, err := scopedResourceSelectorRequirementsAsSelector(selector)
	if err != nil {
		return false, fmt.Errorf("failed to parse and convert selector: %v", err)
	}
	var m map[string]string
	if len(priorityClassName) != 0 {
		m = map[string]string{string(corev1.ResourceQuotaScopePriorityClass): priorityClassName}
	}
	if labelSelector.Matches(labels.Set(m)) {
		return true, nil
	}
	return false, nil
}

// scopedResourceSelectorRequirementsAsSelector converts the ScopedResourceSelectorRequirement api type into a struct that implements
// labels.Selector.
func scopedResourceSelectorRequirementsAsSelector(ssr corev1.ScopedResourceSelectorRequirement) (labels.Selector, error) {
	selector := labels.NewSelector()
	var op selection.Operator
	switch ssr.Operator {
	case corev1.ScopeSelectorOpIn:
		op = selection.In
	case corev1.ScopeSelectorOpNotIn:
		op = selection.NotIn
	case corev1.ScopeSelectorOpExists:
		op = selection.Exists
	case corev1.ScopeSelectorOpDoesNotExist:
		op = selection.DoesNotExist
	default:
		return nil, fmt.Errorf("%q is not a valid scope selector operator", ssr.Operator)
	}
	r, err := labels.NewRequirement(string(ssr.ScopeName), op, ssr.Values)
	if err != nil {
		return nil, err
	}
	selector = selector.Add(*r)
	return selector, nil
}

// computeResources are the set of standard compute resources managed by quota associated with pods.
// This includes CPU and Memory in their base and prefixed (requests./limits.) forms.
// Extended resources (e.g., nvidia.com/gpu) are handled separately via IsExtendedResourceName check.
// Storage resources (ephemeral-storage, PVC) and object count quotas are not supported.
var computeResources = []corev1.ResourceName{
	corev1.ResourceCPU,
	corev1.ResourceMemory,
	corev1.ResourceRequestsCPU,
	corev1.ResourceRequestsMemory,
	corev1.ResourceLimitsCPU,
	corev1.ResourceLimitsMemory,
}

// matchingResources takes the input specified list of resources and returns the set of resources it matches.
func matchingResources(input []corev1.ResourceName) []corev1.ResourceName {
	result := intersection(input, computeResources)

	for _, resourceName := range input {
		// add extended resources
		if corev1helper.IsExtendedResourceName(resourceName) {
			result = append(result, resourceName)
		} else if strings.HasPrefix(string(resourceName), resourceRequestsPrefix) {
			trimmedResourceName := corev1.ResourceName(strings.TrimPrefix(string(resourceName), resourceRequestsPrefix))
			if corev1helper.IsExtendedResourceName(trimmedResourceName) {
				result = append(result, resourceName)
			}
		} else if strings.HasPrefix(string(resourceName), resourceLimitsPrefix) {
			trimmedResourceName := corev1.ResourceName(strings.TrimPrefix(string(resourceName), resourceLimitsPrefix))
			if corev1helper.IsExtendedResourceName(trimmedResourceName) {
				result = append(result, resourceName)
			}
		}
	}
	return result
}

// intersection returns the intersection of both list of resources, deduped and sorted
func intersection(a []corev1.ResourceName, b []corev1.ResourceName) []corev1.ResourceName {
	result := make([]corev1.ResourceName, 0, len(a))
	for _, item := range a {
		if contains(result, item) {
			continue
		}
		if !contains(b, item) {
			continue
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

// contains returns true if the specified item is in the list of items
func contains(items []corev1.ResourceName, item corev1.ResourceName) bool {
	for _, i := range items {
		if i == item {
			return true
		}
	}
	return false
}
