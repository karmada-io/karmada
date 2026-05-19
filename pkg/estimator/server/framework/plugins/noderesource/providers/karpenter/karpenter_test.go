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

package karpenter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	schedcache "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/cache"
)

var (
	nodePoolGVR  = schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1", Resource: "nodepools"}
	nodeClaimGVR = schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1", Resource: "nodeclaims"}
)

type fakeHandle struct {
	dc dynamic.Interface
}

func (f fakeHandle) ClientSet() clientset.Interface                         { return nil }
func (f fakeHandle) DynamicClient() dynamic.Interface                       { return f.dc }
func (f fakeHandle) SharedInformerFactory() informers.SharedInformerFactory { return nil }
func (f fakeHandle) Parallelism() int                                       { return 1 }
func (f fakeHandle) NodeCapacityProviders() []string                        { return nil }

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
	return (&pb.ReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{
		"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI),
	})
}

func TestKarpenter_Estimate_NotAvailable(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			nodePoolGVR:  "NodePoolList",
			nodeClaimGVR: "NodeClaimList",
		},
	)
	p, err := New(fakeHandle{dc: client})
	assert.NoError(t, err)
	snapshot := schedcache.NewEmptySnapshot()
	replicas, ret := p.Estimate(context.Background(), snapshot, gpuReq())
	// No NodePools → 0 potential, but available (empty list is not an error)
	assert.Equal(t, int32(0), replicas)
	assert.True(t, ret.IsSuccess())
}

func TestKarpenter_Estimate_PotentialReplicas(t *testing.T) {
	client := newFakeClient(newNodePool("gpu-pool", 4), newNodeClaim("claim-1", "gpu-pool"))
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	snapshot := schedcache.NewEmptySnapshot()
	replicas, ret := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(3), replicas)
	assert.True(t, ret.IsSuccess())
}

func TestKarpenter_Estimate_NoLimits(t *testing.T) {
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("no-limit-pool")
	client := newFakeClient(np)
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	snapshot := schedcache.NewEmptySnapshot()
	replicas, ret := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(0), replicas)
	assert.True(t, ret.IsSuccess())
}

func TestKarpenter_Estimate_FullyUsed(t *testing.T) {
	client := newFakeClient(newNodePool("gpu-pool", 1), newNodeClaim("claim-1", "gpu-pool"))
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	snapshot := schedcache.NewEmptySnapshot()
	replicas, ret := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(0), replicas)
	assert.True(t, ret.IsSuccess())
}

func TestKarpenter_FailureDetection_PoolFailed(t *testing.T) {
	client := newFakeClient(newNodePool("gpu-pool", 4))
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      map[string]time.Time{"gpu-pool": time.Now().Add(-1 * time.Minute)},
		zeroClaimsSince:  make(map[string]time.Time),
	}
	snapshot := schedcache.NewEmptySnapshot()
	replicas, ret := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(0), replicas)
	assert.True(t, ret.IsSuccess())
}

func TestKarpenter_FailureDetection_RecoveryAfterInterval(t *testing.T) {
	client := newFakeClient(newNodePool("gpu-pool", 4))
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: 10 * time.Minute,
		failedPools:      map[string]time.Time{"gpu-pool": time.Now().Add(-11 * time.Minute)},
		zeroClaimsSince:  make(map[string]time.Time),
	}
	snapshot := schedcache.NewEmptySnapshot()
	replicas, ret := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(4), replicas)
	assert.True(t, ret.IsSuccess())
	assert.Empty(t, p.failedPools)
}

func TestKarpenter_FailureDetection_NodeClaimExists(t *testing.T) {
	client := newFakeClient(newNodePool("gpu-pool", 4), newNodeClaim("claim-1", "gpu-pool"))
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      map[string]time.Time{"gpu-pool": time.Now().Add(-1 * time.Minute)},
		zeroClaimsSince:  make(map[string]time.Time),
	}
	snapshot := schedcache.NewEmptySnapshot()
	replicas, ret := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(3), replicas)
	assert.True(t, ret.IsSuccess())
	assert.Empty(t, p.failedPools)
}

func TestKarpenter_AutoDetect_ThresholdReached(t *testing.T) {
	client := newFakeClient(newNodePool("gpu-pool", 4))
	p := &karpenterProvider{
		client:           client,
		failureThreshold: 3 * time.Minute,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  map[string]time.Time{"gpu-pool": time.Now().Add(-4 * time.Minute)},
	}
	// Need pending pods for auto-detect to trigger
	nodes := []*corev1.Node{{
		ObjectMeta: metav1.ObjectMeta{Name: "n1"},
		Status:     corev1.NodeStatus{Allocatable: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")}},
	}}
	pods := []*corev1.Pod{{
		ObjectMeta: metav1.ObjectMeta{Name: "p1"},
		Spec:       corev1.PodSpec{NodeName: "n1"},
		Status:     corev1.PodStatus{Phase: corev1.PodPending},
	}}
	snapshot := schedcache.NewSnapshot(pods, nodes)
	replicas, ret := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(0), replicas)
	assert.True(t, ret.IsSuccess())
	assert.Contains(t, p.failedPools, "gpu-pool")
}

