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
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func TestWaitingObjectStoreLifecycle(t *testing.T) {
	store := newWaitingObjectStore()
	key := waitingKey("apps/v1", "Deployment", "default", "demo")
	labels := map[string]string{"env": "prod"}

	inserted, labelsChanged := store.Upsert(key, labels)
	if !inserted || labelsChanged {
		t.Fatalf("first Upsert() = (%t, %t), want (true, false)", inserted, labelsChanged)
	}
	if !store.Contains(key) {
		t.Fatal("Contains() should find an inserted key")
	}
	inserted, labelsChanged = store.Upsert(key, labels)
	if inserted || labelsChanged {
		t.Fatalf("duplicate Upsert() = (%t, %t), want (false, false)", inserted, labelsChanged)
	}

	labels["env"] = "mutated-after-upsert"
	assertWaitingKeys(t, store.Match([]policyv1alpha1.ResourceSelector{{
		APIVersion:    "apps/v1",
		Kind:          "Deployment",
		Namespace:     "default",
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
	}}), key)

	inserted, labelsChanged = store.Upsert(key, map[string]string{"env": "staging"})
	if inserted || !labelsChanged {
		t.Fatalf("label update Upsert() = (%t, %t), want (false, true)", inserted, labelsChanged)
	}
	assertWaitingKeys(t, store.Match([]policyv1alpha1.ResourceSelector{{
		APIVersion:    "apps/v1",
		Kind:          "Deployment",
		Namespace:     "default",
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "staging"}},
	}}), key)

	store.Delete(key)
	store.Delete(key)
	if store.Contains(key) {
		t.Fatal("Contains() should not find a deleted key")
	}
	if len(store.objects) != 0 || len(store.byGVK) != 0 || len(store.byScope) != 0 || len(store.byGVKName) != 0 {
		t.Fatalf("Delete() should remove empty index buckets: objects=%d byGVK=%d byScope=%d byGVKName=%d",
			len(store.objects), len(store.byGVK), len(store.byScope), len(store.byGVKName))
	}
}

func TestWaitingObjectStoreStableCandidateLifecycle(t *testing.T) {
	store := newWaitingObjectStore()
	key := waitingKey("apps/v1", "Deployment", "default", "demo")
	gvk := key.GroupVersionKind()
	scopeKey := waitingScopeKey{gvk: gvk, namespace: key.Namespace}
	nameKey := waitingGVKNameKey{gvk: gvk, name: key.Name}

	store.Upsert(key, map[string]string{"env": "prod"})
	candidate := store.objects[key]
	initialObject := candidate.object
	if !store.byGVK[gvk].Has(candidate) || !store.byScope[scopeKey].Has(candidate) {
		t.Fatal("secondary indexes should reference the primary map candidate")
	}
	if got := store.byGVKName[nameKey]; len(got) != 1 || got[0] != candidate {
		t.Fatalf("name index should contain the primary map candidate once, got %#v", got)
	}

	store.Upsert(key, map[string]string{"env": "prod"})
	if store.objects[key] != candidate || candidate.object != initialObject {
		t.Fatal("unchanged Upsert should preserve the candidate and label snapshot pointers")
	}
	if got := store.byGVKName[nameKey]; len(got) != 1 {
		t.Fatalf("duplicate Upsert should not append to the name index, got %d entries", len(got))
	}

	store.Upsert(key, map[string]string{"env": "staging"})
	if store.objects[key] != candidate {
		t.Fatal("label update should preserve the stable candidate pointer")
	}
	if candidate.object == initialObject {
		t.Fatal("label update should replace the immutable label snapshot")
	}
	if initialObject.labels["env"] != "prod" || candidate.object.labels["env"] != "staging" {
		t.Fatalf("unexpected old/new label snapshots: old=%v new=%v", initialObject.labels, candidate.object.labels)
	}
}

