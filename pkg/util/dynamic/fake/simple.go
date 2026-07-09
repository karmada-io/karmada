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

package fake

import (
	"context"
	stdjson "encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/watch"
	clienttesting "k8s.io/client-go/testing"

	"github.com/karmada-io/karmada/pkg/util/dynamic"
)

// NewSimpleDynamicClient creates a fake Karmada dynamic client.
func NewSimpleDynamicClient(scheme *runtime.Scheme, objects ...runtime.Object) *FakeDynamicClient {
	rawObjectScheme := runtime.NewScheme()
	for gvk := range scheme.AllKnownTypes() {
		if rawObjectScheme.Recognizes(gvk) {
			continue
		}
		if strings.HasSuffix(gvk.Kind, "List") {
			rawObjectScheme.AddKnownTypeWithName(gvk, &dynamic.RawObjectList{})
			continue
		}
		rawObjectScheme.AddKnownTypeWithName(gvk, &dynamic.RawObject{})
	}

	objects, err := convertObjectsToRawObjects(scheme, objects)
	if err != nil {
		panic(err)
	}

	for _, obj := range objects {
		gvk := obj.GetObjectKind().GroupVersionKind()
		if !rawObjectScheme.Recognizes(gvk) {
			rawObjectScheme.AddKnownTypeWithName(gvk, &dynamic.RawObject{})
		}
		gvk.Kind += "List"
		if !rawObjectScheme.Recognizes(gvk) {
			rawObjectScheme.AddKnownTypeWithName(gvk, &dynamic.RawObjectList{})
		}
	}

	return NewSimpleDynamicClientWithCustomListKinds(rawObjectScheme, nil, objects...)
}

// NewSimpleDynamicClientWithCustomListKinds try not to use this. In general you want to have the scheme have the List types registered
// and allow the default guessing for resources match. Sometimes that doesn't work, so you can specify a custom mapping here.
func NewSimpleDynamicClientWithCustomListKinds(scheme *runtime.Scheme, gvrToListKind map[schema.GroupVersionResource]string, objects ...runtime.Object) *FakeDynamicClient {
	// In order to use List with this client, you have to have your lists registered so that the object tracker will find them
	// in the scheme to support the t.scheme.New(listGVK) call when it's building the return value.
	// Since the base fake client needs the listGVK passed through the action (in cases where there are no instances, it
	// cannot look up the actual hits), we need to know a mapping of GVR to listGVK here. For GETs and other types of calls,
	// there is no return value that contains a GVK, so it doesn't have to know the mapping in advance.

	// first we attempt to invert known List types from the scheme to auto guess the resource with unsafe guesses
	// this covers common usage of registering types in scheme and passing them
	completeGVRToListKind := map[schema.GroupVersionResource]string{}
	for listGVK := range scheme.AllKnownTypes() {
		if !strings.HasSuffix(listGVK.Kind, "List") {
			continue
		}
		nonListGVK := listGVK.GroupVersion().WithKind(listGVK.Kind[:len(listGVK.Kind)-4])
		plural, _ := meta.UnsafeGuessKindToResource(nonListGVK)
		completeGVRToListKind[plural] = listGVK.Kind
	}

	for gvr, listKind := range gvrToListKind {
		if !strings.HasSuffix(listKind, "List") {
			panic("coding error, listGVK must end in List or this fake client doesn't work right")
		}
		listGVK := gvr.GroupVersion().WithKind(listKind)

		// if we already have this type registered, just skip it
		if _, err := scheme.New(listGVK); err == nil {
			completeGVRToListKind[gvr] = listKind
			continue
		}

		scheme.AddKnownTypeWithName(listGVK, &dynamic.RawObjectList{})
		completeGVRToListKind[gvr] = listKind
	}

	codecs := serializer.NewCodecFactory(scheme)
	o := clienttesting.NewObjectTracker(scheme, codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := addObjectToTracker(o, completeGVRToListKind, obj); err != nil {
			panic(err)
		}
	}

	cs := &FakeDynamicClient{scheme: scheme, gvrToListKind: completeGVRToListKind, tracker: o}
	cs.AddReactor("*", "*", clienttesting.ObjectReaction(o))
	cs.AddWatchReactor("*", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
		var opts metav1.ListOptions
		if watchAction, ok := action.(clienttesting.WatchActionImpl); ok {
			opts = watchAction.ListOptions
		}
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := o.Watch(gvr, ns, opts)
		if err != nil {
			return false, nil, err
		}
		return true, watch, nil
	})

	return cs
}

