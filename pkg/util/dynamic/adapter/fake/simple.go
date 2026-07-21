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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

	utildynamic "github.com/karmada-io/karmada/pkg/util/dynamic"
	utildynamicadapter "github.com/karmada-io/karmada/pkg/util/dynamic/adapter"
	utildynamicfake "github.com/karmada-io/karmada/pkg/util/dynamic/fake"
)

// FakeDynamicClient mirrors client-go's dynamic fake surface while carrying
// the Karmada dynamic fake used by raw dynamic informers.
//
//nolint:revive
type FakeDynamicClient struct {
	*dynamicfake.FakeDynamicClient
	dynamicClient *utildynamicfake.FakeDynamicClient
	informerMode  utildynamicadapter.InformerMode
}

var (
	_ utildynamicadapter.Interface = &FakeDynamicClient{}
	_ clienttesting.FakeClient     = &FakeDynamicClient{}
)

// NewSimpleDynamicClient creates a client-go dynamic fake that uses
// the Karmada dynamic fake for informer list/watch.
func NewSimpleDynamicClient(scheme *runtime.Scheme, objects ...runtime.Object) *FakeDynamicClient {
	client := dynamicfake.NewSimpleDynamicClient(scheme, toUnstructuredObjects(objects)...)
	dynamicClient := utildynamicfake.NewSimpleDynamicClient(scheme, objects...)
	return newFakeDynamicClient(client, dynamicClient)
}

// NewSimpleDynamicClientWithCustomListKinds creates a delegating fake dynamic client with custom list kinds.
func NewSimpleDynamicClientWithCustomListKinds(scheme *runtime.Scheme, gvrToListKind map[schema.GroupVersionResource]string, objects ...runtime.Object) *FakeDynamicClient {
	dynamicClient := utildynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...)
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, toUnstructuredObjects(objects)...)
	return newFakeDynamicClient(client, dynamicClient)
}

func newFakeDynamicClient(client *dynamicfake.FakeDynamicClient, dynamicClient *utildynamicfake.FakeDynamicClient) *FakeDynamicClient {
	mode := utildynamicadapter.InformerModeUnstructured
	if utildynamicadapter.DynamicInformerEnabled() {
		mode = utildynamicadapter.InformerModeRawDynamic
	}

	return &FakeDynamicClient{
		FakeDynamicClient: client,
		dynamicClient:     dynamicClient,
		informerMode:      mode,
	}
}

// InformerMode returns the selected informer mode.
func (c *FakeDynamicClient) InformerMode() utildynamicadapter.InformerMode {
	return c.informerMode
}

// DynamicClient returns the Karmada dynamic client used by the raw informer path.
func (c *FakeDynamicClient) DynamicClient() utildynamic.Interface {
	return c.dynamicClient
}

func toUnstructuredObjects(objects []runtime.Object) []runtime.Object {
	out := make([]runtime.Object, 0, len(objects))
	for _, obj := range objects {
		switch typed := obj.(type) {
		case *utildynamic.RawObject:
			u, err := toUnstructured(typed)
			if err != nil {
				panic(err)
			}
			out = append(out, u)
		default:
			out = append(out, obj)
		}
	}
	return out
}

func toUnstructured(obj *utildynamic.RawObject) (*unstructured.Unstructured, error) {
	u, err := obj.ToUnstructured()
	if err != nil {
		return nil, err
	}
	u.SetAPIVersion(obj.APIVersion)
	u.SetKind(obj.Kind)
	u.SetNamespace(obj.Namespace)
	u.SetName(obj.Name)
	u.SetGenerateName(obj.GenerateName)
	u.SetUID(obj.UID)
	u.SetResourceVersion(obj.ResourceVersion)
	u.SetLabels(obj.Labels)
	u.SetAnnotations(obj.Annotations)
	u.SetOwnerReferences(obj.OwnerReferences)
	u.SetFinalizers(obj.Finalizers)
	return u, nil
}
