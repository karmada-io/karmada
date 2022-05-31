package backendstore

import (
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	klog "k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
)

// BackendStore define BackendStore interface
type BackendStore interface {
	ResourceEventHandlerFuncs() cache.ResourceEventHandler
	Close()
}

var (
	backendLock sync.Mutex
	backends    map[string]BackendStore
	k8sClient   *kubernetes.Clientset
)

// Init init backend store manager
func Init(cs *kubernetes.Clientset) {
	backendLock.Lock()
	backends = make(map[string]BackendStore)
	k8sClient = cs
	backendLock.Unlock()
}

// AddBackend add backend store
func AddBackend(cluster string, cfg *v1alpha1.BackendStoreConfig) {
	backendLock.Lock()
	defer backendLock.Unlock()

	if cfg == nil || cfg.OpenSearch == nil {
		backends[cluster] = NewDefaultBackend(cluster)
		return
	}

	bs, err := NewOpenSearch(cluster, cfg)
	if err != nil {
		klog.Errorf("cannot create backend store %s: %v, use default backend store", cluster, err)
		return
	}
	backends[cluster] = bs
}

// DeleteBackend delete backend store
func DeleteBackend(cluster string) {
	backendLock.Lock()
	defer backendLock.Unlock()

	bs, ok := backends[cluster]
	if ok {
		bs.Close()
		delete(backends, cluster)
	}
}

// GetBackend get backend store
func GetBackend(cluster string) BackendStore {
	backendLock.Lock()
	defer backendLock.Unlock()
	bs, ok := backends[cluster]
	if !ok {
		klog.Errorf("cannot find backend store %s, use defult backend", cluster)
		return NewDefaultBackend(cluster)
	}
	return bs
}
