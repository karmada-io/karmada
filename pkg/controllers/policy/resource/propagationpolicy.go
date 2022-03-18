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

func (c *Controller) lookForMatchedPolicy(object *unstructured.Unstructured, objectKey keys.ClusterWideKey) (*policyv1alpha1.PropagationPolicy, error) {
	if len(objectKey.Namespace) == 0 {
		return nil, nil
	}

	klog.V(2).Infof("attempts to match policy for resource(%s)", objectKey)
	policyObjects, err := c.propagationPolicyLister.ByNamespace(objectKey.Namespace).List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list propagation policy: %v", err)
		return nil, err
	}
	if len(policyObjects) == 0 {
		klog.V(2).Infof("no propagationpolicy find in namespace(%s).", objectKey.Namespace)
		return nil, nil
	}

	policyList := make([]*policyv1alpha1.PropagationPolicy, 0)
	for index := range policyObjects {
		policy, err := helper.ConvertToPropagationPolicy(policyObjects[index].(*unstructured.Unstructured))
		if err != nil {
			klog.Errorf("Failed to convert PropagationPolicy from unstructured object: %v", err)
			return nil, err
		}
		policyList = append(policyList, policy)
	}

	var matchedPolicy *policyv1alpha1.PropagationPolicy
	for _, policy := range policyList {
		if util.ResourceMatchSelectors(object, policy.Spec.ResourceSelectors...) {
			matchedPolicy = GetHigherPriorityPropagationPolicy(matchedPolicy, policy)
		}
	}

	if matchedPolicy == nil {
		klog.V(2).Infof("no propagationpolicy match for resource(%s)", objectKey)
		return nil, nil
	}
	klog.V(2).Infof("Matched policy(%s/%s) for resource(%s)", matchedPolicy.Namespace, matchedPolicy.Name, objectKey)
	return matchedPolicy, nil
}
