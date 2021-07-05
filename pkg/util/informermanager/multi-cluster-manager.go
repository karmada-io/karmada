package informermanager

import (
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	instance MultiClusterInformerManager
	once     sync.Once
)

// GetInstance returns a shared MultiClusterInformerManager instance.
func GetInstance() MultiClusterInformerManager {
	once.Do(func() {
		instance = NewMultiClusterInformerManager()
	})
	return instance
}

// MultiClusterInformerManager manages dynamic shared informer for all resources, include Kubernetes resource and
// custom resources defined by CustomResourceDefinition, across multi-cluster.
type MultiClusterInformerManager interface {
	// ForCluster builds a informer manager for a specific cluster.
	ForCluster(cluster string, client dynamic.Interface, defaultResync time.Duration) SingleClusterInformerManager

	// GetSingleClusterManager gets the informer manager for a specific cluster.
	// The informer manager should be created before, otherwise, nil will be returned.
	GetSingleClusterManager(cluster string) SingleClusterInformerManager

	// IsManagerExist checks if the informer manager for the cluster already created.
	IsManagerExist(cluster string) bool

	// Start will run all informers for a specific cluster, it accepts a stop channel, the informers will keep running until channel closed.
	// Should call after 'ForCluster', otherwise no-ops.
	Start(cluster string, stopCh <-chan struct{})

	// WaitForCacheSync waits for all caches to populate.
	// Should call after 'ForCluster', otherwise no-ops.
	WaitForCacheSync(cluster string, stopCh <-chan struct{}) map[schema.GroupVersionResource]bool
}

// NewMultiClusterInformerManager constructs a new instance of multiClusterInformerManagerImpl.
func NewMultiClusterInformerManager() MultiClusterInformerManager {
	return &multiClusterInformerManagerImpl{
		managers: make(map[string]SingleClusterInformerManager),
	}
}

type multiClusterInformerManagerImpl struct {
	managers map[string]SingleClusterInformerManager
	lock     sync.RWMutex
}

func (m *multiClusterInformerManagerImpl) ForCluster(cluster string, client dynamic.Interface, defaultResync time.Duration) SingleClusterInformerManager {
	// If informer manager already exist, just return
	m.lock.RLock()
	manager, exist := m.managers[cluster]
	if exist {
		m.lock.RUnlock()
		return manager
	}
	m.lock.RUnlock()
	m.lock.Lock()
	defer m.lock.Unlock()
	manager = NewSingleClusterInformerManager(client, defaultResync)
	m.managers[cluster] = manager
	return manager
}

func (m *multiClusterInformerManagerImpl) GetSingleClusterManager(cluster string) SingleClusterInformerManager {
	m.lock.RLock()
	defer m.lock.RUnlock()
	manager, exist := m.managers[cluster]
	if exist {
		return manager
	}
	return nil
}

func (m *multiClusterInformerManagerImpl) IsManagerExist(cluster string) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()

	_, exist := m.managers[cluster]

	return exist
}

func (m *multiClusterInformerManagerImpl) Start(cluster string, stopCh <-chan struct{}) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	// if informer manager haven't been created, just return with no-ops.
	manager, exist := m.managers[cluster]
	if !exist {
		return
	}
	manager.Start(stopCh)
}

func (m *multiClusterInformerManagerImpl) WaitForCacheSync(cluster string, stopCh <-chan struct{}) map[schema.GroupVersionResource]bool {
	m.lock.RLock()
	defer m.lock.RUnlock()

	manager, exist := m.managers[cluster]
	if !exist {
		return nil
	}
	return manager.WaitForCacheSync(stopCh)
}
