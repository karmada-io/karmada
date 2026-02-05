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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

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
			cache := NewCache(tt.clusterLister)

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
			cache := NewCache(mockLister)
			snapshot := cache.Snapshot()

			assert.Equal(t, tt.wantTotal, snapshot.NumOfClusters(), "Incorrect number of total clusters")
			assert.Equal(t, tt.wantTotal, len(snapshot.GetClusters()), "Incorrect number of clusters returned by GetClusters()")
			assert.Equal(t, tt.wantReady, len(snapshot.GetReadyClusters()), "Incorrect number of ready clusters")
			assert.Equal(t, tt.wantReadyNames, snapshot.GetReadyClusterNames(), "Incorrect ready cluster names")

			// Test GetCluster for existing clusters
			for _, cluster := range tt.clusters {
				gotCluster := snapshot.GetCluster(cluster.Name)
				assert.NotNil(t, gotCluster, "GetCluster(%s) returned nil", cluster.Name)
				assert.Equal(t, cluster, gotCluster, "GetCluster(%s) returned incorrect cluster", cluster.Name)
			}

			// Test GetCluster for non-existent cluster
			assert.Nil(t, snapshot.GetCluster("non-existent-cluster"), "GetCluster(non-existent-cluster) should return nil")

			// Verify that the snapshot is a deep copy
			for i, cluster := range tt.clusters {
				snapshotCluster := snapshot.GetClusters()[i]
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
			cache := NewCache(mockLister).(*schedulerCache)

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

	cache := NewCache(mockLister)

	snapshot := cache.Snapshot()

	// Assert that the snapshot is empty
	assert.Equal(t, 0, snapshot.NumOfClusters(), "Snapshot should be empty when there's an error")
	assert.Empty(t, snapshot.GetClusters(), "Snapshot should have no clusters when there's an error")
}

func TestOnResourceBindingAdd_IndexesAffinityAndAntiAffinity(t *testing.T) {
	c := NewCache(&mockClusterLister{}).(*schedulerCache)

	rb := rbWithGroups("default", "rb1", []string{"c1", "c2"}, "gA", "gB")
	c.OnResourceBindingAdd(rb)

	// internal keys should be present
	affKey := makeWorkloadGroupKey("default", GroupTypeAffinity, "gA")
	antiKey := makeWorkloadGroupKey("default", GroupTypeAntiAffinity, "gB")

	c.mu.RLock()
	defer c.mu.RUnlock()

	gotAff := c.groupMembers[affKey]
	assert.NotNil(t, gotAff)
	assert.True(t, gotAff.Equal(sets.New[string]("rb:default/rb1")))

	gotAnti := c.groupMembers[antiKey]
	assert.NotNil(t, gotAnti)
	assert.True(t, gotAnti.Equal(sets.New[string]("rb:default/rb1")))

	clusters := c.clustersByBinding["rb:default/rb1"]
	assert.NotNil(t, clusters)
	assert.True(t, clusters.Has("c1"))
	assert.True(t, clusters.Has("c2"))
}

func TestOnResourceBindingAdd_SkipsWhenNoClusters(t *testing.T) {
	c := NewCache(&mockClusterLister{}).(*schedulerCache)

	// Has groups, but no scheduled clusters -> should not be indexed.
	rb := rbWithGroups("default", "rb1", nil, "gA", "gB")
	c.OnResourceBindingAdd(rb)

	c.mu.RLock()
	defer c.mu.RUnlock()

	assert.Empty(t, c.groupMembers)
	assert.Empty(t, c.clustersByBinding)
}

func TestOnResourceBindingAdd_SkipsWhenNoGroups(t *testing.T) {
	c := NewCache(&mockClusterLister{}).(*schedulerCache)

	// Has clusters, but no WorkloadAffinityGroups -> should not be indexed.
	rb := rbWithGroups("default", "rb1", []string{"c1"}, "", "")
	c.OnResourceBindingAdd(rb)

	c.mu.RLock()
	defer c.mu.RUnlock()

	assert.Empty(t, c.groupMembers)
	assert.Empty(t, c.clustersByBinding)
}

