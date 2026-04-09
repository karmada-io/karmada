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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
)

var (
	nodePoolGVR  = schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1", Resource: "nodepools"}
	nodeClaimGVR = schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1", Resource: "nodeclaims"}
)

func newNodePool(name string, gpuLimit int64) *unstructured.Unstructured {
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName(name)
	if gpuLimit >= 0 {
		_ = unstructured.SetNestedField(np.Object, fmt.Sprintf("%d", gpuLimit), "spec", "limits", "nvidia.com/gpu")
	}
	return np
}

func newNodeClaim(name, nodePoolName string) *unstructured.Unstructured {
	return newNodeClaimWithResources(name, nodePoolName, map[string]string{"nvidia.com/gpu": "1"})
}

func newNodeClaimWithResources(name, nodePoolName string, allocatable map[string]string) *unstructured.Unstructured {
	nc := &unstructured.Unstructured{}
	nc.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodeClaim"})
	nc.SetName(name)
	nc.SetLabels(map[string]string{"karpenter.sh/nodepool": nodePoolName})
	for k, v := range allocatable {
		_ = unstructured.SetNestedField(nc.Object, v, "status", "allocatable", k)
	}
	return nc
}

func TestKarpenterProvider_NotAvailable(t *testing.T) {
	// No NodePool objects → potential=0
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			nodePoolGVR:  "NodePoolList",
			nodeClaimGVR: "NodeClaimList",
		},
	)
	provider := NewKarpenterProvider(client)
	assert.True(t, provider.IsAvailable(context.Background()))
	req := &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI),
		},
	}
	replicas, err := provider.GetPotentialReplicas(context.Background(), req, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), replicas)
}

func TestKarpenterProvider_Available(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			nodePoolGVR:  "NodePoolList",
			nodeClaimGVR: "NodeClaimList",
		},
		newNodePool("gpu-pool", 4),
	)
	provider := NewKarpenterProvider(client)
	assert.True(t, provider.IsAvailable(context.Background()))
}

func TestKarpenterProvider_PotentialReplicas(t *testing.T) {
	// NodePool limit=4 GPU, 1 existing NodeClaim → potential=3
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			nodePoolGVR:  "NodePoolList",
			nodeClaimGVR: "NodeClaimList",
		},
		newNodePool("gpu-pool", 4),
		newNodeClaim("claim-1", "gpu-pool"),
	)
	provider := NewKarpenterProvider(client)
	req := &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI),
		},
	}
	replicas, err := provider.GetPotentialReplicas(context.Background(), req, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(3), replicas)
}

func TestKarpenterProvider_NoLimits(t *testing.T) {
	// NodePool without limits → 0 (conservative fallback)
	scheme := runtime.NewScheme()
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("no-limit-pool")
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			nodePoolGVR:  "NodePoolList",
			nodeClaimGVR: "NodeClaimList",
		},
		np,
	)
	provider := NewKarpenterProvider(client)
	req := &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI),
		},
	}
	replicas, err := provider.GetPotentialReplicas(context.Background(), req, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), replicas)
}

func TestKarpenterProvider_FullyUsed(t *testing.T) {
	// NodePool limit=1, 1 NodeClaim → potential=0
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			nodePoolGVR:  "NodePoolList",
			nodeClaimGVR: "NodeClaimList",
		},
		newNodePool("gpu-pool", 1),
		newNodeClaim("claim-1", "gpu-pool"),
	)
	provider := NewKarpenterProvider(client)
	req := &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI),
		},
	}
	replicas, err := provider.GetPotentialReplicas(context.Background(), req, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), replicas)
}

func newFakeClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			nodePoolGVR:  "NodePoolList",
			nodeClaimGVR: "NodeClaimList",
		},
		objects...,
	)
}

func gpuReq() *pb.ReplicaRequirements {
	return &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI),
		},
	}
}

func TestKarpenterProvider_FailureDetection_FirstCallAllowed(t *testing.T) {
	// First call: no failedPools → should return potential based on limit
	client := newFakeClient(newNodePool("gpu-pool", 4))
	provider := NewKarpenterProvider(client)
	provider.failureThreshold = 3 * time.Minute
	provider.recoveryInterval = 10 * time.Minute

	replicas, err := provider.GetPotentialReplicas(context.Background(), gpuReq(), false)
	assert.NoError(t, err)
	assert.Equal(t, int32(4), replicas)
}

