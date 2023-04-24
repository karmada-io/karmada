package applicationfailover

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

type workloadUnhealthyMap struct {
	sync.RWMutex
	// key is the resource type
	// value is also a map. Its key is the cluster where the unhealthy workload resides.
	// Its value is the time when the unhealthy state was first observed.
	workloadUnhealthy map[keys.ClusterWideKey]map[string]metav1.Time
}

func newWorkloadUnhealthyMap() *workloadUnhealthyMap {
	return &workloadUnhealthyMap{
		workloadUnhealthy: make(map[keys.ClusterWideKey]map[string]metav1.Time),
	}
}

func (m *workloadUnhealthyMap) delete(key keys.ClusterWideKey) {
	m.Lock()
	defer m.Unlock()
	delete(m.workloadUnhealthy, key)
}

func (m *workloadUnhealthyMap) hasWorkloadBeenUnhealthy(resource keys.ClusterWideKey, cluster string) bool {
	m.RLock()
	defer m.RUnlock()

	unhealthyClusters := m.workloadUnhealthy[resource]
	if unhealthyClusters == nil {
		return false
	}

	_, exist := unhealthyClusters[cluster]
	return exist
}

func (m *workloadUnhealthyMap) setTimeStamp(resource keys.ClusterWideKey, cluster string) {
	m.Lock()
	defer m.Unlock()

	unhealthyClusters := m.workloadUnhealthy[resource]
	if unhealthyClusters == nil {
		unhealthyClusters = make(map[string]metav1.Time)
	}

	unhealthyClusters[cluster] = metav1.Now()
	m.workloadUnhealthy[resource] = unhealthyClusters
}

func (m *workloadUnhealthyMap) getTimeStamp(resource keys.ClusterWideKey, cluster string) metav1.Time {
	m.RLock()
	defer m.RUnlock()

	unhealthyClusters := m.workloadUnhealthy[resource]
	return unhealthyClusters[cluster]
}

func (m *workloadUnhealthyMap) deleteIrrelevantClusters(resource keys.ClusterWideKey, allClusters sets.Set[string]) {
	m.Lock()
	defer m.Unlock()

	unhealthyClusters := m.workloadUnhealthy[resource]
	if unhealthyClusters == nil {
		return
	}
	for cluster := range unhealthyClusters {
		if !allClusters.Has(cluster) {
			delete(unhealthyClusters, cluster)
		}
	}
	m.workloadUnhealthy[resource] = unhealthyClusters
}

// filterIrrelevantClusters filters clusters which is in the process of eviction or in the Healthy/Unknown state.
func filterIrrelevantClusters(aggregatedStatusItems []workv1alpha2.AggregatedStatusItem, resourceBindingSpec workv1alpha2.ResourceBindingSpec) []string {
	var filteredClusters []string
	for _, aggregatedStatusItem := range aggregatedStatusItems {
		cluster := aggregatedStatusItem.ClusterName

		if aggregatedStatusItem.Health == workv1alpha2.ResourceUnhealthy && !resourceBindingSpec.ClusterInGracefulEvictionTasks(cluster) {
			filteredClusters = append(filteredClusters, cluster)
		}
	}

	return filteredClusters
}
