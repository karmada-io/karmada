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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func (d *ResourceDetector) propagateResource(object *unstructured.Unstructured,
	objectKey keys.ClusterWideKey, resourceChangeByKarmada bool) error {
	// 1. Check if the object has been claimed by a PropagationPolicy,
	// if so, just apply it.
	policyAnnotations := object.GetAnnotations()
	policyLabels := object.GetLabels()
	claimedNamespace := util.GetAnnotationValue(policyAnnotations, policyv1alpha1.PropagationPolicyNamespaceAnnotation)
	claimedName := util.GetAnnotationValue(policyAnnotations, policyv1alpha1.PropagationPolicyNameAnnotation)
	claimedID := util.GetLabelValue(policyLabels, policyv1alpha1.PropagationPolicyPermanentIDLabel)
	if claimedNamespace != "" && claimedName != "" && claimedID != "" {
		return d.getAndApplyPolicy(object, objectKey, resourceChangeByKarmada, claimedNamespace, claimedName, claimedID)
	}

	// 2. Check if the object has been claimed by a ClusterPropagationPolicy,
	// if so, just apply it.
	claimedName = util.GetAnnotationValue(policyAnnotations, policyv1alpha1.ClusterPropagationPolicyAnnotation)
	claimedID = util.GetLabelValue(policyLabels, policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel)
	if claimedName != "" && claimedID != "" {
		return d.getAndApplyClusterPolicy(object, objectKey, resourceChangeByKarmada, claimedName, claimedID)
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
	resourceChangeByKarmada bool, policyNamespace, policyName, claimedID string) error {
	matchedPropagationPolicy := &policyv1alpha1.PropagationPolicy{}
	err := d.Client.Get(context.TODO(), client.ObjectKey{Namespace: policyNamespace, Name: policyName}, matchedPropagationPolicy)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("PropagationPolicy(%s/%s) has been removed.", policyNamespace, policyName)
			return d.HandlePropagationPolicyDeletion(claimedID)
		}
		klog.Errorf("Failed to get claimed policy(%s/%s),: %v", policyNamespace, policyName, err)
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

	return d.ApplyPolicy(object, objectKey, resourceChangeByKarmada, matchedPropagationPolicy)
}

func (d *ResourceDetector) getAndApplyClusterPolicy(object *unstructured.Unstructured, objectKey keys.ClusterWideKey,
	resourceChangeByKarmada bool, policyName, policyID string) error {
	matchedClusterPropagationPolicy := &policyv1alpha1.ClusterPropagationPolicy{}
	err := d.Client.Get(context.TODO(), client.ObjectKey{Name: policyName}, matchedClusterPropagationPolicy)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("ClusterPropagationPolicy(%s) has been removed.", policyName)
			return d.HandleClusterPropagationPolicyDeletion(policyID)
		}

		klog.Errorf("Failed to get claimed policy(%s),: %v", policyName, err)
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

	return d.ApplyClusterPolicy(object, objectKey, resourceChangeByKarmada, matchedClusterPropagationPolicy)
}

func (d *ResourceDetector) cleanPPUnmatchedRBs(policyID, policyNamespace, policyName string, selectors []policyv1alpha1.ResourceSelector) error {
	bindings, err := d.listPPDerivedRBs(policyID, policyNamespace, policyName)
	if err != nil {
		return err
	}

	return d.removeRBsClaimMetadata(bindings, selectors, propagationPolicyClaimLabels, propagationPolicyClaimAnnotations)
}

func (d *ResourceDetector) cleanCPPUnmatchedRBs(policyID, policyName string, selectors []policyv1alpha1.ResourceSelector) error {
	bindings, err := d.listCPPDerivedRBs(policyID, policyName)
	if err != nil {
		return err
	}

	return d.removeRBsClaimMetadata(bindings, selectors, clusterPropagationPolicyClaimLabels, clusterPropagationPolicyClaimAnnotations)
}

func (d *ResourceDetector) cleanUnmatchedCRBs(policyID, policyName string, selectors []policyv1alpha1.ResourceSelector) error {
	bindings, err := d.listCPPDerivedCRBs(policyID, policyName)
	if err != nil {
		return err
	}

	return d.removeCRBsClaimMetadata(bindings, selectors, clusterPropagationPolicyClaimLabels, clusterPropagationPolicyClaimAnnotations)
}