func TestWaitingObjectStoreNameIndexSliceLifecycle(t *testing.T) {
	store := newWaitingObjectStore()
	keysByNamespace := []keys.ClusterWideKey{
		waitingKey("apps/v1", "Deployment", "team-a", "demo"),
		waitingKey("apps/v1", "Deployment", "team-b", "demo"),
		waitingKey("apps/v1", "Deployment", "team-c", "demo"),
	}
	for _, key := range keysByNamespace {
		store.Upsert(key, map[string]string{"app": "demo"})
	}

	nameKey := waitingGVKNameKey{gvk: keysByNamespace[0].GroupVersionKind(), name: "demo"}
	selector := []policyv1alpha1.ResourceSelector{{APIVersion: "apps/v1", Kind: "Deployment", Name: "demo"}}
	if got := len(store.byGVKName[nameKey]); got != 3 {
		t.Fatalf("name index length = %d, want 3", got)
	}
	assertWaitingKeys(t, store.Match(selector), keysByNamespace...)

	store.Upsert(keysByNamespace[0], map[string]string{"app": "updated"})
	if got := len(store.byGVKName[nameKey]); got != 3 {
		t.Fatalf("label update should not append to the name index, got %d entries", got)
	}

	store.Delete(keysByNamespace[1])
	assertWaitingKeys(t, store.Match(selector), keysByNamespace[0], keysByNamespace[2])
	if got := len(store.byGVKName[nameKey]); got != 2 {
		t.Fatalf("name index length after middle deletion = %d, want 2", got)
	}

	store.Delete(keysByNamespace[0])
	assertWaitingKeys(t, store.Match(selector), keysByNamespace[2])
	if got := len(store.byGVKName[nameKey]); got != 1 {
		t.Fatalf("name index length after first deletion = %d, want 1", got)
	}

	store.Delete(keysByNamespace[2])
	assertWaitingKeys(t, store.Match(selector))
	if _, exists := store.byGVKName[nameKey]; exists {
		t.Fatal("last deletion should remove the empty name index bucket")
	}
}

func TestWaitingObjectStoreClusterScopedExactName(t *testing.T) {
	store := newWaitingObjectStore()
	key := waitingKey("v1", "Namespace", "", "demo")
	store.Upsert(key, map[string]string{"team": "demo"})

	if len(store.byScope) != 0 || len(store.byGVKName) != 0 {
		t.Fatalf("cluster-scoped object should not enter namespace indexes: byScope=%d byGVKName=%d", len(store.byScope), len(store.byGVKName))
	}
	assertWaitingKeys(t, store.Match([]policyv1alpha1.ResourceSelector{{
		APIVersion: "v1", Kind: "Namespace", Name: "demo",
	}}), key)

	store.Delete(key)
	if len(store.objects) != 0 || len(store.byGVK) != 0 {
		t.Fatalf("cluster-scoped deletion should clean primary and GVK indexes: objects=%d byGVK=%d", len(store.objects), len(store.byGVK))
	}
}

func TestWaitingObjectStoreMatchIndexPaths(t *testing.T) {
	store := newWaitingObjectStore()
	deploymentDefault := waitingKey("apps/v1", "Deployment", "default", "demo")
	deploymentOtherNamespace := waitingKey("apps/v1", "Deployment", "other", "demo")
	deploymentOtherName := waitingKey("apps/v1", "Deployment", "default", "other")
	statefulSet := waitingKey("apps/v1", "StatefulSet", "default", "demo")
	corePod := waitingKey("v1", "Pod", "default", "demo")

	for _, key := range []keys.ClusterWideKey{
		deploymentDefault,
		deploymentOtherNamespace,
		deploymentOtherName,
		statefulSet,
		corePod,
	} {
		store.Upsert(key, map[string]string{"app": "demo"})
	}

	tests := []struct {
		name     string
		selector policyv1alpha1.ResourceSelector
		want     []keys.ClusterWideKey
	}{
		{
			name: "namespace and name uses the full key",
			selector: policyv1alpha1.ResourceSelector{
				APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "demo",
			},
			want: []keys.ClusterWideKey{deploymentDefault},
		},
		{
			name: "namespace and name miss does not fall back across namespaces",
			selector: policyv1alpha1.ResourceSelector{
				APIVersion: "apps/v1", Kind: "Deployment", Namespace: "missing", Name: "demo",
			},
		},
		{
			name: "name without namespace matches across namespaces",
			selector: policyv1alpha1.ResourceSelector{
				APIVersion: "apps/v1", Kind: "Deployment", Name: "demo",
			},
			want: []keys.ClusterWideKey{deploymentDefault, deploymentOtherNamespace},
		},
		{
			name: "namespace without name is scope isolated",
			selector: policyv1alpha1.ResourceSelector{
				APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default",
			},
			want: []keys.ClusterWideKey{deploymentDefault, deploymentOtherName},
		},
		{
			name: "gvk bucket is kind and api version isolated",
			selector: policyv1alpha1.ResourceSelector{
				APIVersion: "apps/v1", Kind: "Deployment",
			},
			want: []keys.ClusterWideKey{deploymentDefault, deploymentOtherNamespace, deploymentOtherName},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertWaitingKeys(t, store.Match([]policyv1alpha1.ResourceSelector{tt.selector}), tt.want...)
		})
	}
}