// FakeDynamicClient implements dynamic.Interface
//
//nolint:revive
type FakeDynamicClient struct {
	clienttesting.Fake
	scheme        *runtime.Scheme
	gvrToListKind map[schema.GroupVersionResource]string
	tracker       clienttesting.ObjectTracker
}

type dynamicResourceClient struct {
	client    *FakeDynamicClient
	namespace string
	resource  schema.GroupVersionResource
	listKind  string
}

var (
	_ dynamic.Interface        = &FakeDynamicClient{}
	_ clienttesting.FakeClient = &FakeDynamicClient{}
)

// Tracker returns the underlying object tracker.
func (c *FakeDynamicClient) Tracker() clienttesting.ObjectTracker {
	return c.tracker
}

// Resource returns a dynamic resource client for the given resource.
func (c *FakeDynamicClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &dynamicResourceClient{client: c, resource: resource, listKind: c.gvrToListKind[resource]}
}

// IsWatchListSemanticsUnSupported reports whether watch-list semantics are unsupported.
func (c *FakeDynamicClient) IsWatchListSemanticsUnSupported() bool {
	return true
}

func (c *dynamicResourceClient) Namespace(ns string) dynamic.ResourceInterface {
	ret := *c
	ret.namespace = ns
	return &ret
}

func (c *dynamicResourceClient) Create(_ context.Context, obj *dynamic.RawObject, opts metav1.CreateOptions, subresources ...string) (*dynamic.RawObject, error) {
	var uncastRet runtime.Object
	var err error
	switch {
	case len(c.namespace) == 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewRootCreateActionWithOptions(c.resource, obj, opts), obj)

	case len(c.namespace) == 0 && len(subresources) > 0:
		var accessor metav1.Object
		accessor, err = meta.Accessor(obj)
		if err != nil {
			return nil, err
		}
		name := accessor.GetName()
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewRootCreateSubresourceActionWithOptions(c.resource, name, strings.Join(subresources, "/"), obj, opts), obj)

	case len(c.namespace) > 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewCreateActionWithOptions(c.resource, c.namespace, obj, opts), obj)

	case len(c.namespace) > 0 && len(subresources) > 0:
		var accessor metav1.Object
		accessor, err = meta.Accessor(obj)
		if err != nil {
			return nil, err
		}
		name := accessor.GetName()
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewCreateSubresourceActionWithOptions(c.resource, name, strings.Join(subresources, "/"), c.namespace, obj, opts), obj)
	}

	if err != nil {
		return nil, err
	}
	if uncastRet == nil {
		return nil, err
	}

	return convertToRawObject(c.client.scheme, uncastRet)
}

func (c *dynamicResourceClient) Update(_ context.Context, obj *dynamic.RawObject, opts metav1.UpdateOptions, subresources ...string) (*dynamic.RawObject, error) {
	var uncastRet runtime.Object
	var err error
	switch {
	case len(c.namespace) == 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewRootUpdateActionWithOptions(c.resource, obj, opts), obj)

	case len(c.namespace) == 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewRootUpdateSubresourceActionWithOptions(c.resource, strings.Join(subresources, "/"), obj, opts), obj)

	case len(c.namespace) > 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewUpdateActionWithOptions(c.resource, c.namespace, obj, opts), obj)

	case len(c.namespace) > 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewUpdateSubresourceActionWithOptions(c.resource, strings.Join(subresources, "/"), c.namespace, obj, opts), obj)
	}

	if err != nil {
		return nil, err
	}
	if uncastRet == nil {
		return nil, err
	}

	return convertToRawObject(c.client.scheme, uncastRet)
}

func (c *dynamicResourceClient) UpdateStatus(_ context.Context, obj *dynamic.RawObject, opts metav1.UpdateOptions) (*dynamic.RawObject, error) {
	var uncastRet runtime.Object
	var err error
	switch {
	case len(c.namespace) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewRootUpdateSubresourceActionWithOptions(c.resource, "status", obj, opts), obj)

	case len(c.namespace) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewUpdateSubresourceActionWithOptions(c.resource, "status", c.namespace, obj, opts), obj)
	}

	if err != nil {
		return nil, err
	}
	if uncastRet == nil {
		return nil, err
	}

	return convertToRawObject(c.client.scheme, uncastRet)
}