func TestKarpenterProvider_FailureDetection_PoolFailed(t *testing.T) {
	// NodeClaim=0, pool marked as failed → should return 0
	client := newFakeClient(newNodePool("gpu-pool", 4))
	provider := NewKarpenterProvider(client)
	provider.failureThreshold = 3 * time.Minute
	provider.recoveryInterval = 10 * time.Minute
	// Simulate: pool was marked failed 1 minute ago
	provider.failedPools = map[string]time.Time{
		"gpu-pool": time.Now().Add(-1 * time.Minute),
	}

	replicas, err := provider.GetPotentialReplicas(context.Background(), gpuReq(), false)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), replicas)
}

func TestKarpenterProvider_FailureDetection_RecoveryAfterInterval(t *testing.T) {
	// Pool was marked failed 11 minutes ago, recoveryInterval=10min → should retry
	client := newFakeClient(newNodePool("gpu-pool", 4))
	provider := NewKarpenterProvider(client)
	provider.failureThreshold = 3 * time.Minute
	provider.recoveryInterval = 10 * time.Minute
	provider.failedPools = map[string]time.Time{
		"gpu-pool": time.Now().Add(-11 * time.Minute),
	}

	replicas, err := provider.GetPotentialReplicas(context.Background(), gpuReq(), false)
	assert.NoError(t, err)
	assert.Equal(t, int32(4), replicas)
	// failedPools should be cleared
	assert.Empty(t, provider.failedPools)
}

func TestKarpenterProvider_FailureDetection_NodeClaimExists(t *testing.T) {
	// NodeClaim exists → pool is NOT failed even if in failedPools
	// (Karpenter managed to create a NodeClaim = provisioning is working)
	client := newFakeClient(newNodePool("gpu-pool", 4), newNodeClaim("claim-1", "gpu-pool"))
	provider := NewKarpenterProvider(client)
	provider.failureThreshold = 3 * time.Minute
	provider.recoveryInterval = 10 * time.Minute
	provider.failedPools = map[string]time.Time{
		"gpu-pool": time.Now().Add(-1 * time.Minute),
	}

	replicas, err := provider.GetPotentialReplicas(context.Background(), gpuReq(), false)
	assert.NoError(t, err)
	assert.Equal(t, int32(3), replicas) // limit=4, claims=1 → 3
	// failedPools should be cleared since NodeClaim exists
	assert.Empty(t, provider.failedPools)
}

func TestKarpenterProvider_MarkPoolFailed(t *testing.T) {
	// MarkPoolFailed should register the pool
	client := newFakeClient(newNodePool("gpu-pool", 4))
	provider := NewKarpenterProvider(client)
	provider.failureThreshold = 3 * time.Minute
	provider.recoveryInterval = 10 * time.Minute

	provider.MarkPoolFailed("gpu-pool")
	assert.Contains(t, provider.failedPools, "gpu-pool")

	// After marking, GetPotentialReplicas should return 0
	replicas, err := provider.GetPotentialReplicas(context.Background(), gpuReq(), false)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), replicas)
}

func TestKarpenterProvider_AutoDetect_ThresholdNotReached(t *testing.T) {
	// NodeClaim=0, zeroClaimsSince just now → threshold not reached → still return potential
	client := newFakeClient(newNodePool("gpu-pool", 4))
	provider := NewKarpenterProvider(client)
	provider.failureThreshold = 3 * time.Minute
	provider.recoveryInterval = 10 * time.Minute

	// First call with pending pods: starts tracking
	replicas, err := provider.GetPotentialReplicas(context.Background(), gpuReq(), true)
	assert.NoError(t, err)
	assert.Equal(t, int32(4), replicas)
	assert.Contains(t, provider.zeroClaimsSince, "gpu-pool")

	// Second call immediately: threshold not reached → still returns potential
	replicas, err = provider.GetPotentialReplicas(context.Background(), gpuReq(), true)
	assert.NoError(t, err)
	assert.Equal(t, int32(4), replicas)
}

