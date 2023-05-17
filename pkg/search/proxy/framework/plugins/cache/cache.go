package cache

import (
	"context"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/handlers"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/karmada-io/karmada/pkg/search/proxy/framework"
	pluginruntime "github.com/karmada-io/karmada/pkg/search/proxy/framework/runtime"
	"github.com/karmada-io/karmada/pkg/search/proxy/store"
)

const (
	// We keep a big gap between in-tree plugins, to allow users to insert custom plugins between them.
	order = 1000
)

// Cache caches resource from member clusters, and handle the read request(get/list/watch) for resources.
// For reading requests, we redirect them to cache.
// Users call these requests to read resources in member clusters, such as pods and nodes.
type Cache struct {
	store             store.Store
	restMapper        meta.RESTMapper
	minRequestTimeout time.Duration
}

var _ framework.Plugin = (*Cache)(nil)

// New creates an instance of Cache
func New(dep pluginruntime.PluginDependency) (framework.Plugin, error) {
	return &Cache{
		store:             dep.Store,
		restMapper:        dep.RestMapper,
		minRequestTimeout: dep.MinRequestTimeout,
	}, nil
}

// Order implements Plugin
func (c *Cache) Order() int {
	return order
}

// SupportRequest implements Plugin
func (c *Cache) SupportRequest(request framework.ProxyRequest) bool {
	requestInfo := request.RequestInfo

	return requestInfo.IsResourceRequest &&
		c.store.HasResource(request.GroupVersionResource) &&
		requestInfo.Subresource == "" &&
		(requestInfo.Verb == "get" ||
			requestInfo.Verb == "list" ||
			requestInfo.Verb == "watch")
}

// Connect implements Plugin
func (c *Cache) Connect(_ context.Context, request framework.ProxyRequest) (http.Handler, error) {
	requestInfo := request.RequestInfo
	r := &rester{
		store:          c.store,
		gvr:            request.GroupVersionResource,
		tableConvertor: rest.NewDefaultTableConvertor(request.GroupVersionResource.GroupResource()),
	}

	gvk, err := c.restMapper.KindFor(request.GroupVersionResource)
	if err != nil {
		return nil, err
	}
	mapping, err := c.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	scope := &handlers.RequestScope{
		Kind:     gvk,
		Resource: request.GroupVersionResource,
		Namer: &handlers.ContextBasedNaming{
			Namer:         meta.NewAccessor(),
			ClusterScoped: mapping.Scope.Name() == meta.RESTScopeNameRoot,
		},
		Serializer:       scheme.Codecs.WithoutConversion(),
		Convertor:        runtime.NewScheme(),
		Subresource:      requestInfo.Subresource,
		MetaGroupVersion: metav1.SchemeGroupVersion,
	}

	var h http.Handler
	if requestInfo.Verb == "watch" || requestInfo.Name == "" {
		// for list or watch
		h = handlers.ListResource(r, r, scope, false, c.minRequestTimeout)
	} else {
		h = handlers.GetResource(r, scope)
	}
	return h, nil
}

type rester struct {
	store          store.Store
	gvr            schema.GroupVersionResource
	tableConvertor rest.TableConvertor
}

var _ rest.Getter = &rester{}
var _ rest.Lister = &rester{}
var _ rest.Watcher = &rester{}

// Get implements rest.Getter interface
func (r *rester) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, r.gvr, name, options)
}

// Watch implements rest.Watcher interface
func (r *rester) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return r.store.Watch(ctx, r.gvr, options)
}

// List implements rest.Lister interface
func (r *rester) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	return r.store.List(ctx, r.gvr, options)
}

// NewList implements rest.Lister interface
func (r *rester) NewList() runtime.Object {
	return &unstructured.UnstructuredList{}
}

// ConvertToTable implements rest.Lister interface
func (r *rester) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return r.tableConvertor.ConvertToTable(ctx, object, tableOptions)
}
