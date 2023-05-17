package applicationfailover

import (
	"reflect"
	"testing"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestDistinguishUnhealthyClustersWithOthers(t *testing.T) {
	tests := []struct {
		name                  string
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		resourceBindingSpec   workv1alpha2.ResourceBindingSpec
		expectedClusters      []string
		expectedOthers        []string
	}{
		{
			name: "all applications are healthy",
			aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Health:      workv1alpha2.ResourceHealthy,
				},
				{
					ClusterName: "member2",
					Health:      workv1alpha2.ResourceHealthy,
				},
			},
			resourceBindingSpec: workv1alpha2.ResourceBindingSpec{},
			expectedClusters:    nil,
			expectedOthers:      []string{"member1", "member2"},
		},
		{
			name: "all applications are unknown",
			aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Health:      workv1alpha2.ResourceUnknown,
				},
				{
					ClusterName: "member2",
					Health:      workv1alpha2.ResourceUnknown,
				},
			},
			resourceBindingSpec: workv1alpha2.ResourceBindingSpec{},
			expectedClusters:    nil,
			expectedOthers:      []string{"member1", "member2"},
		},
		{
			name: "one application is unhealthy and not in gracefulEvictionTasks",
			aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Health:      workv1alpha2.ResourceHealthy,
				},
				{
					ClusterName: "member2",
					Health:      workv1alpha2.ResourceUnhealthy,
				},
			},
			resourceBindingSpec: workv1alpha2.ResourceBindingSpec{},
			expectedClusters:    []string{"member2"},
			expectedOthers:      []string{"member1"},
		},
		{
			name: "one application is unhealthy and in gracefulEvictionTasks",
			aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Health:      workv1alpha2.ResourceHealthy,
				},
				{
					ClusterName: "member2",
					Health:      workv1alpha2.ResourceUnhealthy,
				},
			},
			resourceBindingSpec: workv1alpha2.ResourceBindingSpec{
				GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "member2",
					},
				},
			},
			expectedClusters: nil,
			expectedOthers:   []string{"member1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, gotOthers := distinguishUnhealthyClustersWithOthers(tt.aggregatedStatusItems, tt.resourceBindingSpec); !reflect.DeepEqual(got, tt.expectedClusters) || !reflect.DeepEqual(gotOthers, tt.expectedOthers) {
				t.Errorf("distinguishUnhealthyClustersWithOthers() = (%v, %v), want (%v, %v)", got, gotOthers, tt.expectedClusters, tt.expectedOthers)
			}
		})
	}
}
