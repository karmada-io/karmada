/*
Copyright 2026 The Karmada Authors.

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

package lister

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// SyncWaitFunc is a function that blocks until the cache is synced.
// This allows the syncedLister to be decoupled from specific manager implementations.
type SyncWaitFunc func() error

// NewSyncedLister wraps a cache.GenericLister with sync waiting capability.
// The waitForSync function will be called before each List/Get operation
// to ensure the cache is synced before returning data.
//
// This design avoids blocking on Lister() call which could cause deadlock
// if called before informer is started, while still ensuring data consistency
// by waiting for sync before actual data access.
func NewSyncedLister(lister cache.GenericLister, waitForSync SyncWaitFunc) cache.GenericLister {
	return &syncedLister{
		lister:      lister,
		waitForSync: waitForSync,
	}
}

// syncedLister wraps a cache.GenericLister and ensures the cache is synced
// before actually performing List/Get operations.
type syncedLister struct {
	lister      cache.GenericLister
	waitForSync SyncWaitFunc
}

// List returns all objects matching the selector after ensuring cache is synced.
func (l *syncedLister) List(selector labels.Selector) ([]runtime.Object, error) {
	if err := l.waitForSync(); err != nil {
		return nil, err
	}
	return l.lister.List(selector)
}

// Get returns the object with the given name after ensuring cache is synced.
func (l *syncedLister) Get(name string) (runtime.Object, error) {
	if err := l.waitForSync(); err != nil {
		return nil, err
	}
	return l.lister.Get(name)
}

// ByNamespace returns a GenericNamespaceLister that lists/gets objects in the given namespace.
func (l *syncedLister) ByNamespace(namespace string) cache.GenericNamespaceLister {
	return &syncedNamespaceLister{
		namespaceLister: l.lister.ByNamespace(namespace),
		waitForSync:     l.waitForSync,
	}
}

// syncedNamespaceLister wraps a cache.GenericNamespaceLister and ensures the cache
// is synced before actually performing List/Get operations.
type syncedNamespaceLister struct {
	namespaceLister cache.GenericNamespaceLister
	waitForSync     SyncWaitFunc
}

// List returns all objects in the namespace matching the selector after ensuring cache is synced.
func (l *syncedNamespaceLister) List(selector labels.Selector) ([]runtime.Object, error) {
	if err := l.waitForSync(); err != nil {
		return nil, err
	}
	return l.namespaceLister.List(selector)
}

// Get returns the object with the given name in the namespace after ensuring cache is synced.
func (l *syncedNamespaceLister) Get(name string) (runtime.Object, error) {
	if err := l.waitForSync(); err != nil {
		return nil, err
	}
	return l.namespaceLister.Get(name)
}