func TestWaitingObjectStoreLabelSelectors(t *testing.T) {
	store := newWaitingObjectStore()
	matchingKey := waitingKey("apps/v1", "Deployment", "default", "matching")
	nonMatchingKey := waitingKey("apps/v1", "Deployment", "default", "non-matching")
	store.Upsert(matchingKey, map[string]string{"env": "prod", "tier": "frontend", "region": "east"})
	store.Upsert(nonMatchingKey, map[string]string{"env": "dev", "tier": "backend", "backend-only": "true"})

	tests := []struct {
		name     string
		selector *metav1.LabelSelector
	}{
		{
			name: "In",
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key: "env", Operator: metav1.LabelSelectorOpIn, Values: []string{"prod", "staging"},
			}}},
		},
		{
			name: "NotIn",
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key: "tier", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"backend"},
			}}},
		},
		{
			name: "Exists",
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key: "region", Operator: metav1.LabelSelectorOpExists,
			}}},
		},
		{
			name: "DoesNotExist",
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key: "backend-only", Operator: metav1.LabelSelectorOpDoesNotExist,
			}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertWaitingKeys(t, store.Match([]policyv1alpha1.ResourceSelector{{
				APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", LabelSelector: tt.selector,
			}}), matchingKey)
		})
	}

	assertWaitingKeys(t, store.Match([]policyv1alpha1.ResourceSelector{{
		APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default",
	}}), matchingKey, nonMatchingKey)
}

func TestWaitingObjectStoreMatchSelectorSemantics(t *testing.T) {
	store := newWaitingObjectStore()
	key := waitingKey("apps/v1", "Deployment", "default", "demo")
	other := waitingKey("apps/v1", "Deployment", "default", "other")
	store.Upsert(key, map[string]string{"env": "prod"})
	store.Upsert(other, map[string]string{"env": "prod"})

	t.Run("name ignores label selector", func(t *testing.T) {
		assertWaitingKeys(t, store.Match([]policyv1alpha1.ResourceSelector{{
			APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "demo",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "does-not-match"}},
		}}), key)
	})

	t.Run("multiple selectors merge and deduplicate", func(t *testing.T) {
		assertWaitingKeys(t, store.Match([]policyv1alpha1.ResourceSelector{
			{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "demo"},
			{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default"},
		}), key, other)
	})

	t.Run("invalid selectors are skipped", func(t *testing.T) {
		assertWaitingKeys(t, store.Match([]policyv1alpha1.ResourceSelector{
			{APIVersion: "apps/v1/invalid", Kind: "Deployment"},
			{
				APIVersion: "apps/v1", Kind: "Deployment",
				LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key: "env", Operator: metav1.LabelSelectorOperator("Invalid"),
				}},
				},
			},
		}))
	})
}