func (d *ResourceDetector) removeRBsClaimMetadata(bindings *workv1alpha2.ResourceBindingList, selectors []policyv1alpha1.ResourceSelector, labels, annotations []string) error {
	var errs []error
	for _, binding := range bindings.Items {
		removed, err := d.removeResourceClaimMetadataIfNotMatched(binding.Spec.Resource, selectors, labels, annotations)
		if err != nil {
			klog.Errorf("Failed to remove resource labels and annotations when resource not match with policy selectors, err: %v", err)
			errs = append(errs, err)
			continue
		}
		if !removed {
			continue
		}

		bindingCopy := binding.DeepCopy()
		util.RemoveLabels(bindingCopy, labels...)
		util.RemoveAnnotations(bindingCopy, annotations...)
		err = d.Client.Update(context.TODO(), bindingCopy)
		if err != nil {
			klog.Errorf("Failed to update resourceBinding(%s/%s), err: %v", binding.Namespace, binding.Name, err)
			errs = append(errs, err)
		}
	}

	return errors.NewAggregate(errs)
}

func (d *ResourceDetector) removeCRBsClaimMetadata(bindings *workv1alpha2.ClusterResourceBindingList,
	selectors []policyv1alpha1.ResourceSelector, removeLabels, removeAnnotations []string) error {
	var errs []error
	for _, binding := range bindings.Items {
		removed, err := d.removeResourceClaimMetadataIfNotMatched(binding.Spec.Resource, selectors, removeLabels, removeAnnotations)
		if err != nil {
			klog.Errorf("Failed to remove resource labels and annotations when resource not match with policy selectors, err: %v", err)
			errs = append(errs, err)
			continue
		}
		if !removed {
			continue
		}

		bindingCopy := binding.DeepCopy()
		util.RemoveLabels(bindingCopy, removeLabels...)
		util.RemoveAnnotations(bindingCopy, removeAnnotations...)
		err = d.Client.Update(context.TODO(), bindingCopy)
		if err != nil {
			klog.Errorf("Failed to update clusterResourceBinding(%s), err: %v", binding.Name, err)
			errs = append(errs, err)
		}
	}

	return errors.NewAggregate(errs)
}

func (d *ResourceDetector) removeResourceClaimMetadataIfNotMatched(objectReference workv1alpha2.ObjectReference,
	selectors []policyv1alpha1.ResourceSelector, labels, annotations []string) (bool, error) {
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
	util.RemoveLabels(object, labels...)
	util.RemoveAnnotations(object, annotations...)

	err = d.Client.Update(context.TODO(), object)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *ResourceDetector) listPPDerivedRBs(policyID, policyNamespace, policyName string) (*workv1alpha2.ResourceBindingList, error) {
	bindings := &workv1alpha2.ResourceBindingList{}
	listOpt := &client.ListOptions{
		Namespace:             policyNamespace,
		UnsafeDisableDeepCopy: ptr.To(true),
		LabelSelector: labels.SelectorFromSet(labels.Set{
			policyv1alpha1.PropagationPolicyPermanentIDLabel: policyID,
		}),
	}
	err := d.Client.List(context.TODO(), bindings, listOpt)
	if err != nil {
		klog.Errorf("Failed to list ResourceBinding with policy(%s/%s), error: %v", policyNamespace, policyName, err)
		return nil, err
	}

	return bindings, nil
}

func (d *ResourceDetector) listCPPDerivedRBs(policyID, policyName string) (*workv1alpha2.ResourceBindingList, error) {
	bindings := &workv1alpha2.ResourceBindingList{}
	listOpt := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: policyID,
		}),
		UnsafeDisableDeepCopy: ptr.To(true),
	}
	err := d.Client.List(context.TODO(), bindings, listOpt)
	if err != nil {
		klog.Errorf("Failed to list ResourceBinding with policy(%s), error: %v", policyName, err)
		return nil, err
	}

	return bindings, nil
}

func (d *ResourceDetector) listCPPDerivedCRBs(policyID, policyName string) (*workv1alpha2.ClusterResourceBindingList, error) {
	bindings := &workv1alpha2.ClusterResourceBindingList{}
	listOpt := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: policyID,
		}),
		UnsafeDisableDeepCopy: ptr.To(true),
	}
	err := d.Client.List(context.TODO(), bindings, listOpt)
	if err != nil {
		klog.Errorf("Failed to list ClusterResourceBinding with policy(%s), error: %v", policyName, err)
		return nil, err
	}

	return bindings, nil
}

// excludeClusterPolicy excludes cluster propagation policy.
// If propagation policy was claimed, cluster propagation policy should not exist.
func excludeClusterPolicy(obj metav1.Object) (hasClaimedClusterPolicy bool) {
	if _, ok := obj.GetLabels()[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel]; !ok {
		return false
	}
	CleanupCPPClaimMetadata(obj)
	return true
}
