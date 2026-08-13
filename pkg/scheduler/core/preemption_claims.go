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
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

const defaultPreemptionClaimTTL = 10 * time.Minute
const maxInt32 = int64(1<<31 - 1)

type preemptionClaim struct {
	bindingKey   string
	cluster      string
	priority     int32
	replicas     int32
	resourceNeed corev1.ResourceList
	expiry       time.Time
}

// PreemptionClaimStore tracks in-flight preemption reservations.
type PreemptionClaimStore struct {
	lock   sync.Mutex
	claims map[string]preemptionClaim
	ttl    time.Duration
	now    func() time.Time
}

// NewPreemptionClaimStore creates a preemption claim store using the default TTL.
func NewPreemptionClaimStore() *PreemptionClaimStore {
	return newPreemptionClaimStore(defaultPreemptionClaimTTL, time.Now)
}

func newPreemptionClaimStore(ttl time.Duration, now func() time.Time) *PreemptionClaimStore {
	return &PreemptionClaimStore{
		claims: make(map[string]preemptionClaim),
		ttl:    ttl,
		now:    now,
	}
}

// Set stores or replaces the claim for a binding.
func (s *PreemptionClaimStore) Set(claim preemptionClaim) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.sweepExpiredLocked()
	for bindingKey, existing := range s.claims {
		if bindingKey != claim.bindingKey && existing.cluster == claim.cluster && existing.priority < claim.priority {
			delete(s.claims, bindingKey)
		}
	}
	claim.expiry = s.now().Add(s.ttl)
	s.claims[claim.bindingKey] = clonePreemptionClaim(claim)
}

func (s *PreemptionClaimStore) get(bindingKey string) (preemptionClaim, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.sweepExpiredLocked()
	claim, ok := s.claims[bindingKey]
	if !ok {
		return preemptionClaim{}, false
	}
	return clonePreemptionClaim(claim), true
}

// Clear removes the claim for a binding.
func (s *PreemptionClaimStore) Clear(bindingKey string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.claims, bindingKey)
}

// HasClaimOnCluster reports whether any active claim reserves capacity on cluster.
func (s *PreemptionClaimStore) HasClaimOnCluster(cluster string) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.sweepExpiredLocked()
	for _, claim := range s.claims {
		if claim.cluster == cluster {
			return true
		}
	}
	return false
}

// HasBlockingClaimOnCluster reports whether an active claim by another binding
// has equal or higher priority on cluster.
func (s *PreemptionClaimStore) HasBlockingClaimOnCluster(cluster, requesterBindingKey string, requesterPriority int32) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.sweepExpiredLocked()
	for _, claim := range s.claims {
		if claim.bindingKey != requesterBindingKey && claim.cluster == cluster && claim.priority >= requesterPriority {
			return true
		}
	}
	return false
}

func (s *PreemptionClaimStore) list() []preemptionClaim {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.sweepExpiredLocked()
	claims := make([]preemptionClaim, 0, len(s.claims))
	for _, claim := range s.claims {
		claims = append(claims, clonePreemptionClaim(claim))
	}
	return claims
}

func (s *PreemptionClaimStore) sweepExpiredLocked() {
	now := s.now()
	for bindingKey, claim := range s.claims {
		if !claim.expiry.After(now) {
			delete(s.claims, bindingKey)
		}
	}
}

func clonePreemptionClaim(claim preemptionClaim) preemptionClaim {
	if claim.resourceNeed != nil {
		claim.resourceNeed = claim.resourceNeed.DeepCopy()
	}
	return claim
}

func withClaimDeductions(available []workv1alpha2.TargetCluster, claims *PreemptionClaimStore, requesterBindingKey string, requesterPriority int32, requesterRequirements *workv1alpha2.ReplicaRequirements) []workv1alpha2.TargetCluster {
	result := append([]workv1alpha2.TargetCluster(nil), available...)
	if claims == nil {
		return result
	}

	claimsByCluster := make(map[string]int32)
	for _, claim := range claims.list() {
		if claim.bindingKey == requesterBindingKey || claim.priority < requesterPriority || claim.replicas <= 0 {
			continue
		}
		claimsByCluster[claim.cluster] += claimReplicaDeduction(claim, requesterRequirements)
	}

	for i := range result {
		deduction := claimsByCluster[result[i].Name]
		if deduction >= result[i].Replicas {
			result[i].Replicas = 0
			continue
		}
		result[i].Replicas -= deduction
	}
	return result
}

func claimReplicaDeduction(claim preemptionClaim, requesterRequirements *workv1alpha2.ReplicaRequirements) int32 {
	if requesterRequirements == nil || len(requesterRequirements.ResourceRequest) == 0 || len(claim.resourceNeed) == 0 {
		return claim.replicas
	}

	var deduction int64
	for resourceName, requesterQuantity := range requesterRequirements.ResourceRequest {
		if !positiveQuantity(requesterQuantity) {
			continue
		}
		claimedQuantity, ok := claim.resourceNeed[resourceName]
		if !ok || !positiveQuantity(claimedQuantity) {
			continue
		}
		totalClaimed := claimedQuantity.DeepCopy()
		totalClaimed.Mul(int64(claim.replicas))
		replicas := ceilQuantityRatio(totalClaimed, requesterQuantity)
		if replicas > deduction {
			deduction = replicas
		}
	}
	if deduction <= 0 {
		return 0
	}
	if deduction > maxInt32 {
		return int32(maxInt32)
	}
	return int32(deduction)
}

func positiveQuantity(q resource.Quantity) bool {
	return q.Sign() > 0
}

func ceilQuantityRatio(numerator, denominator resource.Quantity) int64 {
	n := numerator.MilliValue()
	d := denominator.MilliValue()
	if n <= 0 || d <= 0 {
		return 0
	}
	return (n + d - 1) / d
}