func TestWaitingObjectStoreOverlappingSelectors(t *testing.T) {
	store := newWaitingObjectStore()
	defaultProd := waitingKey("apps/v1", "Deployment", "default", "prod")
	defaultDev := waitingKey("apps/v1", "Deployment", "default", "dev")
	otherProd := waitingKey("apps/v1", "Deployment", "other", "prod")
	otherDev := waitingKey("apps/v1", "Deployment", "other", "dev")
	store.Upsert(defaultProd, map[string]string{"env": "prod"})
	store.Upsert(defaultDev, map[string]string{"env": "dev"})
	store.Upsert(otherProd, map[string]string{"env": "prod"})
	store.Upsert(otherDev, map[string]string{"env": "dev"})

	selectors := []policyv1alpha1.ResourceSelector{
		{
			APIVersion: "apps/v1", Kind: "Deployment",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
		},
		{
			APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "dev"}},
		},
		{
			APIVersion: "apps/v1", Kind: "Deployment", Namespace: "other", Name: "dev",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "does-not-match"}},
		},
		{
			APIVersion: "apps/v1", Kind: "Deployment",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
		},
	}

	assertWaitingKeys(t, store.Match(selectors), defaultProd, defaultDev, otherProd, otherDev)
}

func TestWaitingObjectStoreDifferentialAgainstResourceMatches(t *testing.T) {
	// A fixed seed makes this differential test reproducible; the generated values are not security-sensitive.
	//nolint:gosec
	random := rand.New(rand.NewSource(42))
	store := newWaitingObjectStore()
	objects := make([]*unstructured.Unstructured, 0, 360)
	apiVersions := []string{"v1", "apps/v1"}
	kinds := []string{"ConfigMap", "Deployment"}
	namespaces := []string{"default", "team-a", "team-b"}
	environments := []string{"dev", "staging", "prod"}

	for i := range 360 {
		apiVersion := apiVersions[random.Intn(len(apiVersions))]
		kind := kinds[random.Intn(len(kinds))]
		namespace := namespaces[random.Intn(len(namespaces))]
		name := fmt.Sprintf("object-%03d", i)
		object := &unstructured.Unstructured{}
		object.SetAPIVersion(apiVersion)
		object.SetKind(kind)
		object.SetNamespace(namespace)
		object.SetName(name)
		object.SetLabels(map[string]string{
			"env":  environments[random.Intn(len(environments))],
			"even": fmt.Sprintf("%t", i%2 == 0),
		})
		objects = append(objects, object)
		store.Upsert(waitingKey(apiVersion, kind, namespace, name), object.GetLabels())
	}

	for i := range 200 {
		selector := policyv1alpha1.ResourceSelector{
			APIVersion: apiVersions[random.Intn(len(apiVersions))],
			Kind:       kinds[random.Intn(len(kinds))],
		}
		switch random.Intn(4) {
		case 0:
			selector.Namespace = namespaces[random.Intn(len(namespaces))]
			selector.Name = objects[random.Intn(len(objects))].GetName()
		case 1:
			selector.Name = objects[random.Intn(len(objects))].GetName()
		case 2:
			selector.Namespace = namespaces[random.Intn(len(namespaces))]
			selector.LabelSelector = &metav1.LabelSelector{MatchLabels: map[string]string{
				"env": environments[random.Intn(len(environments))],
			}}
		case 3:
			selector.LabelSelector = &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key: "even", Operator: metav1.LabelSelectorOpIn, Values: []string{fmt.Sprintf("%t", random.Intn(2) == 0)},
			}}}
		}

		want := sets.New[keys.ClusterWideKey]()
		for _, object := range objects {
			if util.ResourceMatches(object, selector) {
				want.Insert(waitingKey(object.GetAPIVersion(), object.GetKind(), object.GetNamespace(), object.GetName()))
			}
		}
		got := sets.New[keys.ClusterWideKey](store.Match([]policyv1alpha1.ResourceSelector{selector})...)
		if !want.Equal(got) {
			t.Fatalf("iteration %d differs from ResourceMatches(): selector=%+v want=%v got=%v", i, selector, want, got)
		}
	}
}

