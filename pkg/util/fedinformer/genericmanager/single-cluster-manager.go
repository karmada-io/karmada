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
	"slices"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// SingleClusterInformerManager manages dynamic shared informer for all resources, include Kubernetes resource and
// custom resources defined by CustomResourceDefinition.
type SingleClusterInformerManager interface {
	// ForResource builds a dynamic shared informer for 'resource' then set event handler.
	// If the informer already exist, the event handler will be appended to the informer.
	// The handler should not be nil.
	ForResource(resource schema.GroupVersionResource, handler cache.ResourceEventHandler)

	// IsInformerSynced checks if the resource's informer is synced.
	// An informer is synced means:
	// - The informer has been created(by method 'ForResource' or 'Lister').
	// - The informer has started(by method 'Start').
	// - The informer's cache has been synced.
	IsInformerSynced(resource schema.GroupVersionResource) bool

	// IsHandlerExist checks if handler already added to the informer that watches the 'resource'.
	IsHandlerExist(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) bool

	// Lister returns a generic lister used to get 'resource' from informer's store.
	// The informer for 'resource' will be created if not exist, but without any event handler.
	Lister(resource schema.GroupVersionResource) cache.GenericLister

	// Start will run all informers, the informers will keep running until the channel closed.
	// It is intended to be called after create new informer(s), and it's safe to call multi times.
	Start()

	// Stop stops all single cluster informers of a cluster. Once it is stopped, it will be not able
	// to Start again.
	Stop()

	// WaitForCacheSync waits for all caches to populate.
	WaitForCacheSync() map[schema.GroupVersionResource]bool

	// WaitForCacheSyncWithTimeout waits for all caches to populate with a definitive timeout.
	WaitForCacheSyncWithTimeout(cacheSyncTimeout time.Duration) map[schema.GroupVersionResource]bool

	// Context returns the single cluster context.
	Context() context.Context

	// GetClient returns the dynamic client.
	GetClient() dynamic.Interface
}

// NewSingleClusterInformerManager constructs a new instance of singleClusterInformerManagerImpl.
// defaultResync with value '0' means no re-sync.
func NewSingleClusterInformerManager(ctx context.Context, client dynamic.Interface, defaultResync time.Duration) SingleClusterInformerManager {
	ctx, cancel := context.WithCancel(ctx)
	return &singleClusterInformerManagerImpl{
		informerFactory: dynamicinformer.NewDynamicSharedInformerFactory(client, defaultResync),
		ctx:             ctx,
		cancel:          cancel,
		client:          client,
	}
}

type singleClusterInformerManagerImpl struct {
	ctx    context.Context
	cancel context.CancelFunc

	informerFactory dynamicinformer.DynamicSharedInformerFactory

	// initializedInformers caches the informers that have been created by the
	// informer factory, used to avoid the lock contention in the informer factory
	// when getting an informer for a resource.
	// The key is schema.GroupVersionResource, and the value is informers.GenericInformer.
	initializedInformers sync.Map

	// syncedInformers records the resources whose informer caches have been synced,
	// used to quickly answer IsInformerSynced without waiting for cache sync again.
	// The key is schema.GroupVersionResource, and the value is struct{}{} (a placeholder).
	syncedInformers sync.Map

	// handlers records the event handlers that have been added to each resource's
	// informer, used to prevent the same handler from being added more than once.
	// The key is schema.GroupVersionResource, and the value is []cache.ResourceEventHandler.
	handlers sync.Map

	client dynamic.Interface

	lock sync.Mutex
}

func (s *singleClusterInformerManagerImpl) ForResource(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) {
	// if handler already exist, just return, nothing changed.
	if s.isHandlerExist(resource, handler) {
		return
	}

	resourceInformer := s.getOrCreateInformer(resource)

	s.lock.Lock()
	defer s.lock.Unlock()

	// check again, if handler already exist, just return, nothing changed.
	if s.isHandlerExist(resource, handler) {
		return
	}

	_, err := resourceInformer.Informer().AddEventHandler(handler)
	if err != nil {
		klog.Errorf("Failed to add handler for resource(%s): %v", resource.String(), err)
		return
	}

	s.appendHandler(resource, handler)
}

