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
	"fmt"
	"reflect"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

var (
	nodeGVR    = corev1.SchemeGroupVersion.WithResource("nodes")
	podGVR     = corev1.SchemeGroupVersion.WithResource("pods")
	gvrTypeMap = map[reflect.Type]schema.GroupVersionResource{
		reflect.TypeFor[*corev1.Node](): nodeGVR,
		reflect.TypeFor[*corev1.Pod]():  podGVR,
	}
)

// SingleClusterInformerManager manages typed shared informer for all resources, include Kubernetes resource and
// custom resources defined by CustomResourceDefinition.
type SingleClusterInformerManager interface {
	// ForResource builds a typed shared informer for 'resource' then set event handler.
	// If the informer already exist, the event handler will be appended to the informer.
	// The handler must be a non-nil pointer.
	ForResource(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) error

	// IsInformerSynced checks if the resource's informer is synced.
	// An informer is synced means:
	// - The informer has been created(by method 'ForResource' or 'Lister').
	// - The informer has started(by method 'Start').
	// - The informer's cache has been synced.
	IsInformerSynced(resource schema.GroupVersionResource) bool

	// IsHandlerExist checks if handler already added to the informer that watches the 'resource'.
	IsHandlerExist(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) bool

	// Lister returns a lister used to get 'resource' from informer's store.
	// The informer for 'resource' will be created if not exist, but without any event handler.
	Lister(resource schema.GroupVersionResource) (any, error)

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

	// GetClient returns the typed client.
	GetClient() kubernetes.Interface
}

// NewSingleClusterInformerManager constructs a new instance of singleClusterInformerManagerImpl.
// defaultResync with value '0' means no re-sync.
func NewSingleClusterInformerManager(ctx context.Context, client kubernetes.Interface, defaultResync time.Duration, transformFuncs map[schema.GroupVersionResource]cache.TransformFunc) SingleClusterInformerManager {
	ctx, cancel := context.WithCancel(ctx)
	return &singleClusterInformerManagerImpl{
		informerFactory: informers.NewSharedInformerFactory(client, defaultResync),
		transformFuncs:  transformFuncs,
		ctx:             ctx,
		cancel:          cancel,
		client:          client,
	}
}

type singleClusterInformerManagerImpl struct {
	ctx    context.Context
	cancel context.CancelFunc

	informerFactory informers.SharedInformerFactory

	transformFuncs map[schema.GroupVersionResource]cache.TransformFunc

	// initializedInformers contains informers whose transform has been installed. It also keeps
	// steady-state informer lookups from taking informerFactory's lock for an already known resource.
	initializedInformers sync.Map
	syncedInformers      sync.Map
	handlers             sync.Map

	client kubernetes.Interface

	lock sync.Mutex
}

func (s *singleClusterInformerManagerImpl) ForResource(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) error {
	if err := validateResourceEventHandler(handler); err != nil {
		return err
	}

	// if handler already exist, just return, nothing changed.
	if s.isHandlerExist(resource, handler) {
		return nil
	}

	resourceInformer, err := s.informerForResource(resource)
	if err != nil {
		klog.ErrorS(err, "Failed to initialize informer", "resource", resource.String())
		return err
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	// check again, if handler already exist, just return, nothing changed.
	if s.isHandlerExist(resource, handler) {
		return nil
	}

	_, err = resourceInformer.Informer().AddEventHandler(handler)
	if err != nil {
		klog.Errorf("Failed to add handler for resource(%s): %v", resource.String(), err)
		return err
	}

	s.appendHandler(resource, handler)
	return nil
}

func (s *singleClusterInformerManagerImpl) IsInformerSynced(resource schema.GroupVersionResource) bool {
	_, exist := s.syncedInformers.Load(resource)
	return exist
}

func (s *singleClusterInformerManagerImpl) IsHandlerExist(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) bool {
	if validateResourceEventHandler(handler) != nil {
		return false
	}
	return s.isHandlerExist(resource, handler)
}

func validateResourceEventHandler(handler cache.ResourceEventHandler) error {
	handlerValue := reflect.ValueOf(handler)
	if handlerValue.Kind() != reflect.Pointer || handlerValue.IsNil() {
		return fmt.Errorf("resource event handler must be a non-nil pointer, got %T", handler)
	}
	return nil
}

func (s *singleClusterInformerManagerImpl) isHandlerExist(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) bool {
	handlers, ok := s.handlers.Load(resource)
	if !ok {
		return false
	}
	_, ok = handlers.(*sync.Map).Load(handler)
	return ok
}

func (s *singleClusterInformerManagerImpl) Lister(resource schema.GroupVersionResource) (any, error) {
	resourceInformer, err := s.informerForResource(resource)
	if err != nil {
		return nil, err
	}

	if resource == nodeGVR {
		return s.informerFactory.Core().V1().Nodes().Lister(), nil
	}
	if resource == podGVR {
		return s.informerFactory.Core().V1().Pods().Lister(), nil
	}

	return resourceInformer.Lister(), nil
}

func (s *singleClusterInformerManagerImpl) appendHandler(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) {
	handlers, _ := s.handlers.LoadOrStore(resource, &sync.Map{})
	handlers.(*sync.Map).Store(handler, struct{}{})
}

func (s *singleClusterInformerManagerImpl) informerForResource(resource schema.GroupVersionResource) (informers.GenericInformer, error) {
	if resourceInformer, exists := s.initializedInformers.Load(resource); exists {
		return resourceInformer.(informers.GenericInformer), nil
	}
	return s.informerForResourceSlowPath(resource)
}

// informerForResourceSlowPath handles the cache-miss path. It rechecks initializedInformers after acquiring
// lock because another caller may have initialized the informer while this caller was waiting.
func (s *singleClusterInformerManagerImpl) informerForResourceSlowPath(resource schema.GroupVersionResource) (informers.GenericInformer, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if resourceInformer, exists := s.initializedInformers.Load(resource); exists {
		return resourceInformer.(informers.GenericInformer), nil
	}

	resourceInformer, err := s.informerFactory.ForResource(resource)
	if err != nil {
		return nil, err
	}
	if resourceTransformFunc, ok := s.transformFuncs[resource]; ok {
		if err := resourceInformer.Informer().SetTransform(resourceTransformFunc); err != nil {
			return resourceInformer, fmt.Errorf("failed to set transform for resource %s: %w", resource.String(), err)
		}
	}

	s.initializedInformers.Store(resource, resourceInformer)
	return resourceInformer, nil
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
	m := make(map[schema.GroupVersionResource]bool)
	for resource, synced := range res {
		if gvr, exist := gvrTypeMap[resource]; exist {
			m[gvr] = synced
			if synced {
				s.syncedInformers.Store(gvr, struct{}{})
			}
		}
	}
	return m
}

func (s *singleClusterInformerManagerImpl) Context() context.Context {
	return s.ctx
}

func (s *singleClusterInformerManagerImpl) GetClient() kubernetes.Interface {
	return s.client
}