func TestKarpenterProvider_AutoDetect_ThresholdReached(t *testing.T) {
	// NodeClaim=0, zeroClaimsSince 4 minutes ago, threshold=3min → auto-mark failed
	client := newFakeClient(newNodePool("gpu-pool", 4))
	provider := NewKarpenterProvider(client)
	provider.failureThreshold = 3 * time.Minute
	provider.recoveryInterval = 10 * time.Minute
	provider.zeroClaimsSince = map[string]time.Time{
		"gpu-pool": time.Now().Add(-4 * time.Minute),
	}

	replicas, err := provider.GetPotentialReplicas(context.Background(), gpuReq(), true)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), replicas)
	assert.Contains(t, provider.failedPools, "gpu-pool")
}

func TestKarpenterProvider_AutoDetect_NoPendingPods(t *testing.T) {
	// NodeClaim=0 but no pending pods → should NOT start zeroClaimsSince tracking
	client := newFakeClient(newNodePool("gpu-pool", 4))
	provider := NewKarpenterProvider(client)
	provider.failureThreshold = 3 * time.Minute
	provider.recoveryInterval = 10 * time.Minute

	replicas, err := provider.GetPotentialReplicas(context.Background(), gpuReq(), false)
	assert.NoError(t, err)
	assert.Equal(t, int32(4), replicas)
	assert.Empty(t, provider.zeroClaimsSince) // NOT tracking
}

func TestKarpenterProvider_AutoDetect_ClearedWhenNodeClaimAppears(t *testing.T) {
	// Was tracking zeroClaimsSince, then NodeClaim appears → clear tracking
	client := newFakeClient(newNodePool("gpu-pool", 4), newNodeClaim("claim-1", "gpu-pool"))
	provider := NewKarpenterProvider(client)
	provider.failureThreshold = 3 * time.Minute
	provider.recoveryInterval = 10 * time.Minute
	provider.zeroClaimsSince = map[string]time.Time{
		"gpu-pool": time.Now().Add(-4 * time.Minute),
	}

	replicas, err := provider.GetPotentialReplicas(context.Background(), gpuReq(), false)
	assert.NoError(t, err)
	assert.Equal(t, int32(3), replicas) // limit=4, claims=1
	assert.Empty(t, provider.zeroClaimsSince)
	assert.Empty(t, provider.failedPools)
}

func TestKarpenterProvider_CPUWorkload(t *testing.T) {
	// NodePool with cpu limit=64 → CPU-only workload should get potential
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("cpu-pool")
	_ = unstructured.SetNestedField(np.Object, "64", "spec", "limits", "cpu")

	client := newFakeClient(np)
	provider := NewKarpenterProvider(client)
	req := &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			corev1.ResourceCPU: *resource.NewQuantity(4, resource.DecimalSI),
		},
	}
	replicas, err := provider.GetPotentialReplicas(context.Background(), req, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(16), replicas) // 64/4 = 16
}

func TestKarpenterProvider_MultiResource(t *testing.T) {
	// NodePool with gpu=8 and cpu=64, workload requests both → min(8/1, 64/4) = 8
	np := newNodePool("gpu-pool", 8)
	_ = unstructured.SetNestedField(np.Object, "64", "spec", "limits", "cpu")

	client := newFakeClient(np)
	provider := NewKarpenterProvider(client)
	req := &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			"nvidia.com/gpu":   *resource.NewQuantity(1, resource.DecimalSI),
			corev1.ResourceCPU: *resource.NewQuantity(4, resource.DecimalSI),
		},
	}
	replicas, err := provider.GetPotentialReplicas(context.Background(), req, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(8), replicas) // min(8/1, 64/4) = min(8,16) = 8
}

func TestKarpenterProvider_NeuronWorkload(t *testing.T) {
	// AWS Neuron accelerator
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("neuron-pool")
	_ = unstructured.SetNestedField(np.Object, "4", "spec", "limits", "aws.amazon.com/neuroncore")

	client := newFakeClient(np)
	provider := NewKarpenterProvider(client)
	req := &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			corev1.ResourceName("aws.amazon.com/neuroncore"): *resource.NewQuantity(2, resource.DecimalSI),
		},
	}
	replicas, err := provider.GetPotentialReplicas(context.Background(), req, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(2), replicas) // 4/2 = 2
}

