package detector

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func (d *ResourceDetector) propagateResource(object *unstructured.Unstructured, objectKey keys.ClusterWideKey) error {
	// 1. Check if the object has been claimed by a PropagationPolicy,
	// if so, just apply it.
	policyLabels := object.GetLabels()
	claimedNamespace := util.GetLabelValue(policyLabels, policyv1alpha1.PropagationPolicyNamespaceLabel)
	claimedName := util.GetLabelValue(policyLabels, policyv1alpha1.PropagationPolicyNameLabel)
	if claimedNamespace != "" && claimedName != "" {
		return d.getAndApplyPolicy(object, objectKey, claimedNamespace, claimedName)
	}

	// 2. Check if the object has been claimed by a ClusterPropagationPolicy,
	// if so, just apply it.
	claimedName = util.GetLabelValue(policyLabels, policyv1alpha1.ClusterPropagationPolicyLabel)
	if claimedName != "" {
		return d.getAndApplyClusterPolicy(object, objectKey, claimedName)
	}

	// 3. attempt to match policy in its namespace.
	propagationPolicy, err := d.LookForMatchedPolicy(object, objectKey)
	if err != nil {
		klog.Errorf("Failed to retrieve policy for object: %s, error: %v", objectKey.String(), err)
		return err
	}
	if propagationPolicy != nil {
		// return err when dependents not present, that we can retry at next reconcile.
		if present, err := helper.IsDependentOverridesPresent(d.Client, propagationPolicy); err != nil || !present {
			klog.Infof("Waiting for dependent overrides present for policy(%s/%s)", propagationPolicy.Namespace, propagationPolicy.Name)
			return fmt.Errorf("waiting for dependent overrides")
		}
		d.RemoveWaiting(objectKey)
		return d.ApplyPolicy(object, objectKey, propagationPolicy)
	}

	// 4. reaching here means there is no appropriate PropagationPolicy, attempt to match a ClusterPropagationPolicy.
	clusterPolicy, err := d.LookForMatchedClusterPolicy(object, objectKey)
	if err != nil {
		klog.Errorf("Failed to retrieve cluster policy for object: %s, error: %v", objectKey.String(), err)
		return err
	}
	if clusterPolicy != nil {
		// return err when dependents not present, that we can retry at next reconcile.
		if present, err := helper.IsDependentClusterOverridesPresent(d.Client, clusterPolicy); err != nil || !present {
			klog.Infof("Waiting for dependent overrides present for policy(%s)", clusterPolicy.Name)
			return fmt.Errorf("waiting for dependent overrides")
		}
		d.RemoveWaiting(objectKey)
		return d.ApplyClusterPolicy(object, objectKey, clusterPolicy)
	}

	if d.isWaiting(objectKey) {
		// reaching here means there is no appropriate policy for the object
		d.EventRecorder.Event(object, corev1.EventTypeWarning, workv1alpha2.EventReasonApplyPolicyFailed, "No policy match for resource")
		return nil
	}

	// put it into waiting list and retry once in case the resource and propagation policy come at the same time
	// see https://github.com/karmada-io/karmada/issues/1195
	d.AddWaiting(objectKey)
	return fmt.Errorf("no matched propagation policy")
}

func (d *ResourceDetector) getAndApplyPolicy(object *unstructured.Unstructured, objectKey keys.ClusterWideKey, policyNamespace, policyName string) error {
	policyObject, err := d.propagationPolicyLister.ByNamespace(policyNamespace).Get(policyName)
	if err != nil {
		klog.Errorf("Failed to get claimed policy(%s/%s),: %v", policyNamespace, policyName, err)
		return err
	}

	matchedPropagationPolicy := &policyv1alpha1.PropagationPolicy{}
	if err = helper.ConvertToTypedObject(policyObject, matchedPropagationPolicy); err != nil {
		klog.Errorf("Failed to convert PropagationPolicy from unstructured object: %v", err)
		return err
	}

	// return err when dependents not present, that we can retry at next reconcile.
	if present, err := helper.IsDependentOverridesPresent(d.Client, matchedPropagationPolicy); err != nil || !present {
		klog.Infof("Waiting for dependent overrides present for policy(%s/%s)", policyNamespace, policyName)
		return fmt.Errorf("waiting for dependent overrides")
	}

	return d.ApplyPolicy(object, objectKey, matchedPropagationPolicy)
}

func (d *ResourceDetector) getAndApplyClusterPolicy(object *unstructured.Unstructured, objectKey keys.ClusterWideKey, policyName string) error {
	policyObject, err := d.clusterPropagationPolicyLister.Get(policyName)
	if err != nil {
		klog.Errorf("Failed to get claimed policy(%s),: %v", policyName, err)
		return err
	}

	matchedClusterPropagationPolicy := &policyv1alpha1.ClusterPropagationPolicy{}
	if err = helper.ConvertToTypedObject(policyObject, matchedClusterPropagationPolicy); err != nil {
		klog.Errorf("Failed to convert ClusterPropagationPolicy from unstructured object: %v", err)
		return err
	}

	// return err when dependents not present, that we can retry at next reconcile.
	if present, err := helper.IsDependentClusterOverridesPresent(d.Client, matchedClusterPropagationPolicy); err != nil || !present {
		klog.Infof("Waiting for dependent overrides present for policy(%s)", policyName)
		return fmt.Errorf("waiting for dependent overrides")
	}

	return d.ApplyClusterPolicy(object, objectKey, matchedClusterPropagationPolicy)
}
