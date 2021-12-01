package core

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/apis/core/v1/helper"

	quotav1alpha1 "github.com/karmada-io/karmada/pkg/apis/quota/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	quota "github.com/karmada-io/karmada/pkg/util/quota/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/quota/v1alpha1/generic"
)

// the name used for object count quota
var podObjectCountName = generic.ObjectCountQuotaResourceNameFor(corev1.SchemeGroupVersion.WithResource("pods").GroupResource())

var supportedQoSComputeResources = sets.NewString(string(corev1.ResourceCPU), string(corev1.ResourceMemory))

// podResources are the set of resources managed by quota associated with pods.
var podResources = []corev1.ResourceName{
	podObjectCountName,
	corev1.ResourceCPU,
	corev1.ResourceMemory,
	corev1.ResourceEphemeralStorage,
	corev1.ResourceRequestsCPU,
	corev1.ResourceRequestsMemory,
	corev1.ResourceRequestsEphemeralStorage,
	corev1.ResourceLimitsCPU,
	corev1.ResourceLimitsMemory,
	corev1.ResourceLimitsEphemeralStorage,
	corev1.ResourcePods,
}

// podResourcePrefixes are the set of prefixes for resources (Hugepages, and other
// potential extended resources with specific prefix) managed by quota associated with pods.
var podResourcePrefixes = []string{
	corev1.ResourceHugePagesPrefix,
	corev1.ResourceRequestsHugePagesPrefix,
}

// requestedResourcePrefixes are the set of prefixes for resources
// that might be declared in pod's Resources.Requests/Limits
var requestedResourcePrefixes = []string{
	corev1.ResourceHugePagesPrefix,
}

// maskResourceWithPrefix mask resource with certain prefix
// e.g. hugepages-XXX -> requests.hugepages-XXX
func maskResourceWithPrefix(resource corev1.ResourceName, prefix string) corev1.ResourceName {
	return corev1.ResourceName(fmt.Sprintf("%s%s", prefix, string(resource)))
}

// isExtendedResourceNameForQuota returns true if the extended resource name
// has the quota related resource prefix.
func isExtendedResourceNameForQuota(name corev1.ResourceName) bool {
	// As overcommit is not supported by extended resources for now,
	// only quota objects in format of "requests.resourceName" is allowed.
	return !helper.IsNativeResource(corev1.ResourceName(name)) && strings.HasPrefix(string(name), corev1.DefaultResourceRequestsPrefix)
}

// NOTE: it was a mistake, but if a quota tracks cpu or memory related resources,
// the incoming pod is required to have those values set.  we should not repeat
// this mistake for other future resources (gpus, ephemeral-storage,etc).
// do not add more resources to this list!
var validationSet = sets.NewString(
	string(corev1.ResourceCPU),
	string(corev1.ResourceMemory),
	string(corev1.ResourceRequestsCPU),
	string(corev1.ResourceRequestsMemory),
	string(corev1.ResourceLimitsCPU),
	string(corev1.ResourceLimitsMemory),
)

// NewResourceBindingEvaluator returns an evaluator that can evaluate pods
func NewResourceBindingEvaluator(f quota.ListerForResourceFunc, clock clock.Clock) quota.Evaluator {
	listFuncByNamespace := generic.ListResourceUsingListerFunc(f, workv1alpha2.SchemeGroupVersion.WithResource("resourcebindings"))
	ResourceBindingEvaluator := &ResourceBindingEvaluator{listFuncByNamespace: listFuncByNamespace, clock: clock}
	return ResourceBindingEvaluator
}

// ResourceBindingEvaluator knows how to measure usage of pods.
type ResourceBindingEvaluator struct {
	// knows how to list pods
	listFuncByNamespace generic.ListFuncByNamespace
	// used to track time
	clock clock.Clock
}

