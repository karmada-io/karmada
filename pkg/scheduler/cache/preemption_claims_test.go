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

package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/clock"
	testingclock "k8s.io/utils/clock/testing"
)

func newTestClaim(bindingKey, cluster string, priority int32, cpu string) PreemptionClaim {
	return PreemptionClaim{
		BindingKey: bindingKey,
		Cluster:    cluster,
		Priority:   priority,
		ResourceNeed: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse(cpu),
		},
	}
}

func TestNewPreemptionClaimStore(t *testing.T) {
	s := NewPreemptionClaimStore(nil)
	assert.NotNil(t, s.byBinding)
	assert.NotNil(t, s.byCluster)
	assert.Equal(t, clock.RealClock{}, s.clock, "nil clock should default to the real clock")

	fakeClock := testingclock.NewFakeClock(time.Now())
	s = NewPreemptionClaimStore(fakeClock)
	assert.Equal(t, fakeClock, s.clock)
}

func TestPreemptionClaimStore_Set(t *testing.T) {
	s := NewPreemptionClaimStore(testingclock.NewFakeClock(time.Now()))
	claim := newTestClaim("ResourceBinding/default/foo", "member1", 100, "2")
	s.Set(claim)

	assert.Len(t, s.byBinding, 1)
	assert.Len(t, s.byCluster["member1"], 1)
	assert.Equal(t, []PreemptionClaim{claim}, s.ClaimsOnCluster("member1"))
}

func TestPreemptionClaimStore_HasClaimOnCluster(t *testing.T) {
	s := NewPreemptionClaimStore(testingclock.NewFakeClock(time.Now()))
	s.Set(newTestClaim("ResourceBinding/default/foo", "member1", 100, "2"))

	tests := []struct {
		name       string
		bindingKey string
		cluster    string
		want       bool
	}{
		{
			name:       "binding holds a claim on the cluster",
			bindingKey: "ResourceBinding/default/foo",
			cluster:    "member1",
			want:       true,
		},
		{
			name:       "binding holds a claim on another cluster",
			bindingKey: "ResourceBinding/default/foo",
			cluster:    "member2",
			want:       false,
		},
		{
			name:       "another binding holds the claim on the cluster",
			bindingKey: "ResourceBinding/default/bar",
			cluster:    "member1",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, s.HasClaimOnCluster(tt.bindingKey, tt.cluster))
		})
	}
}

func TestPreemptionClaimStore_ClaimReplacement(t *testing.T) {
	t.Run("claim on another cluster replaces the old one", func(t *testing.T) {
		s := NewPreemptionClaimStore(testingclock.NewFakeClock(time.Now()))
		s.Set(newTestClaim("ResourceBinding/default/foo", "member1", 100, "2"))
		s.Set(newTestClaim("ResourceBinding/default/foo", "member2", 100, "2"))

		assert.False(t, s.HasClaimOnCluster("ResourceBinding/default/foo", "member1"))
		assert.True(t, s.HasClaimOnCluster("ResourceBinding/default/foo", "member2"))
		assert.Empty(t, s.ClaimsOnCluster("member1"))
		assert.Len(t, s.ClaimsOnCluster("member2"), 1)
		assert.Len(t, s.byBinding, 1)
		assert.NotContains(t, s.byCluster, "member1", "empty cluster index should be removed")
	})

	t.Run("claim on the same cluster replaces the old one and resets its expiry", func(t *testing.T) {
		fakeClock := testingclock.NewFakeClock(time.Now())
		s := NewPreemptionClaimStore(fakeClock)
		s.Set(newTestClaim("ResourceBinding/default/foo", "member1", 100, "2"))
		fakeClock.Step(DefaultPreemptionClaimTTL / 2)
		updated := newTestClaim("ResourceBinding/default/foo", "member1", 200, "4")
		s.Set(updated)

		assert.Equal(t, []PreemptionClaim{updated}, s.ClaimsOnCluster("member1"))

		// Past the first claim's expiry, but not the second's.
		fakeClock.Step(DefaultPreemptionClaimTTL/2 + time.Second)
		assert.True(t, s.HasClaimOnCluster("ResourceBinding/default/foo", "member1"))
	})

	t.Run("claims of other bindings are kept", func(t *testing.T) {
		s := NewPreemptionClaimStore(testingclock.NewFakeClock(time.Now()))
		s.Set(newTestClaim("ResourceBinding/default/foo", "member1", 100, "2"))
		s.Set(newTestClaim("ResourceBinding/default/bar", "member1", 50, "1"))
		s.Set(newTestClaim("ResourceBinding/default/foo", "member2", 100, "2"))

		assert.Equal(t, []PreemptionClaim{newTestClaim("ResourceBinding/default/bar", "member1", 50, "1")},
			s.ClaimsOnCluster("member1"))
		assert.True(t, s.HasClaimOnCluster("ResourceBinding/default/bar", "member1"))
	})
}

