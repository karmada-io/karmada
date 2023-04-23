package store

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	utiltrace "k8s.io/utils/trace"
)

// Store is the cache for resources from multiple member clusters
type Store interface {
	UpdateCache(resourcesByCluster map[string]map[schema.GroupVersionResource]struct{}) error
	HasResource(resource schema.GroupVersionResource) bool
	GetResourceFromCache(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error)
	Stop()

	Get(ctx context.Context, gvr schema.GroupVersionResource, name string, options *metav1.GetOptions) (runtime.Object, error)
	List(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (runtime.Object, error)
	Watch(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (watch.Interface, error)
}

// MultiClusterCache caches resource from multi member clusters
type MultiClusterCache struct {
	lock            sync.RWMutex
	cache           map[string]*clusterCache
	cachedResources map[schema.GroupVersionResource]struct{}
	restMapper      meta.RESTMapper
	// newClientFunc returns a dynamic client for member cluster apiserver
	newClientFunc func(string) (dynamic.Interface, error)
}

var _ Store = &MultiClusterCache{}

// NewMultiClusterCache return a cache for resources from member clusters
func NewMultiClusterCache(newClientFunc func(string) (dynamic.Interface, error), restMapper meta.RESTMapper) *MultiClusterCache {
	return &MultiClusterCache{
		restMapper:      restMapper,
		newClientFunc:   newClientFunc,
		cache:           map[string]*clusterCache{},
		cachedResources: map[schema.GroupVersionResource]struct{}{},
	}
}

// UpdateCache update cache for multi clusters
func (c *MultiClusterCache) UpdateCache(resourcesByCluster map[string]map[schema.GroupVersionResource]struct{}) error {
	if klog.V(3).Enabled() {
		start := time.Now()
		defer func() {
			klog.Infof("MultiClusterCache update cache takes %v", time.Since(start))
		}()
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	// remove non-exist clusters
	newClusterNames := sets.NewString()
	for clusterName := range resourcesByCluster {
		newClusterNames.Insert(clusterName)
	}
	for clusterName := range c.cache {
		if !newClusterNames.Has(clusterName) {
			klog.Infof("Remove cache for cluster %s", clusterName)
			c.cache[clusterName].stop()
			delete(c.cache, clusterName)
		}
	}

	// add/update cluster cache
	for clusterName, resources := range resourcesByCluster {
		cache, exist := c.cache[clusterName]
		if !exist {
			klog.Infof("Add cache for cluster %v", clusterName)
			cache = newClusterCache(clusterName, c.clientForClusterFunc(clusterName), c.restMapper)
			c.cache[clusterName] = cache
		}
		err := cache.updateCache(resources)
		if err != nil {
			return err
		}
	}

	// update cachedResource
	newCachedResources := make(map[schema.GroupVersionResource]struct{}, len(c.cachedResources))
	for _, resources := range resourcesByCluster {
		for resource := range resources {
			newCachedResources[resource] = struct{}{}
		}
	}
	for resource := range c.cachedResources {
		if _, exist := newCachedResources[resource]; !exist {
			delete(c.cachedResources, resource)
		}
	}
	for resource := range newCachedResources {
		if _, exist := c.cachedResources[resource]; !exist {
			c.cachedResources[resource] = struct{}{}
		}
	}
	return nil
}

// Stop stops the cache for multi cluster.
func (c *MultiClusterCache) Stop() {
	c.lock.RLock()
	defer c.lock.RUnlock()

	for _, cache := range c.cache {
		cache.stop()
	}
}

// HasResource return whether resource is cached.
func (c *MultiClusterCache) HasResource(resource schema.GroupVersionResource) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	_, ok := c.cachedResources[resource]
	return ok
}

// GetResourceFromCache returns which cluster the resource belong to.
func (c *MultiClusterCache) GetResourceFromCache(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error) {
	obj, cluster, _, err := c.getResourceFromCache(ctx, gvr, namespace, name)
	return obj, cluster, err
}

// Get returns the target object
func (c *MultiClusterCache) Get(ctx context.Context, gvr schema.GroupVersionResource, name string, options *metav1.GetOptions) (runtime.Object, error) {
	_, clusterName, cache, err := c.getResourceFromCache(ctx, gvr, request.NamespaceValue(ctx), name)
	if err != nil {
		return nil, err
	}
	obj, err := cache.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}

	cloneObj := obj.DeepCopyObject()
	accessor, err := meta.Accessor(cloneObj)
	if err != nil {
		return nil, err
	}

	mrv := newMultiClusterResourceVersionWithCapacity(1)
	mrv.set(clusterName, accessor.GetResourceVersion())
	accessor.SetResourceVersion(mrv.String())

	addCacheSourceAnnotation(cloneObj, clusterName)

	return cloneObj, err
}

// List returns the object list
// nolint:gocyclo
func (c *MultiClusterCache) List(ctx context.Context, gvr schema.GroupVersionResource, o *metainternalversion.ListOptions) (runtime.Object, error) {
	klog.V(5).Infof("Request list %v with rv=%#v, continue=%#v, limit=%v", gvr.String(),
		newMultiClusterResourceVersionFromString(o.ResourceVersion), newMultiClusterContinueFromString(o.Continue), o.Limit)
	requestCluster, options, requestResourceVersion := prepareBeforeList(o)

	var resultObject runtime.Object
	items := make([]runtime.Object, 0, int(math.Min(float64(options.Limit), 1024)))

	clusters := c.getClusterNames()
	sort.Strings(clusters)
	responseResourceVersion := requestResourceVersion.clone()
	responseContinue := multiClusterContinue{}

	trace := utiltrace.New("MultiClusterCache.List",
		utiltrace.Field{Key: "gvr", Value: gvr},
		utiltrace.Field{Key: "rv", Value: newMultiClusterResourceVersionFromString(o.ResourceVersion)},
		utiltrace.Field{Key: "continue", Value: newMultiClusterContinueFromString(o.Continue)},
		utiltrace.Field{Key: "limit", Value: o.Limit},
	)
	defer trace.LogIfLong(5 * time.Second)

	defer func() {
		klog.V(5).Infof("Response list %v with rv=%#v continue=%#v", gvr.String(), responseResourceVersion, responseContinue)
	}()

	listFunc := func(cluster string) (int, string, error) {
		if requestCluster != "" && requestCluster != cluster {
			return 0, "", nil
		}
		// clear the requestContinue, for searching other clusters at next list
		defer func() {
			requestCluster = ""
			options.Continue = ""
		}()

		cache := c.cacheForClusterResource(cluster, gvr)
		if cache == nil {
			klog.V(4).Infof("cluster %v does not cache resource %v", cluster, gvr.String())
			return 0, "", nil
		}

		if options.Continue != "" {
			// specifying resource version is not allowed when using continue
			options.ResourceVersion = ""
		} else {
			options.ResourceVersion = requestResourceVersion.get(cluster)
		}
		defer trace.Step("list from member cluster",
			utiltrace.Field{Key: "cluster", Value: cluster},
			utiltrace.Field{Key: "options", Value: fmt.Sprintf("%#v", options)},
		)
		obj, err := cache.List(ctx, options)
		if err != nil {
			return 0, "", err
		}

		list, err := meta.ListAccessor(obj)
		if err != nil {
			return 0, "", err
		}

		if resultObject == nil {
			resultObject = obj
		}

		cnt := 0
		err = meta.EachListItem(obj, func(o runtime.Object) error {
			clone := o.DeepCopyObject()
			addCacheSourceAnnotation(clone, cluster)
			items = append(items, clone)
			cnt++
			return nil
		})
		if err != nil {
			return 0, "", err
		}

		responseResourceVersion.set(cluster, list.GetResourceVersion())
		return cnt, list.GetContinue(), nil
	}

	if options.Limit == 0 {
		for _, cluster := range clusters {
			_, _, err := listFunc(cluster)
			if err != nil {
				return nil, err
			}
		}
	} else {
		for clusterIdx, cluster := range clusters {
			n, cont, err := listFunc(cluster)
			if err != nil {
				return nil, err
			}

			options.Limit -= int64(n)

			if options.Limit <= 0 {
				if cont != "" {
					// Current cluster has remaining items, return this cluster name and continue for next list.
					responseContinue.Cluster = cluster
					responseContinue.Continue = cont
				} else if (clusterIdx + 1) < len(clusters) {
					// Current cluster has no remaining items. But we don't know whether next cluster has.
					// So return the next cluster name for continue listing.
					responseContinue.Cluster = clusters[clusterIdx+1]
				}
				// No more items remains. Break the chuck list.
				break
			}
		}
	}

	if err := c.fillMissingClusterResourceVersion(ctx, responseResourceVersion, clusters, gvr); err != nil {
		return nil, err
	}
	responseContinue.RV = responseResourceVersion.String()

	if resultObject == nil {
		resultObject = &metav1.List{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "List",
			},
			ListMeta: metav1.ListMeta{},
			Items:    []runtime.RawExtension{},
		}
	}

	err := meta.SetList(resultObject, items)
	if err != nil {
		return nil, err
	}

	accessor, err := meta.ListAccessor(resultObject)
	if err != nil {
		return nil, err
	}
	accessor.SetResourceVersion(responseResourceVersion.String())
	accessor.SetContinue(responseContinue.String())
	return resultObject, nil
}

