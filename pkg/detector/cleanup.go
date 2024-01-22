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

package detector

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

var (
	// PropagationPolicyRefLabels is a collection of labels added to an object, used to specify the associated propagation policy.
	PropagationPolicyRefLabels = []string{policyv1alpha1.PropagationPolicyNamespaceLabel, policyv1alpha1.PropagationPolicyNameLabel, policyv1alpha1.PropagationPolicyPermanentIDLabel}
	// ClusterPropagationPolicyRefLabels is a collection of labels added to an object, used to specify the associated cluster propagation policy.
	ClusterPropagationPolicyRefLabels = []string{policyv1alpha1.ClusterPropagationPolicyLabel, policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel}
	// PropagationPolicyRefAnnotations is a collection of annotations added to an object, used to specify the associated propagation policy.
	PropagationPolicyRefAnnotations = []string{policyv1alpha1.PropagationPolicyNamespaceAnnotation, policyv1alpha1.PropagationPolicyNameAnnotation}
	// ClusterPropagationPolicyRefAnnotations is a collection of annotations added to an object, used to specify the associated cluster propagation policy.
	ClusterPropagationPolicyRefAnnotations = []string{policyv1alpha1.ClusterPropagationPolicyAnnotation}
)

// CleanupPolicyRefNamespaceScopeResources cleanup labels and annotations from namespaceScopeResources by policyRef.
func (d *ResourceDetector) CleanupPolicyRefNamespaceScopeResources(policyNS, policyName string, rb *workv1alpha2.ResourceBinding) error {
	labels := PropagationPolicyRefLabels
	annotations := PropagationPolicyRefAnnotations
	if policyNS == "" {
		labels = ClusterPropagationPolicyRefLabels
		annotations = ClusterPropagationPolicyRefAnnotations
	}

	// Must remove the labels from the resource template ahead of ResourceBinding, otherwise might lose the chance
	// to do that in a retry loop(in particular, the label was successfully removed from ResourceBinding, but
	// resource template not), since the ResourceBinding will not be listed again.
	if err := d.cleanupObjectRefLabelsAndAnnotations(rb.Spec.Resource, labels, annotations); err != nil {
		klog.Errorf("Failed to clean up label from resource(%s-%s/%s) when propagation policy(%s/%s) removing, error: %v",
			rb.Spec.Resource.Kind, rb.Spec.Resource.Namespace, rb.Spec.Resource.Name, policyNS, policyName, err)
		return err
	}

	// Clean up the labels from the reference binding so that the karmada scheduler won't reschedule the binding.
	if err := d.cleanupRBLabelsAndAnnotations(rb, labels, annotations); err != nil {
		klog.Errorf("Failed to clean up label from resource binding(%s/%s) when propagation policy(%s/%s) removing, error: %v",
			rb.Namespace, rb.Name, policyNS, policyName, err)
		return err
	}
	return nil
}

// CleanupPolicyRefClusterScopeResources cleanup labels and annotations from clusterScopeResources by policyRef.
func (d *ResourceDetector) CleanupPolicyRefClusterScopeResources(policyName string, crb *workv1alpha2.ClusterResourceBinding) error {
	labels := ClusterPropagationPolicyRefLabels
	annotations := ClusterPropagationPolicyRefAnnotations

	// Must remove the labels from the resource template ahead of ClusterResourceBinding, otherwise might lose the chance
	// to do that in a retry loop(in particular, the label was successfully removed from ClusterResourceBinding, but
	// resource template not), since the ClusterResourceBinding will not be listed again.
	if err := d.cleanupObjectRefLabelsAndAnnotations(crb.Spec.Resource, labels, annotations); err != nil {
		klog.Errorf("Failed to clean up labels and annotations from resource(%s-%s) when cluster propagation policy(%s) removing, error: %v",
			crb.Spec.Resource.Kind, crb.Spec.Resource.Name, policyName, err)
		return err
	}

	// Clean up the labels and annotations from the reference binding so that the karmada scheduler won't reschedule the binding.
	if err := d.CleanupCRBLabelsAndAnnotations(crb, labels, annotations); err != nil {
		klog.Errorf("Failed to clean up labels and annotations from resource binding(%s/%s) when cluster propagation policy(%s) removing, error: %v",
			crb.Namespace, crb.Name, policyName, err)
		return err
	}
	return nil
}

