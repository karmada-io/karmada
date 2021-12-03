package informermanager

import (
	"context"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

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
	// A informer is synced means:
	// - The informer has been created(by method 'ForResource' or 'Lister').
	// - The informer has started(by method 'Start').
	// - The informer's cache has been synced.
	IsInformerSynced(resource schema.GroupVersionResource) bool

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
}

// NewSingleClusterInformerManager constructs a new instance of singleClusterInformerManagerImpl.
// defaultResync with value '0' means no re-sync.
func NewSingleClusterInformerManager(client dynamic.Interface, defaultResync time.Duration, parentCh <-chan struct{}) SingleClusterInformerManager {
	ctx, cancel := util.ContextForChannel(parentCh)
	return &singleClusterInformerManagerImpl{
		informerFactory: dynamicinformer.NewDynamicSharedInformerFactory(client, defaultResync),
		handlers:        make(map[schema.GroupVersionResource][]cache.ResourceEventHandler),
		syncedInformers: mapset.NewSet(),
		ctx:             ctx,
		cancel:          cancel,
	}
}

type singleClusterInformerManagerImpl struct {
	ctx    context.Context
	cancel context.CancelFunc

	informerFactory dynamicinformer.DynamicSharedInformerFactory

	syncedInformers mapset.Set

	handlers map[schema.GroupVersionResource][]cache.ResourceEventHandler

	lock sync.RWMutex
}

func (s *singleClusterInformerManagerImpl) ForResource(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	// if handler already exist, just return, nothing changed.
	for _, h := range s.handlers[resource] {
		if h == handler {
			return
		}
	}

	s.informerFactory.ForResource(resource).Informer().AddEventHandler(handler)

	s.handlers[resource] = append(s.handlers[resource], handler)
}

func (s *singleClusterInformerManagerImpl) IsInformerSynced(resource schema.GroupVersionResource) bool {
	return s.syncedInformers.Contains(resource)
}

func (s *singleClusterInformerManagerImpl) Lister(resource schema.GroupVersionResource) cache.GenericLister {
	return s.informerFactory.ForResource(resource).Lister()
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
	res := s.informerFactory.WaitForCacheSync(ctx.Done())
	for resource, synced := range res {
		if !s.IsInformerSynced(resource) && synced {
			s.syncedInformers.Add(resource)
		}
	}
	return res
}

func (s *singleClusterInformerManagerImpl) Context() context.Context {
	return s.ctx
}