func (c *dynamicResourceClient) Delete(_ context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	var err error
	switch {
	case len(c.namespace) == 0 && len(subresources) == 0:
		_, err = c.client.Fake.
			Invokes(clienttesting.NewRootDeleteActionWithOptions(c.resource, name, opts), &metav1.Status{Status: "dynamic delete fail"})

	case len(c.namespace) == 0 && len(subresources) > 0:
		_, err = c.client.Fake.
			Invokes(clienttesting.NewRootDeleteSubresourceActionWithOptions(c.resource, strings.Join(subresources, "/"), name, opts), &metav1.Status{Status: "dynamic delete fail"})

	case len(c.namespace) > 0 && len(subresources) == 0:
		_, err = c.client.Fake.
			Invokes(clienttesting.NewDeleteActionWithOptions(c.resource, c.namespace, name, opts), &metav1.Status{Status: "dynamic delete fail"})

	case len(c.namespace) > 0 && len(subresources) > 0:
		_, err = c.client.Fake.
			Invokes(clienttesting.NewDeleteSubresourceActionWithOptions(c.resource, strings.Join(subresources, "/"), c.namespace, name, opts), &metav1.Status{Status: "dynamic delete fail"})
	}

	return err
}

func (c *dynamicResourceClient) DeleteCollection(_ context.Context, opts metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	var err error
	switch {
	case len(c.namespace) == 0:
		action := clienttesting.NewRootDeleteCollectionActionWithOptions(c.resource, opts, listOptions)
		_, err = c.client.Fake.Invokes(action, &metav1.Status{Status: "dynamic deletecollection fail"})

	case len(c.namespace) > 0:
		action := clienttesting.NewDeleteCollectionActionWithOptions(c.resource, c.namespace, opts, listOptions)
		_, err = c.client.Fake.Invokes(action, &metav1.Status{Status: "dynamic deletecollection fail"})
	}

	return err
}

func (c *dynamicResourceClient) Get(_ context.Context, name string, opts metav1.GetOptions, subresources ...string) (*dynamic.RawObject, error) {
	var uncastRet runtime.Object
	var err error
	switch {
	case len(c.namespace) == 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewRootGetActionWithOptions(c.resource, name, opts), &metav1.Status{Status: "dynamic get fail"})

	case len(c.namespace) == 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewRootGetSubresourceActionWithOptions(c.resource, strings.Join(subresources, "/"), name, opts), &metav1.Status{Status: "dynamic get fail"})

	case len(c.namespace) > 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewGetActionWithOptions(c.resource, c.namespace, name, opts), &metav1.Status{Status: "dynamic get fail"})

	case len(c.namespace) > 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewGetSubresourceActionWithOptions(c.resource, c.namespace, strings.Join(subresources, "/"), name, opts), &metav1.Status{Status: "dynamic get fail"})
	}

	if err != nil {
		return nil, err
	}
	if uncastRet == nil {
		return nil, err
	}

	return convertToRawObject(c.client.scheme, uncastRet)
}

func (c *dynamicResourceClient) List(_ context.Context, opts metav1.ListOptions) (*dynamic.RawObjectList, error) {
	if len(c.listKind) == 0 {
		panic(fmt.Sprintf("coding error: you must register resource to list kind for every resource you're going to LIST when creating the client.  See NewSimpleDynamicClientWithCustomListKinds or register the list into the scheme: %v out of %v", c.resource, c.client.gvrToListKind))
	}
	listGVK := c.resource.GroupVersion().WithKind(c.listKind)
	listForFakeClientGVK := c.resource.GroupVersion().WithKind(c.listKind[:len(c.listKind)-4]) /*base library appends List*/

	var obj runtime.Object
	var err error
	switch {
	case len(c.namespace) == 0:
		obj, err = c.client.Fake.
			Invokes(clienttesting.NewRootListActionWithOptions(c.resource, listForFakeClientGVK, opts), &metav1.Status{Status: "dynamic list fail"})

	case len(c.namespace) > 0:
		obj, err = c.client.Fake.
			Invokes(clienttesting.NewListActionWithOptions(c.resource, listForFakeClientGVK, c.namespace, opts), &metav1.Status{Status: "dynamic list fail"})
	}

	if obj == nil {
		return nil, err
	}

	label, _, _ := clienttesting.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}

	entireList, err := convertToRawObjectList(c.client.scheme, obj)
	if err != nil {
		return nil, err
	}

	list := &dynamic.RawObjectList{}
	list.SetRemainingItemCount(entireList.GetRemainingItemCount())
	list.SetResourceVersion(entireList.GetResourceVersion())
	list.SetContinue(entireList.GetContinue())
	list.GetObjectKind().SetGroupVersionKind(listGVK)
	for i := range entireList.Items {
		item := &entireList.Items[i]
		metadata, err := meta.Accessor(item)
		if err != nil {
			return nil, err
		}
		if label.Matches(labels.Set(metadata.GetLabels())) {
			list.Items = append(list.Items, *item)
		}
	}
	return list, nil
}

