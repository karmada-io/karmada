package resource

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager/keys"
)

// lookForMatchedClusterPolicy tries to find a ClusterPropagationPolicy for object referenced by object key.
func (c *Controller) lookForMatchedClusterPolicy(object *unstructured.Unstructured, objectKey keys.ClusterWideKey) (*policyv1alpha1.ClusterPropagationPolicy, error) {
	klog.V(2).Infof("attempts to match cluster policy for resource(%s)", objectKey)
	policyObjects, err := c.clusterPropagationPolicyLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list cluster propagation policy: %v", err)
		return nil, err
	}
	if len(policyObjects) == 0 {
		klog.V(2).Infof("no clusterpropagationpolicy find.")
		return nil, nil
	}

	policyList := make([]*policyv1alpha1.ClusterPropagationPolicy, 0)
	for index := range policyObjects {
		policy, err := helper.ConvertToClusterPropagationPolicy(policyObjects[index].(*unstructured.Unstructured))
		if err != nil {
			klog.Errorf("Failed to convert ClusterPropagationPolicy from unstructured object: %v", err)
			return nil, err
		}
		policyList = append(policyList, policy)
	}

	var matchedClusterPolicy *policyv1alpha1.ClusterPropagationPolicy
	for _, policy := range policyList {
		if util.ResourceMatchSelectors(object, policy.Spec.ResourceSelectors...) {
			matchedClusterPolicy = GetHigherPriorityClusterPropagationPolicy(matchedClusterPolicy, policy)
		}
	}

	if matchedClusterPolicy == nil {
		klog.V(2).Infof("no propagationpolicy match for resource(%s)", objectKey)
		return nil, nil
	}
	klog.V(2).Infof("Matched cluster policy(%s) for resource(%s)", matchedClusterPolicy.Name, objectKey)
	return matchedClusterPolicy, nil
}