// Watch watches the resource
func (c *MultiClusterCache) Watch(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (watch.Interface, error) {
	klog.V(5).Infof("Request watch %v with rv=%v", gvr.String(), options.ResourceVersion)
	resourceVersion := newMultiClusterResourceVersionFromString(options.ResourceVersion)

	responseResourceVersion := newMultiClusterResourceVersionFromString(options.ResourceVersion)
	responseResourceVersionLock := sync.Mutex{}
	setObjectResourceVersionFunc := func(cluster string, obj runtime.Object) {
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return
		}

		responseResourceVersionLock.Lock()
		defer responseResourceVersionLock.Unlock()

		responseResourceVersion.set(cluster, accessor.GetResourceVersion())
		accessor.SetResourceVersion(responseResourceVersion.String())
	}

	mux := newWatchMux()
	clusters := c.getClusterNames()
	for i := range clusters {
		cluster := clusters[i]
		options.ResourceVersion = resourceVersion.get(cluster)
		cache := c.cacheForClusterResource(cluster, gvr)
		if cache == nil {
			continue
		}
		w, err := cache.Watch(ctx, options)
		if err != nil {
			return nil, err
		}

		mux.AddSource(w, func(e watch.Event) {
			setObjectResourceVersionFunc(cluster, e.Object)
			addCacheSourceAnnotation(e.Object, cluster)
		})
	}
	mux.Start()
	return mux, nil
}