func TestOnResourceBindingAddThenUpdate_IndexesWhenClustersArrive(t *testing.T) {
	c := NewCache(&mockClusterLister{}).(*schedulerCache)

	// 1) ResourceBinding added with anti-affinity but NO clusters -> should be skipped.
	oldRB := rbWithGroups("default", "rb1", nil, "", "gAntiOnly")
	c.OnResourceBindingAdd(oldRB)

	c.mu.RLock()
	assert.Empty(t, c.groupMembers, "groupMembers must be empty after Add with no clusters")
	assert.Empty(t, c.clustersByBinding, "clustersByBinding must be empty after Add with no clusters")
	c.mu.RUnlock()

	// 2) ResourceBinding updated to include a cluster -> cache should be indexed.
	newRB := rbWithGroups("default", "rb1", []string{"c1"}, "", "gAntiOnly")
	c.OnResourceBindingUpdate(oldRB, newRB)

	antiKey := makeWorkloadGroupKey("default", GroupTypeAntiAffinity, "gAntiOnly")
	bindingID := "rb:default/rb1"

	c.mu.RLock()
	defer c.mu.RUnlock()

	// clustersByBinding should now contain the cluster set
	gotClusters, ok := c.clustersByBinding[bindingID]
	assert.True(t, ok, "clustersByBinding should contain the binding after Update")
	assert.True(t, gotClusters.Equal(sets.New[string]("c1")), "clustersByBinding should equal {c1}")

	// groupMembers should now include the binding under the anti-affinity key
	gotMembers, ok := c.groupMembers[antiKey]
	assert.True(t, ok, "groupMembers should contain the anti-affinity key after Update")
	assert.True(t, gotMembers.Equal(sets.New[string](bindingID)), "groupMembers for anti-key should contain the binding ID")
}

func TestOnResourceBindingAddThenUpdate_StillSkipsWhenNoClusters(t *testing.T) {
	c := NewCache(&mockClusterLister{}).(*schedulerCache)

	// 1) Add RB with anti-affinity but NO clusters -> should be skipped.
	oldRB := rbWithGroups("default", "rb1", nil, "", "gAntiOnly")
	c.OnResourceBindingAdd(oldRB)

	c.mu.RLock()
	assert.Empty(t, c.groupMembers, "groupMembers must be empty after Add with no clusters")
	assert.Empty(t, c.clustersByBinding, "clustersByBinding must be empty after Add with no clusters")
	c.mu.RUnlock()

	// 2) Update RB but still NO clusters -> cache should remain skipped.
	// For example, change some other field that is irrelevant to indexing (simulate a no-cluster update).
	updatedRB := rbWithGroups("default", "rb1", nil, "", "gAntiOnly")
	// (optionally mutate a non-indexed field)
	// updatedRB.Annotations = map[string]string{"note": "updated but still unscheduled"}

	c.OnResourceBindingUpdate(oldRB, updatedRB)

	c.mu.RLock()
	defer c.mu.RUnlock()

	// still nothing indexed
	assert.Empty(t, c.groupMembers, "groupMembers must remain empty after Update with no clusters")
	assert.Empty(t, c.clustersByBinding, "clustersByBinding must remain empty after Update with no clusters")
}

