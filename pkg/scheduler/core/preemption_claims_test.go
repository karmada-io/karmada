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

package core

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestPreemptionClaimStoreSetGetClearAndReplace(t *testing.T) {
	now := time.Now()
	store := newPreemptionClaimStore(defaultPreemptionClaimTTL, func() time.Time { return now })

	claim := preemptionClaim{
		bindingKey: "default/preemptor",
		cluster:    "member1",
		priority:   100,
		replicas:   3,
		resourceNeed: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1"),
		},
	}
	store.Set(claim)
	claim.resourceNeed[corev1.ResourceCPU] = resource.MustParse("2")

	got, ok := store.get("default/preemptor")
	if !ok {
		t.Fatal("expected claim to exist")
	}
	if got.cluster != "member1" || got.priority != 100 || got.replicas != 3 {
		t.Fatalf("unexpected claim: %+v", got)
	}
	if got.resourceNeed.Cpu().String() != "1" {
		t.Fatalf("resourceNeed was not copied on Set, got CPU %s", got.resourceNeed.Cpu().String())
	}
	got.resourceNeed[corev1.ResourceCPU] = resource.MustParse("3")
	gotAgain, ok := store.get("default/preemptor")
	if !ok {
		t.Fatal("expected claim to exist")
	}
	if gotAgain.resourceNeed.Cpu().String() != "1" {
		t.Fatalf("resourceNeed was not copied on Get, got CPU %s", gotAgain.resourceNeed.Cpu().String())
	}

	store.Set(preemptionClaim{
		bindingKey: "default/preemptor",
		cluster:    "member2",
		priority:   200,
		replicas:   5,
	})
	replaced, ok := store.get("default/preemptor")
	if !ok {
		t.Fatal("expected replacement claim to exist")
	}
	if replaced.cluster != "member2" || replaced.priority != 200 || replaced.replicas != 5 {
		t.Fatalf("unexpected replacement claim: %+v", replaced)
	}

	store.Clear("default/preemptor")
	if _, ok := store.get("default/preemptor"); ok {
		t.Fatal("expected claim to be cleared")
	}
}

func TestPreemptionClaimStoreExpiresClaims(t *testing.T) {
	now := time.Now()
	store := newPreemptionClaimStore(defaultPreemptionClaimTTL, func() time.Time { return now })

	store.Set(preemptionClaim{
		bindingKey: "default/preemptor",
		cluster:    "member1",
		priority:   100,
		replicas:   3,
	})
	now = now.Add(defaultPreemptionClaimTTL)

	if _, ok := store.get("default/preemptor"); ok {
		t.Fatal("expected expired claim to be swept")
	}
	if store.HasClaimOnCluster("member1") {
		t.Fatal("expected expired claim not to reserve cluster")
	}
}

func TestPreemptionClaimStoreHasClaimOnCluster(t *testing.T) {
	now := time.Now()
	store := newPreemptionClaimStore(defaultPreemptionClaimTTL, func() time.Time { return now })

	store.Set(preemptionClaim{
		bindingKey: "default/preemptor",
		cluster:    "member1",
		priority:   100,
		replicas:   3,
	})

	if !store.HasClaimOnCluster("member1") {
		t.Fatal("expected claim on member1")
	}
	if store.HasClaimOnCluster("member2") {
		t.Fatal("did not expect claim on member2")
	}
}

func TestPreemptionClaimStoreHasBlockingClaimOnCluster(t *testing.T) {
	now := time.Now()
	store := newPreemptionClaimStore(defaultPreemptionClaimTTL, func() time.Time { return now })
	for _, claim := range []preemptionClaim{
		{bindingKey: "default/requester", cluster: "member1", priority: 100, replicas: 1},
		{bindingKey: "default/lower", cluster: "member1", priority: 50, replicas: 1},
		{bindingKey: "default/equal", cluster: "member2", priority: 100, replicas: 1},
		{bindingKey: "default/higher", cluster: "member3", priority: 200, replicas: 1},
	} {
		store.Set(claim)
	}

	if store.HasBlockingClaimOnCluster("member1", "default/requester", 100) {
		t.Fatal("did not expect requester or lower-priority claim to block")
	}
	if !store.HasBlockingClaimOnCluster("member2", "default/requester", 100) {
		t.Fatal("expected equal-priority claim to block")
	}
	if !store.HasBlockingClaimOnCluster("member3", "default/requester", 100) {
		t.Fatal("expected higher-priority claim to block")
	}
}