func (c *MultiClusterCache) getClusterNames() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()

	clusters := make([]string, 0, len(c.cache))
	for c := range c.cache {
		clusters = append(clusters, c)
	}
	return clusters
}

func (c *MultiClusterCache) cacheForClusterResource(cluster string, gvr schema.GroupVersionResource) *resourceCache {
	c.lock.RLock()
	cc := c.cache[cluster]
	c.lock.RUnlock()
	if cc == nil {
		return nil
	}
	return cc.cacheForResource(gvr)
}

func (c *MultiClusterCache) getResourceFromCache(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, *resourceCache, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	var findObjects []runtime.Object
	var findCaches []*resourceCache

	// Set ResourceVersion=0 to get resource from cache.
	options := &metav1.GetOptions{ResourceVersion: "0"}
	for clusterName, cc := range c.cache {
		cache := cc.cache[gvr]
		if cache == nil {
			// This cluster doesn't cache this resource
			continue
		}

		obj, err := cache.Get(request.WithNamespace(ctx, namespace), name, options)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, "", nil, fmt.Errorf("fail to get %v %v from %v: %v", namespace, name, clusterName, err)
		}
		findObjects = append(findObjects, obj)
		findCaches = append(findCaches, cache)
	}

	if len(findObjects) == 0 {
		return nil, "", nil, apierrors.NewNotFound(gvr.GroupResource(), name)
	}

	if len(findObjects) > 1 {
		clusterNames := make([]string, 0, len(findCaches))
		for _, cache := range findCaches {
			clusterNames = append(clusterNames, cache.clusterName)
		}
		return nil, "", nil, apierrors.NewConflict(gvr.GroupResource(), name, fmt.Errorf("ambiguous objects in clusters [%v]", strings.Join(clusterNames, ", ")))
	}
	return findObjects[0], findCaches[0].clusterName, findCaches[0], nil
}