func TestOnResourceBindingUpdate_ReindexesGroupAndClusters(t *testing.T) {
	c := NewCache(&mockClusterLister{}).(*schedulerCache)

	oldRB := rbWithGroups("default", "rb1", []string{"c1"}, "gOld", "gAntiOld")
	newRB := rbWithGroups("default", "rb1", []string{"c2", "c3"}, "gNew", "gAntiNew")

	c.OnResourceBindingAdd(oldRB)
	c.OnResourceBindingUpdate(oldRB, newRB)

	oldAffKey := makeWorkloadGroupKey("default", GroupTypeAffinity, "gOld")
	oldAntiKey := makeWorkloadGroupKey("default", GroupTypeAntiAffinity, "gAntiOld")
	newAffKey := makeWorkloadGroupKey("default", GroupTypeAffinity, "gNew")
	newAntiKey := makeWorkloadGroupKey("default", GroupTypeAntiAffinity, "gAntiNew")

	c.mu.RLock()
	defer c.mu.RUnlock()

	// old keys removed
	_, ok := c.groupMembers[oldAffKey]
	assert.False(t, ok)
	_, ok = c.groupMembers[oldAntiKey]
	assert.False(t, ok)

	// new keys present
	gotNewAff := c.groupMembers[newAffKey]
	assert.NotNil(t, gotNewAff)
	assert.True(t, gotNewAff.Equal(sets.New[string]("rb:default/rb1")))

	gotNewAnti := c.groupMembers[newAntiKey]
	assert.NotNil(t, gotNewAnti)
	assert.True(t, gotNewAnti.Equal(sets.New[string]("rb:default/rb1")))

	// clusters updated
	got := c.clustersByBinding["rb:default/rb1"]
	assert.True(t, got.Equal(sets.New[string]("c2", "c3")))
}

func TestOnResourceBindingDelete_UnindexesAndCleansUp(t *testing.T) {
	c := NewCache(&mockClusterLister{}).(*schedulerCache)

	rb := rbWithGroups("default", "rb1", []string{"c1"}, "gA", "gB")
	c.OnResourceBindingAdd(rb)

	// delete via tombstone to cover that path too
	tombstone := cache.DeletedFinalStateUnknown{Obj: rb}
	c.OnResourceBindingDelete(tombstone)

	c.mu.RLock()
	defer c.mu.RUnlock()

	assert.Empty(t, c.groupMembers)
	assert.Empty(t, c.clustersByBinding)
}

func TestAffinityAndAntiAffinity_SameGroupString_DoNotCollide(t *testing.T) {
	c := NewCache(&mockClusterLister{}).(*schedulerCache)

	// Same string in both fields
	rb := rbWithGroups("default", "rb1", []string{"c1"}, "same", "same")
	c.OnResourceBindingAdd(rb)

	affKey := makeWorkloadGroupKey("default", GroupTypeAffinity, "same")
	antiKey := makeWorkloadGroupKey("default", GroupTypeAntiAffinity, "same")

	c.mu.RLock()
	defer c.mu.RUnlock()

	// both exist independently
	gotAff := c.groupMembers[affKey]
	assert.NotNil(t, gotAff)
	assert.True(t, gotAff.Equal(sets.New[string]("rb:default/rb1")))

	gotAnti := c.groupMembers[antiKey]
	assert.NotNil(t, gotAnti)
	assert.True(t, gotAnti.Equal(sets.New[string]("rb:default/rb1")))

	assert.NotEqual(t, affKey, antiKey)
}

func TestClusterResourceBinding_IsIndexedAndNamespacedSeparately(t *testing.T) {
	c := NewCache(&mockClusterLister{}).(*schedulerCache)

	crb := crbWithGroups("crb1", []string{"c9"}, "gA", "gB")
	c.OnResourceBindingAdd(crb)

	affKey := makeWorkloadGroupKey("", GroupTypeAffinity, "gA")
	antiKey := makeWorkloadGroupKey("", GroupTypeAntiAffinity, "gB")

	c.mu.RLock()
	defer c.mu.RUnlock()

	gotAff := c.groupMembers[affKey]
	assert.NotNil(t, gotAff)
	assert.True(t, gotAff.Equal(sets.New[string]("crb:crb1")))

	gotAnti := c.groupMembers[antiKey]
	assert.NotNil(t, gotAnti)
	assert.True(t, gotAnti.Equal(sets.New[string]("crb:crb1")))

	got := c.clustersByBinding["crb:crb1"]
	assert.True(t, got.Equal(sets.New[string]("c9")))
}