// Constraints verifies that all required resources are present on the pod
// In addition, it validates that the resources are valid (i.e. requests < limits)
func (p *ResourceBindingEvaluator) Constraints(required []corev1.ResourceName, item runtime.Object) error {
	binding, err := ToResourcebindingOrError(item)

	if err != nil {
		klog.Errorf("Failed to convert ResourceBinding from unstructured object: %v", err)
		return err
	}

	if !IsResourcebindingHasRequest(binding) {
		return nil
	}

	// BACKWARD COMPATIBILITY REQUIREMENT: if we quota cpu or memory, then each container
	// must make an explicit request for the resource.  this was a mistake.  it coupled
	// validation with resource counting, but we did this before QoS was even defined.
	// let's not make that mistake again with other resources now that QoS is defined.
	requiredSet := quota.ToSet(required).Intersection(validationSet)
	missingSet := sets.NewString()

	resourceBindingUsage := ResourceUsageHelper(binding)
	resourceBindingSet := quota.ToSet(quota.ResourceNames(resourceBindingUsage))
	if !resourceBindingSet.Equal(requiredSet) {
		difference := requiredSet.Difference(resourceBindingSet)
		missingSet.Insert(difference.List()...)
	}

	if len(missingSet) == 0 {
		return nil
	}
	return fmt.Errorf("must specify %s", strings.Join(missingSet.List(), ","))
}

// GroupResource that this evaluator tracks
func (p *ResourceBindingEvaluator) GroupResource() schema.GroupResource {
	return workv1alpha2.SchemeGroupVersion.WithResource("resourcebindings").GroupResource()
}

// Handles returns true if the evaluator should handle the specified attributes.
func (p *ResourceBindingEvaluator) Handles(a admission.Attributes) bool {
	op := a.GetOperation()

	return op == admission.Create
}

// Matches returns true if the evaluator matches the specified quota with the provided input item
func (p *ResourceBindingEvaluator) Matches(karmadaQuota *quotav1alpha1.KarmadaQuota, item runtime.Object) (bool, error) {
	return generic.Matches(karmadaQuota, item, p.MatchingResources, ResourceBindingMatchesScopeFunc)
}

// MatchingResources takes the input specified list of resources and returns the set of resources it matches.
func (p *ResourceBindingEvaluator) MatchingResources(input []corev1.ResourceName) []corev1.ResourceName {
	result := quota.Intersection(input, podResources)
	for _, resource := range input {
		// for resources with certain prefix, e.g. hugepages
		if quota.ContainsPrefix(podResourcePrefixes, resource) {
			result = append(result, resource)
		}
		// for extended resources
		if isExtendedResourceNameForQuota(resource) {
			result = append(result, resource)
		}
	}

	return result
}

// MatchingScopes takes the input specified list of scopes and pod object. Returns the set of scope selectors pod matches.
func (p *ResourceBindingEvaluator) MatchingScopes(item runtime.Object, scopeSelectors []corev1.ScopedResourceSelectorRequirement) ([]corev1.ScopedResourceSelectorRequirement, error) {
	matchedScopes := []corev1.ScopedResourceSelectorRequirement{}
	for _, selector := range scopeSelectors {
		match, err := ResourceBindingMatchesScopeFunc(selector, item)
		if err != nil {
			return []corev1.ScopedResourceSelectorRequirement{}, fmt.Errorf("error on matching scope %v: %v", selector, err)
		}
		if match {
			matchedScopes = append(matchedScopes, selector)
		}
	}
	return matchedScopes, nil
}

// UncoveredQuotaScopes takes the input matched scopes which are limited by configuration and the matched quota scopes.
// It returns the scopes which are in limited scopes but don't have a corresponding covering quota scope
func (p *ResourceBindingEvaluator) UncoveredQuotaScopes(limitedScopes []corev1.ScopedResourceSelectorRequirement, matchedQuotaScopes []corev1.ScopedResourceSelectorRequirement) ([]corev1.ScopedResourceSelectorRequirement, error) {
	uncoveredScopes := []corev1.ScopedResourceSelectorRequirement{}
	for _, selector := range limitedScopes {
		isCovered := false
		for _, matchedScopeSelector := range matchedQuotaScopes {
			if matchedScopeSelector.ScopeName == selector.ScopeName {
				isCovered = true
				break
			}
		}

		if !isCovered {
			uncoveredScopes = append(uncoveredScopes, selector)
		}
	}
	return uncoveredScopes, nil
}

// Usage knows how to measure usage associated with pods
func (p *ResourceBindingEvaluator) Usage(item runtime.Object) (corev1.ResourceList, error) {
	// delegate to normal usage
	return ResourceBindingUsageFunc(item)
}

