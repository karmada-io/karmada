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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
	"github.com/karmada-io/karmada/pkg/estimator/server/framework"
	schedcache "github.com/karmada-io/karmada/pkg/util/lifted/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/util/lifted/scheduler/framework/parallelize"
)

// fakeHandle implements framework.Handle for testing.
type fakeHandle struct{}

func (fakeHandle) ClientSet() clientset.Interface                         { return nil }
func (fakeHandle) DynamicClient() dynamic.Interface                       { return nil }
func (fakeHandle) SharedInformerFactory() informers.SharedInformerFactory { return nil }
func (fakeHandle) Parallelism() int                                       { return 1 }

// fakeProvider implements CapacityProvider for testing.
type fakeProvider struct {
	available bool
	replicas  int32
	err       error
}

func (f *fakeProvider) IsAvailable(_ context.Context) bool {
	return f.available
}

func (f *fakeProvider) GetPotentialReplicas(_ context.Context, _ *pb.ReplicaRequirements, _ bool) (int32, error) {
	return f.replicas, f.err
}

func gpuRequirements(gpuCount int64) *pb.ReplicaRequirements {
	return &pb.ReplicaRequirements{
		ResourceRequest: corev1.ResourceList{
			"nvidia.com/gpu": *resource.NewQuantity(gpuCount, resource.DecimalSI),
		},
	}
}

func newTestPlugin(providers ...CapacityProvider) *nodeAutoscalerEstimator {
	return &nodeAutoscalerEstimator{
		parallelizer: parallelize.NewParallelizer(1),
		providers:    providers,
	}
}

func TestEstimate_NoNodes_NoProvider(t *testing.T) {
	// No nodes, no autoscaler provider → 0
	pl := newTestPlugin()
	snapshot := schedcache.NewEmptySnapshot()
	replica, ret := pl.Estimate(context.Background(), snapshot, gpuRequirements(1))
	assert.Equal(t, int32(0), replica)
	assert.True(t, ret.IsSuccess())
}

func TestEstimate_NoNodes_WithProvider(t *testing.T) {
	// No nodes, but provider says 4 GPUs available → 4
	pl := newTestPlugin(&fakeProvider{available: true, replicas: 4})
	snapshot := schedcache.NewEmptySnapshot()
	replica, ret := pl.Estimate(context.Background(), snapshot, gpuRequirements(1))
	assert.Equal(t, int32(4), replica)
	assert.True(t, ret.IsSuccess())
}

func TestEstimate_ProviderNotAvailable(t *testing.T) {
	// Provider exists but not available → 0 (no nodes either)
	pl := newTestPlugin(&fakeProvider{available: false, replicas: 10})
	snapshot := schedcache.NewEmptySnapshot()
	replica, ret := pl.Estimate(context.Background(), snapshot, gpuRequirements(1))
	assert.Equal(t, int32(0), replica)
	assert.True(t, ret.IsSuccess())
}

func TestEstimate_ProviderError(t *testing.T) {
	// Provider returns error → still return current available (0), log error
	pl := newTestPlugin(&fakeProvider{available: true, replicas: 0, err: assert.AnError})
	snapshot := schedcache.NewEmptySnapshot()
	replica, ret := pl.Estimate(context.Background(), snapshot, gpuRequirements(1))
	assert.Equal(t, int32(0), replica)
	assert.True(t, ret.IsSuccess())
}

func TestEstimate_MultipleProviders(t *testing.T) {
	// Multiple providers: sum their potential replicas
	pl := newTestPlugin(
		&fakeProvider{available: true, replicas: 3},
		&fakeProvider{available: true, replicas: 2},
	)
	snapshot := schedcache.NewEmptySnapshot()
	replica, ret := pl.Estimate(context.Background(), snapshot, gpuRequirements(1))
	assert.Equal(t, int32(5), replica)
	assert.True(t, ret.IsSuccess())
}

func TestName(t *testing.T) {
	pl := newTestPlugin()
	assert.Equal(t, "NodeAutoscalerEstimator", pl.Name())
}

func TestNew(t *testing.T) {
	// New should return a plugin implementing EstimateReplicasPlugin
	p, err := New(fakeHandle{})
	assert.NoError(t, err)
	assert.NotNil(t, p)
	_, ok := p.(framework.EstimateReplicasPlugin)
	assert.True(t, ok)
}
