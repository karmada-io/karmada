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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

var (
	propagationPolicyClaimLabels = []string{
		policyv1alpha1.PropagationPolicyPermanentIDLabel,
	}
	propagationPolicyClaimAnnotations = []string{
		policyv1alpha1.PropagationPolicyNamespaceAnnotation,
		policyv1alpha1.PropagationPolicyNameAnnotation,
	}
	clusterPropagationPolicyClaimLabels = []string{
		policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel,
	}
	clusterPropagationPolicyClaimAnnotations = []string{
		policyv1alpha1.ClusterPropagationPolicyAnnotation,
	}
)

// AddPPClaimMetadata adds PropagationPolicy claim metadata, such as labels and annotations
func AddPPClaimMetadata(obj metav1.Object, policyID string, policyMeta metav1.ObjectMeta) {
	util.MergeLabel(obj, policyv1alpha1.PropagationPolicyPermanentIDLabel, policyID)

	objectAnnotations := obj.GetAnnotations()
	if objectAnnotations == nil {
		objectAnnotations = make(map[string]string)
	}
	objectAnnotations[policyv1alpha1.PropagationPolicyNamespaceAnnotation] = policyMeta.GetNamespace()
	objectAnnotations[policyv1alpha1.PropagationPolicyNameAnnotation] = policyMeta.GetName()
	obj.SetAnnotations(objectAnnotations)
}

// AddCPPClaimMetadata adds ClusterPropagationPolicy claim metadata, such as labels and annotations
func AddCPPClaimMetadata(obj metav1.Object, policyID string, policyMeta metav1.ObjectMeta) {
	util.MergeLabel(obj, policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel, policyID)

	objectAnnotations := obj.GetAnnotations()
	if objectAnnotations == nil {
		objectAnnotations = make(map[string]string)
	}
	objectAnnotations[policyv1alpha1.ClusterPropagationPolicyAnnotation] = policyMeta.GetName()
	obj.SetAnnotations(objectAnnotations)
}

// CleanupPPClaimMetadata removes PropagationPolicy claim metadata, such as labels and annotations
func CleanupPPClaimMetadata(obj metav1.Object) {
	util.RemoveLabels(obj, propagationPolicyClaimLabels...)
	util.RemoveAnnotations(obj, propagationPolicyClaimAnnotations...)
}

// CleanupCPPClaimMetadata removes ClusterPropagationPolicy claim metadata, such as labels and annotations
func CleanupCPPClaimMetadata(obj metav1.Object) {
	util.RemoveLabels(obj, clusterPropagationPolicyClaimLabels...)
	util.RemoveAnnotations(obj, clusterPropagationPolicyClaimAnnotations...)
}

// NeedCleanupClaimMetadata determines whether the object's claim metadata needs to be cleaned up.
// We need to ensure that the claim metadata being deleted belong to the current PropagationPolicy/ClusterPropagationPolicy,
// otherwise, there is a risk of mistakenly deleting the ones belonging to another PropagationPolicy/ClusterPropagationPolicy.
// This situation could occur during the rapid deletion and creation of PropagationPolicy(s)/ClusterPropagationPolicy(s).
// More info can refer to https://github.com/karmada-io/karmada/issues/5307.
func NeedCleanupClaimMetadata(obj metav1.Object, targetClaimMetadata map[string]string) bool {
	for k, v := range targetClaimMetadata {
		if obj.GetLabels()[k] != v {
			return false
		}
	}
	return true
}