func TestWaitingObjectStoreConcurrentAccess(_ *testing.T) {
	store := newWaitingObjectStore()
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := range 250 {
				key := waitingKey("apps/v1", "Deployment", fmt.Sprintf("ns-%d", worker%2), fmt.Sprintf("object-%d-%d", worker, i))
				store.Upsert(key, map[string]string{"worker": fmt.Sprint(worker), "iteration": fmt.Sprint(i)})
				store.Match([]policyv1alpha1.ResourceSelector{{
					APIVersion: "apps/v1", Kind: "Deployment", Namespace: key.Namespace,
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"worker": fmt.Sprint(worker)}},
				}})
				if i%3 == 0 {
					store.Delete(key)
				}
			}
		}(worker)
	}
	wg.Wait()
}

func TestResourceDetectorWaitingLifecycle(t *testing.T) {
	scheme := setupTestScheme()
	detector := &ResourceDetector{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}
	key := waitingKey("apps/v1", "Deployment", "default", "demo")
	object := &unstructured.Unstructured{}
	object.SetAPIVersion("apps/v1")
	object.SetKind("Deployment")
	object.SetNamespace("default")
	object.SetName("demo")
	object.SetLabels(map[string]string{"env": "prod"})

	if err := detector.propagateResource(object, key, false); err == nil {
		t.Fatal("first reconcile without a policy should request one retry")
	}
	if !detector.isWaiting(key) {
		t.Fatal("first reconcile should add the object to the waiting store")
	}
	if err := detector.propagateResource(object, key, false); err != nil {
		t.Fatalf("repeat reconcile without any state change should not request another retry: %v", err)
	}

	object.SetLabels(map[string]string{"env": "staging"})
	if err := detector.propagateResource(object, key, false); err == nil {
		t.Fatal("label update should request one retry to avoid missing a concurrent policy event")
	}
	assertWaitingKeys(t, detector.GetMatching([]policyv1alpha1.ResourceSelector{{
		APIVersion:    "apps/v1",
		Kind:          "Deployment",
		Namespace:     "default",
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "staging"}},
	}}), key)
	if err := detector.propagateResource(object, key, false); err != nil {
		t.Fatalf("repeat reconcile after the label snapshot is current should not request another retry: %v", err)
	}
}

func TestResourceDetectorWaitingLabelUpdateRetriesAfterStalePolicyMatch(t *testing.T) {
	scheme := setupTestScheme()
	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	blockingClient := &blockingClusterPolicyListClient{
		Client:      baseClient,
		listStarted: make(chan struct{}),
		releaseList: make(chan struct{}),
	}
	detector := &ResourceDetector{Client: blockingClient}
	key := waitingKey("apps/v1", "Deployment", "default", "demo")
	detector.AddWaiting(key, map[string]string{"env": "old"})

	object := &unstructured.Unstructured{}
	object.SetAPIVersion("apps/v1")
	object.SetKind("Deployment")
	object.SetNamespace("default")
	object.SetName("demo")
	object.SetLabels(map[string]string{"env": "new"})

	reconcileResult := make(chan error, 1)
	go func() {
		reconcileResult <- detector.propagateResource(object, key, false)
	}()

	select {
	case <-blockingClient.listStarted:
	case <-time.After(time.Second):
		t.Fatal("resource reconcile did not reach the cluster policy lookup")
	}

	assertWaitingKeys(t, detector.GetMatching([]policyv1alpha1.ResourceSelector{{
		APIVersion:    "apps/v1",
		Kind:          "Deployment",
		Namespace:     "default",
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "new"}},
	}}))
	close(blockingClient.releaseList)

	select {
	case err := <-reconcileResult:
		if err == nil {
			t.Fatal("label update should request a retry after a policy may have matched the stale snapshot")
		}
	case <-time.After(time.Second):
		t.Fatal("resource reconcile did not finish")
	}
}

