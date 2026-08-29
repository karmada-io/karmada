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
	"fmt"
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
func NewSingleClusterInformerManager(ctx context.Context, client dynamic.Interface, defaultResync time.Duration, transformFunc cache.TransformFunc) SingleClusterInformerManager {
	ctx, cancel := context.WithCancel(ctx)
	return &singleClusterInformerManagerImpl{
		informerFactory: dynamicinformer.NewDynamicSharedInformerFactory(client, defaultResync),
		handlers:        make(map[schema.GroupVersionResource][]cache.ResourceEventHandler),
		syncedInformers: make(map[schema.GroupVersionResource]struct{}),
		transformFunc:   transformFunc,
		ctx:             ctx,
		cancel:          cancel,
		client:          client,
	}
}

type singleClusterInformerManagerImpl struct {
	ctx    context.Context
	cancel context.CancelFunc

	informerFactory dynamicinformer.DynamicSharedInformerFactory

	syncedInformers map[schema.GroupVersionResource]struct{}

	handlers map[schema.GroupVersionResource][]cache.ResourceEventHandler

	transformFunc cache.TransformFunc

	// initializedInformers contains GenericInformers whose transform has been installed. It also keeps
	// steady-state informer lookups from taking informerFactory's lock for an already known resource.
	initializedInformers sync.Map

	// informerLifecycleLock serializes the first initialization of an informer with Start.
	// Cache hits in initializedInformers do not take this lock.
	informerLifecycleLock sync.Mutex

	client dynamic.Interface

	lock sync.RWMutex
}

func (s *singleClusterInformerManagerImpl) ForResource(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// if handler already exist, just return, nothing changed.
	if s.isHandlerExist(resource, handler) {
		return
	}

	resourceInformer, err := s.informerForResource(resource)
	if err != nil {
		klog.ErrorS(err, "Failed to initialize informer", "resource", resource.String())
	}

	_, err = resourceInformer.Informer().AddEventHandler(handler)
	if err != nil {
		klog.Errorf("Failed to add handler for resource(%s): %v", resource.String(), err)
		return
	}

	s.appendHandler(resource, handler)
}

func (s *singleClusterInformerManagerImpl) IsInformerSynced(resource schema.GroupVersionResource) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	_, exist := s.syncedInformers[resource]
	return exist
}

func (s *singleClusterInformerManagerImpl) IsHandlerExist(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.isHandlerExist(resource, handler)
}

func (s *singleClusterInformerManagerImpl) isHandlerExist(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) bool {
	handlers, exist := s.handlers[resource]
	if !exist {
		return false
	}

	return slices.Contains(handlers, handler)
}

func (s *singleClusterInformerManagerImpl) Lister(resource schema.GroupVersionResource) cache.GenericLister {
	resourceInformer, err := s.informerForResource(resource)
	if err != nil {
		klog.ErrorS(err, "Failed to initialize informer", "resource", resource.String())
	}
	return resourceInformer.Lister()
}

func (s *singleClusterInformerManagerImpl) appendHandler(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) {
	// assume the handler list exist, caller should ensure for that.
	handlers := s.handlers[resource]

	// assume the handler not exist in it, caller should ensure for that.
	s.handlers[resource] = append(handlers, handler)
}

func (s *singleClusterInformerManagerImpl) informerForResource(resource schema.GroupVersionResource) (informers.GenericInformer, error) {
	if resourceInformer, exists := s.initializedInformers.Load(resource); exists {
		return resourceInformer.(informers.GenericInformer), nil
	}
	return s.informerForResourceSlowPath(resource)
}

// informerForResourceSlowPath handles the cache-miss path. It rechecks initializedInformers after acquiring
// informerLifecycleLock because another caller may have initialized the informer while this caller was waiting.
func (s *singleClusterInformerManagerImpl) informerForResourceSlowPath(resource schema.GroupVersionResource) (informers.GenericInformer, error) {
	s.informerLifecycleLock.Lock()
	defer s.informerLifecycleLock.Unlock()

	if resourceInformer, exists := s.initializedInformers.Load(resource); exists {
		return resourceInformer.(informers.GenericInformer), nil
	}

	resourceInformer := s.informerFactory.ForResource(resource)
	informer := resourceInformer.Informer()
	if s.transformFunc != nil {
		if err := informer.SetTransform(s.transformFunc); err != nil {
			return resourceInformer, fmt.Errorf("failed to set transform for resource %s: %w", resource.String(), err)
		}
	}

	s.initializedInformers.Store(resource, resourceInformer)
	return resourceInformer, nil
}

func (s *singleClusterInformerManagerImpl) Start() {
	s.informerLifecycleLock.Lock()
	defer s.informerLifecycleLock.Unlock()

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

	s.lock.RLock()
	for resource, synced := range res {
		if !synced {
			continue
		}
		if _, exists := s.syncedInformers[resource]; !exists {
			newlySynced = append(newlySynced, resource)
		}
	}
	s.lock.RUnlock()

	if len(newlySynced) == 0 {
		return res
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	for _, resource := range newlySynced {
		s.syncedInformers[resource] = struct{}{}
	}
	return res
}

func (s *singleClusterInformerManagerImpl) Context() context.Context {
	return s.ctx
}

func (s *singleClusterInformerManagerImpl) GetClient() dynamic.Interface {
	return s.client
}
