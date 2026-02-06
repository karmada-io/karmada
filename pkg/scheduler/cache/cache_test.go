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

	assert.ElementsMatch(t, []string{"rb:default/rb1"}, c.groupPeers[affKey])
	assert.ElementsMatch(t, []string{"rb:default/rb1"}, c.groupPeers[antiKey])

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

	assert.Empty(t, c.groupPeers)
	assert.Empty(t, c.clustersByBinding)
}

func TestOnResourceBindingAdd_SkipsWhenNoGroups(t *testing.T) {
	c := NewCache(&mockClusterLister{}).(*schedulerCache)

	// Has clusters, but no WorkloadAffinityGroups -> should not be indexed.
	rb := rbWithGroups("default", "rb1", []string{"c1"}, "", "")
	c.OnResourceBindingAdd(rb)

	c.mu.RLock()
	defer c.mu.RUnlock()

	assert.Empty(t, c.groupPeers)
	assert.Empty(t, c.clustersByBinding)
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
	_, ok := c.groupPeers[oldAffKey]
	assert.False(t, ok)
	_, ok = c.groupPeers[oldAntiKey]
	assert.False(t, ok)

	// new keys present
	assert.ElementsMatch(t, []string{"rb:default/rb1"}, c.groupPeers[newAffKey])
	assert.ElementsMatch(t, []string{"rb:default/rb1"}, c.groupPeers[newAntiKey])

	// clusters updated
	got := c.clustersByBinding["rb:default/rb1"]
	assert.Equal(t, sets.New("c2", "c3"), got)
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

	assert.Empty(t, c.groupPeers)
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
	assert.ElementsMatch(t, []string{"rb:default/rb1"}, c.groupPeers[affKey])
	assert.ElementsMatch(t, []string{"rb:default/rb1"}, c.groupPeers[antiKey])
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

	assert.ElementsMatch(t, []string{"crb:crb1"}, c.groupPeers[affKey])
	assert.ElementsMatch(t, []string{"crb:crb1"}, c.groupPeers[antiKey])

	got := c.clustersByBinding["crb:crb1"]
	assert.Equal(t, sets.New("c9"), got)
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
	assert.Equal(t, sets.New("c1"), c.clustersByBinding[rbID])
	assert.Equal(t, sets.New("c2"), c.clustersByBinding[crbID])

	// groupPeers should contain both, under different namespaces ("default" vs "")
	rbKey := makeWorkloadGroupKey("default", GroupTypeAffinity, "g1")
	crbKey := makeWorkloadGroupKey("", GroupTypeAffinity, "g1")

	assert.ElementsMatch(t, []string{rbID}, c.groupPeers[rbKey])
	assert.ElementsMatch(t, []string{crbID}, c.groupPeers[crbKey])
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