func TestResourceDetectorWaitingPolicyRequeuesForResourceReconcile(t *testing.T) {
	scheme := setupTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	worker := &recordingPriorityWorker{}
	detector := &ResourceDetector{Client: fakeClient, Processor: worker}
	key := waitingKey("apps/v1", "Deployment", "default", "demo")
	detector.AddWaiting(key, map[string]string{"app": "demo"})
	policy := &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "demo-policy",
			Labels: map[string]string{
				policyv1alpha1.PropagationPolicyPermanentIDLabel: "policy-id",
			},
		},
		Spec: policyv1alpha1.PropagationSpec{ResourceSelectors: []policyv1alpha1.ResourceSelector{{
			APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "demo",
		}}},
	}

	if err := detector.HandlePropagationPolicyCreationOrUpdate(policy); err != nil {
		t.Fatalf("HandlePropagationPolicyCreationOrUpdate() returned error: %v", err)
	}
	if detector.isWaiting(key) {
		t.Fatal("policy match should remove the object from the waiting store before resource reconciliation")
	}
	if len(worker.items) != 1 {
		t.Fatalf("policy match should enqueue one resource reconciliation, got %d", len(worker.items))
	}
	if got, ok := worker.items[0].(keys.ClusterWideKeyWithConfig); !ok || got.ClusterWideKey != key {
		t.Fatalf("unexpected resource queue item: %#v", worker.items[0])
	}

	bindings := &workv1alpha2.ResourceBindingList{}
	if err := fakeClient.List(t.Context(), bindings, client.InNamespace("default")); err != nil {
		t.Fatalf("failed to list ResourceBindings: %v", err)
	}
	if len(bindings.Items) != 0 {
		t.Fatal("policy reconciliation must not create bindings; resource reconciliation remains authoritative")
	}
}

func BenchmarkLegacyWaitingExactName24564(b *testing.B) {
	objects := benchmarkWaitingObjects(24564)
	selectors := []policyv1alpha1.ResourceSelector{{
		APIVersion: "apps/v1", Kind: "Deployment", Namespace: "namespace-42", Name: "object-12342",
	}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = legacyWaitingMatch(objects, selectors)
	}
}

func BenchmarkWaitingObjectStoreExactName24564(b *testing.B) {
	store := benchmarkWaitingStore(24564)
	selectors := []policyv1alpha1.ResourceSelector{{
		APIVersion: "apps/v1", Kind: "Deployment", Namespace: "namespace-42", Name: "object-12342",
	}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Match(selectors)
	}
}

func BenchmarkWaitingObjectStoreCrossNamespaceName24564(b *testing.B) {
	store := benchmarkWaitingStoreWithCrossNamespaceName(24564, 100)
	selectors := []policyv1alpha1.ResourceSelector{{
		APIVersion: "apps/v1", Kind: "Deployment", Name: "shared-name",
	}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Match(selectors)
	}
}

func BenchmarkLegacyWaitingLabelSelector24564(b *testing.B) {
	objects := benchmarkWaitingObjects(24564)
	selectors := []policyv1alpha1.ResourceSelector{{
		APIVersion: "apps/v1", Kind: "Deployment", Namespace: "namespace-42",
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"shard": "42"}},
	}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = legacyWaitingMatch(objects, selectors)
	}
}

func BenchmarkWaitingObjectStoreLabelSelector24564(b *testing.B) {
	store := benchmarkWaitingStore(24564)
	selectors := []policyv1alpha1.ResourceSelector{{
		APIVersion: "apps/v1", Kind: "Deployment", Namespace: "namespace-42",
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"shard": "42"}},
	}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Match(selectors)
	}
}

func BenchmarkWaitingObjectStoreMatchAll24564(b *testing.B) {
	store := benchmarkWaitingStore(24564)
	selectors := []policyv1alpha1.ResourceSelector{{APIVersion: "apps/v1", Kind: "Deployment"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Match(selectors)
	}
}

func BenchmarkWaitingObjectStoreOverlappingSelectors24564(b *testing.B) {
	store := benchmarkWaitingStore(24564)
	selectors := make([]policyv1alpha1.ResourceSelector, 100)
	for i := range selectors {
		selectors[i] = policyv1alpha1.ResourceSelector{APIVersion: "apps/v1", Kind: "Deployment"}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Match(selectors)
	}
}

func waitingKey(apiVersion, kind, namespace, name string) keys.ClusterWideKey {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		panic(err)
	}
	return keys.ClusterWideKey{
		Group: gv.Group, Version: gv.Version, Kind: kind, Namespace: namespace, Name: name,
	}
}

