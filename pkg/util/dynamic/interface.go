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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

// Interface is a dynamic client that returns RawObject instances instead of
// unstructured.Unstructured.
type Interface interface {
	Resource(resource schema.GroupVersionResource) NamespaceableResourceInterface
}

// ResourceInterface has the same shape as dynamic.ResourceInterface, but the
// object payload type is RawObject.
type ResourceInterface interface {
	Create(ctx context.Context, obj *RawObject, options metav1.CreateOptions, subresources ...string) (*RawObject, error)
	Update(ctx context.Context, obj *RawObject, options metav1.UpdateOptions, subresources ...string) (*RawObject, error)
	UpdateStatus(ctx context.Context, obj *RawObject, options metav1.UpdateOptions) (*RawObject, error)
	Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error
	DeleteCollection(ctx context.Context, options metav1.DeleteOptions, listOptions metav1.ListOptions) error
	Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*RawObject, error)
	List(ctx context.Context, opts metav1.ListOptions) (*RawObjectList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*RawObject, error)
	Apply(ctx context.Context, name string, obj *RawObject, options metav1.ApplyOptions, subresources ...string) (*RawObject, error)
	ApplyStatus(ctx context.Context, name string, obj *RawObject, options metav1.ApplyOptions) (*RawObject, error)
}

// NamespaceableResourceInterface allows selecting a namespace before issuing
// resource operations.
type NamespaceableResourceInterface interface {
	Namespace(string) ResourceInterface
	ResourceInterface
}