func TestResourceBindingAndClusterResourceBinding_SameName_NoCollision(t *testing.T) {
	c := NewCache(&mockClusterLister{}).(*schedulerCache)

	// RB name "x" and CRB name "x" must not collide
	rb := rbWithGroups("default", "x", []string{"c1"}, "g1", "")
	crb := crbWithGroups("x", []string{"c2"}, "g1", "")

	c.OnResourceBindingAdd(rb)
	c.OnResourceBindingAdd(crb)

	rbID := "rb:default/x"
	crbID := "crb:x"

	c.mu.RLock()
	defer c.mu.RUnlock()

	assert.NotEqual(t, rbID, crbID)
	assert.True(t, c.clustersByBinding[rbID].Equal(sets.New[string]("c1")))
	assert.True(t, c.clustersByBinding[crbID].Equal(sets.New[string]("c2")))

	// groupMembers should contain both, under different namespaces ("default" vs "")
	rbKey := makeWorkloadGroupKey("default", GroupTypeAffinity, "g1")
	crbKey := makeWorkloadGroupKey("", GroupTypeAffinity, "g1")

	gotRB := c.groupMembers[rbKey]
	assert.NotNil(t, gotRB)
	assert.True(t, gotRB.Equal(sets.New[string](rbID)))

	gotCRB := c.groupMembers[crbKey]
	assert.NotNil(t, gotCRB)
	assert.True(t, gotCRB.Equal(sets.New[string](crbID)))
}

// Concurrency unit tests

func TestSchedulerCache_ConcurrentUpdates_SameBinding(t *testing.T) {
	c := NewCache(&mockClusterLister{}).(*schedulerCache)

	initial := rbWithGroups("default", "rb1", []string{"c1"}, "gA", "gB")
	c.OnResourceBindingAdd(initial)

	const workers = 32
	const iters = 200

	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		go func(worker int) {
			defer wg.Done()

			prev := initial
			for i := 0; i < iters; i++ {
				var cur *workv1alpha2.ResourceBinding
				switch (worker + i) % 4 {
				case 0:
					cur = rbWithGroups("default", "rb1", []string{"c1", "c2"}, "gA", "gB")
				case 1:
					cur = rbWithGroups("default", "rb1", []string{"c1"}, "gX", "gB")
				case 2:
					cur = rbWithGroups("default", "rb1", []string{"c1"}, "gA", "gY")
				default:
					cur = rbWithGroups("default", "rb1", nil, "gA", "gB")
				}
				c.OnResourceBindingUpdate(prev, cur)
				prev = cur
			}
		}(w)
	}

	wg.Wait()

	c.mu.RLock()
	defer c.mu.RUnlock()
	assertCacheInvariants(t, c)
}

func TestSchedulerCache_ConcurrentMixedOperations_ManyBindings(t *testing.T) {
	c := NewCache(&mockClusterLister{}).(*schedulerCache)

	const workers = 24
	const bindings = 50
	const iters = 200

	// Pre-create a set of binding names to operate on.
	names := make([]string, 0, bindings)
	for i := 0; i < bindings; i++ {
		names = append(names, fmt.Sprintf("rb-%d", i))
	}

	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		go func(worker int) {
			defer wg.Done()

			for i := 0; i < iters; i++ {
				name := names[(worker+i)%len(names)]

				switch (worker + i) % 5 {
				case 0:
					// Add indexed
					rb := rbWithGroups("default", name, []string{"c1"}, "gA", "gB")
					c.OnResourceBindingAdd(rb)
				case 1:
					// Add unindexable (should skip)
					rb := rbWithGroups("default", name, nil, "gA", "gB")
					c.OnResourceBindingAdd(rb)
				case 2:
					// Update to indexed
					old := rbWithGroups("default", name, nil, "gA", "gB")
					newRB := rbWithGroups("default", name, []string{"c2"}, "gA", "gB")
					c.OnResourceBindingUpdate(old, newRB)
				case 3:
					// Update group change
					old := rbWithGroups("default", name, []string{"c1"}, "gA", "gB")
					newRB := rbWithGroups("default", name, []string{"c1"}, "gX", "gY")
					c.OnResourceBindingUpdate(old, newRB)
				default:
					// Delete via tombstone
					rb := rbWithGroups("default", name, []string{"c1"}, "gA", "gB")
					tombstone := cache.DeletedFinalStateUnknown{Obj: rb}
					c.OnResourceBindingDelete(tombstone)
				}
			}
		}(w)
	}

	wg.Wait()

	c.mu.RLock()
	defer c.mu.RUnlock()
	assertCacheInvariants(t, c)
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

