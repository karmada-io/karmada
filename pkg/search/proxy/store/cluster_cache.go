package store

import (
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

// clusterCache caches resources for single member cluster
type clusterCache struct {
	lock        sync.RWMutex
	cache       map[schema.GroupVersionResource]*resourceCache
	clusterName string
	restMapper  meta.RESTMapper
	// newClientFunc returns a dynamic client for member cluster apiserver
	newClientFunc func() (dynamic.Interface, error)
}

func newClusterCache(clusterName string, newClientFunc func() (dynamic.Interface, error), restMapper meta.RESTMapper) *clusterCache {
	return &clusterCache{
		clusterName:   clusterName,
		newClientFunc: newClientFunc,
		restMapper:    restMapper,
		cache:         map[schema.GroupVersionResource]*resourceCache{},
	}
}

func (c *clusterCache) updateCache(resources map[schema.GroupVersionResource]struct{}) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// remove non-exist resources
	for resource := range c.cache {
		if _, exist := resources[resource]; !exist {
			klog.Infof("Remove cache for %s %s", c.clusterName, resource.String())
			c.cache[resource].stop()
			delete(c.cache, resource)
		}
	}

	// add resource cache
	for resource := range resources {
		_, exist := c.cache[resource]
		if !exist {
			kind, err := c.restMapper.KindFor(resource)
			if err != nil {
				return err
			}
			mapping, err := c.restMapper.RESTMapping(kind.GroupKind(), kind.Version)
			if err != nil {
				return err
			}
			namespaced := mapping.Scope.Name() == meta.RESTScopeNameNamespace

			klog.Infof("Add cache for %s %s", c.clusterName, resource.String())
			cache, err := newResourceCache(c.clusterName, resource, kind, namespaced, c.clientForResourceFunc(resource))
			if err != nil {
				return err
			}
			c.cache[resource] = cache
		}
	}
	return nil
}

func (c *clusterCache) stop() {
	c.lock.RLock()
	defer c.lock.RUnlock()

	for _, cache := range c.cache {
		cache.stop()
	}
}

func (c *clusterCache) clientForResourceFunc(resource schema.GroupVersionResource) func() (dynamic.NamespaceableResourceInterface, error) {
	return func() (dynamic.NamespaceableResourceInterface, error) {
		client, err := c.newClientFunc()
		if err != nil {
			return nil, err
		}
		return client.Resource(resource), nil
	}
}

func (c *clusterCache) cacheForResource(gvr schema.GroupVersionResource) *resourceCache {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.cache[gvr]
}
