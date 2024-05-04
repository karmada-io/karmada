/*
Copyright 2022 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/features"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	cacherstorage "k8s.io/apiserver/pkg/storage/cacher"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// resourceCache cache one kind resource from single member cluster
type resourceCache struct {
	*genericregistry.Store
	clusterName string
	resource    schema.GroupVersionResource
	multiNS     *MultiNamespace
}

func (c *resourceCache) stop() {
	klog.Infof("Stop store for %s %s", c.clusterName, c.resource)
	go c.Store.DestroyFunc()
}

func newResourceCache(clusterName string, gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, singularName string,
	namespaced bool, multiNS *MultiNamespace, configGetter func() (*restclient.Config, error)) (*resourceCache, error) {

	// TODO: it's unsafe guesses kind name for resource list
	listGVK := gvk.GroupVersion().WithKind(gvk.Kind + "List")

	var (
		isUnstructured bool
		codecs         serializer.CodecFactory
		paramCodec     runtime.ParameterCodec
		codec          runtime.Codec
		obj, listObj   runtime.Object
		err            error
	)

	if kubescheme.Scheme.Recognizes(gvk) {
		scheme := kubescheme.Scheme
		codecs = serializer.NewCodecFactory(scheme)
		codec = codecs.LegacyCodec(gvk.GroupVersion())
		paramCodec = kubescheme.ParameterCodec

		obj, err = scheme.New(gvk)
		if err != nil {
			return nil, err
		}

		listObj, err = scheme.New(listGVK)
		if err != nil {
			return nil, err
		}

	} else {
		isUnstructured = true
		codecs = serializer.NewCodecFactory(kubescheme.Scheme)
		codec = unstructured.UnstructuredJSONScheme
		paramCodec = noConversionParamCodec{}

		unstructuredObj := &unstructured.Unstructured{}
		unstructuredObj.SetGroupVersionKind(gvk)
		obj = unstructuredObj

		unstructuredList := &unstructured.UnstructuredList{}
		unstructuredList.SetGroupVersionKind(gvk)
		listObj = unstructuredList
	}

	s := &genericregistry.Store{
		DefaultQualifiedResource:  gvr.GroupResource(),
		SingularQualifiedResource: schema.GroupResource{Group: gvr.Group, Resource: singularName},
		NewFunc: func() runtime.Object {
			return obj.DeepCopyObject()
		},
		NewListFunc: func() runtime.Object {
			return listObj.DeepCopyObject()
		},
		// TODO: add typed convertor
		TableConvertor: rest.NewDefaultTableConvertor(gvr.GroupResource()),
		// CreateStrategy tells whether the resource is namespaced.
		// see: vendor/k8s.io/apiserver/pkg/registry/generic/registry/store.go#L1310-L1318
		CreateStrategy: restCreateStrategy(namespaced),
		// Assign `DeleteStrategy` to pass vendor/k8s.io/apiserver/pkg/registry/generic/registry/store.go#L1320-L1322
		DeleteStrategy: restDeleteStrategy,
	}

	err = s.CompleteWithOptions(&generic.StoreOptions{
		RESTOptions: &generic.RESTOptions{
			StorageConfig: &storagebackend.ConfigForResource{
				Config: storagebackend.Config{
					Paging: utilfeature.DefaultFeatureGate.Enabled(features.APIListChunking),
					Codec:  codec,
				},
				GroupResource: gvr.GroupResource(),
			},
			ResourcePrefix: gvr.Group + "/" + gvr.Resource,
			Decorator:      storageWithCacher(gvr, gvk, isUnstructured, codecs, paramCodec, configGetter, multiNS, defaultVersioner),
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
		multiNS:     multiNS,
	}, nil
}

func storageWithCacher(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, isUnstructured bool,
	codecs serializer.CodecFactory, paramCodec runtime.ParameterCodec, configGetter func() (*restclient.Config, error),
	multiNS *MultiNamespace, versioner storage.Versioner) generic.StorageDecorator {
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
			Storage: &store{
				versioner:      versioner,
				prefix:         resourcePrefix,
				configGetter:   configGetter,
				isUnstructured: isUnstructured,
				gvk:            gvk,
				gvr:            gvr,
				codecs:         codecs,
				paramCodec:     paramCodec,
				multiNS:        multiNS,
			},
			Versioner:      versioner,
			GroupResource:  storageConfig.GroupResource,
			ResourcePrefix: resourcePrefix,
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

var (
	restCreateStrategyForNamespaced = &simpleRESTCreateStrategy{namespaced: true}
	restCreateStrategyForCluster    = &simpleRESTCreateStrategy{namespaced: false}
	restDeleteStrategy              = &simpleRESTDeleteStrategy{RESTDeleteStrategy: runtime.NewScheme()}
	defaultVersioner                = storage.APIObjectVersioner{}
)

type simpleRESTDeleteStrategy struct {
	rest.RESTDeleteStrategy
}

type simpleRESTCreateStrategy struct {
	namespaced bool
}

func (s *simpleRESTCreateStrategy) NamespaceScoped() bool {
	return s.namespaced
}

func (s *simpleRESTCreateStrategy) ObjectKinds(runtime.Object) ([]schema.GroupVersionKind, bool, error) {
	panic("simpleRESTCreateStrategy.ObjectKinds is not supported")
}

func (s *simpleRESTCreateStrategy) Recognizes(schema.GroupVersionKind) bool {
	panic("simpleRESTCreateStrategy.Recognizes is not supported")
}

func (s *simpleRESTCreateStrategy) GenerateName(string) string {
	panic("simpleRESTCreateStrategy.GenerateName is not supported")
}

func (s *simpleRESTCreateStrategy) PrepareForCreate(context.Context, runtime.Object) {
	panic("simpleRESTCreateStrategy.PrepareForCreate is not supported")
}

func (s *simpleRESTCreateStrategy) Validate(context.Context, runtime.Object) field.ErrorList {
	panic("simpleRESTCreateStrategy.Validate is not supported")
}

func (s *simpleRESTCreateStrategy) WarningsOnCreate(context.Context, runtime.Object) []string {
	panic("simpleRESTCreateStrategy.WarningsOnCreate is not supported")
}

func (s *simpleRESTCreateStrategy) Canonicalize(runtime.Object) {
	panic("simpleRESTCreateStrategy.Canonicalize is not supported")
}

func restCreateStrategy(namespaced bool) rest.RESTCreateStrategy {
	if namespaced {
		return restCreateStrategyForNamespaced
	}
	return restCreateStrategyForCluster
}

func getAttrsFunc(namespaced bool) func(runtime.Object) (labels.Set, fields.Set, error) {
	return func(object runtime.Object) (label labels.Set, field fields.Set, err error) {
		accessor, err := meta.Accessor(object)
		if err != nil {
			return nil, nil, err
		}

		label = accessor.GetLabels()

		// In proxy, fieldSelector only support name and namespace
		if namespaced {
			field = fields.Set{
				"metadata.name":      accessor.GetName(),
				"metadata.namespace": accessor.GetNamespace(),
			}
		} else {
			field = fields.Set{
				"metadata.name": accessor.GetName(),
			}
		}
		return label, field, nil
	}
}
