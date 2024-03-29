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

	v1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	scheme "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ClusterOverridePoliciesGetter has a method to return a ClusterOverridePolicyInterface.
// A group's client should implement this interface.
type ClusterOverridePoliciesGetter interface {
	ClusterOverridePolicies() ClusterOverridePolicyInterface
}

// ClusterOverridePolicyInterface has methods to work with ClusterOverridePolicy resources.
type ClusterOverridePolicyInterface interface {
	Create(ctx context.Context, clusterOverridePolicy *v1alpha1.ClusterOverridePolicy, opts v1.CreateOptions) (*v1alpha1.ClusterOverridePolicy, error)
	Update(ctx context.Context, clusterOverridePolicy *v1alpha1.ClusterOverridePolicy, opts v1.UpdateOptions) (*v1alpha1.ClusterOverridePolicy, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.ClusterOverridePolicy, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ClusterOverridePolicyList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ClusterOverridePolicy, err error)
	ClusterOverridePolicyExpansion
}

// clusterOverridePolicies implements ClusterOverridePolicyInterface
type clusterOverridePolicies struct {
	client rest.Interface
}

// newClusterOverridePolicies returns a ClusterOverridePolicies
func newClusterOverridePolicies(c *PolicyV1alpha1Client) *clusterOverridePolicies {
	return &clusterOverridePolicies{
		client: c.RESTClient(),
	}
}

// Get takes name of the clusterOverridePolicy, and returns the corresponding clusterOverridePolicy object, and an error if there is any.
func (c *clusterOverridePolicies) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ClusterOverridePolicy, err error) {
	result = &v1alpha1.ClusterOverridePolicy{}
	err = c.client.Get().
		Resource("clusteroverridepolicies").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterOverridePolicies that match those selectors.
func (c *clusterOverridePolicies) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ClusterOverridePolicyList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ClusterOverridePolicyList{}
	err = c.client.Get().
		Resource("clusteroverridepolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterOverridePolicies.
func (c *clusterOverridePolicies) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("clusteroverridepolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a clusterOverridePolicy and creates it.  Returns the server's representation of the clusterOverridePolicy, and an error, if there is any.
func (c *clusterOverridePolicies) Create(ctx context.Context, clusterOverridePolicy *v1alpha1.ClusterOverridePolicy, opts v1.CreateOptions) (result *v1alpha1.ClusterOverridePolicy, err error) {
	result = &v1alpha1.ClusterOverridePolicy{}
	err = c.client.Post().
		Resource("clusteroverridepolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(clusterOverridePolicy).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a clusterOverridePolicy and updates it. Returns the server's representation of the clusterOverridePolicy, and an error, if there is any.
func (c *clusterOverridePolicies) Update(ctx context.Context, clusterOverridePolicy *v1alpha1.ClusterOverridePolicy, opts v1.UpdateOptions) (result *v1alpha1.ClusterOverridePolicy, err error) {
	result = &v1alpha1.ClusterOverridePolicy{}
	err = c.client.Put().
		Resource("clusteroverridepolicies").
		Name(clusterOverridePolicy.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(clusterOverridePolicy).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the clusterOverridePolicy and deletes it. Returns an error if one occurs.
func (c *clusterOverridePolicies) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("clusteroverridepolicies").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterOverridePolicies) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("clusteroverridepolicies").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched clusterOverridePolicy.
func (c *clusterOverridePolicies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ClusterOverridePolicy, err error) {
	result = &v1alpha1.ClusterOverridePolicy{}
	err = c.client.Patch(pt).
		Resource("clusteroverridepolicies").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
