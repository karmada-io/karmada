package restmapper

import (
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// GetGroupVersionResource is a helper to map GVK(schema.GroupVersionKind) to GVR(schema.GroupVersionResource).
func GetGroupVersionResource(restMapper meta.RESTMapper, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	restMapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return restMapping.Resource, nil
}

// cachedRESTMapper caches the previous result to accelerate subsequent queries.
// Note: now the acceleration applies only to RESTMapping() which is heavily used by Karmada.
type cachedRESTMapper struct {
	restMapper      meta.RESTMapper
	discoveryClient discovery.DiscoveryInterface
	gvkToGVR        sync.Map
	// mu is used to provide thread-safe mapper reloading.
	mu sync.RWMutex
}

func (g *cachedRESTMapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return g.getMapper().KindFor(resource)
}

func (g *cachedRESTMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return g.getMapper().KindsFor(resource)
}

func (g *cachedRESTMapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return g.getMapper().ResourceFor(input)
}

func (g *cachedRESTMapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return g.getMapper().ResourcesFor(input)
}

func (g *cachedRESTMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	return g.getMapper().RESTMappings(gk, versions...)
}

func (g *cachedRESTMapper) ResourceSingularizer(resource string) (singular string, err error) {
	return g.getMapper().ResourceSingularizer(resource)
}

func (g *cachedRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	// in case of multi-versions or no versions, cachedRESTMapper don't know which is the preferred version,
	// so just bypass the cache and consult the underlying mapper.
	if len(versions) != 1 {
		return g.getMapper().RESTMapping(gk, versions...)
	}

	gvk := gk.WithVersion(versions[0])
	value, ok := g.gvkToGVR.Load(gvk)
	if ok { // hit cache, just return
		return value.(*meta.RESTMapping), nil
	}

	// consult underlying mapper and then update cache
	restMapping, err := g.getMapper().RESTMapping(gk, versions...)
	if meta.IsNoMatchError(err) {
		// hit here means a resource might be missing from the current rest mapper,
		// probably because a new resource(CRD) has been added, we have to reload
		// resource and rebuild the rest mapper.

		var groupResources []*restmapper.APIGroupResources
		groupResources, err = restmapper.GetAPIGroupResources(g.discoveryClient)
		if err != nil {
			return nil, err
		}

		newMapper := restmapper.NewDiscoveryRESTMapper(groupResources)
		restMapping, err = newMapper.RESTMapping(gk, versions...)
		if err == nil {
			// hit here means after reloading, the new rest mapper can recognize
			// the resource, we have to replace the mapper and clear cache.
			g.mu.Lock()
			g.restMapper = newMapper
			g.mu.Unlock()
			g.gvkToGVR.Range(func(key, value any) bool {
				g.gvkToGVR.Delete(key)
				return true
			})
		}
	}

	if err != nil {
		return restMapping, err
	}
	g.gvkToGVR.Store(gvk, restMapping)

	return restMapping, nil
}

func (g *cachedRESTMapper) getMapper() meta.RESTMapper {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.restMapper
}

// NewCachedRESTMapper builds a cachedRESTMapper with a customized underlyingMapper.
// If underlyingMapper is nil, defaults to DiscoveryRESTMapper.
func NewCachedRESTMapper(cfg *rest.Config, underlyingMapper meta.RESTMapper) (meta.RESTMapper, error) {
	cachedMapper := cachedRESTMapper{}

	// short path, build with customized underlying mapper.
	if underlyingMapper != nil {
		cachedMapper.restMapper = underlyingMapper
		return &cachedMapper, nil
	}

	client, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}

	// loading current resources for building a base rest mapper.
	groupResources, err := restmapper.GetAPIGroupResources(client)
	if err != nil {
		return nil, err
	}

	cachedMapper.restMapper = restmapper.NewDiscoveryRESTMapper(groupResources)
	cachedMapper.discoveryClient = client

	return &cachedMapper, nil
}

// MapperProvider is a wrapper of cachedRESTMapper which is typically used
// to generate customized RESTMapper for controller-runtime framework.
func MapperProvider(c *rest.Config, _ *http.Client) (meta.RESTMapper, error) {
	return NewCachedRESTMapper(c, nil)
}
