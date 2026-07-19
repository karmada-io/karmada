/*
Copyright 2024 The Karmada Authors.

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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
)

func TestNewCache(t *testing.T) {
	tests := []struct {
		name          string
		clusterLister clusterlister.ClusterLister
	}{
		{
			name:          "Create cache with empty mock lister",
			clusterLister: &mockClusterLister{},
		},
		{
			name:          "Create cache with non-empty mock lister",
			clusterLister: &mockClusterLister{clusters: []*clusterv1alpha1.Cluster{{}}},
		},
		{
			name:          "Create cache with nil lister",
			clusterLister: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewCache(tt.clusterLister, nil, 0)

			assert.NotNil(t, cache, "NewCache() returned nil cache")

			// Assert that the cache is of the correct type
			sc, ok := cache.(*schedulerCache)
			assert.True(t, ok, "NewCache() did not return a *schedulerCache")

			// Assert that the clusterLister is correctly set
			assert.Equal(t, tt.clusterLister, sc.clusterLister, "clusterLister not set correctly")
		})
	}
}

func TestSnapshot(t *testing.T) {
	tests := []struct {
		name           string
		clusters       []*clusterv1alpha1.Cluster
		wantTotal      int
		wantReady      int
		wantReadyNames sets.Set[string]
	}{
		{
			name:           "empty cluster list",
			clusters:       []*clusterv1alpha1.Cluster{},
			wantTotal:      0,
			wantReady:      0,
			wantReadyNames: sets.New[string](),
		},
		{
			name: "mixed ready and not ready clusters",
			clusters: []*clusterv1alpha1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Status:     clusterv1alpha1.ClusterStatus{Conditions: []metav1.Condition{{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue}}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster2"},
					Status:     clusterv1alpha1.ClusterStatus{Conditions: []metav1.Condition{{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionFalse}}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster3"},
					Status:     clusterv1alpha1.ClusterStatus{Conditions: []metav1.Condition{{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue}}},
				},
			},
			wantTotal:      3,
			wantReady:      2,
			wantReadyNames: sets.New("cluster1", "cluster3"),
		},
		{
			name: "all ready clusters",
			clusters: []*clusterv1alpha1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Status:     clusterv1alpha1.ClusterStatus{Conditions: []metav1.Condition{{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue}}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster2"},
					Status:     clusterv1alpha1.ClusterStatus{Conditions: []metav1.Condition{{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue}}},
				},
			},
			wantTotal:      2,
			wantReady:      2,
			wantReadyNames: sets.New("cluster1", "cluster2"),
		},
		{
			name: "all not ready clusters",
			clusters: []*clusterv1alpha1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Status:     clusterv1alpha1.ClusterStatus{Conditions: []metav1.Condition{{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionFalse}}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster2"},
					Status:     clusterv1alpha1.ClusterStatus{Conditions: []metav1.Condition{{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionFalse}}},
				},
			},
			wantTotal:      2,
			wantReady:      0,
			wantReadyNames: sets.New[string](),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLister := &mockClusterLister{clusters: tt.clusters}
			cache := NewCache(mockLister, nil, 0)
			snapshot := cache.Snapshot()

			assert.Equal(t, tt.wantTotal, snapshot.NumOfClusters(), "Incorrect number of total clusters")
			assert.Equal(t, tt.wantTotal, len(snapshot.GetClusters()), "Incorrect number of clusters returned by GetClusters()")
			assert.Equal(t, tt.wantReady, len(snapshot.GetReadyClusters()), "Incorrect number of ready clusters")
			assert.Equal(t, tt.wantReadyNames, snapshot.GetReadyClusterNames(), "Incorrect ready cluster names")

			// Test GetCluster for existing clusters
			for _, cluster := range tt.clusters {
				gotCluster := snapshot.GetCluster(cluster.Name)
				assert.NotNil(t, gotCluster, "GetCluster(%s) returned nil", cluster.Name)
				assert.Equal(t, cluster, gotCluster.Cluster(), "GetCluster(%s) returned incorrect cluster", cluster.Name)
			}

			// Test GetCluster for non-existent cluster
			assert.Nil(t, snapshot.GetCluster("non-existent-cluster"), "GetCluster(non-existent-cluster) should return nil")

			// Verify that the snapshot is a deep copy
			for i, cluster := range tt.clusters {
				snapshotCluster := snapshot.GetClusters()[i].Cluster()
				assert.Equal(t, cluster, snapshotCluster, "Snapshot cluster should be equal to original")
			}
		})
	}
}

func TestNewEmptySnapshot(t *testing.T) {
	snapshot := NewEmptySnapshot()
	assert.Equal(t, 0, snapshot.NumOfClusters(), "New empty snapshot should have 0 clusters")
	assert.Empty(t, snapshot.GetClusters(), "New empty snapshot should return empty cluster list")
	assert.Empty(t, snapshot.GetReadyClusters(), "New empty snapshot should return empty ready cluster list")
	assert.Empty(t, snapshot.GetReadyClusterNames(), "New empty snapshot should return empty ready cluster names")
	assert.Nil(t, snapshot.GetCluster("any-cluster"), "GetCluster on empty snapshot should return nil")
}

func TestAddUpdateDeleteCluster(t *testing.T) {
	tests := []struct {
		name   string
		action func(*schedulerCache, *clusterv1alpha1.Cluster)
	}{
		{
			name: "AddCluster",
			action: func(cache *schedulerCache, cluster *clusterv1alpha1.Cluster) {
				cache.AddCluster(cluster)
			},
		},
		{
			name: "UpdateCluster",
			action: func(cache *schedulerCache, cluster *clusterv1alpha1.Cluster) {
				cache.UpdateCluster(cluster)
			},
		},
		{
			name: "DeleteCluster",
			action: func(cache *schedulerCache, cluster *clusterv1alpha1.Cluster) {
				cache.DeleteCluster(cluster)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLister := &mockClusterLister{}
			cache := NewCache(mockLister, nil, 0).(*schedulerCache)

			cluster := &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			tt.action(cache, cluster)

			// Verify that the action doesn't modify the cache
			assert.Empty(t, mockLister.clusters, "SchedulerCache.%s() modified the cache, which it shouldn't", tt.name)
		})
	}
}

func TestSnapshotError(t *testing.T) {
	// mock error
	mockError := errors.New("mock list error")

	mockLister := &mockClusterLister{
		clusters: nil,
		err:      mockError,
	}

	cache := NewCache(mockLister, nil, 0)

	snapshot := cache.Snapshot()

	// Assert that the snapshot is empty
	assert.Equal(t, 0, snapshot.NumOfClusters(), "Snapshot should be empty when there's an error")
	assert.Empty(t, snapshot.GetClusters(), "Snapshot should have no clusters when there's an error")
}

// Mock Implementations

type mockClusterLister struct {
	clusters []*clusterv1alpha1.Cluster
	err      error
}

func (m *mockClusterLister) List(_ labels.Selector) ([]*clusterv1alpha1.Cluster, error) {
	return m.clusters, m.err
}

func (m *mockClusterLister) Get(name string) (*clusterv1alpha1.Cluster, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, cluster := range m.clusters {
		if cluster.Name == name {
			return cluster, nil
		}
	}
	return nil, nil
}

// newTestAssumptionCache returns an AssigningResourceBindingCache pre-configured for reservation tests.
func newTestAssumptionCache(ttl time.Duration) *AssigningResourceBindingCache {
	return &AssigningResourceBindingCache{
		items:       make(map[string]*workv1alpha2.ResourceBinding),
		assumptions: make(map[string]*BindingAssumption),
		ttl:         ttl,
	}
}

func makeAssumedWorkload(ns string) AssumedWorkload {
	return AssumedWorkload{
		Namespace: ns,
		Components: []workv1alpha2.Component{
			{Name: "worker", Replicas: 2},
		},
	}
}

func TestAssume(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)

	c.Assume("ns/rb1", "cluster1", makeAssumedWorkload("ns"))

	assert.Len(t, c.assumptions, 1)
	br := c.assumptions["ns/rb1"]
	assert.NotNil(t, br)
	assert.Contains(t, br.entries, "cluster1")
	assert.True(t, br.ExpiresAt.After(time.Now()), "ExpiresAt should be in the future")
}

func TestAssume_Replace(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)

	first := makeAssumedWorkload("ns")
	first.Components[0].Replicas = 1
	c.Assume("ns/rb1", "cluster1", first)

	second := makeAssumedWorkload("ns")
	second.Components[0].Replicas = 3
	c.Assume("ns/rb1", "cluster1", second)

	entry := c.assumptions["ns/rb1"].entries["cluster1"]
	assert.Equal(t, int32(3), entry.Components[0].Replicas, "Assume should replace the previous entry")
}

func TestAssume_MultipleBindings(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)

	c.Assume("ns/rb1", "cluster1", makeAssumedWorkload("ns"))
	c.Assume("ns/rb2", "cluster1", makeAssumedWorkload("ns"))

	assert.Len(t, c.assumptions, 2)
}

func TestAssume_IsolatesCallerMutation(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)

	entry := makeAssumedWorkload("ns")
	c.Assume("ns/rb1", "cluster1", entry)

	// Mutate the original slice after Assume — cache should be unaffected.
	entry.Components[0].Replicas = 999

	stored := c.assumptions["ns/rb1"].entries["cluster1"]
	assert.Equal(t, int32(2), stored.Components[0].Replicas, "Assume should deep-copy Components; caller mutation must not affect cached value")
}

func TestReleaseClusterAssumption(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)
	c.Assume("ns/rb1", "cluster1", makeAssumedWorkload("ns"))
	c.Assume("ns/rb1", "cluster2", makeAssumedWorkload("ns"))

	c.ReleaseClusterAssumption("ns/rb1", "cluster1")

	br := c.assumptions["ns/rb1"]
	assert.NotNil(t, br, "binding entry should remain while other clusters still have assumptions")
	assert.NotContains(t, br.entries, "cluster1")
	assert.Contains(t, br.entries, "cluster2")
}

func TestReleaseClusterAssumption_RemovesBindingWhenEmpty(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)
	c.Assume("ns/rb1", "cluster1", makeAssumedWorkload("ns"))

	c.ReleaseClusterAssumption("ns/rb1", "cluster1")

	assert.NotContains(t, c.assumptions, "ns/rb1", "binding entry should be removed when all clusters are released")
}

func TestReleaseClusterAssumption_NoOp(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)
	// Should not panic when called on a non-existent key
	c.ReleaseClusterAssumption("ns/nonexistent", "cluster1")
	assert.Empty(t, c.assumptions)
}

func TestReleaseAssumption(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)
	c.Assume("ns/rb1", "cluster1", makeAssumedWorkload("ns"))
	c.Assume("ns/rb1", "cluster2", makeAssumedWorkload("ns"))

	c.ReleaseAssumption("ns/rb1")

	assert.NotContains(t, c.assumptions, "ns/rb1")
}

func TestGetAssumedWorkloads(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)
	c.Assume("ns/rb1", "cluster1", makeAssumedWorkload("ns"))
	c.Assume("ns/rb2", "cluster1", makeAssumedWorkload("ns"))
	c.Assume("ns/rb3", "cluster2", makeAssumedWorkload("ns"))

	result := c.GetAssumedWorkloads("cluster1")

	assert.Len(t, result, 2, "should return assumptions from all bindings for the given cluster")
}

func TestGetAssumedWorkloads_IsolatesCallerMutation(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)
	c.Assume("ns/rb1", "cluster1", makeAssumedWorkload("ns"))

	result := c.GetAssumedWorkloads("cluster1")
	// Mutate the returned value — cache should be unaffected.
	result[0].Components[0].Replicas = 999

	stored := c.assumptions["ns/rb1"].entries["cluster1"]
	assert.Equal(t, int32(2), stored.Components[0].Replicas, "GetAssumedWorkloads should return deep copies; caller mutation must not affect cached value")
}

func TestGetAssumedWorkloads_SkipsExpired(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)
	c.Assume("ns/rb1", "cluster1", makeAssumedWorkload("ns"))
	// Directly set ExpiresAt to the past to avoid time.Sleep flakiness.
	c.assumptions["ns/rb1"].ExpiresAt = time.Now().Add(-time.Second)

	result := c.GetAssumedWorkloads("cluster1")

	assert.Empty(t, result, "expired assumptions should not be returned")
}

func TestGetAssumedWorkloads_EmptyForUnknownCluster(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)
	c.Assume("ns/rb1", "cluster1", makeAssumedWorkload("ns"))

	result := c.GetAssumedWorkloads("cluster99")
	assert.Empty(t, result)
}

func TestGC(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)
	c.Assume("ns/rb1", "cluster1", makeAssumedWorkload("ns"))
	c.Assume("ns/rb2", "cluster2", makeAssumedWorkload("ns"))
	// Directly set ExpiresAt to the past to avoid time.Sleep flakiness.
	c.assumptions["ns/rb1"].ExpiresAt = time.Now().Add(-time.Second)
	c.assumptions["ns/rb2"].ExpiresAt = time.Now().Add(-time.Second)

	// Add a fresh entry that should NOT be collected
	c.assumptions["ns/rb3"] = &BindingAssumption{
		entries:   map[string]AssumedWorkload{"cluster3": makeAssumedWorkload("ns")},
		ExpiresAt: time.Now().Add(time.Minute),
	}

	removed := c.GC()

	assert.Equal(t, 2, removed)
	assert.NotContains(t, c.assumptions, "ns/rb1")
	assert.NotContains(t, c.assumptions, "ns/rb2")
	assert.Contains(t, c.assumptions, "ns/rb3", "non-expired entry should be kept")
}

func makeBinding(ns, name, resourceVersion string) *workv1alpha2.ResourceBinding {
	return &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, ResourceVersion: resourceVersion},
	}
}

func TestRemove(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)
	binding := makeBinding("ns", "rb1", "1000")
	c.Add(binding)
	assert.Contains(t, c.items, "ns/rb1")

	c.Remove(binding)

	assert.NotContains(t, c.items, "ns/rb1")
}

func TestRemove_NoOp(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)
	// Should not panic when called on a non-existent key
	c.Remove(makeBinding("ns", "rb1", "1000"))
	assert.Empty(t, c.items)
}

func TestUpdateIfExist(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)
	c.Add(makeBinding("ns", "rb1", "1000"))

	updated := makeBinding("ns", "rb1", "1001")
	c.UpdateIfExist(updated)

	assert.Same(t, updated, c.items["ns/rb1"], "existing entry should be replaced")
}

func TestUpdateIfExist_SkipsAbsentEntry(t *testing.T) {
	c := newTestAssumptionCache(time.Minute)

	c.UpdateIfExist(makeBinding("ns", "rb1", "1001"))

	assert.Empty(t, c.items, "UpdateIfExist must not resurrect an entry the Informer has already cleaned up")
}

// BenchmarkGetAssumedWorkloads measures the performance of GetAssumedWorkloads under
// varying cache sizes.  The benchmark covers the realistic operating range (N ≤ 50,
// governed by the reservation TTL) as well as stress sizes to quantify O(N) scaling.
//
// Run with:
//
//	go test ./pkg/scheduler/cache/... -bench=BenchmarkGetAssumedWorkloads -benchmem
func BenchmarkGetAssumedWorkloads(b *testing.B) {
	// totalBindings is the total number of in-flight bindings in the cache.
	// targetClusterFraction controls the ratio of bindings that are assigned to
	// the cluster being queried; the rest are assigned to "other-cluster".
	cases := []struct {
		totalBindings     int
		targetClusterFrac float64 // fraction of bindings hitting "cluster1"
	}{
		// Realistic: TTL = 5 min, moderate scheduling rate → O(10) bindings
		{totalBindings: 10, targetClusterFrac: 0.5},
		// Upper realistic bound
		{totalBindings: 50, targetClusterFrac: 0.5},
		// Stress: worst-case if TTL window is very long or scheduling rate is high
		{totalBindings: 200, targetClusterFrac: 0.5},
		{totalBindings: 1000, targetClusterFrac: 0.5},
	}

	for _, tc := range cases {
		name := fmt.Sprintf("N=%d/frac=%.1f", tc.totalBindings, tc.targetClusterFrac)
		b.Run(name, func(b *testing.B) {
			c := newTestAssumptionCache(time.Hour) // large TTL so nothing expires during bench

			targetCount := int(float64(tc.totalBindings) * tc.targetClusterFrac)
			for i := 0; i < tc.totalBindings; i++ {
				bindingKey := fmt.Sprintf("ns/rb%d", i)
				clusterName := "other-cluster"
				if i < targetCount {
					clusterName = "cluster1"
				}
				c.Assume(bindingKey, clusterName, makeAssumedWorkload("ns"))
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = c.GetAssumedWorkloads("cluster1")
			}
		})
	}
}
