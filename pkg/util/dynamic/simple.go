/*
Copyright 2026 The Karmada Authors.

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

package dynamic

import (
	"context"
	"fmt"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

// DynamicClient is a dynamic client whose response decoder preserves JSON
// objects as raw bytes.
//
//nolint:revive
type DynamicClient struct {
	client rest.Interface
}

var _ Interface = &DynamicClient{}

// ConfigFor returns a copy of the provided config with raw JSON defaults.
//
// This intentionally does not opt into CBOR. The cache object stores JSON bytes,
// so accepting CBOR would require transcoding before objects enter the store.
func ConfigFor(inConfig *rest.Config) *rest.Config {
	config := rest.CopyConfig(inConfig)
	config.ContentType = "application/json"
	config.AcceptContentTypes = "application/json"

	config.NegotiatedSerializer = newBasicNegotiatedSerializer()
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	return config
}

// New creates a new DynamicClient for the given RESTClient.
func New(c rest.Interface) *DynamicClient {
	return &DynamicClient{client: c}
}

// NewForConfigOrDie creates a new DynamicClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *DynamicClient {
	ret, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return ret
}

// NewForConfig creates a new dynamic client or returns an error.
// NewForConfig is equivalent to NewForConfigAndClient(c, httpClient),
// where httpClient was generated with rest.HTTPClientFor(c).
func NewForConfig(inConfig *rest.Config) (*DynamicClient, error) {
	config := ConfigFor(inConfig)

	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, err
	}
	return NewForConfigAndClient(config, httpClient)
}

// NewForConfigAndClient creates a new dynamic client for the given config and http client.
// Note the http client provided takes precedence over the configured transport values.
func NewForConfigAndClient(inConfig *rest.Config, h *http.Client) (*DynamicClient, error) {
	config := ConfigFor(inConfig)
	config.GroupVersion = nil
	config.APIPath = "/if-you-see-this-search-for-the-break"

	restClient, err := rest.UnversionedRESTClientForConfigAndClient(config, h)
	if err != nil {
		return nil, err
	}
	return &DynamicClient{client: restClient}, nil
}

type dynamicResourceClient struct {
	client    *DynamicClient
	namespace string
	resource  schema.GroupVersionResource
}

// Resource returns a dynamic resource client for the given resource.
func (c *DynamicClient) Resource(resource schema.GroupVersionResource) NamespaceableResourceInterface {
	return &dynamicResourceClient{client: c, resource: resource}
}

func (c *dynamicResourceClient) Namespace(ns string) ResourceInterface {
	ret := *c
	ret.namespace = ns
	return &ret
}

func (c *dynamicResourceClient) Create(ctx context.Context, obj *RawObject, opts metav1.CreateOptions, subresources ...string) (*RawObject, error) {
	if obj == nil {
		return nil, fmt.Errorf("object is required")
	}
	name := ""
	if len(subresources) > 0 {
		name = obj.GetName()
		if len(name) == 0 {
			return nil, fmt.Errorf("name is required")
		}
	}
	if err := validateNamespaceWithOptionalName(c.namespace, name); err != nil {
		return nil, err
	}

	body, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return rawObjectResult(c.client.client.
		Post().
		AbsPath(append(c.makeURLSegments(name), subresources...)...).
		Body(body).
		SetHeader("Content-Type", runtime.ContentTypeJSON).
		SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).
		Do(ctx).
		Raw())
}

func (c *dynamicResourceClient) Update(ctx context.Context, obj *RawObject, opts metav1.UpdateOptions, subresources ...string) (*RawObject, error) {
	if obj == nil {
		return nil, fmt.Errorf("object is required")
	}
	name := obj.GetName()
	if len(name) == 0 {
		return nil, fmt.Errorf("name is required")
	}
	if err := validateNamespaceWithOptionalName(c.namespace, name); err != nil {
		return nil, err
	}

	body, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return rawObjectResult(c.client.client.
		Put().
		AbsPath(append(c.makeURLSegments(name), subresources...)...).
		Body(body).
		SetHeader("Content-Type", runtime.ContentTypeJSON).
		SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).
		Do(ctx).
		Raw())
}

func (c *dynamicResourceClient) UpdateStatus(ctx context.Context, obj *RawObject, opts metav1.UpdateOptions) (*RawObject, error) {
	if obj == nil {
		return nil, fmt.Errorf("object is required")
	}
	name := obj.GetName()
	if len(name) == 0 {
		return nil, fmt.Errorf("name is required")
	}
	if err := validateNamespaceWithOptionalName(c.namespace, name); err != nil {
		return nil, err
	}

	body, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return rawObjectResult(c.client.client.
		Put().
		AbsPath(append(c.makeURLSegments(name), "status")...).
		Body(body).
		SetHeader("Content-Type", runtime.ContentTypeJSON).
		SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).
		Do(ctx).
		Raw())
}

func (c *dynamicResourceClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	if len(name) == 0 {
		return fmt.Errorf("name is required")
	}
	if err := validateNamespaceWithOptionalName(c.namespace, name); err != nil {
		return err
	}

	return c.client.client.
		Delete().
		AbsPath(append(c.makeURLSegments(name), subresources...)...).
		Body(&opts).
		Do(ctx).
		Error()
}

func (c *dynamicResourceClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	if err := validateNamespaceWithOptionalName(c.namespace); err != nil {
		return err
	}

	return c.client.client.
		Delete().
		AbsPath(c.makeURLSegments("")...).
		Body(&opts).
		SpecificallyVersionedParams(&listOptions, dynamicParameterCodec, versionV1).
		Do(ctx).
		Error()
}

func (c *dynamicResourceClient) Get(ctx context.Context, name string, opts metav1.GetOptions, subresources ...string) (*RawObject, error) {
	if len(name) == 0 {
		return nil, fmt.Errorf("name is required")
	}
	if err := validateNamespaceWithOptionalName(c.namespace, name); err != nil {
		return nil, err
	}

	return rawObjectResult(c.client.client.
		Get().
		AbsPath(append(c.makeURLSegments(name), subresources...)...).
		SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).
		Do(ctx).
		Raw())
}

func (c *dynamicResourceClient) List(ctx context.Context, opts metav1.ListOptions) (*RawObjectList, error) {
	if err := validateNamespaceWithOptionalName(c.namespace); err != nil {
		return nil, err
	}

	return rawObjectListResult(c.client.client.
		Get().
		AbsPath(c.makeURLSegments("")...).
		SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).
		Do(ctx).
		Raw())
}

func (c *dynamicResourceClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	if err := validateNamespaceWithOptionalName(c.namespace); err != nil {
		return nil, err
	}
	return c.client.client.
		Get().
		AbsPath(c.makeURLSegments("")...).
		SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).
		Watch(ctx)
}

func (c *dynamicResourceClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*RawObject, error) {
	if len(name) == 0 {
		return nil, fmt.Errorf("name is required")
	}
	if err := validateNamespaceWithOptionalName(c.namespace, name); err != nil {
		return nil, err
	}

	return rawObjectResult(c.client.client.
		Patch(pt).
		AbsPath(append(c.makeURLSegments(name), subresources...)...).
		Body(data).
		SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1).
		Do(ctx).
		Raw())
}

func (c *dynamicResourceClient) Apply(ctx context.Context, name string, obj *RawObject, opts metav1.ApplyOptions, subresources ...string) (*RawObject, error) {
	if len(name) == 0 {
		return nil, fmt.Errorf("name is required")
	}
	if err := validateNamespaceWithOptionalName(c.namespace, name); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, fmt.Errorf("object is required")
	}
	if len(obj.GetManagedFields()) > 0 {
		return nil, fmt.Errorf("cannot apply an object with managed fields already set")
	}
	body, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	patchOpts := opts.ToPatchOptions()

	return rawObjectResult(c.client.client.
		Patch(types.ApplyPatchType).
		AbsPath(append(c.makeURLSegments(name), subresources...)...).
		Body(body).
		SpecificallyVersionedParams(&patchOpts, dynamicParameterCodec, versionV1).
		Do(ctx).
		Raw())
}

func (c *dynamicResourceClient) ApplyStatus(ctx context.Context, name string, obj *RawObject, opts metav1.ApplyOptions) (*RawObject, error) {
	return c.Apply(ctx, name, obj, opts, "status")
}

func rawObjectResult(data []byte, err error) (*RawObject, error) {
	if err != nil {
		return nil, err
	}
	return NewRawObject(data)
}

func rawObjectListResult(data []byte, err error) (*RawObjectList, error) {
	if err != nil {
		return nil, err
	}
	return NewRawObjectList(data)
}

func validateNamespaceWithOptionalName(namespace string, name ...string) error {
	if msgs := rest.IsValidPathSegmentName(namespace); len(msgs) != 0 {
		return fmt.Errorf("invalid namespace %q: %v", namespace, msgs)
	}
	if len(name) > 1 {
		panic("invalid number of names")
	}
	if len(name) == 1 {
		if msgs := rest.IsValidPathSegmentName(name[0]); len(msgs) != 0 {
			return fmt.Errorf("invalid resource name %q: %v", name[0], msgs)
		}
	}
	return nil
}

func (c *dynamicResourceClient) makeURLSegments(name string) []string {
	url := []string{}
	if len(c.resource.Group) == 0 {
		url = append(url, "api")
	} else {
		url = append(url, "apis", c.resource.Group)
	}
	url = append(url, c.resource.Version)

	if len(c.namespace) > 0 {
		url = append(url, "namespaces", c.namespace)
	}
	url = append(url, c.resource.Resource)

	if len(name) > 0 {
		url = append(url, name)
	}
	return url
}
