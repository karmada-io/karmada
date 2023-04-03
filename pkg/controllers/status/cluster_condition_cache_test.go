package status

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

func TestThresholdAdjustedReadyCondition(t *testing.T) {
	clusterSuccessThreshold := 30 * time.Second
	clusterFailureThreshold := 30 * time.Second

	tests := []struct {
		name              string
		clusterData       *clusterData
		currentCondition  *metav1.Condition
		observedCondition *metav1.Condition
		expectedCondition *metav1.Condition
	}{
		{
			name:             "cluster just joined in ready state",
			clusterData:      nil, // no cache yet
			currentCondition: nil, // no condition was set on cluster object yet
			observedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionTrue,
			},
			expectedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name:             "cluster just joined in not-ready state",
			clusterData:      nil, // no cache yet
			currentCondition: nil, // no condition was set on cluster object yet
			observedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionFalse,
			},
			expectedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionFalse,
			},
		},
		{
			name: "cluster stays ready",
			clusterData: &clusterData{
				readyCondition: metav1.ConditionTrue,
			},
			currentCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionTrue,
			},
			observedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionTrue,
			},
			expectedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "cluster becomes not ready but still not reach failure threshold",
			clusterData: &clusterData{
				readyCondition:     metav1.ConditionFalse,
				thresholdStartTime: time.Now().Add(-clusterFailureThreshold / 2),
			},
			currentCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionTrue,
			},
			observedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionFalse,
			},
			expectedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "cluster becomes not ready and reaches failure threshold",
			clusterData: &clusterData{
				readyCondition:     metav1.ConditionFalse,
				thresholdStartTime: time.Now().Add(-clusterFailureThreshold),
			},
			currentCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionTrue,
			},
			observedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionFalse,
			},
			expectedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionFalse,
			},
		},
		{
			name: "cluster stays not ready",
			clusterData: &clusterData{
				readyCondition:     metav1.ConditionFalse,
				thresholdStartTime: time.Now().Add(-2 * clusterFailureThreshold),
			},
			currentCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionFalse,
			},
			observedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionFalse,
			},
			expectedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionFalse,
			},
		},
		{
			name: "cluster recovers but still not reach success threshold",
			clusterData: &clusterData{
				readyCondition:     metav1.ConditionTrue,
				thresholdStartTime: time.Now().Add(-clusterSuccessThreshold / 2),
			},
			currentCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionFalse,
			},
			observedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionTrue,
			},
			expectedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionFalse,
			},
		},
		{
			name: "cluster recovers and reaches success threshold",
			clusterData: &clusterData{
				readyCondition:     metav1.ConditionTrue,
				thresholdStartTime: time.Now().Add(-clusterSuccessThreshold),
			},
			currentCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionFalse,
			},
			observedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionTrue,
			},
			expectedCondition: &metav1.Condition{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: metav1.ConditionTrue,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := clusterConditionStore{
				successThreshold: clusterSuccessThreshold,
				failureThreshold: clusterFailureThreshold,
			}

			if tt.clusterData != nil {
				cache.update("member", tt.clusterData)
			}

			cluster := &clusterv1alpha1.Cluster{}
			cluster.Name = "member"
			if tt.currentCondition != nil {
				meta.SetStatusCondition(&cluster.Status.Conditions, *tt.currentCondition)
			}

			thresholdReadyCondition := cache.thresholdAdjustedReadyCondition(cluster, tt.observedCondition)

			if tt.expectedCondition.Status != thresholdReadyCondition.Status {
				t.Fatalf("expected: %s, but got: %s", tt.expectedCondition.Status, thresholdReadyCondition.Status)
			}
		})
	}
}

func TestClusterConditionStore_Get(t *testing.T) {
	// Create a new clusterConditionStore
	store := clusterConditionStore{}

	// Create a new clusterData object
	cluster := "test-cluster"
	now := time.Now()
	data := &clusterData{
		readyCondition:     metav1.ConditionTrue,
		thresholdStartTime: now,
	}

	// Add the clusterData object to the store
	store.clusterDataMap.Store(cluster, data)

	// Call the get function and check the returned value
	result := store.get(cluster)
	if result == nil {
		t.Errorf("Expected non-nil result, got nil")
	}
	if result != data {
		t.Errorf("Expected %v, got %v", data, result)
	}

	// Call the get function with a non-existent cluster and check the returned value
	result = store.get("non-existent-cluster")
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
}

func TestClusterConditionStore_Update(t *testing.T) {
	// Create a new clusterConditionStore
	store := &clusterConditionStore{}

	// Create a new clusterData object
	cluster := "test-cluster"
	now := time.Now()
	data := &clusterData{
		readyCondition:     metav1.ConditionTrue,
		thresholdStartTime: now,
	}

	t.Run("Test update with new data", func(t *testing.T) {
		// Update the cluster with new data
		store.update(cluster, data)

		// Retrieve the updated data and verify it
		result := store.get(cluster)
		if result == nil {
			t.Errorf("Expected non-nil result, got nil")
		}
		if result != data {
			t.Errorf("Expected %v, got %v", data, result)
		}
	})

	t.Run("Test update with same data", func(t *testing.T) {
		// Update the cluster with the same data
		store.update(cluster, data)

		// Retrieve the updated data and verify it
		result := store.get(cluster)
		if result == nil {
			t.Errorf("Expected non-nil result, got nil")
		}
		if result != data {
			t.Errorf("Expected %v, got %v", data, result)
		}
	})

	t.Run("Test update with nil data", func(t *testing.T) {
		// Update the cluster with nil data
		store.update(cluster, nil)

		// Retrieve the updated data and verify it
		result := store.get(cluster)
		if result != nil {
			t.Errorf("Expected nil result, got %v", result)
		}
	})
}

func TestClusterConditionStore_Delete(t *testing.T) {
	// Create a new clusterConditionStore
	store := clusterConditionStore{}

	// Create a new clusterData object
	cluster := "test-cluster"
	now := time.Now()
	data := &clusterData{
		readyCondition:     metav1.ConditionTrue,
		thresholdStartTime: now,
	}

	// Add the clusterData object to the store
	store.clusterDataMap.Store(cluster, data)

	// Delete the clusterData object from the store
	store.delete(cluster)

	// Call the get function with the deleted cluster and check the returned value
	result := store.get(cluster)
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
}
