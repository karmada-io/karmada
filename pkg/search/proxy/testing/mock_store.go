package testing

import (
	"context"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/karmada-io/karmada/pkg/search/proxy/store"
)

// MockStore is a mock for store.Store interface
type MockStore struct {
	UpdateCacheFunc          func(resourcesByCluster map[string]map[schema.GroupVersionResource]*store.MultiNamespace) error
	HasResourceFunc          func(resource schema.GroupVersionResource) bool
	GetResourceFromCacheFunc func(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error)
	StopFunc                 func()
	GetFunc                  func(ctx context.Context, gvr schema.GroupVersionResource, name string, options *metav1.GetOptions) (runtime.Object, error)
	ListFunc                 func(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (runtime.Object, error)
	WatchFunc                func(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (watch.Interface, error)
}

var _ store.Store = &MockStore{}

// UpdateCache implements store.Store interface
func (c *MockStore) UpdateCache(resourcesByCluster map[string]map[schema.GroupVersionResource]*store.MultiNamespace) error {
	if c.UpdateCacheFunc == nil {
		panic("implement me")
	}
	return c.UpdateCacheFunc(resourcesByCluster)
}

// HasResource implements store.Store interface
func (c *MockStore) HasResource(resource schema.GroupVersionResource) bool {
	if c.HasResourceFunc == nil {
		panic("implement me")
	}
	return c.HasResourceFunc(resource)
}

// GetResourceFromCache implements store.Store interface
func (c *MockStore) GetResourceFromCache(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error) {
	if c.GetResourceFromCacheFunc == nil {
		panic("implement me")
	}
	return c.GetResourceFromCacheFunc(ctx, gvr, namespace, name)
}

// Stop implements store.Store interface
func (c *MockStore) Stop() {
	if c.StopFunc != nil {
		c.StopFunc()
	}
}

// Get implements store.Store interface
func (c *MockStore) Get(ctx context.Context, gvr schema.GroupVersionResource, name string, options *metav1.GetOptions) (runtime.Object, error) {
	if c.GetFunc == nil {
		panic("implement me")
	}

	return c.GetFunc(ctx, gvr, name, options)
}

// List implements store.Store interface
func (c *MockStore) List(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (runtime.Object, error) {
	if c.ListFunc == nil {
		panic("implement me")
	}

	return c.ListFunc(ctx, gvr, options)
}

// Watch implements store.Store interface
func (c *MockStore) Watch(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (watch.Interface, error) {
	if c.WatchFunc == nil {
		panic("implement me")
	}

	return c.WatchFunc(ctx, gvr, options)
}
