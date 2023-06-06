package applicationfailover

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestTimeStampProcess(t *testing.T) {
	key := types.NamespacedName{
		Namespace: "default",
		Name:      "test",
	}
	cluster := "cluster-1"

	m := newWorkloadUnhealthyMap()
	m.setTimeStamp(key, cluster)
	res := m.hasWorkloadBeenUnhealthy(key, cluster)
	assert.Equal(t, true, res)

	time := m.getTimeStamp(key, cluster)
	assert.NotEmpty(t, time)

	m.delete(key)
	res = m.hasWorkloadBeenUnhealthy(key, cluster)
	assert.Equal(t, false, res)
}

func TestWorkloadUnhealthyMap_deleteIrrelevantClusters(t *testing.T) {
	cluster1 := "cluster-1"
	cluster2 := "cluster-2"
	cluster3 := "cluster-3"
	t.Run("normal case", func(t *testing.T) {
		key := types.NamespacedName{
			Namespace: "default",
			Name:      "test",
		}

		m := newWorkloadUnhealthyMap()

		m.setTimeStamp(key, cluster1)
		m.setTimeStamp(key, cluster2)
		m.setTimeStamp(key, cluster3)

		allClusters := sets.New[string](cluster2, cluster3)
		healthyClusters := []string{cluster3}

		m.deleteIrrelevantClusters(key, allClusters, healthyClusters)
		res1 := m.hasWorkloadBeenUnhealthy(key, cluster1)
		assert.Equal(t, false, res1)
		res2 := m.hasWorkloadBeenUnhealthy(key, cluster2)
		assert.Equal(t, true, res2)
		res3 := m.hasWorkloadBeenUnhealthy(key, cluster3)
		assert.Equal(t, false, res3)
	})

	t.Run("unhealthyClusters is nil", func(t *testing.T) {
		key := types.NamespacedName{
			Namespace: "default",
			Name:      "test",
		}

		m := newWorkloadUnhealthyMap()

		allClusters := sets.New[string](cluster2, cluster3)
		healthyClusters := []string{cluster3}

		m.deleteIrrelevantClusters(key, allClusters, healthyClusters)
		res := m.hasWorkloadBeenUnhealthy(key, cluster2)
		assert.Equal(t, false, res)
	})
}

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
