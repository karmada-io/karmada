// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/huawei-cloudnative/karmada/pkg/apis/propagationstrategy/v1alpha1"
	scheme "github.com/huawei-cloudnative/karmada/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// PropagationBindingsGetter has a method to return a PropagationBindingInterface.
// A group's client should implement this interface.
type PropagationBindingsGetter interface {
	PropagationBindings(namespace string) PropagationBindingInterface
}

// PropagationBindingInterface has methods to work with PropagationBinding resources.
type PropagationBindingInterface interface {
	Create(ctx context.Context, propagationBinding *v1alpha1.PropagationBinding, opts v1.CreateOptions) (*v1alpha1.PropagationBinding, error)
	Update(ctx context.Context, propagationBinding *v1alpha1.PropagationBinding, opts v1.UpdateOptions) (*v1alpha1.PropagationBinding, error)
	UpdateStatus(ctx context.Context, propagationBinding *v1alpha1.PropagationBinding, opts v1.UpdateOptions) (*v1alpha1.PropagationBinding, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.PropagationBinding, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.PropagationBindingList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.PropagationBinding, err error)
	PropagationBindingExpansion
}

// propagationBindings implements PropagationBindingInterface
type propagationBindings struct {
	client rest.Interface
	ns     string
}

// newPropagationBindings returns a PropagationBindings
func newPropagationBindings(c *PropagationstrategyV1alpha1Client, namespace string) *propagationBindings {
	return &propagationBindings{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the propagationBinding, and returns the corresponding propagationBinding object, and an error if there is any.
func (c *propagationBindings) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.PropagationBinding, err error) {
	result = &v1alpha1.PropagationBinding{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("propagationbindings").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of PropagationBindings that match those selectors.
func (c *propagationBindings) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.PropagationBindingList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.PropagationBindingList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("propagationbindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested propagationBindings.
func (c *propagationBindings) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("propagationbindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a propagationBinding and creates it.  Returns the server's representation of the propagationBinding, and an error, if there is any.
func (c *propagationBindings) Create(ctx context.Context, propagationBinding *v1alpha1.PropagationBinding, opts v1.CreateOptions) (result *v1alpha1.PropagationBinding, err error) {
	result = &v1alpha1.PropagationBinding{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("propagationbindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(propagationBinding).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a propagationBinding and updates it. Returns the server's representation of the propagationBinding, and an error, if there is any.
func (c *propagationBindings) Update(ctx context.Context, propagationBinding *v1alpha1.PropagationBinding, opts v1.UpdateOptions) (result *v1alpha1.PropagationBinding, err error) {
	result = &v1alpha1.PropagationBinding{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("propagationbindings").
		Name(propagationBinding.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(propagationBinding).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *propagationBindings) UpdateStatus(ctx context.Context, propagationBinding *v1alpha1.PropagationBinding, opts v1.UpdateOptions) (result *v1alpha1.PropagationBinding, err error) {
	result = &v1alpha1.PropagationBinding{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("propagationbindings").
		Name(propagationBinding.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(propagationBinding).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the propagationBinding and deletes it. Returns an error if one occurs.
func (c *propagationBindings) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("propagationbindings").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *propagationBindings) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("propagationbindings").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched propagationBinding.
func (c *propagationBindings) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.PropagationBinding, err error) {
	result = &v1alpha1.PropagationBinding{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("propagationbindings").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
