package detector

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/metrics"
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
	start := time.Now()
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
		metrics.ObserveFindMatchedPolicyLatency(start)
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
		metrics.ObserveFindMatchedPolicyLatency(start)
		return d.ApplyClusterPolicy(object, objectKey, clusterPolicy)
	}

	if d.isWaiting(objectKey) {
		// reaching here means there is no appropriate policy for the object
		klog.V(4).Infof("No matched policy for object: %s", objectKey.String())
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
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("PropagationPolicy(%s/%s) has been removed.", policyNamespace, policyName)
			return d.HandlePropagationPolicyDeletion(policyNamespace, policyName)
		}
		klog.Errorf("Failed to get claimed policy(%s/%s),: %v", policyNamespace, policyName, err)
		return err
	}

	matchedPropagationPolicy := &policyv1alpha1.PropagationPolicy{}
	if err = helper.ConvertToTypedObject(policyObject, matchedPropagationPolicy); err != nil {
		klog.Errorf("Failed to convert PropagationPolicy from unstructured object: %v", err)
		return err
	}

	// Some resources are available in more than one group in the same kubernetes version.
	// Therefore, the following scenarios occurs:
	// In v1.21 kubernetes cluster, Ingress are available in both networking.k8s.io and extensions groups.
	// When user creates an Ingress(networking.k8s.io/v1) and specifies a PropagationPolicy to propagate it
	// to the member clusters, the detector will listen two resource creation events:
	// Ingress(networking.k8s.io/v1) and Ingress(extensions/v1beta1). In order to prevent
	// Ingress(extensions/v1beta1) from being propagated, we need to ignore it.
	if !util.ResourceMatchSelectors(object, matchedPropagationPolicy.Spec.ResourceSelectors...) {
		return nil
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
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("ClusterPropagationPolicy(%s) has been removed.", policyName)
			return d.HandleClusterPropagationPolicyDeletion(policyName)
		}

		klog.Errorf("Failed to get claimed policy(%s),: %v", policyName, err)
		return err
	}

	matchedClusterPropagationPolicy := &policyv1alpha1.ClusterPropagationPolicy{}
	if err = helper.ConvertToTypedObject(policyObject, matchedClusterPropagationPolicy); err != nil {
		klog.Errorf("Failed to convert ClusterPropagationPolicy from unstructured object: %v", err)
		return err
	}

	// Some resources are available in more than one group in the same kubernetes version.
	// Therefore, the following scenarios occurs:
	// In v1.21 kubernetes cluster, Ingress are available in both networking.k8s.io and extensions groups.
	// When user creates an Ingress(networking.k8s.io/v1) and specifies a ClusterPropagationPolicy to
	// propagate it to the member clusters, the detector will listen two resource creation events:
	// Ingress(networking.k8s.io/v1) and Ingress(extensions/v1beta1). In order to prevent
	// Ingress(extensions/v1beta1) from being propagated, we need to ignore it.
	if !util.ResourceMatchSelectors(object, matchedClusterPropagationPolicy.Spec.ResourceSelectors...) {
		return nil
	}

	// return err when dependents not present, that we can retry at next reconcile.
	if present, err := helper.IsDependentClusterOverridesPresent(d.Client, matchedClusterPropagationPolicy); err != nil || !present {
		klog.Infof("Waiting for dependent overrides present for policy(%s)", policyName)
		return fmt.Errorf("waiting for dependent overrides")
	}

	return d.ApplyClusterPolicy(object, objectKey, matchedClusterPropagationPolicy)
}

func (d *ResourceDetector) cleanPPUnmatchedResourceBindings(policyNamespace, policyName string, selectors []policyv1alpha1.ResourceSelector) error {
	bindings, err := d.listPPDerivedRB(policyNamespace, policyName)
	if err != nil {
		return err
	}

	removeLabels := []string{
		policyv1alpha1.PropagationPolicyNamespaceLabel,
		policyv1alpha1.PropagationPolicyNameLabel,
	}
	return d.removeResourceBindingsLabels(bindings, selectors, removeLabels)
}

func (d *ResourceDetector) cleanCPPUnmatchedResourceBindings(policyName string, selectors []policyv1alpha1.ResourceSelector) error {
	bindings, err := d.listCPPDerivedRB(policyName)
	if err != nil {
		return err
	}

	removeLabels := []string{
		policyv1alpha1.ClusterPropagationPolicyLabel,
	}
	return d.removeResourceBindingsLabels(bindings, selectors, removeLabels)
}

func (d *ResourceDetector) cleanUnmatchedClusterResourceBinding(policyName string, selectors []policyv1alpha1.ResourceSelector) error {
	bindings, err := d.listCPPDerivedCRB(policyName)
	if err != nil {
		return err
	}

	return d.removeClusterResourceBindingsLabels(bindings, selectors)
}

