package informermanager

import (
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
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

	// IsHandlerExist checks if handler already added to a the informer that watches the 'resource'.
	IsHandlerExist(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) bool

	// Lister returns a generic lister used to get 'resource' from informer's store.
	// The informer for 'resource' will be created if not exist, but without any event handler.
	Lister(resource schema.GroupVersionResource) cache.GenericLister

	// Start will run all informers with a stop channel, the informers will keep running until channel closed.
	// It is intended to be called after create new informer(s), and it's safe to call multi times.
	Start(stopCh <-chan struct{})

	// WaitForCacheSync waits for all caches to populate.
	WaitForCacheSync(stopCh <-chan struct{}) map[schema.GroupVersionResource]bool
}

// NewSingleClusterInformerManager constructs a new instance of singleClusterInformerManagerImpl.
// defaultResync with value '0' means no re-sync.
func NewSingleClusterInformerManager(client dynamic.Interface, defaultResync time.Duration) SingleClusterInformerManager {
	return &singleClusterInformerManagerImpl{
		informerFactory: dynamicinformer.NewDynamicSharedInformerFactory(client, defaultResync),
		handlers:        make(map[schema.GroupVersionResource][]cache.ResourceEventHandler),
		syncedInformers: make(map[schema.GroupVersionResource]struct{}),
	}
}

type singleClusterInformerManagerImpl struct {
	informerFactory dynamicinformer.DynamicSharedInformerFactory

	syncedInformers map[schema.GroupVersionResource]struct{}

	handlers map[schema.GroupVersionResource][]cache.ResourceEventHandler

	lock sync.RWMutex
}

func (s *singleClusterInformerManagerImpl) ForResource(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) {
	// if handler already exist, just return, nothing changed.
	if s.IsHandlerExist(resource, handler) {
		return
	}

	s.informerFactory.ForResource(resource).Informer().AddEventHandler(handler)
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
	s.lock.Lock()
	defer s.lock.Unlock()

	// assume the handler list exist, caller should ensure for that.
	handlers := s.handlers[resource]

	// assume the handler not exist in it, caller should ensure for that.
	s.handlers[resource] = append(handlers, handler)
}

func (s *singleClusterInformerManagerImpl) Start(stopCh <-chan struct{}) {
	s.informerFactory.Start(stopCh)
}

func (s *singleClusterInformerManagerImpl) WaitForCacheSync(stopCh <-chan struct{}) map[schema.GroupVersionResource]bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	res := s.informerFactory.WaitForCacheSync(stopCh)
	for resource, synced := range res {
		if _, exist := s.syncedInformers[resource]; !exist && synced {
			s.syncedInformers[resource] = struct{}{}
		}
	}
	return res
}
