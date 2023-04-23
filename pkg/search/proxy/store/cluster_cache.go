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
	lock            sync.RWMutex
	cache           map[schema.GroupVersionResource]*resourceCache
	namespacedCache map[schema.GroupVersionResource]map[string]*resourceCache
	clusterName     string
	restMapper      meta.RESTMapper
	// newClientFunc returns a dynamic client for member cluster apiserver
	newClientFunc func() (dynamic.Interface, error)
}

func newClusterCache(clusterName string, newClientFunc func() (dynamic.Interface, error), restMapper meta.RESTMapper) *clusterCache {
	return &clusterCache{
		clusterName:     clusterName,
		newClientFunc:   newClientFunc,
		restMapper:      restMapper,
		cache:           map[schema.GroupVersionResource]*resourceCache{},
		namespacedCache: map[schema.GroupVersionResource]map[string]*resourceCache{},
	}
}

func (c *clusterCache) updateCache(resources map[schema.GroupVersionResource]*NamespaceScope) error {
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
	for resource := range c.namespacedCache {
		if _, exist := resources[resource]; !exist {
			for _, cache := range c.namespacedCache[resource] {
				klog.Infof("Remove cache for %s %s", c.clusterName, resource.String())
				cache.stop()
			}
			delete(c.namespacedCache, resource)
		}
	}

	// add resource cache
	for resource, nsScope := range resources {
		kind, err := c.restMapper.KindFor(resource)
		if err != nil {
			return err
		}
		mapping, err := c.restMapper.RESTMapping(kind.GroupKind(), kind.Version)
		if err != nil {
			return err
		}
		namespaced := mapping.Scope.Name() == meta.RESTScopeNameNamespace
		if namespaced {
			if cacheMap, exist := c.namespacedCache[resource]; !exist {
				cacheMap = map[string]*resourceCache{}
				c.namespacedCache[resource] = cacheMap
				if nsScope.allNamespaces {
					klog.Infof("Add cache for %s %s", c.clusterName, resource.String())
					cache, err := newNamespacedResourceCache(c.clusterName, resource, kind, namespaced, AllNamespace, c.clientForResourceFunc(resource))
					if err != nil {
						return err
					}
					cacheMap[AllNamespace] = cache
				} else {
					for ns := range nsScope.namespaces {
						klog.Infof("Add cache for %s %s in %s", c.clusterName, resource.String(), ns)
						cache, err := newNamespacedResourceCache(c.clusterName, resource, kind, namespaced, ns, c.clientForResourceFunc(resource))
						if err != nil {
							return err
						}
						cacheMap[ns] = cache
					}
				}
			} else {
				if nsScope.allNamespaces {
					if _, exist := cacheMap[AllNamespace]; !exist {
						klog.Infof("Add cache for %s %s", c.clusterName, resource.String())
						cache, err := newNamespacedResourceCache(c.clusterName, resource, kind, namespaced, AllNamespace, c.clientForResourceFunc(resource))
						if err != nil {
							return err
						}
						newCacheMap := map[string]*resourceCache{
							AllNamespace: cache,
						}
						oldCacheMap := c.namespacedCache[resource]
						c.namespacedCache[resource] = newCacheMap
						for _, oldCache := range oldCacheMap {
							oldCache.stop()
						}
					}
				} else {
					if cache, exist := cacheMap[AllNamespace]; exist {
						for ns := range nsScope.namespaces {
							klog.Infof("Add cache for %s %s in %s", c.clusterName, resource.String(), ns)
							cache, err := newNamespacedResourceCache(c.clusterName, resource, kind, namespaced, ns, c.clientForResourceFunc(resource))
							if err != nil {
								return err
							}
							cacheMap[ns] = cache
						}
						cache.stop()
						delete(cacheMap, AllNamespace)
					} else {
						//remove
						for ns, cache := range cacheMap {
							if !nsScope.Has(ns) {
								klog.Infof("Remove cache for %s %s in %s", c.clusterName, resource.String(), ns)
								cache.stop()
								delete(cacheMap, ns)
							}
						}
						//add
						for ns := range nsScope.namespaces {
							if _, exist := cacheMap[ns]; !exist {
								klog.Infof("Add cache for %s %s in %s", c.clusterName, resource.String(), ns)
								cache, err := newNamespacedResourceCache(c.clusterName, resource, kind, namespaced, ns, c.clientForResourceFunc(resource))
								if err != nil {
									return err
								}
								cacheMap[ns] = cache
							}
						}

					}

				}
			}
		} else {
			_, exist := c.cache[resource]
			if !exist {
				klog.Infof("Add cache for %s %s", c.clusterName, resource.String())
				cache, err := newResourceCache(c.clusterName, resource, kind, namespaced, c.clientForResourceFunc(resource))
				if err != nil {
					return err
				}
				c.cache[resource] = cache
			}
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
	for _, cacheMap := range c.namespacedCache {
		for _, cache := range cacheMap {
			cache.stop()
		}
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

func (c *clusterCache) cacheForResource(gvr schema.GroupVersionResource, namespace string) *resourceCache {
	kind, err := c.restMapper.KindFor(gvr)
	if err != nil {
		return nil
	}
	mapping, err := c.restMapper.RESTMapping(kind.GroupKind(), kind.Version)
	if err != nil {
		return nil
	}
	namespaced := mapping.Scope.Name() == meta.RESTScopeNameNamespace
	c.lock.RLock()
	defer c.lock.RUnlock()
	if namespaced {
		if cache, ok := c.namespacedCache[gvr][AllNamespace]; ok {
			return cache
		} else {
			return c.namespacedCache[gvr][namespace]
		}
	} else {
		return c.cache[gvr]
	}
}