func TestKarpenter_Estimate_NilClient(t *testing.T) {
	p := &karpenterProvider{
		failedPools:     make(map[string]time.Time),
		zeroClaimsSince: make(map[string]time.Time),
	}
	snapshot := schedcache.NewEmptySnapshot()
	replicas, ret := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(0), replicas)
	assert.True(t, ret.IsNoOperation())
}

func TestKarpenter_CPUWorkload(t *testing.T) {
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("cpu-pool")
	_ = unstructured.SetNestedField(np.Object, "64", "spec", "limits", "cpu")
	client := newFakeClient(np)
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	req := (&pb.ReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{
		corev1.ResourceCPU: *resource.NewQuantity(4, resource.DecimalSI),
	})
	snapshot := schedcache.NewEmptySnapshot()
	replicas, ret := p.Estimate(context.Background(), snapshot, req)
	assert.NoError(t, ret.AsError())
	assert.Equal(t, int32(16), replicas)
}

func TestKarpenter_MultiGPUPerNode(t *testing.T) {
	np := newNodePool("gpu-pool", 16)
	nc := newNodeClaimWithResources("claim-1", "gpu-pool", map[string]string{"nvidia.com/gpu": "8"})
	client := newFakeClient(np, nc)
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	snapshot := schedcache.NewEmptySnapshot()
	replicas, ret := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(8), replicas)
	assert.True(t, ret.IsSuccess())
}

func TestKarpenter_Name(t *testing.T) {
	p := &karpenterProvider{}
	assert.Equal(t, "karpenter", p.Name())
}

func TestKarpenter_MarkPoolFailed(t *testing.T) {
	client := newFakeClient(newNodePool("gpu-pool", 4))
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	p.markPoolFailed("gpu-pool")
	assert.Contains(t, p.failedPools, "gpu-pool")

	// After marking, Estimate should return 0 for that pool
	snapshot := schedcache.NewEmptySnapshot()
	replicas, ret := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(0), replicas)
	assert.True(t, ret.IsSuccess())
}

func TestKarpenter_AutoDetect_ThresholdNotReached(t *testing.T) {
	client := newFakeClient(newNodePool("gpu-pool", 4))
	p := &karpenterProvider{
		client:           client,
		failureThreshold: 3 * time.Minute,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	// Need pending pods in snapshot for auto-detect
	nodes := []*corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "n1"}, Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")}}}}
	pods := []*corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "p1"}, Spec: corev1.PodSpec{NodeName: "n1"}, Status: corev1.PodStatus{Phase: corev1.PodPending}}}
	snapshot := schedcache.NewSnapshot(pods, nodes)

	// First call: starts tracking
	replicas, _ := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(4), replicas)
	assert.Contains(t, p.zeroClaimsSince, "gpu-pool")

	// Second call immediately: threshold not reached
	replicas, _ = p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(4), replicas)
}

func TestKarpenter_AutoDetect_NoPendingPods(t *testing.T) {
	client := newFakeClient(newNodePool("gpu-pool", 4))
	p := &karpenterProvider{
		client:           client,
		failureThreshold: 3 * time.Minute,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	snapshot := schedcache.NewEmptySnapshot()
	replicas, _ := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(4), replicas)
	assert.Empty(t, p.zeroClaimsSince)
}

func TestKarpenter_AutoDetect_ClearedWhenNodeClaimAppears(t *testing.T) {
	client := newFakeClient(newNodePool("gpu-pool", 4), newNodeClaim("claim-1", "gpu-pool"))
	p := &karpenterProvider{
		client:           client,
		failureThreshold: 3 * time.Minute,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  map[string]time.Time{"gpu-pool": time.Now().Add(-4 * time.Minute)},
	}
	snapshot := schedcache.NewEmptySnapshot()
	replicas, _ := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(3), replicas)
	assert.Empty(t, p.zeroClaimsSince)
	assert.Empty(t, p.failedPools)
}

func TestKarpenter_FailureDetection_FirstCallAllowed(t *testing.T) {
	client := newFakeClient(newNodePool("gpu-pool", 4))
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	snapshot := schedcache.NewEmptySnapshot()
	replicas, ret := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(4), replicas)
	assert.True(t, ret.IsSuccess())
}