func (c *MultiClusterCache) clientForClusterFunc(cluster string) func() (dynamic.Interface, error) {
	return func() (dynamic.Interface, error) {
		return c.newClientFunc(cluster)
	}
}

func (c *MultiClusterCache) fillMissingClusterResourceVersion(ctx context.Context, mcv *multiClusterResourceVersion, clusters []string, gvr schema.GroupVersionResource) error {
	errChan := make(chan error)
	var lock sync.Mutex
	var wg sync.WaitGroup

	for _, cluster := range clusters {
		if _, ok := mcv.rvs[cluster]; ok {
			continue
		}

		wg.Add(1)
		go func(cluster string) {
			defer wg.Done()
			klog.V(5).Infof("fillMissingClusterResourceVersion gvr=%v cluster=%v", gvr, cluster)
			rv, err := c.getClusterResourceVersion(ctx, cluster, gvr)
			if err != nil {
				errChan <- err
				return
			}
			if rv == "" {
				return
			}

			lock.Lock()
			defer lock.Unlock()
			mcv.set(cluster, rv)
		}(cluster)
	}

	waitChan := make(chan struct{}, 1)
	go func() {
		wg.Wait()
		waitChan <- struct{}{}
	}()

	var err error
	select {
	case <-waitChan:
	case err = <-errChan:
	}
	return err
}

func (c *MultiClusterCache) getClusterResourceVersion(ctx context.Context, cluster string, gvr schema.GroupVersionResource) (string, error) {
	cache := c.cacheForClusterResource(cluster, gvr)
	if cache == nil {
		klog.V(4).Infof("cluster %v does not cache resource %v", cluster, gvr.String())
		return "", nil
	}
	obj, err := cache.List(ctx, &metainternalversion.ListOptions{
		Limit: 1,
	})
	if err != nil {
		return "", err
	}
	listObj, err := meta.ListAccessor(obj)
	if err != nil {
		return "", err
	}
	return listObj.GetResourceVersion(), nil
}

// Inputs and outputs:
// o.ResourceVersion  o.Continue                     | cluster options.ResourceVersion options.Continue mrv
// xxxx               ""                             | ""      xxxx                    ""               decode(xxx)
// ""               {rv=xxx,cluster=c2,continue=}    | c2      decode(xxx)[c2]         ""               decode(xxx)
// ""               {rv=xxx,cluster=c2,continue=yyy} | c2      ""                      yyy              decode(xxx)
func prepareBeforeList(o *metainternalversion.ListOptions) (cluster string, options *metainternalversion.ListOptions, mrv *multiClusterResourceVersion) {
	options = o.DeepCopy()
	if o.Continue == "" {
		mrv = newMultiClusterResourceVersionFromString(o.ResourceVersion)
		return
	}

	mc := newMultiClusterContinueFromString(o.Continue)
	cluster = mc.Cluster
	mrv = newMultiClusterResourceVersionFromString(mc.RV)
	if mc.Continue == "" {
		options.ResourceVersion = mrv.get(cluster)
		options.Continue = ""
	} else {
		options.ResourceVersion = ""
		options.Continue = mc.Continue
	}
	return
}
