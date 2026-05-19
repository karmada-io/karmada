/*
Copyright 2025 The Karmada Authors.

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

package noderesource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
	schedcache "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework/parallelize"
)

func init() {
	// Register a test provider for testing New() with --node-capacity-providers.
	RegisterProvider("test-provider", func(_ framework.Handle) (CapacityProvider, error) {
		return &fakeProvider{name: "test-provider", replicas: 5, result: framework.NewResult(framework.Success)}, nil
	})
}

type fakeHandle struct {
	providers []string
}

func (f fakeHandle) ClientSet() clientset.Interface                         { return nil }
func (f fakeHandle) DynamicClient() dynamic.Interface                       { return nil }
func (f fakeHandle) SharedInformerFactory() informers.SharedInformerFactory { return nil }
func (f fakeHandle) Parallelism() int                                       { return 1 }
func (f fakeHandle) NodeCapacityProviders() []string                        { return f.providers }

// fakeProvider implements CapacityProvider for testing.
type fakeProvider struct {
	name     string
	replicas int32
	result   *framework.Result
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Estimate(_ context.Context, _ *schedcache.Snapshot, _ *pb.ReplicaRequirements) (int32, *framework.Result) {
	return f.replicas, f.result
}

func TestNew_NoProviders(t *testing.T) {
	// Default: no additional providers, just existing nodes
	p, err := New(fakeHandle{})
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, Name, p.Name())
}

func TestNew_WithProvider(t *testing.T) {
	p, err := New(fakeHandle{providers: []string{"test-provider"}})
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNew_UnknownProvider(t *testing.T) {
	_, err := New(fakeHandle{providers: []string{"nonexistent"}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown node capacity provider")
}

func TestEstimate_ExistingNodesOnly(t *testing.T) {
	// No additional providers, 4 CPU node, request 1 CPU → 4 replicas
	nodes := []*corev1.Node{makeNode("n1", nil, corev1.ResourceList{
		corev1.ResourceCPU:  resource.MustParse("4"),
		corev1.ResourcePods: resource.MustParse("10"),
	})}
	snapshot := schedcache.NewSnapshot(nil, nodes)
	pl := &nodeResourceEstimator{parallelizer: parallelize.NewParallelizer(1)}
	replicas, ret := pl.Estimate(context.Background(), snapshot, (&pb.ReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}))
	assert.Equal(t, int32(4), replicas)
	assert.True(t, ret.IsSuccess())
}

func TestEstimate_ExistingNodesPlusProvider(t *testing.T) {
	// 4 CPU from existing nodes + 5 from provider = 9
	nodes := []*corev1.Node{makeNode("n1", nil, corev1.ResourceList{
		corev1.ResourceCPU:  resource.MustParse("4"),
		corev1.ResourcePods: resource.MustParse("10"),
	})}
	snapshot := schedcache.NewSnapshot(nil, nodes)
	pl := &nodeResourceEstimator{
		parallelizer: parallelize.NewParallelizer(1),
		providers:    []CapacityProvider{&fakeProvider{name: "k", replicas: 5, result: framework.NewResult(framework.Success)}},
	}
	replicas, ret := pl.Estimate(context.Background(), snapshot, (&pb.ReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}))
	assert.Equal(t, int32(9), replicas)
	assert.True(t, ret.IsSuccess())
}

func TestEstimate_ProviderNoOp(t *testing.T) {
	// Provider returns NoOp → only existing nodes count
	nodes := []*corev1.Node{makeNode("n1", nil, corev1.ResourceList{
		corev1.ResourceCPU:  resource.MustParse("2"),
		corev1.ResourcePods: resource.MustParse("10"),
	})}
	snapshot := schedcache.NewSnapshot(nil, nodes)
	pl := &nodeResourceEstimator{
		parallelizer: parallelize.NewParallelizer(1),
		providers:    []CapacityProvider{&fakeProvider{name: "k", replicas: 0, result: framework.NewResult(framework.Noopperation)}},
	}
	replicas, ret := pl.Estimate(context.Background(), snapshot, (&pb.ReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}))
	assert.Equal(t, int32(2), replicas)
	assert.True(t, ret.IsSuccess())
}

func TestEstimate_ProviderFailed(t *testing.T) {
	// Provider fails → propagate error
	snapshot := schedcache.NewEmptySnapshot()
	pl := &nodeResourceEstimator{
		parallelizer: parallelize.NewParallelizer(1),
		providers:    []CapacityProvider{&fakeProvider{name: "k", replicas: 0, result: framework.NewResult(framework.Error, "fail")}},
	}
	replicas, ret := pl.Estimate(context.Background(), snapshot, (&pb.ReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}))
	assert.Equal(t, int32(0), replicas)
	assert.True(t, ret.IsFailed())
}

func TestEstimate_EmptySnapshot(t *testing.T) {
	// No nodes, no providers → 0
	snapshot := schedcache.NewEmptySnapshot()
	pl := &nodeResourceEstimator{parallelizer: parallelize.NewParallelizer(1)}
	replicas, ret := pl.Estimate(context.Background(), snapshot, (&pb.ReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}))
	assert.Equal(t, int32(0), replicas)
	assert.True(t, ret.IsSuccess())
}

func TestEstimateComponents(t *testing.T) {
	tests := []struct {
		name       string
		nodes      []*corev1.Node
		pods       []*corev1.Pod
		components []*pb.Component
		expected   int32
		wantCode   framework.Code
	}{
		{
			name: "single component fits",
			nodes: []*corev1.Node{makeNode("n1", nil, corev1.ResourceList{
				corev1.ResourceCPU:  resource.MustParse("4"),
				corev1.ResourcePods: resource.MustParse("10"),
			})},
			components: []*pb.Component{{
				ReplicaRequirements: (&pb.ComponentReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}),
				Replicas:            1,
			}},
			expected: 4,
			wantCode: framework.Success,
		},
		{
			name: "insufficient resources",
			nodes: []*corev1.Node{makeNode("n1", nil, corev1.ResourceList{
				corev1.ResourceCPU:  resource.MustParse("2"),
				corev1.ResourcePods: resource.MustParse("10"),
			})},
			components: []*pb.Component{{
				ReplicaRequirements: (&pb.ComponentReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("3")}),
				Replicas:            1,
			}},
			expected: 0,
			wantCode: framework.Unschedulable,
		},
		{
			name: "empty components",
			nodes: []*corev1.Node{makeNode("n1", nil, corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("4"),
			})},
			components: []*pb.Component{},
			expected:   noNodeConstraint,
			wantCode:   framework.Noopperation,
		},
		{
			name: "with existing pods",
			nodes: []*corev1.Node{makeNode("n1", nil, corev1.ResourceList{
				corev1.ResourceCPU:  resource.MustParse("4"),
				corev1.ResourcePods: resource.MustParse("10"),
			})},
			pods: []*corev1.Pod{makePod("p1", "n1", corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			})},
			components: []*pb.Component{{
				ReplicaRequirements: (&pb.ComponentReplicaRequirements{}).MustSetResourceRequest(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}),
				Replicas:            1,
			}},
			expected: 3,
			wantCode: framework.Success,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := schedcache.NewSnapshot(tt.pods, tt.nodes)
			pl := &nodeResourceEstimator{parallelizer: parallelize.NewParallelizer(1)}
			result, status := pl.EstimateComponents(context.Background(), framework.ComponentEstimationContext{
				Snapshot:   snapshot,
				Components: tt.components,
			})
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.wantCode, status.Code())
		})
	}
}

func makeNode(name string, labels map[string]string, allocatable corev1.ResourceList) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Status:     corev1.NodeStatus{Allocatable: allocatable, Capacity: allocatable},
	}
}

func makePod(name, nodeName string, requests corev1.ResourceList) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PodSpec{
			NodeName:   nodeName,
			Containers: []corev1.Container{{Resources: corev1.ResourceRequirements{Requests: requests}}},
		},
	}
}