func assertWaitingKeys(t *testing.T, got []keys.ClusterWideKey, want ...keys.ClusterWideKey) {
	t.Helper()
	gotSet := sets.New[keys.ClusterWideKey](got...)
	wantSet := sets.New[keys.ClusterWideKey](want...)
	if !wantSet.Equal(gotSet) {
		t.Fatalf("unexpected matching keys: want=%v got=%v", wantSet, gotSet)
	}
}

type recordingPriorityWorker struct {
	items []any
}

type blockingClusterPolicyListClient struct {
	client.Client
	listStarted chan struct{}
	releaseList chan struct{}
}

func (c *blockingClusterPolicyListClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*policyv1alpha1.ClusterPropagationPolicyList); ok {
		close(c.listStarted)
		<-c.releaseList
	}
	return c.Client.List(ctx, list, opts...)
}

func (w *recordingPriorityWorker) Add(item any) {
	w.items = append(w.items, item)
}

func (w *recordingPriorityWorker) AddAfter(item any, _ time.Duration) {
	w.Add(item)
}

func (w *recordingPriorityWorker) Enqueue(item any) {
	w.Add(item)
}

func (w *recordingPriorityWorker) Run(_ context.Context, _ int) {}

func (w *recordingPriorityWorker) AddWithOpts(_ util.AddOpts, items ...any) {
	for _, item := range items {
		w.Add(item)
	}
}

func (w *recordingPriorityWorker) EnqueueWithOpts(_ util.AddOpts, item any) {
	w.Enqueue(item)
}

func benchmarkWaitingObjects(count int) []*unstructured.Unstructured {
	objects := make([]*unstructured.Unstructured, 0, count)
	for i := range count {
		object := &unstructured.Unstructured{}
		object.SetAPIVersion("apps/v1")
		object.SetKind("Deployment")
		object.SetNamespace(fmt.Sprintf("namespace-%d", i%100))
		object.SetName(fmt.Sprintf("object-%d", i))
		object.SetLabels(map[string]string{"shard": fmt.Sprint(i % 100)})
		objects = append(objects, object)
	}
	return objects
}

func benchmarkWaitingStore(count int) *waitingObjectStore {
	store := newWaitingObjectStore()
	for _, object := range benchmarkWaitingObjects(count) {
		store.Upsert(
			waitingKey(object.GetAPIVersion(), object.GetKind(), object.GetNamespace(), object.GetName()),
			object.GetLabels(),
		)
	}
	return store
}

func benchmarkWaitingStoreWithCrossNamespaceName(count, namespaceCount int) *waitingObjectStore {
	store := newWaitingObjectStore()
	for i := range count {
		name := fmt.Sprintf("object-%d", i)
		if i < namespaceCount {
			name = "shared-name"
		}
		store.Upsert(
			waitingKey("apps/v1", "Deployment", fmt.Sprintf("namespace-%d", i%namespaceCount), name),
			map[string]string{"shard": fmt.Sprint(i % namespaceCount)},
		)
	}
	return store
}

func legacyWaitingMatch(objects []*unstructured.Unstructured, selectors []policyv1alpha1.ResourceSelector) []keys.ClusterWideKey {
	result := make([]keys.ClusterWideKey, 0)
	for _, object := range objects {
		converted, err := helper.ToUnstructured(object)
		if err != nil {
			continue
		}
		candidate := converted.DeepCopy()
		for _, selector := range selectors {
			if util.ResourceMatches(candidate, selector) {
				result = append(result, waitingKey(candidate.GetAPIVersion(), candidate.GetKind(), candidate.GetNamespace(), candidate.GetName()))
				break
			}
		}
	}
	return result
}
