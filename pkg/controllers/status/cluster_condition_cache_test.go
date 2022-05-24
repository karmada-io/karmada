package status

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

func TestThresholdAdjustedReadyCondition(t *testing.T) {
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
			name: "cluster becomes not ready but still not reach threshold",
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
			name: "cluster becomes not ready and reaches threshold",
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
			name: "cluster recovers",
			clusterData: &clusterData{
				readyCondition:     metav1.ConditionFalse,
				thresholdStartTime: time.Now().Add(-3 * clusterFailureThreshold),
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