func TestKarpenter_MultiResource(t *testing.T) {
	np := newNodePool("gpu-pool", 8)
	_ = unstructured.SetNestedField(np.Object, "64", "spec", "limits", "cpu")
	client := newFakeClient(np)
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	req := (&pb.ReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{
		"nvidia.com/gpu":   *resource.NewQuantity(1, resource.DecimalSI),
		corev1.ResourceCPU: *resource.NewQuantity(4, resource.DecimalSI),
	})
	snapshot := schedcache.NewEmptySnapshot()
	replicas, _ := p.Estimate(context.Background(), snapshot, req)
	assert.Equal(t, int32(8), replicas) // min(8/1, 64/4) = 8
}

func TestKarpenter_NeuronWorkload(t *testing.T) {
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("neuron-pool")
	_ = unstructured.SetNestedField(np.Object, "4", "spec", "limits", "aws.amazon.com/neuroncore")
	client := newFakeClient(np)
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	req := (&pb.ReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{
		corev1.ResourceName("aws.amazon.com/neuroncore"): *resource.NewQuantity(2, resource.DecimalSI),
	})
	snapshot := schedcache.NewEmptySnapshot()
	replicas, _ := p.Estimate(context.Background(), snapshot, req)
	assert.Equal(t, int32(2), replicas) // 4/2 = 2
}

func TestKarpenter_NoMatchingLimit(t *testing.T) {
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("cpu-pool")
	_ = unstructured.SetNestedField(np.Object, "64", "spec", "limits", "cpu")
	client := newFakeClient(np)
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	snapshot := schedcache.NewEmptySnapshot()
	replicas, _ := p.Estimate(context.Background(), snapshot, gpuReq())
	assert.Equal(t, int32(0), replicas)
}

func TestKarpenter_MultiGPUPerNode_MultiClaim(t *testing.T) {
	np := newNodePool("gpu-pool", 16)
	nc1 := newNodeClaimWithResources("claim-1", "gpu-pool", map[string]string{"nvidia.com/gpu": "4"})
	nc2 := newNodeClaimWithResources("claim-2", "gpu-pool", map[string]string{"nvidia.com/gpu": "4"})
	client := newFakeClient(np, nc1, nc2)
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	req := (&pb.ReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{
		"nvidia.com/gpu": *resource.NewQuantity(2, resource.DecimalSI),
	})
	snapshot := schedcache.NewEmptySnapshot()
	replicas, _ := p.Estimate(context.Background(), snapshot, req)
	assert.Equal(t, int32(4), replicas) // (16 - 8) / 2 = 4
}

func TestKarpenter_CPUWorkload_WithExistingClaims(t *testing.T) {
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("cpu-pool")
	_ = unstructured.SetNestedField(np.Object, "16", "spec", "limits", "cpu")
	nc1 := newNodeClaimWithResources("claim-1", "cpu-pool", map[string]string{"cpu": "2"})
	nc2 := newNodeClaimWithResources("claim-2", "cpu-pool", map[string]string{"cpu": "2"})
	client := newFakeClient(np, nc1, nc2)
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	req := (&pb.ReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{
		corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI),
	})
	snapshot := schedcache.NewEmptySnapshot()
	replicas, _ := p.Estimate(context.Background(), snapshot, req)
	assert.Equal(t, int32(12), replicas) // (16 - 4) / 1 = 12
}

func TestKarpenter_MemoryWorkload(t *testing.T) {
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("mem-pool")
	_ = unstructured.SetNestedField(np.Object, "64Gi", "spec", "limits", "memory")
	client := newFakeClient(np)
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	req := (&pb.ReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{
		corev1.ResourceMemory: *resource.NewQuantity(8*1024*1024*1024, resource.BinarySI),
	})
	snapshot := schedcache.NewEmptySnapshot()
	replicas, _ := p.Estimate(context.Background(), snapshot, req)
	assert.Equal(t, int32(8), replicas) // 64Gi / 8Gi = 8
}

func TestKarpenter_CPUMillicoreWorkload(t *testing.T) {
	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodePool"})
	np.SetName("cpu-pool")
	_ = unstructured.SetNestedField(np.Object, "4", "spec", "limits", "cpu")
	client := newFakeClient(np)
	p := &karpenterProvider{
		client:           client,
		failureThreshold: defaultFailureThreshold,
		recoveryInterval: defaultRecoveryInterval,
		failedPools:      make(map[string]time.Time),
		zeroClaimsSince:  make(map[string]time.Time),
	}
	req := (&pb.ReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("500m"),
	})
	snapshot := schedcache.NewEmptySnapshot()
	replicas, _ := p.Estimate(context.Background(), snapshot, req)
	assert.Equal(t, int32(8), replicas) // 4000m / 500m = 8
}
