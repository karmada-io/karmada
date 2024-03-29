/*
Copyright The Karmada Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	scheme "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ResourceInterpreterWebhookConfigurationsGetter has a method to return a ResourceInterpreterWebhookConfigurationInterface.
// A group's client should implement this interface.
type ResourceInterpreterWebhookConfigurationsGetter interface {
	ResourceInterpreterWebhookConfigurations() ResourceInterpreterWebhookConfigurationInterface
}

// ResourceInterpreterWebhookConfigurationInterface has methods to work with ResourceInterpreterWebhookConfiguration resources.
type ResourceInterpreterWebhookConfigurationInterface interface {
	Create(ctx context.Context, resourceInterpreterWebhookConfiguration *v1alpha1.ResourceInterpreterWebhookConfiguration, opts v1.CreateOptions) (*v1alpha1.ResourceInterpreterWebhookConfiguration, error)
	Update(ctx context.Context, resourceInterpreterWebhookConfiguration *v1alpha1.ResourceInterpreterWebhookConfiguration, opts v1.UpdateOptions) (*v1alpha1.ResourceInterpreterWebhookConfiguration, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.ResourceInterpreterWebhookConfiguration, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ResourceInterpreterWebhookConfigurationList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ResourceInterpreterWebhookConfiguration, err error)
	ResourceInterpreterWebhookConfigurationExpansion
}

// resourceInterpreterWebhookConfigurations implements ResourceInterpreterWebhookConfigurationInterface
type resourceInterpreterWebhookConfigurations struct {
	client rest.Interface
}

// newResourceInterpreterWebhookConfigurations returns a ResourceInterpreterWebhookConfigurations
func newResourceInterpreterWebhookConfigurations(c *ConfigV1alpha1Client) *resourceInterpreterWebhookConfigurations {
	return &resourceInterpreterWebhookConfigurations{
		client: c.RESTClient(),
	}
}

// Get takes name of the resourceInterpreterWebhookConfiguration, and returns the corresponding resourceInterpreterWebhookConfiguration object, and an error if there is any.
func (c *resourceInterpreterWebhookConfigurations) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ResourceInterpreterWebhookConfiguration, err error) {
	result = &v1alpha1.ResourceInterpreterWebhookConfiguration{}
	err = c.client.Get().
		Resource("resourceinterpreterwebhookconfigurations").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ResourceInterpreterWebhookConfigurations that match those selectors.
func (c *resourceInterpreterWebhookConfigurations) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ResourceInterpreterWebhookConfigurationList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ResourceInterpreterWebhookConfigurationList{}
	err = c.client.Get().
		Resource("resourceinterpreterwebhookconfigurations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested resourceInterpreterWebhookConfigurations.
func (c *resourceInterpreterWebhookConfigurations) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("resourceinterpreterwebhookconfigurations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a resourceInterpreterWebhookConfiguration and creates it.  Returns the server's representation of the resourceInterpreterWebhookConfiguration, and an error, if there is any.
func (c *resourceInterpreterWebhookConfigurations) Create(ctx context.Context, resourceInterpreterWebhookConfiguration *v1alpha1.ResourceInterpreterWebhookConfiguration, opts v1.CreateOptions) (result *v1alpha1.ResourceInterpreterWebhookConfiguration, err error) {
	result = &v1alpha1.ResourceInterpreterWebhookConfiguration{}
	err = c.client.Post().
		Resource("resourceinterpreterwebhookconfigurations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(resourceInterpreterWebhookConfiguration).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a resourceInterpreterWebhookConfiguration and updates it. Returns the server's representation of the resourceInterpreterWebhookConfiguration, and an error, if there is any.
func (c *resourceInterpreterWebhookConfigurations) Update(ctx context.Context, resourceInterpreterWebhookConfiguration *v1alpha1.ResourceInterpreterWebhookConfiguration, opts v1.UpdateOptions) (result *v1alpha1.ResourceInterpreterWebhookConfiguration, err error) {
	result = &v1alpha1.ResourceInterpreterWebhookConfiguration{}
	err = c.client.Put().
		Resource("resourceinterpreterwebhookconfigurations").
		Name(resourceInterpreterWebhookConfiguration.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(resourceInterpreterWebhookConfiguration).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the resourceInterpreterWebhookConfiguration and deletes it. Returns an error if one occurs.
func (c *resourceInterpreterWebhookConfigurations) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("resourceinterpreterwebhookconfigurations").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *resourceInterpreterWebhookConfigurations) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("resourceinterpreterwebhookconfigurations").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched resourceInterpreterWebhookConfiguration.
func (c *resourceInterpreterWebhookConfigurations) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ResourceInterpreterWebhookConfiguration, err error) {
	result = &v1alpha1.ResourceInterpreterWebhookConfiguration{}
	err = c.client.Patch(pt).
		Resource("resourceinterpreterwebhookconfigurations").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
