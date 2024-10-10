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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func TestAddPPClaimMetadata(t *testing.T) {
	tests := []struct {
		name       string
		policyID   string
		policyMeta metav1.ObjectMeta
		obj        metav1.Object
		result     metav1.Object
	}{
		{
			name:       "add policy claim metadata",
			policyID:   "f2507cgb-f3f3-4a4b-b289-5691a4fef979",
			policyMeta: metav1.ObjectMeta{Name: "pp-example", Namespace: "test"},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{},
					},
				},
			},
			result: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels":      map[string]interface{}{policyv1alpha1.PropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979"},
						"annotations": map[string]interface{}{policyv1alpha1.PropagationPolicyNamespaceAnnotation: "test", policyv1alpha1.PropagationPolicyNameAnnotation: "pp-example"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddPPClaimMetadata(tt.obj, tt.policyID, tt.policyMeta)
			assert.Equal(t, tt.obj, tt.result)
		})
	}
}

func TestAddCPPClaimMetadata(t *testing.T) {
	tests := []struct {
		name       string
		policyID   string
		policyMeta metav1.ObjectMeta
		obj        metav1.Object
		result     metav1.Object
	}{
		{
			name:       "add cluster policy claim metadata",
			policyID:   "f2507cgb-f3f3-4a4b-b289-5691a4fef979",
			policyMeta: metav1.ObjectMeta{Name: "cpp-example"},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{},
					},
				},
			},
			result: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels":      map[string]interface{}{policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979"},
						"annotations": map[string]interface{}{policyv1alpha1.ClusterPropagationPolicyAnnotation: "cpp-example"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddCPPClaimMetadata(tt.obj, tt.policyID, tt.policyMeta)
			assert.Equal(t, tt.obj, tt.result)
		})
	}
}

func TestCleanupPPClaimMetadata(t *testing.T) {
	tests := []struct {
		name   string
		obj    metav1.Object
		result metav1.Object
	}{
		{
			name: "clean up policy claim metadata",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels":      map[string]interface{}{policyv1alpha1.PropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979"},
						"annotations": map[string]interface{}{policyv1alpha1.PropagationPolicyNamespaceAnnotation: "default", policyv1alpha1.PropagationPolicyNameAnnotation: "pp-example"},
					},
				},
			},
			result: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels":      map[string]interface{}{},
						"annotations": map[string]interface{}{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CleanupPPClaimMetadata(tt.obj)
			assert.Equal(t, tt.obj, tt.result)
		})
	}
}

func TestCleanupCPPClaimMetadata(t *testing.T) {
	tests := []struct {
		name   string
		obj    metav1.Object
		result metav1.Object
	}{
		{
			name: "clean up policy claim metadata",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels":      map[string]interface{}{policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979"},
						"annotations": map[string]interface{}{policyv1alpha1.ClusterPropagationPolicyAnnotation: "cpp-example"},
					},
				},
			},
			result: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels":      map[string]interface{}{},
						"annotations": map[string]interface{}{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CleanupCPPClaimMetadata(tt.obj)
			assert.Equal(t, tt.obj, tt.result)
		})
	}
}

func TestNeedCleanupClaimMetadata(t *testing.T) {
	tests := []struct {
		name                string
		obj                 metav1.Object
		targetClaimMetadata map[string]string
		needCleanup         bool
	}{
		{
			name: "need cleanup",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels":      map[string]interface{}{policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979"},
						"annotations": map[string]interface{}{policyv1alpha1.ClusterPropagationPolicyAnnotation: "cpp-example"},
					},
				},
			},
			targetClaimMetadata: map[string]string{policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979"},
			needCleanup:         true,
		},
		{
			name: "no need cleanup",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels":      map[string]interface{}{policyv1alpha1.PropagationPolicyPermanentIDLabel: "b0907cgb-f3f3-4a4b-b289-5691a4fef979"},
						"annotations": map[string]interface{}{policyv1alpha1.PropagationPolicyNamespaceAnnotation: "default", policyv1alpha1.PropagationPolicyNameAnnotation: "pp-example"},
					},
				},
			},
			targetClaimMetadata: map[string]string{policyv1alpha1.PropagationPolicyPermanentIDLabel: "f2507cgb-f3f3-4a4b-b289-5691a4fef979"},
			needCleanup:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.needCleanup, NeedCleanupClaimMetadata(tt.obj, tt.targetClaimMetadata))
		})
	}
}