func (d *ResourceDetector) removeResourceBindingsLabels(bindings *workv1alpha2.ResourceBindingList, selectors []policyv1alpha1.ResourceSelector, removeLabels []string) error {
	var errs []error
	for _, binding := range bindings.Items {
		removed, err := d.removeResourceLabelsIfNotMatch(binding.Spec.Resource, selectors, removeLabels...)
		if err != nil {
			klog.Errorf("Failed to remove resource labels when resource not match with policy selectors, err: %v", err)
			errs = append(errs, err)
			continue
		}
		if !removed {
			continue
		}

		bindingCopy := binding.DeepCopy()
		for _, l := range removeLabels {
			delete(bindingCopy.Labels, l)
		}
		err = d.Client.Update(context.TODO(), bindingCopy)
		if err != nil {
			klog.Errorf("Failed to update resourceBinding(%s/%s), err: %v", binding.Namespace, binding.Name, err)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}

	return nil
}

func (d *ResourceDetector) removeClusterResourceBindingsLabels(bindings *workv1alpha2.ClusterResourceBindingList, selectors []policyv1alpha1.ResourceSelector) error {
	var errs []error
	for _, binding := range bindings.Items {
		removed, err := d.removeResourceLabelsIfNotMatch(binding.Spec.Resource, selectors, []string{policyv1alpha1.ClusterPropagationPolicyLabel}...)
		if err != nil {
			klog.Errorf("Failed to remove resource labels when resource not match with policy selectors, err: %v", err)
			errs = append(errs, err)
			continue
		}
		if !removed {
			continue
		}

		bindingCopy := binding.DeepCopy()
		delete(bindingCopy.Labels, policyv1alpha1.ClusterPropagationPolicyLabel)
		err = d.Client.Update(context.TODO(), bindingCopy)
		if err != nil {
			klog.Errorf("Failed to update clusterResourceBinding(%s), err: %v", binding.Name, err)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}
	return nil
}

func (d *ResourceDetector) removeResourceLabelsIfNotMatch(objectReference workv1alpha2.ObjectReference, selectors []policyv1alpha1.ResourceSelector, labelKeys ...string) (bool, error) {
	objectKey, err := helper.ConstructClusterWideKey(objectReference)
	if err != nil {
		return false, err
	}

	object, err := d.GetUnstructuredObject(objectKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	if util.ResourceMatchSelectors(object, selectors...) {
		return false, nil
	}

	object = object.DeepCopy()
	util.RemoveLabels(object, labelKeys...)

	err = d.Client.Update(context.TODO(), object)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *ResourceDetector) listPPDerivedRB(policyNamespace, policyName string) (*workv1alpha2.ResourceBindingList, error) {
	bindings := &workv1alpha2.ResourceBindingList{}
	listOpt := &client.ListOptions{
		Namespace: policyNamespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			policyv1alpha1.PropagationPolicyNamespaceLabel: policyNamespace,
			policyv1alpha1.PropagationPolicyNameLabel:      policyName,
		}),
	}
	err := d.Client.List(context.TODO(), bindings, listOpt)
	if err != nil {
		klog.Errorf("Failed to list ResourceBinding with policy(%s/%s), error: %v", policyNamespace, policyName, err)
		return nil, err
	}

	return bindings, nil
}

func (d *ResourceDetector) listCPPDerivedRB(policyName string) (*workv1alpha2.ResourceBindingList, error) {
	bindings := &workv1alpha2.ResourceBindingList{}
	listOpt := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			policyv1alpha1.ClusterPropagationPolicyLabel: policyName,
		})}
	err := d.Client.List(context.TODO(), bindings, listOpt)
	if err != nil {
		klog.Errorf("Failed to list ResourceBinding with policy(%s), error: %v", policyName, err)
		return nil, err
	}

	return bindings, nil
}

func (d *ResourceDetector) listCPPDerivedCRB(policyName string) (*workv1alpha2.ClusterResourceBindingList, error) {
	bindings := &workv1alpha2.ClusterResourceBindingList{}
	listOpt := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			policyv1alpha1.ClusterPropagationPolicyLabel: policyName,
		})}
	err := d.Client.List(context.TODO(), bindings, listOpt)
	if err != nil {
		klog.Errorf("Failed to list ClusterResourceBinding with policy(%s), error: %v", policyName, err)
		return nil, err
	}

	return bindings, nil
}

// excludeClusterPolicy excludes cluster propagation policy.
// If propagation policy was claimed, cluster propagation policy should not exists.
func excludeClusterPolicy(objLabels map[string]string) bool {
	if _, ok := objLabels[policyv1alpha1.ClusterPropagationPolicyLabel]; !ok {
		return false
	}
	delete(objLabels, policyv1alpha1.ClusterPropagationPolicyLabel)
	return true
}
