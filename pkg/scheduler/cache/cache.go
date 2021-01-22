package cache

import (
	"sync"

	"github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

// Cache is an interface for scheduler internal cache.
type Cache interface {
	AddCluster(cluster *v1alpha1.Cluster)
	UpdateCluster(cluster *v1alpha1.Cluster)
	DeleteCluster(cluster *v1alpha1.Cluster)
	// Snapshot returns a snapshot of the current clusters info
	Snapshot() *Snapshot
}

type schedulerCache struct {
	mutex    sync.RWMutex
	clusters map[string]*v1alpha1.Cluster
}

// NewCache instantiates a cache used only by scheduler.
func NewCache() Cache {
	return &schedulerCache{
		clusters: make(map[string]*v1alpha1.Cluster),
	}
}

func (c *schedulerCache) AddCluster(cluster *v1alpha1.Cluster) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.clusters[cluster.Name] = cluster
}

func (c *schedulerCache) UpdateCluster(cluster *v1alpha1.Cluster) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.clusters[cluster.Name] = cluster
}

func (c *schedulerCache) DeleteCluster(cluster *v1alpha1.Cluster) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.clusters, cluster.Name)
}

// TODO: need optimization, only clone when necessary
func (c *schedulerCache) Snapshot() *Snapshot {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	out := NewEmptySnapshot()
	out.clusterInfoList = make([]*framework.ClusterInfo, 0, len(c.clusters))
	for _, cluster := range c.clusters {
		cloned := cluster.DeepCopy()
		out.clusterInfoList = append(out.clusterInfoList, framework.NewClusterInfo(cloned))
	}

	return out
}
