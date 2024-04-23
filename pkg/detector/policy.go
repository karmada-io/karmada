/*
Copyright 2022 The Karmada Authors.

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
	"github.com/karmada-io/karmada/pkg/util/names"
)

func (d *ResourceDetector) propagateResource(object *unstructured.Unstructured,
	objectKey keys.ClusterWideKey, resourceChangeByKarmada bool) error {
	// 1. Check if the object has been claimed by a PropagationPolicy,
	// if so, just apply it.
	policyLabels := object.GetLabels()
	claimedNamespace := util.GetLabelValue(policyLabels, policyv1alpha1.PropagationPolicyNamespaceLabel)
	claimedName := util.GetLabelValue(policyLabels, policyv1alpha1.PropagationPolicyNameLabel)
	if claimedNamespace != "" && claimedName != "" {
		return d.getAndApplyPolicy(object, objectKey, resourceChangeByKarmada, claimedNamespace, claimedName)
	}

	// 2. Check if the object has been claimed by a ClusterPropagationPolicy,
	// if so, just apply it.
	claimedName = util.GetLabelValue(policyLabels, policyv1alpha1.ClusterPropagationPolicyLabel)
	if claimedName != "" {
		return d.getAndApplyClusterPolicy(object, objectKey, resourceChangeByKarmada, claimedName)
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
		return d.ApplyPolicy(object, objectKey, resourceChangeByKarmada, propagationPolicy)
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
		return d.ApplyClusterPolicy(object, objectKey, resourceChangeByKarmada, clusterPolicy)
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

func (d *ResourceDetector) getAndApplyPolicy(object *unstructured.Unstructured, objectKey keys.ClusterWideKey,
	resourceChangeByKarmada bool, policyNamespace, policyName string) error {
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

	// if the updated object no longer match to the previously bound propagation policy, unbind them.
	// in this case, we should remove the policy labels of resource and corresponding binding, and then directly return.
	if !util.ResourceMatchSelectors(object, matchedPropagationPolicy.Spec.ResourceSelectors...) ||
		util.ResourceMatchExcluded(object, matchedPropagationPolicy.Spec.ExcludedResources...) {
		removeLabels := []string{
			policyv1alpha1.PropagationPolicyNamespaceLabel,
			policyv1alpha1.PropagationPolicyNameLabel,
		}
		return d.removeSpecificResourceAndRBLabels(object, removeLabels)
	}

	// return err when dependents not present, that we can retry at next reconcile.
	if present, err := helper.IsDependentOverridesPresent(d.Client, matchedPropagationPolicy); err != nil || !present {
		klog.Infof("Waiting for dependent overrides present for policy(%s/%s)", policyNamespace, policyName)
		return fmt.Errorf("waiting for dependent overrides")
	}

	return d.ApplyPolicy(object, objectKey, resourceChangeByKarmada, matchedPropagationPolicy)
}

func (d *ResourceDetector) getAndApplyClusterPolicy(object *unstructured.Unstructured, objectKey keys.ClusterWideKey,
	resourceChangeByKarmada bool, policyName string) error {
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

	// if the updated object no longer match to the previously bound cluster propagation policy, unbind them.
	// in this case, we should remove the policy labels of resource and corresponding binding, and then directly return.
	if !util.ResourceMatchSelectors(object, matchedClusterPropagationPolicy.Spec.ResourceSelectors...) ||
		util.ResourceMatchExcluded(object, matchedClusterPropagationPolicy.Spec.ExcludedResources...) {
		removeLabels := []string{policyv1alpha1.ClusterPropagationPolicyLabel}
		// judge whether the object is namespace scoped, which affects determining what type of binding to clean up.
		if object.GetNamespace() == "" {
			return d.removeSpecificResourceAndCRBLabels(object, removeLabels)
		}
		return d.removeSpecificResourceAndRBLabels(object, removeLabels)
	}

	// return err when dependents not present, that we can retry at next reconcile.
	if present, err := helper.IsDependentClusterOverridesPresent(d.Client, matchedClusterPropagationPolicy); err != nil || !present {
		klog.Infof("Waiting for dependent overrides present for policy(%s)", policyName)
		return fmt.Errorf("waiting for dependent overrides")
	}

	return d.ApplyClusterPolicy(object, objectKey, resourceChangeByKarmada, matchedClusterPropagationPolicy)
}

// removeSpecificResourceAndRBLabels remove the propagation policy labels of specific resource and corresponding binding.
func (d *ResourceDetector) removeSpecificResourceAndRBLabels(resource *unstructured.Unstructured, removeLabels []string) error {
	// 1. get rb of given resource
	rbName := names.GenerateBindingName(resource.GetKind(), resource.GetName())
	rb := &workv1alpha2.ResourceBinding{}
	if err := d.Client.Get(context.TODO(), client.ObjectKey{Namespace: resource.GetNamespace(), Name: rbName}, rb); err != nil {
		klog.Errorf("Failed to get ResourceBinding: %s, err: %+v", rbName, err)
		return err
	}

	// 2. check whether rb related resource match to the given resource.
	// the mismatch would never happen under normal circumstances, if it really happened, just print logs and return nil.
	// this logic is also intended to be compatible with previous versions which talked about the issue of Ingress(networking.k8s.io/v1) and Ingress(extensions/v1beta1).
	if rb.Spec.Resource.APIVersion != resource.GetAPIVersion() || rb.Spec.Resource.Kind != resource.GetKind() ||
		rb.Spec.Resource.Name != resource.GetName() || rb.Spec.Resource.Namespace != resource.GetNamespace() {
		klog.Errorf("The rb (%s) related resource is not match to the given resource: %s/%s", rbName, resource.GetNamespace(), resource.GetName())
		return nil
	}

	// 3. clean the labels in binding
	if err := d.CleanupResourceBindingLabels(rb, removeLabels...); err != nil {
		klog.Errorf("Failed to remove binding labels, %s/%s, err: %+v", rb.GetNamespace(), rb.GetName(), err)
		return err
	}

	// 4. clean the labels in resource
	resourceCopy := resource.DeepCopy()
	util.RemoveLabels(resourceCopy, removeLabels...)
	if err := d.Client.Update(context.TODO(), resourceCopy); err != nil {
		klog.Errorf("Failed to remove resource labels, %s/%s, err: %+v", resourceCopy, resourceCopy, err)
		return err
	}

	klog.Infof("Successfully removed the policy labels in resource (%s/%s) and rb (%s) for policy (%s) no longer matched",
		resource.GetNamespace(), resource.GetName(), rbName, resource.GetLabels()[policyv1alpha1.PropagationPolicyNameLabel])
	return nil
}

// removeSpecificResourceAndCRBLabels remove the cluster propagation policy labels of specific resource and corresponding cluster binding.
func (d *ResourceDetector) removeSpecificResourceAndCRBLabels(resource *unstructured.Unstructured, removeLabels []string) error {
	// 1. get crb of given resource
	crbName := names.GenerateBindingName(resource.GetKind(), resource.GetName())
	crb := &workv1alpha2.ClusterResourceBinding{}
	if err := d.Client.Get(context.TODO(), client.ObjectKey{Name: crbName}, crb); err != nil {
		klog.Errorf("Failed to get ClusterResourceBinding: %s, err: %+v", crbName, err)
		return err
	}

	// 2. check whether crb related resource match to the given resource
	// the mismatch would never happen under normal circumstances, if it really happened, just print logs and return nil.
	// this logic is also intended to be compatible with previous versions which talked about the issue of Ingress(networking.k8s.io/v1) and Ingress(extensions/v1beta1).
	if crb.Spec.Resource.APIVersion != resource.GetAPIVersion() || crb.Spec.Resource.Kind != resource.GetKind() ||
		crb.Spec.Resource.Name != resource.GetName() {
		klog.Errorf("The crb (%s) related resource is not match to the given resource: %+v/%s", crbName, resource.GroupVersionKind(), resource.GetName())
		return nil
	}

	// 3. clean the labels in binding
	if err := d.CleanupClusterResourceBindingLabels(crb, removeLabels...); err != nil {
		klog.Errorf("Failed to remove binding labels, %s/%s, err: %+v", crb.GetNamespace(), crb.GetName(), err)
		return err
	}

	// 4. clean the labels in resource
	resourceCopy := resource.DeepCopy()
	util.RemoveLabels(resourceCopy, removeLabels...)
	if err := d.Client.Update(context.TODO(), resourceCopy); err != nil {
		klog.Errorf("Failed to remove resource labels, %+v/%s/%s, err: %+v", resourceCopy.GroupVersionKind(), resourceCopy.GetNamespace(), resourceCopy.GetName(), err)
		return err
	}

	klog.Infof("Successfully removed the cluster policy labels in resource (%s) and crb (%s) for policy (%s) no longer matched",
		resource.GetName(), crbName, resource.GetLabels()[policyv1alpha1.ClusterPropagationPolicyLabel])
	return nil
}

func (d *ResourceDetector) cleanPPUnmatchedResourceBindings(policyNamespace, policyName string, selectors []policyv1alpha1.ResourceSelector,
	excludedResources []policyv1alpha1.ExcludedResource) error {
	bindings, err := d.listPPDerivedRB(policyNamespace, policyName)
	if err != nil {
		return err
	}

	removeLabels := []string{
		policyv1alpha1.PropagationPolicyNamespaceLabel,
		policyv1alpha1.PropagationPolicyNameLabel,
	}
	return d.removeResourceBindingsLabels(bindings, selectors, excludedResources, removeLabels)
}

func (d *ResourceDetector) cleanCPPUnmatchedResourceBindings(policyName string, selectors []policyv1alpha1.ResourceSelector,
	excludedResources []policyv1alpha1.ExcludedResource) error {
	bindings, err := d.listCPPDerivedRB(policyName)
	if err != nil {
		return err
	}

	removeLabels := []string{
		policyv1alpha1.ClusterPropagationPolicyLabel,
	}
	return d.removeResourceBindingsLabels(bindings, selectors, excludedResources, removeLabels)
}

func (d *ResourceDetector) cleanUnmatchedClusterResourceBinding(policyName string, selectors []policyv1alpha1.ResourceSelector,
	excludedResources []policyv1alpha1.ExcludedResource) error {
	bindings, err := d.listCPPDerivedCRB(policyName)
	if err != nil {
		return err
	}

	return d.removeClusterResourceBindingsLabels(bindings, selectors, excludedResources)
}

func (d *ResourceDetector) removeResourceBindingsLabels(bindings *workv1alpha2.ResourceBindingList, selectors []policyv1alpha1.ResourceSelector,
	excludedResources []policyv1alpha1.ExcludedResource, removeLabels []string) error {
	var errs []error
	for _, binding := range bindings.Items {
		removed, err := d.removeResourceLabelsIfNotMatch(binding.Spec.Resource, selectors, excludedResources, removeLabels...)
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

func (d *ResourceDetector) removeClusterResourceBindingsLabels(bindings *workv1alpha2.ClusterResourceBindingList,
	selectors []policyv1alpha1.ResourceSelector, excludedResources []policyv1alpha1.ExcludedResource) error {
	var errs []error
	for _, binding := range bindings.Items {
		removed, err := d.removeResourceLabelsIfNotMatch(binding.Spec.Resource, selectors, excludedResources, []string{policyv1alpha1.ClusterPropagationPolicyLabel}...)
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

func (d *ResourceDetector) removeResourceLabelsIfNotMatch(objectReference workv1alpha2.ObjectReference, selectors []policyv1alpha1.ResourceSelector,
	excludedResources []policyv1alpha1.ExcludedResource, labelKeys ...string) (bool, error) {
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

	if util.ResourceMatchSelectors(object, selectors...) && !util.ResourceMatchExcluded(object, excludedResources...) {
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