// UsageStats calculates aggregate usage for the object.
func (p *ResourceBindingEvaluator) UsageStats(options quota.UsageStatsOptions) (quota.UsageStats, error) {
	return generic.CalculateUsageStats(options, p.listFuncByNamespace, ResourceBindingMatchesScopeFunc, p.Usage)
}

// verifies we implement the required interface.
var _ quota.Evaluator = &ResourceBindingEvaluator{}

// ForReplicasResources return request * replicas
func ForReplicasResources(request corev1.ResourceList, replicas int) corev1.ResourceList {
	result := corev1.ResourceList{}
	for name, quantity := range request {
		itemquantity := quantity
		for i := 0; i < replicas-1; i++ {
			itemquantity.Add(quantity)
		}
		result[name] = itemquantity
	}
	return result
}

// ResourceReplicasComputeUsageHelper can summarize the pod compute quota usage based on requests and limits
func ResourceReplicasComputeUsageHelper(binding *workv1alpha2.ResourceBinding) corev1.ResourceList {
	requests := binding.Spec.ReplicaRequirements.ResourceRequest
	replicas := binding.Spec.Replicas
	result := corev1.ResourceList{}

	result[corev1.ResourcePods] = *resource.NewQuantity(int64(replicas), resource.DecimalSI)
	replicasResources := ForReplicasResources(requests, int(replicas))

	if request, found := replicasResources[corev1.ResourceCPU]; found {
		result[corev1.ResourceCPU] = request
		result[corev1.ResourceRequestsCPU] = request
	}
	if request, found := replicasResources[corev1.ResourceMemory]; found {
		result[corev1.ResourceMemory] = request
		result[corev1.ResourceRequestsMemory] = request
	}
	if request, found := replicasResources[corev1.ResourceEphemeralStorage]; found {
		result[corev1.ResourceEphemeralStorage] = request
		result[corev1.ResourceRequestsEphemeralStorage] = request
	}
	for resource, request := range replicasResources {
		// for resources with certain prefix, e.g. hugepages
		if quota.ContainsPrefix(requestedResourcePrefixes, resource) {
			result[resource] = request
			result[maskResourceWithPrefix(resource, corev1.DefaultResourceRequestsPrefix)] = request
		}
		// for extended resources
		if helper.IsExtendedResourceName(resource) {
			// only quota objects in format of "requests.resourceName" is allowed for extended resource.
			result[maskResourceWithPrefix(resource, corev1.DefaultResourceRequestsPrefix)] = request
		}
	}

	return result
}

// ResourceUsageHelper can summarize the pod compute quota usage based on requests and limits
func ResourceUsageHelper(binding *workv1alpha2.ResourceBinding) corev1.ResourceList {
	requests := binding.Spec.ReplicaRequirements.ResourceRequest
	replicas := binding.Spec.Replicas
	result := corev1.ResourceList{}

	result[corev1.ResourcePods] = *resource.NewQuantity(int64(replicas), resource.DecimalSI)

	if request, found := requests[corev1.ResourceCPU]; found {
		result[corev1.ResourceCPU] = request
		result[corev1.ResourceRequestsCPU] = request
	}
	if request, found := requests[corev1.ResourceMemory]; found {
		result[corev1.ResourceMemory] = request
		result[corev1.ResourceRequestsMemory] = request
	}
	if request, found := requests[corev1.ResourceEphemeralStorage]; found {
		result[corev1.ResourceEphemeralStorage] = request
		result[corev1.ResourceRequestsEphemeralStorage] = request
	}
	for resource, request := range requests {
		// for resources with certain prefix, e.g. hugepages
		if quota.ContainsPrefix(requestedResourcePrefixes, resource) {
			result[resource] = request
			result[maskResourceWithPrefix(resource, corev1.DefaultResourceRequestsPrefix)] = request
		}
		// for extended resources
		if helper.IsExtendedResourceName(resource) {
			// only quota objects in format of "requests.resourceName" is allowed for extended resource.
			result[maskResourceWithPrefix(resource, corev1.DefaultResourceRequestsPrefix)] = request
		}
	}

	return result
}

