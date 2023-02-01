package typedmanager

import (
	"context"
	"reflect"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/util"
)

var (
	nodeGVR    = corev1.SchemeGroupVersion.WithResource("nodes")
	podGVR     = corev1.SchemeGroupVersion.WithResource("pods")
	gvrTypeMap = map[reflect.Type]schema.GroupVersionResource{
		reflect.TypeOf(&corev1.Node{}): nodeGVR,
		reflect.TypeOf(&corev1.Pod{}):  podGVR,
	}
)

// SingleClusterInformerManager manages typed shared informer for all resources, include Kubernetes resource and
// custom resources defined by CustomResourceDefinition.
type SingleClusterInformerManager interface {
	// ForResource builds a typed shared informer for 'resource' then set event handler.
	// If the informer already exist, the event handler will be appended to the informer.
	// The handler should not be nil.
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
	Lister(resource schema.GroupVersionResource) (interface{}, error)

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
func NewSingleClusterInformerManager(client kubernetes.Interface, defaultResync time.Duration, parentCh <-chan struct{}, transformFuncs map[schema.GroupVersionResource]cache.TransformFunc) SingleClusterInformerManager {
	ctx, cancel := util.ContextForChannel(parentCh)
	return &singleClusterInformerManagerImpl{
		informerFactory:  informers.NewSharedInformerFactory(client, defaultResync),
		handlers:         make(map[schema.GroupVersionResource][]cache.ResourceEventHandler),
		syncedInformers:  make(map[schema.GroupVersionResource]struct{}),
		informers:        make(map[schema.GroupVersionResource]struct{}),
		startedInformers: make(map[schema.GroupVersionResource]struct{}),
		transformFuncs:   transformFuncs,
		ctx:              ctx,
		cancel:           cancel,
		client:           client,
	}
}

type singleClusterInformerManagerImpl struct {
	ctx    context.Context
	cancel context.CancelFunc

	informerFactory informers.SharedInformerFactory

	syncedInformers map[schema.GroupVersionResource]struct{}

	informers map[schema.GroupVersionResource]struct{}

	startedInformers map[schema.GroupVersionResource]struct{}

	handlers map[schema.GroupVersionResource][]cache.ResourceEventHandler

	transformFuncs map[schema.GroupVersionResource]cache.TransformFunc

	client kubernetes.Interface

	lock sync.RWMutex
}

func (s *singleClusterInformerManagerImpl) ForResource(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) error {
	// if handler already exist, just return, nothing changed.
	if s.IsHandlerExist(resource, handler) {
		return nil
	}

	resourceInformer, err := s.informerFactory.ForResource(resource)
	if err != nil {
		return err
	}

	s.lock.Lock()
	if _, exist := s.informers[resource]; !exist {
		s.informers[resource] = struct{}{}
	}
	s.lock.Unlock()

	s.lock.RLock()
	if resourceTransformFunc, ok := s.transformFuncs[resource]; ok && !s.isInformerStarted(resource) {
		err = resourceInformer.Informer().SetTransform(resourceTransformFunc)
		if err != nil {
			return err
		}
	}
	s.lock.RUnlock()

	_, err = resourceInformer.Informer().AddEventHandler(handler)
	if err != nil {
		klog.Errorf("Failed to add handler for resource(%s): %v", resource.String(), err)
		return err
	}

	s.appendHandler(resource, handler)
	return nil
}

func (s *singleClusterInformerManagerImpl) IsInformerSynced(resource schema.GroupVersionResource) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	_, exist := s.syncedInformers[resource]
	return exist
}

func (s *singleClusterInformerManagerImpl) isInformerStarted(resource schema.GroupVersionResource) bool {
	_, exist := s.startedInformers[resource]
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

func (s *singleClusterInformerManagerImpl) Lister(resource schema.GroupVersionResource) (interface{}, error) {
	resourceInformer, err := s.informerFactory.ForResource(resource)
	if err != nil {
		return nil, err
	}

	s.lock.Lock()
	if _, exist := s.informers[resource]; !exist {
		s.informers[resource] = struct{}{}
	}
	s.lock.Unlock()

	s.lock.RLock()
	if resourceTransformFunc, ok := s.transformFuncs[resource]; ok && !s.isInformerStarted(resource) {
		err = resourceInformer.Informer().SetTransform(resourceTransformFunc)
		if err != nil {
			return nil, err
		}
	}
	s.lock.RUnlock()

	if resource == nodeGVR {
		return s.informerFactory.Core().V1().Nodes().Lister(), nil
	}
	if resource == podGVR {
		return s.informerFactory.Core().V1().Pods().Lister(), nil
	}

	return resourceInformer.Lister(), nil
}

func (s *singleClusterInformerManagerImpl) appendHandler(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// assume the handler list exist, caller should ensure for that.
	handlers := s.handlers[resource]

	// assume the handler not exist in it, caller should ensure for that.
	s.handlers[resource] = append(handlers, handler)
}

func (s *singleClusterInformerManagerImpl) Start() {
	s.lock.Lock()
	s.informerFactory.Start(s.ctx.Done())
	for resource := range s.informers {
		if _, exist := s.startedInformers[resource]; !exist {
			s.startedInformers[resource] = struct{}{}
		}
	}
	s.lock.Unlock()
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
	m := make(map[schema.GroupVersionResource]bool)
	for resource, synced := range res {
		if gvr, exist := gvrTypeMap[resource]; exist {
			m[gvr] = synced
			if _, exist = s.syncedInformers[gvr]; !exist && synced {
				s.syncedInformers[gvr] = struct{}{}
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
