package detector

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

func getHighestPriorityPropagationPolicies(policies []*policyv1alpha1.PropagationPolicy, resource *unstructured.Unstructured, objectKey keys.ClusterWideKey) *policyv1alpha1.PropagationPolicy {
	var matchedPolicy *policyv1alpha1.PropagationPolicy
	matchedPolicyPriority := util.PriorityMisMatch
	for _, policy := range policies {
		if p := util.ResourceMatchSelectorsPriority(resource, policy.Spec.ResourceSelectors...); p > matchedPolicyPriority {
			matchedPolicy = policy
			matchedPolicyPriority = p
		} else if p > util.PriorityMisMatch && p == matchedPolicyPriority {
			matchedPolicy = getHigherPriorityPropagationPolicy(matchedPolicy, policy)
		}
	}

	if matchedPolicy == nil {
		klog.V(2).Infof("No propagationpolicy match for resource(%s)", objectKey)
		return nil
	}
	klog.V(2).Infof("Matched policy(%s/%s) for resource(%s)", matchedPolicy.Namespace, matchedPolicy.Name, objectKey)
	return matchedPolicy
}

func getHighestPriorityClusterPropagationPolicies(policies []*policyv1alpha1.ClusterPropagationPolicy, resource *unstructured.Unstructured, objectKey keys.ClusterWideKey) *policyv1alpha1.ClusterPropagationPolicy {
	var matchedClusterPolicy *policyv1alpha1.ClusterPropagationPolicy
	matchedClusterPolicyPriority := util.PriorityMisMatch
	for _, policy := range policies {
		if p := util.ResourceMatchSelectorsPriority(resource, policy.Spec.ResourceSelectors...); p > matchedClusterPolicyPriority {
			matchedClusterPolicy = policy
			matchedClusterPolicyPriority = p
		} else if p > util.PriorityMisMatch && p == matchedClusterPolicyPriority {
			matchedClusterPolicy = getHigherPriorityClusterPropagationPolicy(matchedClusterPolicy, policy)
		}
	}

	if matchedClusterPolicy == nil {
		klog.V(2).Infof("No clusterpropagationpolicy match for resource(%s)", objectKey)
		return nil
	}
	klog.V(2).Infof("Matched cluster policy(%s) for resource(%s)", matchedClusterPolicy.Name, objectKey)
	return matchedClusterPolicy
}

// getHigherPriorityPropagationPolicy compare two PropagationPolicies with some priority comparison logic
func getHigherPriorityPropagationPolicy(a, b *policyv1alpha1.PropagationPolicy) *policyv1alpha1.PropagationPolicy {
	if a == nil {
		return b
	}
	// logic of priority comparison
	if a.Name < b.Name {
		return a
	}
	return b
}

// getHigherPriorityClusterPropagationPolicy compare two ClusterPropagationPolicies with some priority comparison logic
func getHigherPriorityClusterPropagationPolicy(a, b *policyv1alpha1.ClusterPropagationPolicy) *policyv1alpha1.ClusterPropagationPolicy {
	if a == nil {
		return b
	}
	// logic of priority comparison
	if a.Name < b.Name {
		return a
	}
	return b
}
