/*
Copyright 2025 The Karmada Authors.

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

package testing

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

// FakeSingleClusterManager implements SingleClusterInformerManager interface.
// It is used to simulate SingleClusterInformerManager for testing.
type FakeSingleClusterManager struct {
	synced        bool
	handlerExists bool
	listerFunc    func(gvr schema.GroupVersionResource) cache.GenericLister
}

// NewFakeSingleClusterManager creates a new FakeSingleClusterManager.
func NewFakeSingleClusterManager(synced bool, handlerExists bool, listerFunc func(gvr schema.GroupVersionResource) cache.GenericLister) *FakeSingleClusterManager {
	return &FakeSingleClusterManager{
		synced:        synced,
		handlerExists: handlerExists,
		listerFunc:    listerFunc,
	}
}

// IsInformerSynced returns true if the informer is synced.
func (m *FakeSingleClusterManager) IsInformerSynced(_ schema.GroupVersionResource) bool {
	return m.synced
}

// Lister returns the lister for the given GVR.
func (m *FakeSingleClusterManager) Lister(gvr schema.GroupVersionResource) cache.GenericLister {
	if m.listerFunc != nil {
		return m.listerFunc(gvr)
	}
	return nil
}

// ForResource adds a resource event handler to the informer.
func (m *FakeSingleClusterManager) ForResource(_ schema.GroupVersionResource, _ cache.ResourceEventHandler) {
}

// Start starts the informer.
func (m *FakeSingleClusterManager) Start() {}

// Stop stops the informer.
func (m *FakeSingleClusterManager) Stop() {}

// IsHandlerExist returns true if the handler exists for the given GVR.
func (m *FakeSingleClusterManager) IsHandlerExist(_ schema.GroupVersionResource, _ cache.ResourceEventHandler) bool {
	return m.handlerExists
}

// WaitForCacheSync waits for the cache to sync.
func (m *FakeSingleClusterManager) WaitForCacheSync() map[schema.GroupVersionResource]bool {
	return nil
}

// WaitForCacheSyncWithTimeout waits for the cache to sync with a timeout.
func (m *FakeSingleClusterManager) WaitForCacheSyncWithTimeout(_ time.Duration) map[schema.GroupVersionResource]bool {
	return nil
}

// Context returns the context for the informer.
func (m *FakeSingleClusterManager) Context() context.Context {
	return context.TODO()
}

// GetClient returns the client for the informer.
func (m *FakeSingleClusterManager) GetClient() dynamic.Interface {
	return nil
}

// FakeMultiClusterInformerManager implements MultiClusterInformerManager interface.
// It is used to simulate MultiClusterInformerManager for testing.
type FakeMultiClusterInformerManager struct {
	managers map[string]genericmanager.SingleClusterInformerManager
}

// NewFakeMultiClusterInformerManager creates a new FakeMultiClusterInformerManager.
func NewFakeMultiClusterInformerManager(managers map[string]genericmanager.SingleClusterInformerManager) *FakeMultiClusterInformerManager {
	return &FakeMultiClusterInformerManager{
		managers: managers,
	}
}

// GetSingleClusterManager returns the single cluster manager for the given cluster name.
func (m *FakeMultiClusterInformerManager) GetSingleClusterManager(clusterName string) genericmanager.SingleClusterInformerManager {
	if m.managers == nil {
		return nil
	}
	return m.managers[clusterName]
}

// ForCluster adds a cluster event handler to the informer.
func (m *FakeMultiClusterInformerManager) ForCluster(_ string, _ dynamic.Interface, _ time.Duration) genericmanager.SingleClusterInformerManager {
	return nil
}

// ForClusterWithUID adds a cluster event handler to the informer with UID tracking.
func (m *FakeMultiClusterInformerManager) ForClusterWithUID(_ string, _ types.UID, _ dynamic.Interface, _ time.Duration) genericmanager.SingleClusterInformerManager {
	return nil
}

// Start starts the informer.
func (m *FakeMultiClusterInformerManager) Start(_ string) {}

// Stop stops the informer.
func (m *FakeMultiClusterInformerManager) Stop(_ string) {}

// IsManagerExist returns true if the manager exists for the given cluster name.
func (m *FakeMultiClusterInformerManager) IsManagerExist(_ string) bool { return false }

// IsManagerExistWithUID returns true if the manager exists for the given cluster name with the specified UID.
func (m *FakeMultiClusterInformerManager) IsManagerExistWithUID(_ string, _ types.UID) bool { return false }

// WaitForCacheSync waits for the cache to sync.
func (m *FakeMultiClusterInformerManager) WaitForCacheSync(_ string) map[schema.GroupVersionResource]bool {
	return nil
}

// WaitForCacheSyncWithTimeout waits for the cache to sync with a timeout.
func (m *FakeMultiClusterInformerManager) WaitForCacheSyncWithTimeout(_ string, _ time.Duration) map[schema.GroupVersionResource]bool {
	return nil
}
