package applicationfailover

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

type workloadUnhealthyMap struct {
	sync.RWMutex
	// key is the NamespacedName of the binding
	// value is also a map. Its key is the cluster where the unhealthy workload resides.
	// Its value is the time when the unhealthy state was first observed.
	workloadUnhealthy map[types.NamespacedName]map[string]metav1.Time
}

func newWorkloadUnhealthyMap() *workloadUnhealthyMap {
	return &workloadUnhealthyMap{
		workloadUnhealthy: make(map[types.NamespacedName]map[string]metav1.Time),
	}
}

func (m *workloadUnhealthyMap) delete(key types.NamespacedName) {
	m.Lock()
	defer m.Unlock()
	delete(m.workloadUnhealthy, key)
}

func (m *workloadUnhealthyMap) hasWorkloadBeenUnhealthy(key types.NamespacedName, cluster string) bool {
	m.RLock()
	defer m.RUnlock()

	unhealthyClusters := m.workloadUnhealthy[key]
	if unhealthyClusters == nil {
		return false
	}

	_, exist := unhealthyClusters[cluster]
	return exist
}

func (m *workloadUnhealthyMap) setTimeStamp(key types.NamespacedName, cluster string) {
	m.Lock()
	defer m.Unlock()

	unhealthyClusters := m.workloadUnhealthy[key]
	if unhealthyClusters == nil {
		unhealthyClusters = make(map[string]metav1.Time)
	}

	unhealthyClusters[cluster] = metav1.Now()
	m.workloadUnhealthy[key] = unhealthyClusters
}

func (m *workloadUnhealthyMap) getTimeStamp(key types.NamespacedName, cluster string) metav1.Time {
	m.RLock()
	defer m.RUnlock()

	unhealthyClusters := m.workloadUnhealthy[key]
	return unhealthyClusters[cluster]
}

func (m *workloadUnhealthyMap) deleteIrrelevantClusters(key types.NamespacedName, allClusters sets.Set[string], healthyClusters []string) {
	m.Lock()
	defer m.Unlock()

	unhealthyClusters := m.workloadUnhealthy[key]
	if unhealthyClusters == nil {
		return
	}
	for cluster := range unhealthyClusters {
		if !allClusters.Has(cluster) {
			delete(unhealthyClusters, cluster)
		}
	}
	for _, cluster := range healthyClusters {
		delete(unhealthyClusters, cluster)
	}

	m.workloadUnhealthy[key] = unhealthyClusters
}

// distinguishUnhealthyClustersWithOthers distinguishes clusters which is in the unHealthy state(not in the process of eviction) with others.
func distinguishUnhealthyClustersWithOthers(aggregatedStatusItems []workv1alpha2.AggregatedStatusItem, resourceBindingSpec workv1alpha2.ResourceBindingSpec) ([]string, []string) {
	var unhealthyClusters, others []string
	for _, aggregatedStatusItem := range aggregatedStatusItems {
		cluster := aggregatedStatusItem.ClusterName

		if aggregatedStatusItem.Health == workv1alpha2.ResourceUnhealthy && !resourceBindingSpec.ClusterInGracefulEvictionTasks(cluster) {
			unhealthyClusters = append(unhealthyClusters, cluster)
		}

		if aggregatedStatusItem.Health == workv1alpha2.ResourceHealthy || aggregatedStatusItem.Health == workv1alpha2.ResourceUnknown {
			others = append(others, cluster)
		}
	}

	return unhealthyClusters, others
}