func TestKarpenterProvider_NoMatchingLimit(t *testing.T) {
	// NodePool has cpu limit only, workload requests GPU → 0
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("cpu-pool")
	_ = unstructured.SetNestedField(np.Object, "64", "spec", "limits", "cpu")

	client := newFakeClient(np)
	provider := NewKarpenterProvider(client)
	req := &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI),
		},
	}
	replicas, err := provider.GetPotentialReplicas(context.Background(), req, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), replicas)
}

func TestKarpenterProvider_MultiGPUPerNode(t *testing.T) {
	// p5.48xlarge: 8 GPUs per node. NodePool limit=16, 1 NodeClaim with 8 GPUs → remaining=8
	np := newNodePool("gpu-pool", 16)
	nc := newNodeClaimWithResources("claim-1", "gpu-pool", map[string]string{"nvidia.com/gpu": "8"})
	client := newFakeClient(np, nc)
	provider := NewKarpenterProvider(client)
	req := &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI),
		},
	}
	replicas, err := provider.GetPotentialReplicas(context.Background(), req, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(8), replicas) // (16 - 8) / 1 = 8
}

func TestKarpenterProvider_MultiGPUPerNode_MultiClaim(t *testing.T) {
	// NodePool limit=16, 2 NodeClaims with 4 GPUs each → used=8, remaining=8
	np := newNodePool("gpu-pool", 16)
	nc1 := newNodeClaimWithResources("claim-1", "gpu-pool", map[string]string{"nvidia.com/gpu": "4"})
	nc2 := newNodeClaimWithResources("claim-2", "gpu-pool", map[string]string{"nvidia.com/gpu": "4"})
	client := newFakeClient(np, nc1, nc2)
	provider := NewKarpenterProvider(client)
	req := &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			"nvidia.com/gpu": *resource.NewQuantity(2, resource.DecimalSI),
		},
	}
	replicas, err := provider.GetPotentialReplicas(context.Background(), req, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(4), replicas) // (16 - 8) / 2 = 4
}

func TestKarpenterProvider_CPUWorkload_WithExistingClaims(t *testing.T) {
	// NodePool cpu limit=16, 2 NodeClaims with cpu=2 each → used=4, remaining=12
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("cpu-pool")
	_ = unstructured.SetNestedField(np.Object, "16", "spec", "limits", "cpu")

	nc1 := newNodeClaimWithResources("claim-1", "cpu-pool", map[string]string{"cpu": "2"})
	nc2 := newNodeClaimWithResources("claim-2", "cpu-pool", map[string]string{"cpu": "2"})

	client := newFakeClient(np, nc1, nc2)
	provider := NewKarpenterProvider(client)
	req := &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI),
		},
	}
	replicas, err := provider.GetPotentialReplicas(context.Background(), req, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(12), replicas) // (16 - 4) / 1 = 12
}

func TestKarpenterProvider_MemoryWorkload(t *testing.T) {
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("mem-pool")
	_ = unstructured.SetNestedField(np.Object, "64Gi", "spec", "limits", "memory")

	client := newFakeClient(np)
	provider := NewKarpenterProvider(client)
	req := &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			corev1.ResourceMemory: *resource.NewQuantity(8*1024*1024*1024, resource.BinarySI),
		},
	}
	replicas, err := provider.GetPotentialReplicas(context.Background(), req, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(8), replicas) // 64Gi / 8Gi = 8
}

func TestKarpenterProvider_CPUMillicoreWorkload(t *testing.T) {
	// NodePool cpu limit=4 (=4000m), workload requests 500m → potential = 4000/500 = 8
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("cpu-pool")
	_ = unstructured.SetNestedField(np.Object, "4", "spec", "limits", "cpu")

	client := newFakeClient(np)
	provider := NewKarpenterProvider(client)
	req := &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("500m"),
		},
	}
	replicas, err := provider.GetPotentialReplicas(context.Background(), req, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(8), replicas) // 4000m / 500m = 8
}
