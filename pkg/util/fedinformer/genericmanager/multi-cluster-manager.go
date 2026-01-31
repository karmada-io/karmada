/*
Copyright 2020 The Karmada Authors.

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

package genericmanager

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

var (
	instance    MultiClusterInformerManager
	once        sync.Once
	ctx, cancel = context.WithCancel(context.Background())
)

// GetInstance returns a shared MultiClusterInformerManager instance.
func GetInstance() MultiClusterInformerManager {
	once.Do(func() {
		instance = NewMultiClusterInformerManager(ctx)
	})
	return instance
}

// StopInstance will stop the shared MultiClusterInformerManager instance.
func StopInstance() {
	cancel()
}

// MultiClusterInformerManager manages dynamic shared informer for all resources, include Kubernetes resource and
// custom resources defined by CustomResourceDefinition, across multi-cluster.
type MultiClusterInformerManager interface {
	// ForCluster builds an informer manager for a specific cluster.
	// Deprecated: Use ForClusterWithUID instead to ensure proper cache invalidation
	// when a cluster with the same name but different identity is registered.
	ForCluster(cluster string, client dynamic.Interface, defaultResync time.Duration) SingleClusterInformerManager

	// ForClusterWithUID builds an informer manager for a specific cluster with UID tracking.
	// If a manager already exists for the cluster name but with a different UID (indicating the
	// cluster was re-registered with the same name), the old manager is stopped and a new one
	// is created to prevent stale cache issues.
	ForClusterWithUID(cluster string, clusterUID types.UID, client dynamic.Interface, defaultResync time.Duration) SingleClusterInformerManager

	// GetSingleClusterManager gets the informer manager for a specific cluster.
	// The informer manager should be created before, otherwise, nil will be returned.
	GetSingleClusterManager(cluster string) SingleClusterInformerManager

	// IsManagerExist checks if the informer manager for the cluster already created.
	IsManagerExist(cluster string) bool

	// IsManagerExistWithUID checks if the informer manager for the cluster exists with the specified UID.
	// Returns true only if the manager exists AND was registered with the same UID.
	// This is useful to determine if a new manager needs to be created due to cluster re-registration.
	IsManagerExistWithUID(cluster string, uid types.UID) bool

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
func NewMultiClusterInformerManager(ctx context.Context) MultiClusterInformerManager {
	return &multiClusterInformerManagerImpl{
		managers:    make(map[string]SingleClusterInformerManager),
		clusterUIDs: make(map[string]types.UID),
		ctx:         ctx,
	}
}

type multiClusterInformerManagerImpl struct {
	managers map[string]SingleClusterInformerManager
	// clusterUIDs tracks the UID of each cluster to detect when a cluster with the same name
	// but different identity is registered (e.g., after cluster removal and re-registration).
	clusterUIDs map[string]types.UID
	ctx         context.Context
	lock        sync.RWMutex
}

func (m *multiClusterInformerManagerImpl) getManager(cluster string) (SingleClusterInformerManager, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	manager, exist := m.managers[cluster]
	return manager, exist
}

func (m *multiClusterInformerManagerImpl) ForCluster(cluster string, client dynamic.Interface, defaultResync time.Duration) SingleClusterInformerManager {
	m.lock.Lock()
	defer m.lock.Unlock()

	// If informer manager already exist, just return
	if manager, exist := m.managers[cluster]; exist {
		return manager
	}

	manager := NewSingleClusterInformerManager(m.ctx, client, defaultResync)
	m.managers[cluster] = manager
	return manager
}

func (m *multiClusterInformerManagerImpl) ForClusterWithUID(cluster string, clusterUID types.UID, client dynamic.Interface, defaultResync time.Duration) SingleClusterInformerManager {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Check if a manager exists for this cluster name
	existingManager, managerExists := m.managers[cluster]
	existingUID, uidExists := m.clusterUIDs[cluster]

	// If manager exists and UID matches, return the existing manager
	if managerExists && uidExists && existingUID == clusterUID {
		return existingManager
	}

	// If manager exists but was created without UID tracking (via ForCluster),
	// record the UID now and return the existing manager for backward compatibility.
	// This happens when transitioning from old code that used ForCluster.
	if managerExists && !uidExists {
		m.clusterUIDs[cluster] = clusterUID
		return existingManager
	}

	// If manager exists but UID is different (cluster was re-registered with same name),
	// stop the old manager to invalidate stale cache
	if managerExists && uidExists && existingUID != clusterUID {
		klog.Infof("Cluster %s re-registered with different UID (old: %s, new: %s), invalidating stale informer cache",
			cluster, existingUID, clusterUID)
		existingManager.Stop()
		delete(m.managers, cluster)
		delete(m.clusterUIDs, cluster)
	}

	// Create a new manager for this cluster
	manager := NewSingleClusterInformerManager(m.ctx, client, defaultResync)
	m.managers[cluster] = manager
	m.clusterUIDs[cluster] = clusterUID
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

func (m *multiClusterInformerManagerImpl) IsManagerExistWithUID(cluster string, uid types.UID) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()
	_, managerExists := m.managers[cluster]
	existingUID, uidExists := m.clusterUIDs[cluster]
	// Strictly verify both manager exists and UID matches.
	// ForClusterWithUID handles backward compatibility for managers without UID tracking.
	return managerExists && uidExists && existingUID == uid
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
	delete(m.clusterUIDs, cluster)
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
