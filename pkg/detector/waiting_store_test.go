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

package detector

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

func TestWaitingObjectStoreMatchesResourceSelectors(t *testing.T) {
	objects := []*unstructured.Unstructured{
		newWaitingStoreTestObject("apps/v1", "Deployment", "default", "demo", map[string]string{"env": "prod"}),
		newWaitingStoreTestObject("apps/v1", "Deployment", "default", "other", map[string]string{"env": "dev"}),
		newWaitingStoreTestObject("apps/v1", "Deployment", "team-a", "demo", map[string]string{"env": "prod"}),
		newWaitingStoreTestObject("v1", "ConfigMap", "default", "demo", map[string]string{"env": "prod"}),
	}
	detector := &ResourceDetector{}
	for _, object := range objects {
		key, err := keys.ClusterWideKeyFunc(object)
		if err != nil {
			t.Fatalf("failed to build object key: %v", err)
		}
		detector.AddWaiting(key, object)
	}

	tests := []struct {
		name      string
		selectors []policyv1alpha1.ResourceSelector
	}{
		{
			name: "exact namespaced object",
			selectors: []policyv1alpha1.ResourceSelector{{
				APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "demo",
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "dev"}},
			}},
		},
		{
			name: "same name across namespaces",
			selectors: []policyv1alpha1.ResourceSelector{{
				APIVersion: "apps/v1", Kind: "Deployment", Name: "demo",
			}},
		},
		{
			name: "namespaced label selector",
			selectors: []policyv1alpha1.ResourceSelector{{
				APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default",
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			}},
		},
		{
			name: "global label expression",
			selectors: []policyv1alpha1.ResourceSelector{{
				APIVersion: "apps/v1", Kind: "Deployment",
				LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key: "env", Operator: metav1.LabelSelectorOpIn, Values: []string{"prod"},
				}}},
			}},
		},
		{
			name: "global and namespace selectors are combined",
			selectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: "apps/v1", Kind: "Deployment",
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
				},
				{
					APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default",
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "dev"}},
				},
			},
		},
		{
			name: "missing global GVK does not stop other selectors",
			selectors: []policyv1alpha1.ResourceSelector{
				{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "demo"},
				{APIVersion: "batch/v1", Kind: "Job"},
				{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "demo"},
			},
		},
		{
			name: "overlapping selectors are deduplicated",
			selectors: []policyv1alpha1.ResourceSelector{
				{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default"},
				{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "demo"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := sets.New[keys.ClusterWideKey]()
			for _, object := range objects {
				for _, selector := range test.selectors {
					if util.ResourceMatches(object, selector) {
						key, err := keys.ClusterWideKeyFunc(object)
						if err != nil {
							t.Fatalf("failed to build matched object key: %v", err)
						}
						want.Insert(key)
						break
					}
				}
			}

			got := sets.New(detector.GetMatching(test.selectors)...)
			if !got.Equal(want) {
				t.Fatalf("GetMatching() = %v, want %v", got, want)
			}
		})
	}
}

func TestWaitingObjectStoreNamespaceLessSelectorClusterScopedFastPath(t *testing.T) {
	// A GVK has a single scope, so a hit in the empty-namespace bucket proves
	// there are no namespaced objects of the same GVK. This artificial
	// mixed-scope state verifies that namespace-less selectors return after
	// inspecting the empty-namespace bucket instead of scanning all namespaces.
	clusterScoped := newWaitingStoreTestObject("example.com/v1", "Widget", "", "shared", map[string]string{"scope": "cluster"})
	namespaced := newWaitingStoreTestObject("example.com/v1", "Widget", "default", "shared", map[string]string{"scope": "cluster"})

	detector := &ResourceDetector{}
	for _, object := range []*unstructured.Unstructured{clusterScoped, namespaced} {
		key, err := keys.ClusterWideKeyFunc(object)
		if err != nil {
			t.Fatalf("failed to build object key: %v", err)
		}
		detector.AddWaiting(key, object)
	}

	clusterScopedKey, err := keys.ClusterWideKeyFunc(clusterScoped)
	if err != nil {
		t.Fatalf("failed to build cluster-scoped key: %v", err)
	}
	namespacedKey, err := keys.ClusterWideKeyFunc(namespaced)
	if err != nil {
		t.Fatalf("failed to build namespaced key: %v", err)
	}

	tests := []struct {
		name     string
		selector policyv1alpha1.ResourceSelector
	}{
		{
			name: "name selector",
			selector: policyv1alpha1.ResourceSelector{
				APIVersion: "example.com/v1",
				Kind:       "Widget",
				Name:       "shared",
			},
		},
		{
			name: "label selector",
			selector: policyv1alpha1.ResourceSelector{
				APIVersion: "example.com/v1",
				Kind:       "Widget",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"scope": "cluster"},
				},
			},
		},
		{
			name: "match all",
			selector: policyv1alpha1.ResourceSelector{
				APIVersion: "example.com/v1",
				Kind:       "Widget",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := sets.New(detector.GetMatching([]policyv1alpha1.ResourceSelector{test.selector})...)
			if !got.Has(clusterScopedKey) {
				t.Fatalf("GetMatching() = %v, want cluster-scoped key %v", got, clusterScopedKey)
			}
			if got.Has(namespacedKey) {
				t.Fatalf("GetMatching() = %v, must not scan namespace buckets after a cluster-scoped hit", got)
			}
		})
	}
}

func newWaitingStoreTestObject(apiVersion, kind, namespace, name string, objectLabels map[string]string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{}
	object.SetAPIVersion(apiVersion)
	object.SetKind(kind)
	object.SetNamespace(namespace)
	object.SetName(name)
	object.SetLabels(objectLabels)
	return object
}