func (s *singleClusterInformerManagerImpl) IsInformerSynced(resource schema.GroupVersionResource) bool {
	_, exist := s.syncedInformers.Load(resource)
	return exist
}

func (s *singleClusterInformerManagerImpl) IsHandlerExist(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) bool {
	return s.isHandlerExist(resource, handler)
}

func (s *singleClusterInformerManagerImpl) isHandlerExist(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) bool {
	handlers, ok := s.handlers.Load(resource)
	if !ok {
		return false
	}
	return slices.Contains(handlers.([]cache.ResourceEventHandler), handler)
}

func (s *singleClusterInformerManagerImpl) Lister(resource schema.GroupVersionResource) cache.GenericLister {
	return s.getOrCreateInformer(resource).Lister()
}

func (s *singleClusterInformerManagerImpl) appendHandler(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) {
	var handlers []cache.ResourceEventHandler
	if currentHandlers, ok := s.handlers.Load(resource); ok {
		handlers = currentHandlers.([]cache.ResourceEventHandler)
	}
	// Publish a new slice so concurrent lock-free readers never observe a slice being modified.
	s.handlers.Store(resource, append(slices.Clone(handlers), handler))
}

// getOrCreateInformer returns the informer for the given resource, creating it
// if it doesn't exist yet. It first tries a lock-free lookup in initializedInformers,
// and creates the informer if not found.
func (s *singleClusterInformerManagerImpl) getOrCreateInformer(resource schema.GroupVersionResource) informers.GenericInformer {
	if resourceInformer, exists := s.initializedInformers.Load(resource); exists {
		return resourceInformer.(informers.GenericInformer)
	}
	return s.createInformer(resource)
}

// createInformer creates the informer for the given resource and records it
// in initializedInformers.
func (s *singleClusterInformerManagerImpl) createInformer(resource schema.GroupVersionResource) informers.GenericInformer {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Recheck after acquiring the lock, as another caller may have created
	// the informer while this caller was waiting for the lock.
	if resourceInformer, exists := s.initializedInformers.Load(resource); exists {
		return resourceInformer.(informers.GenericInformer)
	}

	resourceInformer := s.informerFactory.ForResource(resource)
	s.initializedInformers.Store(resource, resourceInformer)
	return resourceInformer
}

func (s *singleClusterInformerManagerImpl) Start() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.informerFactory.Start(s.ctx.Done())
}

func (s *singleClusterInformerManagerImpl) Stop() {
	s.cancel()
}

func (s *singleClusterInformerManagerImpl) WaitForCacheSync() map[schema.GroupVersionResource]bool {
	return s.waitForCacheSync(s.ctx)
}

func (s *singleClusterInformerManagerImpl) WaitForCacheSyncWithTimeout(cacheSyncTimeout time.Duration) map[schema.GroupVersionResource]bool {
	ctx, cancel := context.WithTimeout(s.ctx, cacheSyncTimeout)
	defer cancel()

	return s.waitForCacheSync(ctx)
}

func (s *singleClusterInformerManagerImpl) waitForCacheSync(ctx context.Context) map[schema.GroupVersionResource]bool {
	res := s.informerFactory.WaitForCacheSync(ctx.Done())

	var newlySynced []schema.GroupVersionResource
	for resource, synced := range res {
		if !synced {
			continue
		}
		if _, ok := s.syncedInformers.Load(resource); !ok {
			newlySynced = append(newlySynced, resource)
		}
	}

	if len(newlySynced) == 0 {
		return res
	}

	for _, resource := range newlySynced {
		s.syncedInformers.Store(resource, struct{}{})
	}
	return res
}

func (s *singleClusterInformerManagerImpl) Context() context.Context {
	return s.ctx
}

func (s *singleClusterInformerManagerImpl) GetClient() dynamic.Interface {
	return s.client
}
