package typedManager

import (
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

var (
	instance MultiClusterInformerManager
	once     sync.Once
	stopOnce sync.Once
	stopCh   chan struct{}
)

func init() {
	stopCh = make(chan struct{})
}

// GetInstance returns a shared MultiClusterInformerManager instance.
func GetInstance() MultiClusterInformerManager {
	once.Do(func() {
		instance = NewMultiClusterInformerManager(stopCh)
	})
	return instance
}

// StopInstance will stop the shared MultiClusterInformerManager instance.
func StopInstance() {
	stopOnce.Do(func() {
		close(stopCh)
	})
}

// MultiClusterInformerManager manages typed shared informer for all resources, include Kubernetes resource and
// custom resources defined by CustomResourceDefinition, across multi-cluster.
type MultiClusterInformerManager interface {
	// ForCluster builds an informer manager for a specific cluster.
	ForCluster(cluster string, client kubernetes.Interface, defaultResync time.Duration) SingleClusterInformerManager

	// GetSingleClusterManager gets the informer manager for a specific cluster.
	// The informer manager should be created before, otherwise, nil will be returned.
	GetSingleClusterManager(cluster string) SingleClusterInformerManager

	// IsManagerExist checks if the informer manager for the cluster already created.
	IsManagerExist(cluster string) bool

	// Start will run all informers for a specific cluster.
	// Should call after 'ForCluster', otherwise no-ops.
	Start(cluster string)

	// Stop will stop all informers for a specific cluster, and delete the cluster from informer managers.
	Stop(cluster string)

	// WaitForCacheSync waits for all caches to populate.
	// Should call after 'ForCluster', otherwise no-ops.
	WaitForCacheSync(cluster string) map[schema.GroupVersionResource]bool

	// WaitForCacheSyncWithTimeout waits for all caches to populate with a definitive timeout.
	// Should call after 'ForCluster', otherwise no-ops.
	WaitForCacheSyncWithTimeout(cluster string, cacheSyncTimeout time.Duration) map[schema.GroupVersionResource]bool
}

// NewMultiClusterInformerManager constructs a new instance of multiClusterInformerManagerImpl.
func NewMultiClusterInformerManager(stopCh <-chan struct{}) MultiClusterInformerManager {
	return &multiClusterInformerManagerImpl{
		managers: make(map[string]SingleClusterInformerManager),
		stopCh:   stopCh,
	}
}

type multiClusterInformerManagerImpl struct {
	managers map[string]SingleClusterInformerManager
	stopCh   <-chan struct{}
	lock     sync.RWMutex
}

func (m *multiClusterInformerManagerImpl) getManager(cluster string) (SingleClusterInformerManager, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	manager, exist := m.managers[cluster]
	return manager, exist
}

func (m *multiClusterInformerManagerImpl) ForCluster(cluster string, client kubernetes.Interface, defaultResync time.Duration) SingleClusterInformerManager {
	// If informer manager already exist, just return
	if manager, exist := m.getManager(cluster); exist {
		return manager
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	manager := NewSingleClusterInformerManager(client, defaultResync, m.stopCh)
	m.managers[cluster] = manager
	return manager
}

func (m *multiClusterInformerManagerImpl) GetSingleClusterManager(cluster string) SingleClusterInformerManager {
	if manager, exist := m.getManager(cluster); exist {
		return manager
	}
	return nil
}

func (m *multiClusterInformerManagerImpl) IsManagerExist(cluster string) bool {
	_, exist := m.getManager(cluster)
	return exist
}

func (m *multiClusterInformerManagerImpl) Start(cluster string) {
	// if informer manager haven't been created, just return with no-ops.
	manager, exist := m.getManager(cluster)
	if !exist {
		return
	}
	manager.Start()
}

func (m *multiClusterInformerManagerImpl) Stop(cluster string) {
	manager, exist := m.getManager(cluster)
	if !exist {
		return
	}
	manager.Stop()
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.managers, cluster)
}

func (m *multiClusterInformerManagerImpl) WaitForCacheSync(cluster string) map[schema.GroupVersionResource]bool {
	manager, exist := m.getManager(cluster)
	if !exist {
		return nil
	}
	return manager.WaitForCacheSync()
}

func (m *multiClusterInformerManagerImpl) WaitForCacheSyncWithTimeout(cluster string, cacheSyncTimeout time.Duration) map[schema.GroupVersionResource]bool {
	manager, exist := m.getManager(cluster)
	if !exist {
		return nil
	}
	return manager.WaitForCacheSyncWithTimeout(cacheSyncTimeout)
}
