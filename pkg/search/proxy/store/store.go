package store

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/client-go/dynamic"
)

// store implements storage.Interface, Providing Get/Watch/List resources from member clusters.
type store struct {
	versioner storage.Versioner
	prefix    string
	// newClientFunc returns a resource client for member cluster apiserver
	newClientFunc func() (dynamic.NamespaceableResourceInterface, error)
}

var _ storage.Interface = &store{}

func newStore(newClientFunc func() (dynamic.NamespaceableResourceInterface, error), versioner storage.Versioner, prefix string) *store {
	return &store{
		newClientFunc: newClientFunc,
		versioner:     versioner,
		prefix:        prefix,
	}
}

// Versioner implements storage.Interface.
func (s *store) Versioner() storage.Versioner {
	return s.versioner
}

// Get implements storage.Interface.
func (s *store) Get(ctx context.Context, key string, opts storage.GetOptions, objPtr runtime.Object) error {
	var namespace, name string
	part1, part2 := s.splitKey(key)
	if part2 == "" {
		// for cluster scope resource, key is /prefix/name. So parts are [name, ""]
		name = part1
	} else {
		// for namespace scope resource, key is /prefix/namespace/name. So parts are [namespace, name]
		namespace, name = part1, part2
	}

	client, err := s.client(namespace)
	if err != nil {
		return err
	}

	obj, err := client.Get(ctx, name, convertToMetaV1GetOptions(opts))
	if err != nil {
		return err
	}

	unstructuredObj := objPtr.(*unstructured.Unstructured)
	obj.DeepCopyInto(unstructuredObj)
	return nil
}

// GetList implements storage.Interface.
func (s *store) GetList(ctx context.Context, key string, opts storage.ListOptions, listObj runtime.Object) error {
	return s.List(ctx, key, opts, listObj)
}

// List implements storage.Interface.
func (s *store) List(ctx context.Context, key string, opts storage.ListOptions, listObj runtime.Object) error {
	// For cluster scope resources, key is /prefix. Parts are ["", ""]
	// For namespace scope resources, key is /prefix/namespace. Parts are [namespace, ""]
	namespace, _ := s.splitKey(key)

	client, err := s.client(namespace)
	if err != nil {
		return err
	}

	options := convertToMetaV1ListOptions(opts)
	objects, err := client.List(ctx, options)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	objects.DeepCopyInto(listObj.(*unstructured.UnstructuredList))
	return nil
}

// WatchList implements storage.Interface.
func (s *store) WatchList(ctx context.Context, key string, opts storage.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, key, opts)
}

// Watch implements storage.Interface.
func (s *store) Watch(ctx context.Context, key string, opts storage.ListOptions) (watch.Interface, error) {
	// For cluster scope resources, key is /prefix. Parts are ["", ""]
	// For namespace scope resources, key is /prefix/namespace. Parts are [namespace, ""]
	namespace, _ := s.splitKey(key)

	client, err := s.client(namespace)
	if err != nil {
		return nil, err
	}

	options := convertToMetaV1ListOptions(opts)
	return client.Watch(ctx, options)
}

// Create implements storage.Interface.
func (s *store) Create(context.Context, string, runtime.Object, runtime.Object, uint64) error {
	return fmt.Errorf("create is not suppported in proxy store")
}

// Delete implements storage.Interface.
func (s *store) Delete(context.Context, string, runtime.Object, *storage.Preconditions, storage.ValidateObjectFunc, runtime.Object) error {
	return fmt.Errorf("delete is not suppported in proxy store")
}

// GuaranteedUpdate implements storage.Interface.
func (s *store) GuaranteedUpdate(context.Context, string, runtime.Object, bool, *storage.Preconditions, storage.UpdateFunc, runtime.Object) error {
	return fmt.Errorf("guaranteedUpdate is not suppported in proxy store")
}

// Count implements storage.Interface.
func (s *store) Count(string) (int64, error) {
	return 0, fmt.Errorf("count is not suppported in proxy store")
}

func (s *store) client(namespace string) (dynamic.ResourceInterface, error) {
	client, err := s.newClientFunc()
	if err != nil {
		return nil, err
	}

	if len(namespace) > 0 {
		return client.Namespace(namespace), nil
	}
	return client, nil
}

func (s *store) splitKey(key string) (string, string) {
	// a key is like:
	// - /prefix
	// - /prefix/name
	// - /prefix/namespace
	// - /prefix/namespace/name
	k := strings.TrimPrefix(key, s.prefix)
	k = strings.TrimPrefix(k, "/")
	parts := strings.SplitN(k, "/", 2)

	part0, part1 := parts[0], ""
	if len(parts) == 2 {
		part1 = parts[1]
	}
	return part0, part1
}
