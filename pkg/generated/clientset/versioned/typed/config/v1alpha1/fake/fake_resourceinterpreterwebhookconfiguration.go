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

package fake

import (
	"context"

	v1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeResourceInterpreterWebhookConfigurations implements ResourceInterpreterWebhookConfigurationInterface
type FakeResourceInterpreterWebhookConfigurations struct {
	Fake *FakeConfigV1alpha1
}

var resourceinterpreterwebhookconfigurationsResource = v1alpha1.SchemeGroupVersion.WithResource("resourceinterpreterwebhookconfigurations")

var resourceinterpreterwebhookconfigurationsKind = v1alpha1.SchemeGroupVersion.WithKind("ResourceInterpreterWebhookConfiguration")

// Get takes name of the resourceInterpreterWebhookConfiguration, and returns the corresponding resourceInterpreterWebhookConfiguration object, and an error if there is any.
func (c *FakeResourceInterpreterWebhookConfigurations) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ResourceInterpreterWebhookConfiguration, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(resourceinterpreterwebhookconfigurationsResource, name), &v1alpha1.ResourceInterpreterWebhookConfiguration{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ResourceInterpreterWebhookConfiguration), err
}

// List takes label and field selectors, and returns the list of ResourceInterpreterWebhookConfigurations that match those selectors.
func (c *FakeResourceInterpreterWebhookConfigurations) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ResourceInterpreterWebhookConfigurationList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(resourceinterpreterwebhookconfigurationsResource, resourceinterpreterwebhookconfigurationsKind, opts), &v1alpha1.ResourceInterpreterWebhookConfigurationList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ResourceInterpreterWebhookConfigurationList{ListMeta: obj.(*v1alpha1.ResourceInterpreterWebhookConfigurationList).ListMeta}
	for _, item := range obj.(*v1alpha1.ResourceInterpreterWebhookConfigurationList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested resourceInterpreterWebhookConfigurations.
func (c *FakeResourceInterpreterWebhookConfigurations) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(resourceinterpreterwebhookconfigurationsResource, opts))
}

// Create takes the representation of a resourceInterpreterWebhookConfiguration and creates it.  Returns the server's representation of the resourceInterpreterWebhookConfiguration, and an error, if there is any.
func (c *FakeResourceInterpreterWebhookConfigurations) Create(ctx context.Context, resourceInterpreterWebhookConfiguration *v1alpha1.ResourceInterpreterWebhookConfiguration, opts v1.CreateOptions) (result *v1alpha1.ResourceInterpreterWebhookConfiguration, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(resourceinterpreterwebhookconfigurationsResource, resourceInterpreterWebhookConfiguration), &v1alpha1.ResourceInterpreterWebhookConfiguration{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ResourceInterpreterWebhookConfiguration), err
}

// Update takes the representation of a resourceInterpreterWebhookConfiguration and updates it. Returns the server's representation of the resourceInterpreterWebhookConfiguration, and an error, if there is any.
func (c *FakeResourceInterpreterWebhookConfigurations) Update(ctx context.Context, resourceInterpreterWebhookConfiguration *v1alpha1.ResourceInterpreterWebhookConfiguration, opts v1.UpdateOptions) (result *v1alpha1.ResourceInterpreterWebhookConfiguration, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(resourceinterpreterwebhookconfigurationsResource, resourceInterpreterWebhookConfiguration), &v1alpha1.ResourceInterpreterWebhookConfiguration{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ResourceInterpreterWebhookConfiguration), err
}

// Delete takes name of the resourceInterpreterWebhookConfiguration and deletes it. Returns an error if one occurs.
func (c *FakeResourceInterpreterWebhookConfigurations) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(resourceinterpreterwebhookconfigurationsResource, name, opts), &v1alpha1.ResourceInterpreterWebhookConfiguration{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeResourceInterpreterWebhookConfigurations) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(resourceinterpreterwebhookconfigurationsResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.ResourceInterpreterWebhookConfigurationList{})
	return err
}

// Patch applies the patch and returns the patched resourceInterpreterWebhookConfiguration.
func (c *FakeResourceInterpreterWebhookConfigurations) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ResourceInterpreterWebhookConfiguration, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(resourceinterpreterwebhookconfigurationsResource, name, pt, data, subresources...), &v1alpha1.ResourceInterpreterWebhookConfiguration{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ResourceInterpreterWebhookConfiguration), err
}
