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
)

// resourceQuotaEstimator is to estimate how many replica allowed by the ResourceQuota constrain for a given pb.ReplicaRequirements
// Kubernetes ResourceQuota object provides constraints that limit aggregate resource consumption per namespace
// It is categorized into 3 types:
// 1) compute resource including cpu, memory, hugepages-<size> and extended resource like gpu
// 2) storage including pvc, ephemeral-storage etc.
// 3) object count including count/services, count/secrets etc.
// ResourceQuota supports 6 scope
// 1) Terminating
// 2) NotTerminating
// 3) BestEffort
// 4) NotBestEffort
// 5) PriorityClass
// 6) CrossNamespacePodAffinity
// For now, we only support ComputeResource (cpu, memory and extended resource) and PriorityClass scope, the reasons are
// 1) pb.ReplicaRequirements only contains requested resources currently, we cannot determine if its QoS (BestEffort or not)
// 2) storage and object count do not support scope selector
// ToDo (@wengyao04): we can extend the resourceQuotaEstimator support if further requirements are needed
type resourceQuotaEstimator struct {
	enabled  bool
	rqLister corelisters.ResourceQuotaLister
}

var _ framework.EstimateReplicasPlugin = &resourceQuotaEstimator{}

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

// Estimate the replica allowed by the ResourceQuota
func (pl *resourceQuotaEstimator) Estimate(_ context.Context,
	_ *schedcache.Snapshot,
	replicaRequirements *pb.ReplicaRequirements) (int32, *framework.Result) {
	var replica int32 = math.MaxInt32
	if !pl.enabled {
		klog.V(5).Info("Estimator Plugin", "name", Name, "enabled", pl.enabled)
		return replica, framework.NewResult(framework.Noopperation, fmt.Sprintf("%s is disabled", pl.Name()))
	}
	namespace := replicaRequirements.Namespace
	priorityClassName := replicaRequirements.PriorityClassName

	rqList, err := pl.rqLister.ResourceQuotas(namespace).List(labels.Everything())
	if err != nil {
		klog.Error(err, "fail to list resource quota", "namespace", namespace)
		return replica, framework.AsResult(err)
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
	case math.MaxInt32:
		result = framework.NewResult(framework.Noopperation, fmt.Sprintf("%s has no operation on input replicaRequirements", pl.Name()))
	case 0:
		result = framework.NewResult(framework.Unschedulable, fmt.Sprintf("zero replica is estimated by %s", pl.Name()))
	default:
		result = framework.NewResult(framework.Success)
	}
	return replica, result
}

type resourceQuotaEvaluator struct {
	//key (string) is the name of the resource quota
	//value (ResourceList) is the free resources calculated by (hard - used) from the resource quota status
	resourceRequest map[string]corev1.ResourceList
}

func newResourceQuotaEvaluator(rq *corev1.ResourceQuota, priorityClassName string) *resourceQuotaEvaluator {
	selectors := getScopeSelectorsFromQuota(rq)
	resources := make(map[string]corev1.ResourceList)

	for _, selector := range selectors {
		matchScope, err := matchesScope(selector, priorityClassName)
		if err != nil {
			klog.Error(err, "matchesScope failed")
			continue
		}
		if !matchScope {
			continue
		}
		matchResource := matchingResources(resourceNames(rq.Status.Hard))
		if len(matchResource) != 0 {
			resource := calculateFreeResources(rq, matchResource)
			resources[rq.Name] = resource
			break
		}
	}
	return &resourceQuotaEvaluator{
		resourceRequest: resources,
	}
}

func (e *resourceQuotaEvaluator) evaluate(replicaRequirements *pb.ReplicaRequirements) int32 {
	var result int32 = math.MaxInt32
	for _, resourceList := range e.resourceRequest {
		filteredRequiredResourceList := corev1.ResourceList{}
		// If the resource in pb.ReplicaRequirements is not the ResourceQuota ResourceList, we skip it.
		for resourceName, request := range replicaRequirements.ResourceRequest {
			if _, ok := resourceList[resourceName]; ok {
				filteredRequiredResourceList[resourceName] = request
			}
		}
		resource := util.NewResource(resourceList)
		resource.AllowedPodNumber = math.MaxInt64
		allowed := resource.MaxDivided(filteredRequiredResourceList)
		// continue the loop to avoid integer overflow
		if allowed > math.MaxInt32 {
			continue
		}

		replica := int32(allowed) // #nosec G115: integer overflow conversion int64 -> int32
		if replica < result {
			result = replica
		}
	}
	return result
}

// calculateFreeResources calculates the free resources from input resource quota
// it only calculates the free resources that in resourceNames
func calculateFreeResources(rq *corev1.ResourceQuota, resourceNames []corev1.ResourceName) corev1.ResourceList {
	hardResourceList := corev1.ResourceList{}
	usedResourceList := corev1.ResourceList{}
	for _, resourceName := range resourceNames {
		rNameStr := string(resourceName)
		//skip limits because pb.ReplicaRequirements only support requested resource
		if strings.HasPrefix(rNameStr, resourceLimitsPrefix) {
			continue
		}
		//requests.cpu is same as cpu
		//requests.memory is same as memory
		//we merge them together
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

// matchesScope is a function that knows how to evaluate if a pod matches a scope
// we only support PriorityClass scope now.
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
			// This is just checking for existence of a priorityClass on the pod,
			// no need to take the overhead of selector parsing/evaluation.
			return len(priorityClassName) != 0, nil
		}
		return matchesSelector(priorityClassName, selector)
	case corev1.ResourceQuotaScopeCrossNamespacePodAffinity:
		return false, nil
	default:
		// hit here means Kubernetes introduced a new resource quota scope
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

// computeResources are the set of resources managed by quota associated with pods.
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

	for _, resource := range input {
		// add extended resources
		if corev1helper.IsExtendedResourceName(resource) {
			result = append(result, resource)
		} else if strings.HasPrefix(string(resource), resourceRequestsPrefix) {
			trimmedResourceName := corev1.ResourceName(strings.TrimPrefix(string(resource), resourceRequestsPrefix))
			if corev1helper.IsExtendedResourceName(trimmedResourceName) {
				result = append(result, resource)
			}
		} else if strings.HasPrefix(string(resource), resourceLimitsPrefix) {
			trimmedResourceName := corev1.ResourceName(strings.TrimPrefix(string(resource), resourceLimitsPrefix))
			if corev1helper.IsExtendedResourceName(trimmedResourceName) {
				result = append(result, resource)
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
