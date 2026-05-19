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

package nodeautoscaler

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
)

const (
	defaultFailureThreshold = 3 * time.Minute
	defaultRecoveryInterval = 10 * time.Minute
)

var (
	karpenterNodePoolGVR  = schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1", Resource: "nodepools"}
	karpenterNodeClaimGVR = schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1", Resource: "nodeclaims"}
)

// KarpenterProvider implements CapacityProvider using Karpenter NodePool/NodeClaim CRDs.
type KarpenterProvider struct {
	client           dynamic.Interface
	failureThreshold time.Duration // how long Pending+no NodeClaim before marking failed
	recoveryInterval time.Duration // how long to wait before retrying a failed pool
	mu               sync.Mutex
	failedPools      map[string]time.Time // poolName → time marked failed
	zeroClaimsSince  map[string]time.Time // poolName → first time observed with 0 NodeClaims
}

// NewKarpenterProvider creates a KarpenterProvider.
func NewKarpenterProvider(client dynamic.Interface) *KarpenterProvider {
	return &KarpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
}

// IsAvailable returns true if Karpenter NodePool CRD exists in the cluster.
func (k *KarpenterProvider) IsAvailable(ctx context.Context) bool {
	_, err := k.client.Resource(karpenterNodePoolGVR).List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

// GetPotentialReplicas calculates how many additional replicas could be scheduled
// based on NodePool limits minus current NodeClaim usage.
// Pools marked as failed are skipped until recoveryInterval elapses.
func (k *KarpenterProvider) GetPotentialReplicas(ctx context.Context, req *pb.ReplicaRequirements, hasPendingPods bool) (int32, error) {
	npList, err := k.client.Resource(karpenterNodePoolGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	ncList, err := k.client.Resource(karpenterNodeClaimGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		ncList = nil
	}

	usagePerPool, claimCountPerPool := aggregateNodeClaims(ncList)

	k.mu.Lock()
	defer k.mu.Unlock()

	var total int32
	for _, np := range npList.Items {
		poolName := np.GetName()
		claimCount := claimCountPerPool[poolName]
		usage := usagePerPool[poolName]

		if claimCount > 0 {
			delete(k.failedPools, poolName)
			delete(k.zeroClaimsSince, poolName)
		}

		if k.isPoolSkipped(poolName, claimCount, hasPendingPods) {
			continue
		}

		potential := calcPotentialFromLimits(np.Object, usage, req)
		if potential > 0 {
			klog.V(4).Infof("NodePool %s: potential=%d (claims=%d)", poolName, potential, claimCount)
		}
		total += potential
	}
	return total, nil
}

// aggregateNodeClaims returns per-pool resource usage and claim counts from NodeClaims.
func aggregateNodeClaims(ncList *unstructured.UnstructuredList) (usagePerPool map[string]map[string]int64, claimCountPerPool map[string]int64) {
	usagePerPool = map[string]map[string]int64{}
	claimCountPerPool = map[string]int64{}
	if ncList == nil {
		return
	}
	for _, nc := range ncList.Items {
		poolName := nc.GetLabels()["karpenter.sh/nodepool"]
		if poolName == "" {
			continue
		}
		claimCountPerPool[poolName]++
		if usagePerPool[poolName] == nil {
			usagePerPool[poolName] = map[string]int64{}
		}
		allocatable, _, _ := nestedStringMap(nc.Object, "status", "allocatable")
		for resName, valStr := range allocatable {
			q, err := resource.ParseQuantity(valStr)
			if err == nil {
				usagePerPool[poolName][resName] += q.MilliValue()
			}
		}
	}
	return
}

// isPoolSkipped returns true if the pool should be skipped due to failure detection.
// Must be called with k.mu held.
func (k *KarpenterProvider) isPoolSkipped(poolName string, claimCount int64, hasPendingPods bool) bool {
	// Check if pool is marked as failed
	if failedAt, ok := k.failedPools[poolName]; ok {
		if time.Since(failedAt) < k.recoveryInterval {
			klog.V(4).Infof("NodePool %s: skipping (failed %s ago, recovery in %s)",
				poolName, time.Since(failedAt).Round(time.Second), k.recoveryInterval)
			return true
		}
		klog.V(4).Infof("NodePool %s: recovery interval elapsed, retrying", poolName)
		delete(k.failedPools, poolName)
		delete(k.zeroClaimsSince, poolName)
	}

	// Auto-detect failure: NodeClaim=0 for longer than failureThreshold.
	// Only track when there are Pending pods to avoid false positives.
	if claimCount == 0 && hasPendingPods {
		if first, ok := k.zeroClaimsSince[poolName]; ok {
			if time.Since(first) > k.failureThreshold {
				klog.V(2).Infof("NodePool %s: no NodeClaims for %s (threshold %s), marking as failed",
					poolName, time.Since(first).Round(time.Second), k.failureThreshold)
				k.failedPools[poolName] = time.Now()
				return true
			}
		} else {
			k.zeroClaimsSince[poolName] = time.Now()
		}
	} else if claimCount == 0 && !hasPendingPods {
		delete(k.zeroClaimsSince, poolName)
	}

	return false
}

// MarkPoolFailed marks a NodePool as failed provisioning.
func (k *KarpenterProvider) MarkPoolFailed(poolName string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.failedPools[poolName] = time.Now()
	klog.V(2).Infof("NodePool %s: marked as failed, will retry after %s", poolName, k.recoveryInterval)
}

// calcPotentialFromLimits calculates potential replicas by matching workload resource
// requests against NodePool spec.limits. For each requested resource that has a
// corresponding limit, it computes (limit - used) / request and returns the min.
func calcPotentialFromLimits(npObj map[string]any, usedResources map[string]int64, req *pb.ReplicaRequirements) int32 {
	if req == nil || len(req.ResourceRequest) == 0 {
		return 0
	}

	limitsRaw, found, _ := nestedStringMap(npObj, "spec", "limits")
	if !found || len(limitsRaw) == 0 {
		return 0
	}

	var minPotential int32 = -1
	matched := false

	for resName, quantity := range req.ResourceRequest {
		perReplica := quantity.MilliValue()
		if perReplica <= 0 {
			continue
		}

		limitStr, ok := limitsRaw[string(resName)]
		if !ok {
			continue
		}

		q, err := resource.ParseQuantity(limitStr)
		if err != nil {
			continue
		}
		limit := q.MilliValue()

		used := usedResources[string(resName)]
		remaining := limit - used
		if remaining <= 0 {
			return 0
		}

		potential := int32(remaining / perReplica) // #nosec G115
		matched = true
		if minPotential < 0 || potential < minPotential {
			minPotential = potential
		}
	}

	if !matched {
		return 0
	}
	return minPotential
}

// nestedStringMap retrieves a map[string]string from a nested unstructured object.
func nestedStringMap(obj map[string]any, fields ...string) (map[string]string, bool, error) {
	var current any = obj
	for _, f := range fields {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		current, ok = m[f]
		if !ok {
			return nil, false, nil
		}
	}
	m, ok := current.(map[string]any)
	if !ok {
		return nil, false, nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result, true, nil
}
