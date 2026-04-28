/*
Copyright 2026 The Karmada Authors.

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

package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	searchapis "github.com/karmada-io/karmada/pkg/apis/search"
)

func TestValidateResourceRegistry(t *testing.T) {
	tests := []struct {
		name             string
		resourceRegistry *searchapis.ResourceRegistry
		wantErrFields    []string
	}{
		{
			name: "valid resource registry",
			resourceRegistry: &searchapis.ResourceRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "registry-a"},
				Spec: searchapis.ResourceRegistrySpec{
					TargetCluster: policyv1alpha1.ClusterAffinity{},
					ResourceSelectors: []searchapis.ResourceSelector{
						{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default"},
					},
					BackendStore: &searchapis.BackendStoreConfig{
						OpenSearch: &searchapis.OpenSearchConfig{
							Addresses: []string{"https://localhost:9200"},
							SecretRef: clusterv1alpha1.LocalSecretReference{Namespace: "default", Name: "os-auth"},
						},
					},
				},
			},
			wantErrFields: nil,
		},
		{
			name: "valid: empty resource selectors",
			resourceRegistry: &searchapis.ResourceRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "registry-b"},
				Spec: searchapis.ResourceRegistrySpec{
					TargetCluster:     policyv1alpha1.ClusterAffinity{},
					ResourceSelectors: nil,
				},
			},
			wantErrFields: nil,
		},
		{
			name: "invalid: malformed selector and backend",
			resourceRegistry: &searchapis.ResourceRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "registry-c"},
				Spec: searchapis.ResourceRegistrySpec{
					TargetCluster: policyv1alpha1.ClusterAffinity{},
					ResourceSelectors: []searchapis.ResourceSelector{
						{APIVersion: "apps/v1/extra", Kind: "", Namespace: "Bad_Namespace"},
					},
					BackendStore: &searchapis.BackendStoreConfig{
						OpenSearch: &searchapis.OpenSearchConfig{
							Addresses: []string{"localhost:9200"},
							SecretRef: clusterv1alpha1.LocalSecretReference{},
						},
					},
				},
			},
			wantErrFields: []string{
				"spec.resourceSelectors[0].apiVersion",
				"spec.resourceSelectors[0].namespace",
				"spec.backendStore.openSearch.addresses[0]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateResourceRegistry(tt.resourceRegistry)
			if len(tt.wantErrFields) == 0 {
				if len(errs) != 0 {
					t.Fatalf("expected no validation errors, got: %v", errs)
				}
				return
			}

			if len(errs) == 0 {
				t.Fatalf("expected validation errors, got none")
			}

			for _, wantField := range tt.wantErrFields {
				if !containsFieldError(errs, wantField) {
					t.Fatalf("expected error on field %q, got: %v", wantField, errs)
				}
			}
		})
	}
}

func containsFieldError(errs field.ErrorList, wantField string) bool {
	for _, err := range errs {
		if err.Field == wantField {
			return true
		}
	}

	return false
}
