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

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
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
