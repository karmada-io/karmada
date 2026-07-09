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

package dynamiclister_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	"github.com/karmada-io/karmada/pkg/util/dynamic"
	"github.com/karmada-io/karmada/pkg/util/dynamic/dynamiclister"
)

func TestNamespaceGetMethod(t *testing.T) {
	tests := []struct {
		name            string
		existingObjects []runtime.Object
		namespaceToSync string
		gvrToSync       schema.GroupVersionResource
		objectToGet     string
		expectedObject  *dynamic.RawObject
		expectError     bool
	}{
		{
			name: "scenario 1: gets name-foo1 resource from the indexer from ns-foo namespace",
			existingObjects: []runtime.Object{
				newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
				newUnstructured("group/version", "TheKind", "ns-foo", "name-foo1"),
				newUnstructured("group/version", "TheKind", "ns-bar", "name-bar"),
			},
			namespaceToSync: "ns-foo",
			gvrToSync:       schema.GroupVersionResource{Group: "group", Version: "version", Resource: "TheKinds"},
			objectToGet:     "name-foo1",
			expectedObject:  newUnstructured("group/version", "TheKind", "ns-foo", "name-foo1"),
		},
		{
			name: "scenario 2: gets name-foo-non-existing resource from the indexer from ns-foo namespace",
			existingObjects: []runtime.Object{
				newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
				newUnstructured("group/version", "TheKind", "ns-foo", "name-foo1"),
				newUnstructured("group/version", "TheKind", "ns-bar", "name-bar"),
			},
			namespaceToSync: "ns-foo",
			gvrToSync:       schema.GroupVersionResource{Group: "group", Version: "version", Resource: "TheKinds"},
			objectToGet:     "name-foo-non-existing",
			expectError:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// test data
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			for _, obj := range test.existingObjects {
				err := indexer.Add(obj)
				if err != nil {
					t.Fatal(err)
				}
			}
			// act
			target := dynamiclister.New(indexer, test.gvrToSync).Namespace(test.namespaceToSync)
			actualObject, err := target.Get(test.objectToGet)

			// validate
			if test.expectError {
				if err == nil {
					t.Fatal("expected to get an error but non was returned")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertRawObjectEqual(t, test.expectedObject, actualObject)
		})
	}
}

func TestNamespaceListMethod(t *testing.T) {
	// test data
	objs := []runtime.Object{
		newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
		newUnstructured("group/version", "TheKind", "ns-foo", "name-foo1"),
		newUnstructured("group/version", "TheKind", "ns-bar", "name-bar"),
	}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, obj := range objs {
		err := indexer.Add(obj)
		if err != nil {
			t.Fatal(err)
		}
	}
	expectedOutput := []*dynamic.RawObject{
		newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
		newUnstructured("group/version", "TheKind", "ns-foo", "name-foo1"),
	}
	namespaceToList := "ns-foo"

	// act
	target := dynamiclister.New(indexer, schema.GroupVersionResource{Group: "group", Version: "version", Resource: "TheKinds"}).Namespace(namespaceToList)
	actualOutput, err := target.List(labels.Everything())

	// validate
	if err != nil {
		t.Fatal(err)
	}
	assertListOrDie(expectedOutput, actualOutput, t)
}

func TestListerGetMethod(t *testing.T) {
	tests := []struct {
		name            string
		existingObjects []runtime.Object
		namespaceToSync string
		gvrToSync       schema.GroupVersionResource
		objectToGet     string
		expectedObject  *dynamic.RawObject
		expectError     bool
	}{
		{
			name: "scenario 1: gets name-foo1 resource from the indexer",
			existingObjects: []runtime.Object{
				newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
				newUnstructured("group/version", "TheKind", "", "name-foo1"),
				newUnstructured("group/version", "TheKind", "ns-bar", "name-bar"),
			},
			namespaceToSync: "",
			gvrToSync:       schema.GroupVersionResource{Group: "group", Version: "version", Resource: "TheKinds"},
			objectToGet:     "name-foo1",
			expectedObject:  newUnstructured("group/version", "TheKind", "", "name-foo1"),
		},
		{
			name: "scenario 2: doesn't get name-foo resource from the indexer from ns-foo namespace",
			existingObjects: []runtime.Object{
				newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
				newUnstructured("group/version", "TheKind", "ns-foo", "name-foo1"),
				newUnstructured("group/version", "TheKind", "ns-bar", "name-bar"),
			},
			namespaceToSync: "ns-foo",
			gvrToSync:       schema.GroupVersionResource{Group: "group", Version: "version", Resource: "TheKinds"},
			objectToGet:     "name-foo",
			expectError:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// test data
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			for _, obj := range test.existingObjects {
				err := indexer.Add(obj)
				if err != nil {
					t.Fatal(err)
				}
			}
			// act
			target := dynamiclister.New(indexer, test.gvrToSync)
			actualObject, err := target.Get(test.objectToGet)

			// validate
			if test.expectError {
				if err == nil {
					t.Fatal("expected to get an error but non was returned")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertRawObjectEqual(t, test.expectedObject, actualObject)
		})
	}
}

func TestListerListMethod(t *testing.T) {
	// test data
	objs := []runtime.Object{
		newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
		newUnstructured("group/version", "TheKind", "ns-foo", "name-bar"),
	}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, obj := range objs {
		err := indexer.Add(obj)
		if err != nil {
			t.Fatal(err)
		}
	}
	expectedOutput := []*dynamic.RawObject{
		newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
		newUnstructured("group/version", "TheKind", "ns-foo", "name-bar"),
	}

	// act
	target := dynamiclister.New(indexer, schema.GroupVersionResource{Group: "group", Version: "version", Resource: "TheKinds"})
	actualOutput, err := target.List(labels.Everything())

	// validate
	if err != nil {
		t.Fatal(err)
	}
	assertListOrDie(expectedOutput, actualOutput, t)
}

func newUnstructured(apiVersion, kind, namespace, name string) *dynamic.RawObject {
	obj, err := dynamic.NewRawObject(fmt.Appendf(nil, `{"apiVersion":%q,"kind":%q,"metadata":{"namespace":%q,"name":%q}}`, apiVersion, kind, namespace, name))
	if err != nil {
		panic(err)
	}
	return obj
}

func assertListOrDie(expected []*dynamic.RawObject, actual []*dynamic.RawObject, t *testing.T) {
	if len(actual) != len(expected) {
		t.Fatalf("unexpected number of items returned, expected = %d, actual = %d", len(expected), len(actual))
	}
	for _, expectedObject := range expected {
		found := false
		for _, actualObject := range actual {
			if actualObject.GetName() == expectedObject.GetName() {
				assertRawObjectEqual(t, expectedObject, actualObject)
				found = true
			}
		}
		if !found {
			t.Fatalf("the resource with the name = %s was not found in the returned output", expectedObject.GetName())
		}
	}
}

func assertRawObjectEqual(t *testing.T, expected, actual *dynamic.RawObject) {
	t.Helper()
	if diff := cmp.Diff(rawObjectComparable(expected), rawObjectComparable(actual)); diff != "" {
		t.Fatalf("unexpected object diff (-want, +got): %s", diff)
	}
}

func rawObjectComparable(obj *dynamic.RawObject) any {
	if obj == nil {
		return nil
	}
	u, err := obj.ToUnstructured()
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	u.SetAPIVersion(obj.APIVersion)
	u.SetKind(obj.Kind)
	return u.UnstructuredContent()
}
