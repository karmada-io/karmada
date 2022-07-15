package restmapper

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
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
	restMapper meta.RESTMapper
	gvkToGVR   sync.Map
}

func (g *cachedRESTMapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return g.restMapper.KindFor(resource)
}

func (g *cachedRESTMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return g.restMapper.KindsFor(resource)
}

func (g *cachedRESTMapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return g.restMapper.ResourceFor(input)
}

func (g *cachedRESTMapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return g.restMapper.ResourcesFor(input)
}

func (g *cachedRESTMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	return g.restMapper.RESTMappings(gk, versions...)
}

func (g *cachedRESTMapper) ResourceSingularizer(resource string) (singular string, err error) {
	return g.restMapper.ResourceSingularizer(resource)
}

func (g *cachedRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	// in case of multi-versions, cachedRESTMapper don't know which is the preferred version,
	// so just bypass the cache and consult the underlying mapper.
	if len(versions) > 1 {
		return g.restMapper.RESTMapping(gk, versions...)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("expected at least one version")
	}

	gvk := gk.WithVersion(versions[0])
	value, ok := g.gvkToGVR.Load(gvk)
	if ok { // hit cache, just return
		return value.(*meta.RESTMapping), nil
	}

	// consult underlying mapper and then update cache
	restMapping, err := g.restMapper.RESTMapping(gk, versions...)
	if err != nil {
		return restMapping, err
	}
	g.gvkToGVR.Store(gvk, restMapping)
	return restMapping, nil
}

// NewCachedRESTMapper builds a cachedRESTMapper with a customized underlyingMapper.
// If underlyingMapper is nil, defaults to DynamicRESTMapper.
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

	option := apiutil.WithCustomMapper(func() (meta.RESTMapper, error) {
		groupResources, err := restmapper.GetAPIGroupResources(client)
		if err != nil {
			return nil, err
		}
		// clear the cache map when reloading DiscoveryRESTMapper
		cachedMapper.gvkToGVR = sync.Map{}
		return restmapper.NewDiscoveryRESTMapper(groupResources), nil
	})

	underlyingMapper, err = apiutil.NewDynamicRESTMapper(cfg, option)
	if err != nil {
		return nil, err
	}
	cachedMapper.restMapper = underlyingMapper

	return &cachedMapper, nil
}

// MapperProvider is a wrapper of cachedRESTMapper which is typically used
// to generate customized RESTMapper for controller-runtime framework.
func MapperProvider(c *rest.Config) (meta.RESTMapper, error) {
	return NewCachedRESTMapper(c, nil)
}