// ResourceBindingMatchesScopeFunc is a function that knows how to evaluate if a pod matches a scope
func ResourceBindingMatchesScopeFunc(selector corev1.ScopedResourceSelectorRequirement, object runtime.Object) (bool, error) {
	binding, err := ToResourcebindingOrError(object)
	if err != nil {
		klog.Errorf("Failed to convert ResourceBinding from unstructured object: %v", err)
		return false, err
	}

	// if Resourcebinding has no request, return true.
	if !IsResourcebindingHasRequest(binding) {
		return false, nil
	}

	switch selector.ScopeName {
	case corev1.ResourceQuotaScopeBestEffort:
		return isBestEffort(binding), nil
	case corev1.ResourceQuotaScopeNotBestEffort:
		return !isBestEffort(binding), nil
	case corev1.ResourceQuotaScopePriorityClass:
		return resourcebindingMatchesSelector(binding, selector)
	}
	return false, nil
}

// ResourceBindingUsageFunc  returns the quota usage for a ResourceBinding.
// A ResourceBinding is charged for quota if the following are not true.
//  - ResourceBinding has a terminal phase (failed or succeeded)
//  - ResourceBinding has been marked for deletion and grace period has expired
func ResourceBindingUsageFunc(obj runtime.Object) (corev1.ResourceList, error) {
	binding, err := ToResourcebindingOrError(obj)
	if err != nil {
		klog.Errorf("Failed to convert ResourceBinding from unstructured object: %v", err)
		return corev1.ResourceList{}, err
	}
	if !IsResourcebindingHasRequest(binding) {
		return corev1.ResourceList{}, nil
	}
	// always quota the object count (even if the pod is end of life)
	// object count quotas track all objects that are in storage.
	// where "pods" tracks all pods that have not reached a terminal state,
	// count/pods tracks all pods independent of state.
	result := corev1.ResourceList{
		podObjectCountName: *(resource.NewQuantity(int64(binding.Spec.Replicas), resource.DecimalSI)),
	}

	result = quota.Add(result, ResourceReplicasComputeUsageHelper(binding))
	return result, nil
}

// isSupportedQoSComputeResource
func isSupportedQoSComputeResource(name corev1.ResourceName) bool {
	return supportedQoSComputeResources.Has(string(name))
}

func isBestEffort(binding *workv1alpha2.ResourceBinding) bool {
	requests := corev1.ResourceList{}
	zeroQuantity := resource.MustParse("0")
	for name, quantity := range binding.Spec.ReplicaRequirements.ResourceRequest {
		if !isSupportedQoSComputeResource(name) {
			continue
		}
		if quantity.Cmp(zeroQuantity) == 1 {
			delta := quantity.DeepCopy()
			if _, exists := requests[name]; !exists {
				requests[name] = delta
			} else {
				delta.Add(requests[name])
				requests[name] = delta
			}
		}
	}

	return len(requests) == 0
}

// resourcebindingMatchesSelector
func resourcebindingMatchesSelector(binding *workv1alpha2.ResourceBinding, selector corev1.ScopedResourceSelectorRequirement) (bool, error) {
	labelSelector, err := helper.ScopedResourceSelectorRequirementsAsSelector(selector)
	if err != nil {
		return false, fmt.Errorf("failed to parse and convert selector: %v", err)
	}
	var m map[string]string
	//TODO: resourcebinding support PriorityClass
	//if len(binding.Spec.Resource.PriorityClassName) != 0 {
	//	m = map[string]string{string(corev1.ResourceQuotaScopePriorityClass): binding.Spec.Resource.PriorityClassName}
	//}
	if labelSelector.Matches(labels.Set(m)) {
		return true, nil
	}
	return false, nil
}

// ToResourcebindingOrError trans Object to workv1alpha2.ResourceBinding
func ToResourcebindingOrError(obj runtime.Object) (*workv1alpha2.ResourceBinding, error) {
	switch t := obj.(type) {
	case *workv1alpha2.ResourceBinding:
		return t, nil
	default:
		return nil, fmt.Errorf("expect *workv1alpha2.ResourceBinding, got %v", t)
	}
}

// IsResourcebindingHasRequest return true if the Resourcebinding has request
func IsResourcebindingHasRequest(resourcebinding *workv1alpha2.ResourceBinding) bool {
	if resourcebinding.Spec.Replicas == 0 || resourcebinding.Spec.ReplicaRequirements == nil {
		klog.Infof("the resoucebinding[%s/%s] has no resource request. Replicas:[%d]",
			resourcebinding.GetNamespace(), resourcebinding.GetName(), resourcebinding.Spec.Replicas)
		return false
	}
	return true
}
