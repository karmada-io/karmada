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

package typedmanager

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/karmada-io/karmada/pkg/util/fedinformer"
)

var (
	instance    MultiClusterInformerManager
	once        sync.Once
	ctx, cancel = context.WithCancel(context.Background())
)

// GetInstance returns a shared MultiClusterInformerManager instance.
func GetInstance() MultiClusterInformerManager {
	once.Do(func() {
		transforms := map[schema.GroupVersionResource]cache.TransformFunc{
			nodeGVR: fedinformer.NodeTransformFunc,
			podGVR:  fedinformer.PodTransformFunc,
		}
		instance = NewMultiClusterInformerManager(ctx, transforms)
	})
	return instance
}

// StopInstance will stop the shared MultiClusterInformerManager instance.
func StopInstance() {
	cancel()
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
func NewMultiClusterInformerManager(ctx context.Context, transformFuncs map[schema.GroupVersionResource]cache.TransformFunc) MultiClusterInformerManager {
	return &multiClusterInformerManagerImpl{
		managers:       make(map[string]SingleClusterInformerManager),
		transformFuncs: transformFuncs,
		ctx:            ctx,
	}
}

type multiClusterInformerManagerImpl struct {
	managers       map[string]SingleClusterInformerManager
	transformFuncs map[schema.GroupVersionResource]cache.TransformFunc
	ctx            context.Context
	lock           sync.RWMutex
}

func (m *multiClusterInformerManagerImpl) getManager(cluster string) (SingleClusterInformerManager, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	manager, exist := m.managers[cluster]
	return manager, exist
}

func (m *multiClusterInformerManagerImpl) ForCluster(cluster string, client kubernetes.Interface, defaultResync time.Duration) SingleClusterInformerManager {
	m.lock.Lock()
	defer m.lock.Unlock()

	// If informer manager already exist, just return
	if manager, exist := m.managers[cluster]; exist {
		return manager
	}

	manager := NewSingleClusterInformerManager(m.ctx, client, defaultResync, m.transformFuncs)
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
