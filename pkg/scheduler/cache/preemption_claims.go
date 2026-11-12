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
	"slices"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/clock"
)

// DefaultPreemptionClaimTTL is how long a preemption claim stays active after it is set.
// It is longer than the 5-minute unschedulableBindings flush plus the victims' eviction
// grace period, so a preemptor that is only retried by the periodic flush still finds
// its claim in place.
const DefaultPreemptionClaimTTL = 10 * time.Minute

// PreemptionClaim records that a preemptor has evicted victims from a cluster and is
// waiting for the freed capacity there. It is the cluster-level equivalent of the
// Kubernetes scheduler's nominated node.
type PreemptionClaim struct {
	// BindingKey identifies the preemptor binding, e.g. "Kind/Namespace/Name".
	// The store treats it as an opaque key.
	BindingKey string
	// Cluster is the cluster the preemptor evicted victims from.
	Cluster string
	// Priority is the preemptor's schedule priority.
	Priority int32
	// ResourceNeed is the resources the preemptor needs on Cluster.
	ResourceNeed corev1.ResourceList
}

// claimEntry is a claim together with the time it expires.
type claimEntry struct {
	claim     PreemptionClaim
	expiresAt time.Time
}

// isExpired reports whether the claim has expired at the given time.
func (e *claimEntry) isExpired(now time.Time) bool {
	return now.After(e.expiresAt)
}

// PreemptionClaimStore keeps the active preemption claims in memory.
//
// The store is indexed two ways: byBinding holds at most one claim per preemptor, so
// setting a new claim replaces the old one instead of accumulating; byCluster holds
// every claim on a cluster, for per-cluster capacity accounting.
//
// Claims expire DefaultPreemptionClaimTTL after they are set. Expiry is lazy: readers
// ignore expired claims, and an expired claim is dropped from the store when its
// binding's claim is replaced or cleared.
//
// Claims are not persisted. They are lost when the scheduler restarts or loses
// leadership, in which case an in-flight preemptor may preempt once more.
type PreemptionClaimStore struct {
	mu sync.RWMutex
	// byBinding maps binding key → the binding's claim.
	byBinding map[string]*claimEntry
	// byCluster maps cluster name → all claims on that cluster.
	byCluster map[string][]*claimEntry

	clock clock.PassiveClock
}

// NewPreemptionClaimStore returns an empty PreemptionClaimStore.
// If clk is nil, the real clock is used.
func NewPreemptionClaimStore(clk clock.PassiveClock) *PreemptionClaimStore {
	if clk == nil {
		clk = clock.RealClock{}
	}
	return &PreemptionClaimStore{
		byBinding: make(map[string]*claimEntry),
		byCluster: make(map[string][]*claimEntry),
		clock:     clk,
	}
}

// Set records a claim for claim.BindingKey, replacing any claim the binding already
// holds, whether on the same cluster or another one. The claim expires
// DefaultPreemptionClaimTTL from now.
// claim.ResourceNeed is deep-copied, so the caller may modify it afterwards.
func (s *PreemptionClaimStore) Set(claim PreemptionClaim) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.deleteLocked(claim.BindingKey)

	claim.ResourceNeed = claim.ResourceNeed.DeepCopy()
	entry := &claimEntry{
		claim:     claim,
		expiresAt: s.clock.Now().Add(DefaultPreemptionClaimTTL),
	}
	s.byBinding[claim.BindingKey] = entry
	s.byCluster[claim.Cluster] = append(s.byCluster[claim.Cluster], entry)
}

// HasClaimOnCluster reports whether the binding holds an unexpired claim on the cluster.
func (s *PreemptionClaimStore) HasClaimOnCluster(bindingKey, cluster string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.byBinding[bindingKey]
	if !ok {
		return false
	}
	return entry.claim.Cluster == cluster && !entry.isExpired(s.clock.Now())
}

// ClaimsOnCluster returns the unexpired claims on the cluster, in the order they were set.
// Each returned claim is a deep copy, so callers may modify the result without affecting
// the store.
func (s *PreemptionClaimStore) ClaimsOnCluster(cluster string) []PreemptionClaim {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := s.clock.Now()
	var result []PreemptionClaim
	for _, entry := range s.byCluster[cluster] {
		if entry.isExpired(now) {
			continue
		}
		claim := entry.claim
		claim.ResourceNeed = claim.ResourceNeed.DeepCopy()
		result = append(result, claim)
	}
	return result
}

// ClearBinding removes the binding's claim, if any. It is called when the preemptor is
// scheduled or deleted, or when none of its victims could be evicted.
func (s *PreemptionClaimStore) ClearBinding(bindingKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.deleteLocked(bindingKey)
}

// deleteLocked removes the binding's claim from both indexes. The caller must hold s.mu.
func (s *PreemptionClaimStore) deleteLocked(bindingKey string) {
	entry, ok := s.byBinding[bindingKey]
	if !ok {
		return
	}
	delete(s.byBinding, bindingKey)

	cluster := entry.claim.Cluster
	claims := s.byCluster[cluster]
	if i := slices.Index(claims, entry); i >= 0 {
		claims = slices.Delete(claims, i, i+1)
	}
	if len(claims) == 0 {
		delete(s.byCluster, cluster)
		return
	}
	s.byCluster[cluster] = claims
}