func (c *dynamicResourceClient) Watch(_ context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	switch {
	case len(c.namespace) == 0:
		return c.client.Fake.
			InvokesWatch(clienttesting.NewRootWatchActionWithOptions(c.resource, opts))

	case len(c.namespace) > 0:
		return c.client.Fake.
			InvokesWatch(clienttesting.NewWatchActionWithOptions(c.resource, c.namespace, opts))
	}

	panic("math broke")
}

// TODO: opts are currently ignored.
func (c *dynamicResourceClient) Patch(_ context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*dynamic.RawObject, error) {
	var uncastRet runtime.Object
	var err error
	switch {
	case len(c.namespace) == 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewRootPatchActionWithOptions(c.resource, name, pt, data, opts), &metav1.Status{Status: "dynamic patch fail"})

	case len(c.namespace) == 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewRootPatchSubresourceActionWithOptions(c.resource, name, pt, data, opts, subresources...), &metav1.Status{Status: "dynamic patch fail"})

	case len(c.namespace) > 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewPatchActionWithOptions(c.resource, c.namespace, name, pt, data, opts), &metav1.Status{Status: "dynamic patch fail"})

	case len(c.namespace) > 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewPatchSubresourceActionWithOptions(c.resource, c.namespace, name, pt, data, opts, subresources...), &metav1.Status{Status: "dynamic patch fail"})
	}

	if err != nil {
		return nil, err
	}
	if uncastRet == nil {
		return nil, err
	}

	return convertToRawObject(c.client.scheme, uncastRet)
}

// TODO: opts are currently ignored.
func (c *dynamicResourceClient) Apply(_ context.Context, name string, obj *dynamic.RawObject, options metav1.ApplyOptions, subresources ...string) (*dynamic.RawObject, error) {
	outBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	patchOptions := metav1.PatchOptions{
		Force:        &options.Force,
		DryRun:       options.DryRun,
		FieldManager: options.FieldManager,
	}
	var uncastRet runtime.Object
	switch {
	case len(c.namespace) == 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewRootPatchActionWithOptions(c.resource, name, types.ApplyPatchType, outBytes, patchOptions), &metav1.Status{Status: "dynamic patch fail"})

	case len(c.namespace) == 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewRootPatchSubresourceActionWithOptions(c.resource, name, types.ApplyPatchType, outBytes, patchOptions, subresources...), &metav1.Status{Status: "dynamic patch fail"})

	case len(c.namespace) > 0 && len(subresources) == 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewPatchActionWithOptions(c.resource, c.namespace, name, types.ApplyPatchType, outBytes, patchOptions), &metav1.Status{Status: "dynamic patch fail"})

	case len(c.namespace) > 0 && len(subresources) > 0:
		uncastRet, err = c.client.Fake.
			Invokes(clienttesting.NewPatchSubresourceActionWithOptions(c.resource, c.namespace, name, types.ApplyPatchType, outBytes, patchOptions, subresources...), &metav1.Status{Status: "dynamic patch fail"})
	}

	if err != nil {
		return nil, err
	}
	if uncastRet == nil {
		return nil, err
	}

	return convertToRawObject(c.client.scheme, uncastRet)
}

func (c *dynamicResourceClient) ApplyStatus(ctx context.Context, name string, obj *dynamic.RawObject, options metav1.ApplyOptions) (*dynamic.RawObject, error) {
	return c.Apply(ctx, name, obj, options, "status")
}