func TestPreemptionClaimStoreSetSupersedesLowerPriorityClaimsOnCluster(t *testing.T) {
	now := time.Now()
	store := newPreemptionClaimStore(defaultPreemptionClaimTTL, func() time.Time { return now })
	store.Set(preemptionClaim{bindingKey: "default/lower", cluster: "member1", priority: 50, replicas: 1})
	store.Set(preemptionClaim{bindingKey: "default/equal", cluster: "member1", priority: 100, replicas: 1})
	store.Set(preemptionClaim{bindingKey: "default/other-cluster", cluster: "member2", priority: 50, replicas: 1})

	store.Set(preemptionClaim{bindingKey: "default/requester", cluster: "member1", priority: 100, replicas: 1})

	if _, ok := store.get("default/lower"); ok {
		t.Fatal("expected lower-priority claim on same cluster to be superseded")
	}
	if _, ok := store.get("default/equal"); !ok {
		t.Fatal("expected equal-priority claim on same cluster to remain")
	}
	if _, ok := store.get("default/other-cluster"); !ok {
		t.Fatal("expected lower-priority claim on another cluster to remain")
	}
	if _, ok := store.get("default/requester"); !ok {
		t.Fatal("expected requester claim to be stored")
	}
}

func TestWithClaimDeductions(t *testing.T) {
	now := time.Now()
	store := newPreemptionClaimStore(defaultPreemptionClaimTTL, func() time.Time { return now })
	for _, claim := range []preemptionClaim{
		{
			bindingKey: "default/higher-priority",
			cluster:    "member1",
			priority:   200,
			replicas:   2,
			resourceNeed: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			},
		},
		{
			bindingKey: "default/requester",
			cluster:    "member1",
			priority:   100,
			replicas:   4,
			resourceNeed: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			},
		},
		{
			bindingKey: "default/equal-priority",
			cluster:    "member1",
			priority:   100,
			replicas:   3,
			resourceNeed: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			},
		},
		{
			bindingKey: "default/lower-priority",
			cluster:    "member1",
			priority:   50,
			replicas:   9,
		},
		{
			bindingKey: "default/overflow",
			cluster:    "member2",
			priority:   100,
			replicas:   10,
			resourceNeed: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			},
		},
	} {
		store.Set(claim)
	}

	available := []workv1alpha2.TargetCluster{
		{Name: "member1", Replicas: 10},
		{Name: "member2", Replicas: 5},
		{Name: "member3", Replicas: 3},
	}
	got := withClaimDeductions(available, store, "default/requester", 100, &workv1alpha2.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1"),
		},
	})
	want := []workv1alpha2.TargetCluster{
		{Name: "member1", Replicas: 5},
		{Name: "member2", Replicas: 0},
		{Name: "member3", Replicas: 3},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d clusters, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cluster %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if available[0].Replicas != 10 || available[1].Replicas != 5 {
		t.Fatalf("withClaimDeductions mutated input: %+v", available)
	}
}

func TestWithClaimDeductionsNilStore(t *testing.T) {
	available := []workv1alpha2.TargetCluster{{Name: "member1", Replicas: 10}}
	got := withClaimDeductions(available, nil, "default/requester", 100, nil)

	if len(got) != 1 || got[0] != available[0] {
		t.Fatalf("got %+v, want %+v", got, available)
	}
	got[0].Replicas = 1
	if available[0].Replicas != 10 {
		t.Fatalf("withClaimDeductions mutated input: %+v", available)
	}
}

func TestWithClaimDeductionsUsesBindingIdentity(t *testing.T) {
	now := time.Now()
	store := newPreemptionClaimStore(defaultPreemptionClaimTTL, func() time.Time { return now })
	store.Set(preemptionClaim{
		bindingKey: "resourcebinding/default/deployment-foo",
		cluster:    "member1",
		priority:   100,
		replicas:   2,
		resourceNeed: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1"),
		},
	})

	got := withClaimDeductions(
		[]workv1alpha2.TargetCluster{{Name: "member1", Replicas: 5}},
		store,
		"resourcebinding/default/statefulset-foo",
		100,
		&workv1alpha2.ReplicaRequirements{ResourceRequest: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}},
	)
	if got[0].Replicas != 3 {
		t.Fatalf("withClaimDeductions() replicas = %d, want 3", got[0].Replicas)
	}
}
