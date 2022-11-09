package detector

import (
	"math"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

func getHighestPriorityPropagationPolicy(policies []*policyv1alpha1.PropagationPolicy, resource *unstructured.Unstructured, objectKey keys.ClusterWideKey) *policyv1alpha1.PropagationPolicy {
	matchedPolicyImplicitPriority := util.PriorityMisMatch
	matchedPolicyExplicitPriority := int32(math.MinInt32)
	var matchedPolicy *policyv1alpha1.PropagationPolicy

	for _, policy := range policies {
		implicitPriority := util.ResourceMatchSelectorsPriority(resource, policy.Spec.ResourceSelectors...)
		if implicitPriority <= util.PriorityMisMatch {
			continue
		}
		explicitPriority := policy.ExplicitPriority()

		if matchedPolicyExplicitPriority < explicitPriority {
			matchedPolicyImplicitPriority = implicitPriority
			matchedPolicyExplicitPriority = explicitPriority
			matchedPolicy = policy
			continue
		}

		if matchedPolicyExplicitPriority == explicitPriority {
			if implicitPriority > matchedPolicyImplicitPriority {
				matchedPolicyImplicitPriority = implicitPriority
				matchedPolicy = policy
			} else if implicitPriority == matchedPolicyImplicitPriority {
				matchedPolicy = getHigherPriorityPropagationPolicy(matchedPolicy, policy)
			}
		}
	}

	if matchedPolicy == nil {
		klog.V(2).Infof("No propagationpolicy match for resource(%s)", objectKey)
		return nil
	}
	klog.V(2).Infof("Matched policy(%s/%s) for resource(%s)", matchedPolicy.Namespace, matchedPolicy.Name, objectKey)
	return matchedPolicy
}

func getHighestPriorityClusterPropagationPolicy(policies []*policyv1alpha1.ClusterPropagationPolicy, resource *unstructured.Unstructured, objectKey keys.ClusterWideKey) *policyv1alpha1.ClusterPropagationPolicy {
	matchedClusterPolicyImplicitPriority := util.PriorityMisMatch
	matchedClusterPolicyExplicitPriority := int32(math.MinInt32)
	var matchedClusterPolicy *policyv1alpha1.ClusterPropagationPolicy

	for _, policy := range policies {
		implicitPriority := util.ResourceMatchSelectorsPriority(resource, policy.Spec.ResourceSelectors...)
		if implicitPriority <= util.PriorityMisMatch {
			continue
		}
		explicitPriority := policy.ExplicitPriority()

		if matchedClusterPolicyExplicitPriority < explicitPriority {
			matchedClusterPolicyImplicitPriority = implicitPriority
			matchedClusterPolicyExplicitPriority = explicitPriority
			matchedClusterPolicy = policy
			continue
		}

		if matchedClusterPolicyExplicitPriority == explicitPriority {
			if implicitPriority > matchedClusterPolicyImplicitPriority {
				matchedClusterPolicyImplicitPriority = implicitPriority
				matchedClusterPolicy = policy
			} else if implicitPriority == matchedClusterPolicyImplicitPriority {
				matchedClusterPolicy = getHigherPriorityClusterPropagationPolicy(matchedClusterPolicy, policy)
			}
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