// Helper methods

// assertCacheInvariants verifies that the data structures managed by the cache are being maintained correctly during concurrent operations
// This method checks:
// 1. groupMembers is populated correctly: if bindingID exists, then it should also be in bindingGroups + clustersByBinding
// 2. bindingGroups is populated correctly: if groupKey exists, then it should also be in groupMembers
// 3. clustersByBinding is populated correctly and does not contain orphan entries
func assertCacheInvariants(t *testing.T, c *schedulerCache) {
	t.Helper()

	// 1) groupMembers -> bindingGroups and clustersByBinding
	for gk, members := range c.groupMembers {
		assert.NotNil(t, members, "groupMembers[%v] is nil", gk)
		assert.Greater(t, members.Len(), 0, "groupMembers[%v] is empty but present in map", gk)

		for id := range members {
			bg := c.bindingGroups[id]
			assert.NotNil(t, bg, "bindingGroups[%s] missing but referenced by groupMembers[%v]", id, gk)
			assert.True(t, bg.Has(gk), "bindingGroups[%s] missing key %v", id, gk)

			cl := c.clustersByBinding[id]
			assert.NotNil(t, cl, "clustersByBinding[%s] missing but binding is indexed in groupMembers", id)
			assert.Greater(t, cl.Len(), 0, "clustersByBinding[%s] is empty but binding is indexed", id)
		}
	}

	// 2) bindingGroups -> groupMembers
	for id, groups := range c.bindingGroups {
		assert.NotNil(t, groups, "bindingGroups[%s] is nil", id)
		assert.Greater(t, groups.Len(), 0, "bindingGroups[%s] is empty but present in map", id)

		for gk := range groups {
			members := c.groupMembers[gk]
			assert.NotNil(t, members, "groupMembers[%v] missing but referenced by bindingGroups[%s]", gk, id)
			assert.True(t, members.Has(id), "groupMembers[%v] missing binding %s", gk, id)
		}
	}

	// 3) clustersByBinding should not contain entries for bindings that have no groups
	// clusters map exists iff at least one group is indexed
	for id, cl := range c.clustersByBinding {
		assert.NotNil(t, cl, "clustersByBinding[%s] is nil", id)
		assert.Greater(t, cl.Len(), 0, "clustersByBinding[%s] is empty but present in map", id)

		bg := c.bindingGroups[id]
		assert.NotNil(t, bg, "clustersByBinding[%s] exists but bindingGroups missing", id)
		assert.Greater(t, bg.Len(), 0, "clustersByBinding[%s] exists but bindingGroups empty", id)
	}
}

func rbWithGroups(ns, name string, clusters []string, affinityGroup, antiGroup string) *workv1alpha2.ResourceBinding {
	rb := &workv1alpha2.ResourceBinding{}
	rb.Namespace = ns
	rb.Name = name

	for _, c := range clusters {
		rb.Spec.Clusters = append(rb.Spec.Clusters, workv1alpha2.TargetCluster{Name: c})
	}
	if affinityGroup != "" || antiGroup != "" {
		rb.Spec.WorkloadAffinityGroups = &workv1alpha2.WorkloadAffinityGroups{
			AffinityGroup:     affinityGroup,
			AntiAffinityGroup: antiGroup,
		}
	}
	return rb
}

func crbWithGroups(name string, clusters []string, affinityGroup, antiGroup string) *workv1alpha2.ClusterResourceBinding {
	crb := &workv1alpha2.ClusterResourceBinding{}
	crb.Name = name

	for _, c := range clusters {
		crb.Spec.Clusters = append(crb.Spec.Clusters, workv1alpha2.TargetCluster{Name: c})
	}
	if affinityGroup != "" || antiGroup != "" {
		crb.Spec.WorkloadAffinityGroups = &workv1alpha2.WorkloadAffinityGroups{
			AffinityGroup:     affinityGroup,
			AntiAffinityGroup: antiGroup,
		}
	}
	return crb
}