// removes labels from object referencing by objRef.
func (d *ResourceDetector) cleanupObjectRefLabelsAndAnnotations(objRef workv1alpha2.ObjectReference, labels, annotations []string) error {
	workload, err := helper.FetchResourceTemplate(d.DynamicClient, d.InformerManager, d.RESTMapper, objRef)
	if err != nil {
		// do nothing if resource template not exist, it might have been removed.
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to fetch resource(kind=%s, %s/%s): %v", objRef.Kind, objRef.Namespace, objRef.Name, err)
		return err
	}

	workload = workload.DeepCopy()

	return d.cleanupWorkLoadLabelsAndAnnotations(workload, labels, annotations)
}

// removes labels and annotations from object.
func (d *ResourceDetector) cleanupWorkLoadLabelsAndAnnotations(workload *unstructured.Unstructured, labels, annotations []string) error {
	cleanupMapKeysFunc := func(m map[string]string, keys []string) map[string]string {
		for _, key := range keys {
			delete(m, key)
		}
		return m
	}

	workload.SetLabels(cleanupMapKeysFunc(workload.GetLabels(), labels))
	workload.SetAnnotations(cleanupMapKeysFunc(workload.GetAnnotations(), annotations))

	gvr, err := restmapper.GetGroupVersionResource(d.RESTMapper, workload.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to delete resource(%s/%s) labels as mapping GVK to GVR failed: %v", workload.GetNamespace(), workload.GetName(), err)
		return err
	}

	newWorkload, err := d.DynamicClient.Resource(gvr).Namespace(workload.GetNamespace()).Update(context.TODO(), workload, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update resource %v/%v, err is %v ", workload.GetNamespace(), workload.GetName(), err)
		return err
	}
	klog.V(2).Infof("Updated resource template(kind=%s, %s/%s) successfully", newWorkload.GetKind(), newWorkload.GetNamespace(), newWorkload.GetName())
	return nil
}

// removes labels and annotations from resource binding.
func (d *ResourceDetector) cleanupRBLabelsAndAnnotations(rb *workv1alpha2.ResourceBinding, labels, annotations []string) error {
	cleanupMapKeysFunc := func(m map[string]string, keys []string) map[string]string {
		for _, key := range keys {
			delete(m, key)
		}
		return m
	}

	bindingLabels := cleanupMapKeysFunc(rb.GetLabels(), labels)
	bindingAnnotations := cleanupMapKeysFunc(rb.GetAnnotations(), annotations)
	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		rb.SetLabels(bindingLabels)
		rb.SetAnnotations(bindingAnnotations)
		updateErr := d.Client.Update(context.TODO(), rb)
		if updateErr == nil {
			return nil
		}

		updated := &workv1alpha2.ResourceBinding{}
		if err = d.Client.Get(context.TODO(), client.ObjectKey{Namespace: rb.GetNamespace(), Name: rb.GetName()}, updated); err == nil {
			rb = updated
		} else {
			klog.Errorf("Failed to get updated resource binding %s/%s: %v", rb.GetNamespace(), rb.GetName(), err)
		}
		return updateErr
	})
}

// CleanupCRBLabelsAndAnnotations removes labels and annotations from cluster resource binding.
func (d *ResourceDetector) CleanupCRBLabelsAndAnnotations(crb *workv1alpha2.ClusterResourceBinding, labels, annotations []string) error {
	cleanupMapKeysFunc := func(m map[string]string, keys []string) map[string]string {
		for _, key := range keys {
			delete(m, key)
		}
		return m
	}

	bindingLabels := cleanupMapKeysFunc(crb.GetLabels(), labels)
	bindingAnnotations := cleanupMapKeysFunc(crb.GetAnnotations(), annotations)
	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		crb.SetLabels(bindingLabels)
		crb.SetAnnotations(bindingAnnotations)
		updateErr := d.Client.Update(context.TODO(), crb)
		if updateErr == nil {
			return nil
		}

		updated := &workv1alpha2.ClusterResourceBinding{}
		if err = d.Client.Get(context.TODO(), client.ObjectKey{Name: crb.GetName()}, updated); err == nil {
			crb = updated
		} else {
			klog.Errorf("Failed to get updated cluster resource binding %s: %v", crb.GetName(), err)
		}
		return updateErr
	})
}
