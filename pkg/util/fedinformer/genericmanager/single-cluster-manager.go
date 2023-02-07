package genericmanager

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/util"
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
func NewSingleClusterInformerManager(client dynamic.Interface, defaultResync time.Duration, parentCh <-chan struct{}) SingleClusterInformerManager {
	ctx, cancel := util.ContextForChannel(parentCh)
	return &singleClusterInformerManagerImpl{
		informerFactory: dynamicinformer.NewDynamicSharedInformerFactory(client, defaultResync),
		handlers:        make(map[schema.GroupVersionResource][]cache.ResourceEventHandler),
		syncedInformers: make(map[schema.GroupVersionResource]struct{}),
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

	_, err := s.informerFactory.ForResource(resource).Informer().AddEventHandler(handler)
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

	for _, h := range handlers {
		if h == handler {
			return true
		}
	}

	return false
}

func (s *singleClusterInformerManagerImpl) Lister(resource schema.GroupVersionResource) cache.GenericLister {
	return s.informerFactory.ForResource(resource).Lister()
}

func (s *singleClusterInformerManagerImpl) appendHandler(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) {
	// assume the handler list exist, caller should ensure for that.
	handlers := s.handlers[resource]

	// assume the handler not exist in it, caller should ensure for that.
	s.handlers[resource] = append(handlers, handler)
}

func (s *singleClusterInformerManagerImpl) Start() {
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
	s.lock.Lock()
	defer s.lock.Unlock()
	res := s.informerFactory.WaitForCacheSync(ctx.Done())
	for resource, synced := range res {
		if _, exist := s.syncedInformers[resource]; !exist && synced {
			s.syncedInformers[resource] = struct{}{}
		}
	}
	return res
}

func (s *singleClusterInformerManagerImpl) Context() context.Context {
	return s.ctx
}

func (s *singleClusterInformerManagerImpl) GetClient() dynamic.Interface {
	return s.client
}
