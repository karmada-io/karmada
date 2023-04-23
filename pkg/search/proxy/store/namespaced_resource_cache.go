package store

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/features"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	cacherstorage "k8s.io/apiserver/pkg/storage/cacher"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

func newNamespacedResourceCache(clusterName string, gvr schema.GroupVersionResource, gvk schema.GroupVersionKind,
	namespaced bool, namespace string, newClientFunc func() (dynamic.NamespaceableResourceInterface, error)) (*resourceCache, error) {
	s := &genericregistry.Store{
		DefaultQualifiedResource: gvr.GroupResource(),
		NewFunc: func() runtime.Object {
			o := &unstructured.Unstructured{}
			o.SetAPIVersion(gvk.GroupVersion().String())
			o.SetKind(gvk.Kind)
			return o
		},
		NewListFunc: func() runtime.Object {
			o := &unstructured.UnstructuredList{}
			o.SetAPIVersion(gvk.GroupVersion().String())
			// TODO: it's unsafe guesses kind name for resource list
			o.SetKind(gvk.Kind + "List")
			return o
		},
		TableConvertor: rest.NewDefaultTableConvertor(gvr.GroupResource()),
		// CreateStrategy tells whether the resource is namespaced.
		// see: vendor/k8s.io/apiserver/pkg/registry/generic/registry/store.go#L1310-L1318
		CreateStrategy: restCreateStrategy(namespaced),
		// Assign `DeleteStrategy` to pass vendor/k8s.io/apiserver/pkg/registry/generic/registry/store.go#L1320-L1322
		DeleteStrategy: restDeleteStrategy,
	}

	err := s.CompleteWithOptions(&generic.StoreOptions{
		RESTOptions: &generic.RESTOptions{
			StorageConfig: &storagebackend.ConfigForResource{
				Config: storagebackend.Config{
					Paging: utilfeature.DefaultFeatureGate.Enabled(features.APIListChunking),
					Codec:  unstructured.UnstructuredJSONScheme,
				},
				GroupResource: gvr.GroupResource(),
			},
			ResourcePrefix: gvr.Group + "/" + gvr.Resource,
			Decorator:      namespacedStorageWithCacher(newClientFunc, namespace, defaultVersioner),
		},
		AttrFunc: getAttrsFunc(namespaced),
	})
	if err != nil {
		return nil, err
	}

	return &resourceCache{
		clusterName: clusterName,
		Store:       s,
		resource:    gvr,
	}, nil
}

func namespacedStorageWithCacher(newClientFunc func() (dynamic.NamespaceableResourceInterface, error), namespace string, versioner storage.Versioner) generic.StorageDecorator {
	return func(
		storageConfig *storagebackend.ConfigForResource,
		resourcePrefix string,
		keyFunc func(obj runtime.Object) (string, error),
		newFunc func() runtime.Object,
		newListFunc func() runtime.Object,
		getAttrsFunc storage.AttrFunc,
		triggerFuncs storage.IndexerFuncs,
		indexers *cache.Indexers) (storage.Interface, factory.DestroyFunc, error) {
		cacherConfig := cacherstorage.Config{
			Storage:        newStore(newClientFunc, versioner, resourcePrefix),
			Versioner:      versioner,
			ResourcePrefix: resourcePrefix + "/" + namespace,
			KeyFunc:        keyFunc,
			GetAttrsFunc:   getAttrsFunc,
			Indexers:       indexers,
			NewFunc:        newFunc,
			NewListFunc:    newListFunc,
			Codec:          storageConfig.Codec,
		}
		cacher, err := cacherstorage.NewCacherFromConfig(cacherConfig)
		if err != nil {
			return nil, nil, err
		}
		return cacher, cacher.Stop, nil
	}
}