func addObjectToTracker(o clienttesting.ObjectTracker, gvrToListKind map[schema.GroupVersionResource]string, obj runtime.Object) error {
	switch typed := obj.(type) {
	case *dynamic.RawObject:
		return createRawObject(o, gvrToListKind, typed)
	case *dynamic.RawObjectList:
		items, err := meta.ExtractListWithAlloc(typed)
		if err != nil {
			return err
		}
		for _, item := range items {
			raw, ok := item.(*dynamic.RawObject)
			if !ok {
				return fmt.Errorf("expected *dynamic.RawObject in RawObjectList, got %T", item)
			}
			if err := createRawObject(o, gvrToListKind, raw); err != nil {
				return err
			}
		}
		return nil
	default:
		return o.Add(obj)
	}
}

func createRawObject(o clienttesting.ObjectTracker, gvrToListKind map[schema.GroupVersionResource]string, obj *dynamic.RawObject) error {
	return o.Create(resourceForRawObject(gvrToListKind, obj.GroupVersionKind()), obj, obj.GetNamespace())
}

func resourceForRawObject(gvrToListKind map[schema.GroupVersionResource]string, gvk schema.GroupVersionKind) schema.GroupVersionResource {
	for gvr, listKind := range gvrToListKind {
		if gvr.Group == gvk.Group && gvr.Version == gvk.Version && strings.TrimSuffix(listKind, "List") == gvk.Kind {
			return gvr
		}
	}
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	return gvr
}

func convertObjectsToRawObjects(s *runtime.Scheme, objs []runtime.Object) ([]runtime.Object, error) {
	ul := make([]runtime.Object, 0, len(objs))

	for _, obj := range objs {
		u, err := convertToRawObject(s, obj)
		if err != nil {
			return nil, err
		}

		ul = append(ul, u)
	}
	return ul, nil
}

func convertToRawObject(s *runtime.Scheme, obj runtime.Object) (*dynamic.RawObject, error) {
	if raw, ok := obj.(*dynamic.RawObject); ok {
		return raw.DeepCopy(), nil
	}

	content, gvk, err := objectJSONAndKind(s, obj)
	if err != nil {
		return nil, err
	}
	apiVersion, kind := gvk.ToAPIVersionAndKind()
	apiVersionRaw, err := json.Marshal(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to raw object: %w", err)
	}
	kindRaw, err := json.Marshal(kind)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to raw object: %w", err)
	}
	content["apiVersion"] = stdjson.RawMessage(apiVersionRaw)
	content["kind"] = stdjson.RawMessage(kindRaw)

	raw, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to raw object: %w", err)
	}
	return dynamic.NewRawObject(raw)
}

func convertToRawObjectList(s *runtime.Scheme, obj runtime.Object) (*dynamic.RawObjectList, error) {
	if rawList, ok := obj.(*dynamic.RawObjectList); ok {
		return rawList.DeepCopy(), nil
	}

	content, gvk, err := objectJSONAndKind(s, obj)
	if err != nil {
		return nil, err
	}
	apiVersion, kind := gvk.ToAPIVersionAndKind()
	apiVersionRaw, err := json.Marshal(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to raw object list: %w", err)
	}
	kindRaw, err := json.Marshal(kind)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to raw object list: %w", err)
	}
	content["apiVersion"] = stdjson.RawMessage(apiVersionRaw)
	content["kind"] = stdjson.RawMessage(kindRaw)

	raw, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to raw object list: %w", err)
	}
	return dynamic.NewRawObjectList(raw)
}

func objectJSONAndKind(s *runtime.Scheme, obj runtime.Object) (map[string]stdjson.RawMessage, schema.GroupVersionKind, error) {
	raw, err := json.Marshal(obj)
	if err != nil {
		return nil, schema.GroupVersionKind{}, fmt.Errorf("failed to convert to raw object: %w", err)
	}
	content := map[string]stdjson.RawMessage{}
	if err := json.Unmarshal(raw, &content); err != nil {
		return nil, schema.GroupVersionKind{}, fmt.Errorf("failed to convert to raw object: %w", err)
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Group == "" || gvk.Kind == "" {
		gvks, _, err := s.ObjectKinds(obj)
		if err != nil {
			return nil, schema.GroupVersionKind{}, fmt.Errorf("failed to convert to raw object - unable to get GVK %w", err)
		}
		gvk = gvks[0]
	}
	return content, gvk, nil
}