func TestPreemptionClaimStore_ClearBinding(t *testing.T) {
	s := NewPreemptionClaimStore(testingclock.NewFakeClock(time.Now()))
	s.Set(newTestClaim("ResourceBinding/default/foo", "member1", 100, "2"))
	s.Set(newTestClaim("ResourceBinding/default/bar", "member1", 50, "1"))

	s.ClearBinding("ResourceBinding/default/foo")
	assert.False(t, s.HasClaimOnCluster("ResourceBinding/default/foo", "member1"))
	assert.True(t, s.HasClaimOnCluster("ResourceBinding/default/bar", "member1"))
	assert.Len(t, s.ClaimsOnCluster("member1"), 1)

	// Clearing a binding without a claim is a no-op.
	s.ClearBinding("ResourceBinding/default/unknown")
	assert.Len(t, s.byBinding, 1)

	s.ClearBinding("ResourceBinding/default/bar")
	assert.Empty(t, s.byBinding)
	assert.Empty(t, s.byCluster)
}

func TestPreemptionClaimStore_TTLExpiry(t *testing.T) {
	fakeClock := testingclock.NewFakeClock(time.Now())
	s := NewPreemptionClaimStore(fakeClock)
	s.Set(newTestClaim("ResourceBinding/default/foo", "member1", 100, "2"))

	// A claim is still active at exactly its TTL.
	fakeClock.Step(DefaultPreemptionClaimTTL)
	assert.True(t, s.HasClaimOnCluster("ResourceBinding/default/foo", "member1"))
	assert.Len(t, s.ClaimsOnCluster("member1"), 1)

	fakeClock.Step(time.Nanosecond)
	assert.False(t, s.HasClaimOnCluster("ResourceBinding/default/foo", "member1"))
	assert.Empty(t, s.ClaimsOnCluster("member1"))
	assert.Len(t, s.byBinding, 1, "expired claims are filtered lazily, not removed on read")

	// Setting a claim again after expiry makes it active again.
	s.Set(newTestClaim("ResourceBinding/default/foo", "member1", 100, "2"))
	assert.True(t, s.HasClaimOnCluster("ResourceBinding/default/foo", "member1"))
	assert.Len(t, s.byCluster["member1"], 1)

	// Clearing removes an expired claim from the store.
	fakeClock.Step(DefaultPreemptionClaimTTL + time.Nanosecond)
	s.ClearBinding("ResourceBinding/default/foo")
	assert.Empty(t, s.byBinding)
	assert.Empty(t, s.byCluster)
}

func TestPreemptionClaimStore_ClaimsOnCluster(t *testing.T) {
	fakeClock := testingclock.NewFakeClock(time.Now())
	s := NewPreemptionClaimStore(fakeClock)
	assert.Empty(t, s.ClaimsOnCluster("member1"), "no claims on an unknown cluster")

	foo := newTestClaim("ResourceBinding/default/foo", "member1", 100, "2")
	s.Set(foo)
	fakeClock.Step(time.Minute)
	bar := newTestClaim("ResourceBinding/default/bar", "member1", 50, "1")
	s.Set(bar)
	baz := newTestClaim("ResourceBinding/default/baz", "member2", 10, "1")
	s.Set(baz)

	assert.Equal(t, []PreemptionClaim{foo, bar}, s.ClaimsOnCluster("member1"), "claims are returned in the order they were set")
	assert.Equal(t, []PreemptionClaim{baz}, s.ClaimsOnCluster("member2"))

	// Only foo has expired.
	fakeClock.Step(DefaultPreemptionClaimTTL - time.Minute + time.Nanosecond)
	assert.Equal(t, []PreemptionClaim{bar}, s.ClaimsOnCluster("member1"))
}

func TestPreemptionClaimStore_ResourceNeedIsCopied(t *testing.T) {
	s := NewPreemptionClaimStore(testingclock.NewFakeClock(time.Now()))
	claim := newTestClaim("ResourceBinding/default/foo", "member1", 100, "2")
	s.Set(claim)

	// Modifying the caller's claim after Set does not affect the store.
	claim.ResourceNeed[corev1.ResourceCPU] = resource.MustParse("8")
	got := s.ClaimsOnCluster("member1")
	assert.Equal(t, resource.MustParse("2"), got[0].ResourceNeed[corev1.ResourceCPU])

	// Modifying a returned claim does not affect the store.
	got[0].ResourceNeed[corev1.ResourceCPU] = resource.MustParse("16")
	assert.Equal(t, resource.MustParse("2"), s.ClaimsOnCluster("member1")[0].ResourceNeed[corev1.ResourceCPU])
}

func TestPreemptionClaimStore_Concurrency(t *testing.T) {
	s := NewPreemptionClaimStore(testingclock.NewFakeClock(time.Now()))

	var wg sync.WaitGroup
	for i := range int32(10) {
		wg.Add(1)
		go func(i int32) {
			defer wg.Done()
			key := fmt.Sprintf("ResourceBinding/default/binding-%d", i)
			for j := range 100 {
				cluster := fmt.Sprintf("member%d", j%3)
				s.Set(newTestClaim(key, cluster, i, "1"))
				s.HasClaimOnCluster(key, cluster)
				s.ClaimsOnCluster(cluster)
				if j%10 == 0 {
					s.ClearBinding(key)
				}
			}
		}(i)
	}
	wg.Wait()

	// Every binding ends with exactly one claim, and the two indexes agree.
	assert.Len(t, s.byBinding, 10)
	total := 0
	for cluster, claims := range s.byCluster {
		for _, entry := range claims {
			assert.Equal(t, cluster, entry.claim.Cluster)
			assert.Same(t, s.byBinding[entry.claim.BindingKey], entry)
		}
		total += len(claims)
	}
	assert.Equal(t, 10, total)
}
