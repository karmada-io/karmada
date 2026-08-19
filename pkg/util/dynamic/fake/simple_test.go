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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/watchlist"

	"github.com/karmada-io/karmada/pkg/util/dynamic"
)

const (
	testGroup      = "testgroup"
	testVersion    = "testversion"
	testResource   = "testkinds"
	testNamespace  = "testns"
	testName       = "testname"
	testKind       = "TestKind"
	testAPIVersion = "testgroup/testversion"
)

func newUnstructured(apiVersion, kind, namespace, name string) *dynamic.RawObject {
	return newRawObject(fmt.Appendf(nil, `{"apiVersion":%q,"kind":%q,"metadata":{"namespace":%q,"name":%q}}`, apiVersion, kind, namespace, name))
}

func newUnstructuredWithSpec(spec map[string]any) *dynamic.RawObject {
	raw, err := json.Marshal(map[string]any{
		"apiVersion": testAPIVersion,
		"kind":       testKind,
		"metadata": map[string]any{
			"namespace": testNamespace,
			"name":      testName,
		},
		"spec": spec,
	})
	if err != nil {
		panic(err)
	}
	return newRawObject(raw)
}

func TestDoesClientSupportWatchListSemantics(t *testing.T) {
	target := &FakeDynamicClient{}
	if !watchlist.DoesClientNotSupportWatchListSemantics(target) {
		t.Fatalf("FakeDynamicClient should NOT support WatchList semantics")
	}
}

func TestGet(t *testing.T) {
	scheme := runtime.NewScheme()

	client := NewSimpleDynamicClient(scheme, newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"))
	get, err := client.Resource(schema.GroupVersionResource{Group: "group", Version: "version", Resource: "thekinds"}).Namespace("ns-foo").Get(context.TODO(), "name-foo", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	expected := newUnstructured("group/version", "TheKind", "ns-foo", "name-foo")
	assertRawObjectEqual(t, expected, get)
}

func TestListDecoding(t *testing.T) {
	// this the duplication of logic from the real List API. This will prove that our dynamic client actually returns the gvk
	list, err := dynamic.NewRawObjectList([]byte(`{"apiVersion": "group/version", "kind": "TheKindList", "items":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	expectedList := &dynamic.RawObjectList{
		TypeMeta: metav1.TypeMeta{APIVersion: "group/version", Kind: "TheKindList"},
		Items:    []dynamic.RawObject{},
	}
	assertRawObjectListEqual(t, expectedList, list)
}

func TestGetDecoding(t *testing.T) {
	// this the duplication of logic from the real Get API. This will prove that our dynamic client actually returns the gvk
	get, err := dynamic.NewRawObject([]byte(`{"apiVersion": "group/version", "kind": "TheKind"}`))
	if err != nil {
		t.Fatal(err)
	}
	expectedObj := newRawObject([]byte(`{"apiVersion": "group/version", "kind": "TheKind"}`))
	assertRawObjectEqual(t, expectedObj, get)
}

func TestList(t *testing.T) {
	scheme := runtime.NewScheme()

	client := NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "group", Version: "version", Resource: "thekinds"}: "TheKindList",
		},
		newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
		newUnstructured("group2/version", "TheKind", "ns-foo", "name2-foo"),
		newUnstructured("group/version", "TheKind", "ns-foo", "name-bar"),
		newUnstructured("group/version", "TheKind", "ns-foo", "name-baz"),
		newUnstructured("group2/version", "TheKind", "ns-foo", "name2-baz"),
	)
	listFirst, err := client.Resource(schema.GroupVersionResource{Group: "group", Version: "version", Resource: "thekinds"}).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	expected := []dynamic.RawObject{
		*newUnstructured("group/version", "TheKind", "ns-foo", "name-bar"),
		*newUnstructured("group/version", "TheKind", "ns-foo", "name-baz"),
		*newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
	}
	assertRawObjectSliceEqual(t, expected, listFirst.Items)
}

func Test_ListKind(t *testing.T) {
	scheme := runtime.NewScheme()

	client := NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "group", Version: "version", Resource: "thekinds"}: "TheKindList",
		},
		&dynamic.RawObjectList{
			TypeMeta: metav1.TypeMeta{APIVersion: "group/version", Kind: "TheKindList"},
			Items: []dynamic.RawObject{
				*newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
				*newUnstructured("group/version", "TheKind", "ns-foo", "name-bar"),
				*newUnstructured("group/version", "TheKind", "ns-foo", "name-baz"),
			},
		},
	)
	listFirst, err := client.Resource(schema.GroupVersionResource{Group: "group", Version: "version", Resource: "thekinds"}).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	expectedList := &dynamic.RawObjectList{
		TypeMeta: metav1.TypeMeta{APIVersion: "group/version", Kind: "TheKindList"},
		ListMeta: metav1.ListMeta{
			Continue:        "",
			ResourceVersion: "4", // Three objects created so far, starting value is 1.
		},
		Items: []dynamic.RawObject{
			*newUnstructured("group/version", "TheKind", "ns-foo", "name-bar"),
			*newUnstructured("group/version", "TheKind", "ns-foo", "name-baz"),
			*newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
		},
	}
	assertRawObjectListEqual(t, expectedList, listFirst)
}

type patchTestCase struct {
	name                  string
	object                runtime.Object
	patchType             types.PatchType
	patchBytes            []byte
	wantErrMsg            string
	expectedPatchedObject *dynamic.RawObject
}

func (tc *patchTestCase) runner(t *testing.T) {
	client := NewSimpleDynamicClient(runtime.NewScheme(), tc.object)
	resourceInterface := client.Resource(schema.GroupVersionResource{Group: testGroup, Version: testVersion, Resource: testResource}).Namespace(testNamespace)

	got, recErr := resourceInterface.Patch(context.TODO(), testName, tc.patchType, tc.patchBytes, metav1.PatchOptions{})

	if err := tc.verifyErr(recErr); err != nil {
		t.Error(err)
	}

	if err := tc.verifyResult(got); err != nil {
		t.Error(err)
	}
}

// verifyErr verifies that the given error returned from Patch is the error
// expected by the test case.
func (tc *patchTestCase) verifyErr(err error) error {
	if tc.wantErrMsg != "" && err == nil {
		return fmt.Errorf("want error, got nil")
	}

	if tc.wantErrMsg == "" && err != nil {
		return fmt.Errorf("want no error, got %v", err)
	}

	if err != nil {
		if want, got := tc.wantErrMsg, err.Error(); want != got {
			return fmt.Errorf("incorrect error: want: %q got: %q", want, got)
		}
	}
	return nil
}

func (tc *patchTestCase) verifyResult(result *dynamic.RawObject) error {
	if tc.expectedPatchedObject == nil && result == nil {
		return nil
	}
	if !rawObjectSemanticDeepEqual(tc.expectedPatchedObject, result) {
		return fmt.Errorf("unexpected diff in received object: %s", cmp.Diff(rawObjectComparable(tc.expectedPatchedObject), rawObjectComparable(result)))
	}
	return nil
}

func TestPatch(t *testing.T) {
	testCases := []patchTestCase{
		{
			name:       "jsonpatch fails with merge type",
			object:     newUnstructuredWithSpec(map[string]any{"foo": "bar"}),
			patchType:  types.StrategicMergePatchType,
			patchBytes: []byte(`[]`),
			wantErrMsg: "invalid JSON document",
		}, {
			name:      "jsonpatch works with empty patch",
			object:    newUnstructuredWithSpec(map[string]any{"foo": "bar"}),
			patchType: types.JSONPatchType,
			// No-op
			patchBytes:            []byte(`[]`),
			expectedPatchedObject: newUnstructuredWithSpec(map[string]any{"foo": "bar"}),
		}, {
			name:      "jsonpatch works with simple change patch",
			object:    newUnstructuredWithSpec(map[string]any{"foo": "bar"}),
			patchType: types.JSONPatchType,
			// change spec.foo from bar to foobar
			patchBytes:            []byte(`[{"op": "replace", "path": "/spec/foo", "value": "foobar"}]`),
			expectedPatchedObject: newUnstructuredWithSpec(map[string]any{"foo": "foobar"}),
		}, {
			name:      "jsonpatch works with simple addition",
			object:    newUnstructuredWithSpec(map[string]any{"foo": "bar"}),
			patchType: types.JSONPatchType,
			// add spec.newvalue = dummy
			patchBytes:            []byte(`[{"op": "add", "path": "/spec/newvalue", "value": "dummy"}]`),
			expectedPatchedObject: newUnstructuredWithSpec(map[string]any{"foo": "bar", "newvalue": "dummy"}),
		}, {
			name:      "jsonpatch works with simple deletion",
			object:    newUnstructuredWithSpec(map[string]any{"foo": "bar", "toremove": "shouldnotbehere"}),
			patchType: types.JSONPatchType,
			// remove spec.newvalue = dummy
			patchBytes:            []byte(`[{"op": "remove", "path": "/spec/toremove"}]`),
			expectedPatchedObject: newUnstructuredWithSpec(map[string]any{"foo": "bar"}),
		}, {
			name:      "strategic merge patch fails with JSONPatch",
			object:    newUnstructuredWithSpec(map[string]any{"foo": "bar"}),
			patchType: types.StrategicMergePatchType,
			// add spec.newvalue = dummy
			patchBytes: []byte(`[{"op": "add", "path": "/spec/newvalue", "value": "dummy"}]`),
			wantErrMsg: "invalid JSON document",
		}, {
			name:                  "merge patch works with simple replacement",
			object:                newUnstructuredWithSpec(map[string]any{"foo": "bar"}),
			patchType:             types.MergePatchType,
			patchBytes:            []byte(`{ "spec": { "foo": "baz" } }`),
			expectedPatchedObject: newUnstructuredWithSpec(map[string]any{"foo": "baz"}),
		},
		// TODO: Add tests for strategic merge using v1.Pod for example to ensure the test cases
		// demonstrate expected use cases.
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.runner)
	}
}

// This test ensures list works when the fake dynamic client is seeded with a typed scheme and
// unstructured type fixtures
func TestListWithUnstructuredObjectsAndTypedScheme(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: testGroup, Version: testVersion, Resource: testResource}
	gvk := gvr.GroupVersion().WithKind(testKind)

	listGVK := gvk
	listGVK.Kind += "List"

	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName("name")
	u.SetNamespace("namespace")

	typedScheme := runtime.NewScheme()
	typedScheme.AddKnownTypeWithName(gvk, &mockResource{})
	typedScheme.AddKnownTypeWithName(listGVK, &mockResourceList{})

	client := NewSimpleDynamicClient(typedScheme, &u)
	list, err := client.Resource(gvr).Namespace("namespace").List(context.Background(), metav1.ListOptions{})

	if err != nil {
		t.Error("error listing", err)
	}

	expectedList := newRawObjectList(listGVK, newUnstructured(testAPIVersion, testKind, "namespace", "name"))
	expectedList.SetResourceVersion("2") // One object created so far, initial value is 1.
	expectedList.SetContinue("")

	assertRawObjectListEqual(t, expectedList, list)
}

func TestListWithNoFixturesAndTypedScheme(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: testGroup, Version: testVersion, Resource: testResource}
	gvk := gvr.GroupVersion().WithKind(testKind)

	listGVK := gvk
	listGVK.Kind += "List"

	typedScheme := runtime.NewScheme()
	typedScheme.AddKnownTypeWithName(gvk, &mockResource{})
	typedScheme.AddKnownTypeWithName(listGVK, &mockResourceList{})

	client := NewSimpleDynamicClient(typedScheme)
	list, err := client.Resource(gvr).Namespace("namespace").List(context.Background(), metav1.ListOptions{})

	if err != nil {
		t.Error("error listing", err)
	}

	expectedList := newRawObjectList(listGVK)
	expectedList.SetResourceVersion("1") // No objects created so far.
	expectedList.SetContinue("")

	assertRawObjectListEqual(t, expectedList, list)
}

// This test ensures list works when the dynamic client is seeded with an empty scheme and
// unstructured typed fixtures
func TestListWithNoScheme(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: testGroup, Version: testVersion, Resource: testResource}
	gvk := gvr.GroupVersion().WithKind(testKind)

	listGVK := gvk
	listGVK.Kind += "List"

	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName("name")
	u.SetNamespace("namespace")

	emptyScheme := runtime.NewScheme()

	client := NewSimpleDynamicClient(emptyScheme, &u)
	list, err := client.Resource(gvr).Namespace("namespace").List(context.Background(), metav1.ListOptions{})

	if err != nil {
		t.Error("error listing", err)
	}

	expectedList := newRawObjectList(listGVK, newUnstructured(testAPIVersion, testKind, "namespace", "name"))
	expectedList.SetResourceVersion("2") // One object created so far, initial value is 1.
	expectedList.SetContinue("")

	assertRawObjectListEqual(t, expectedList, list)
}

// This test ensures list works when the dynamic client is seeded with an empty scheme and
// unstructured typed fixtures
func TestListWithTypedFixtures(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: testGroup, Version: testVersion, Resource: testResource}
	gvk := gvr.GroupVersion().WithKind(testKind)

	listGVK := gvk
	listGVK.Kind += "List"

	r := mockResource{}
	r.SetGroupVersionKind(gvk)
	r.SetName("name")
	r.SetNamespace("namespace")

	typedScheme := runtime.NewScheme()
	typedScheme.AddKnownTypeWithName(gvk, &mockResource{})
	typedScheme.AddKnownTypeWithName(listGVK, &mockResourceList{})

	client := NewSimpleDynamicClient(typedScheme, &r)
	list, err := client.Resource(gvr).Namespace("namespace").List(context.Background(), metav1.ListOptions{})

	if err != nil {
		t.Error("error listing", err)
	}

	expectedList := newRawObjectList(listGVK, newUnstructured(testAPIVersion, testKind, "namespace", "name"))
	expectedList.SetResourceVersion("2") // One object created so far, initial value is 1.
	expectedList.SetContinue("")

	assertRawObjectListEqual(t, expectedList, list)
}

type (
	mockResource struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata"`
	}
	mockResourceList struct {
		metav1.TypeMeta `json:",inline"`
		metav1.ListMeta `json:"metadata"`

		Items []mockResource
	}
)

func (l *mockResourceList) DeepCopyObject() runtime.Object {
	o := *l
	return &o
}

func (r *mockResource) DeepCopyObject() runtime.Object {
	o := *r
	return &o
}

var _ runtime.Object = (*mockResource)(nil)
var _ runtime.Object = (*mockResourceList)(nil)

func newRawObject(raw []byte) *dynamic.RawObject {
	obj, err := dynamic.NewRawObject(raw)
	if err != nil {
		panic(err)
	}
	return obj
}

func newRawObjectList(gvk schema.GroupVersionKind, items ...*dynamic.RawObject) *dynamic.RawObjectList {
	list := &dynamic.RawObjectList{
		TypeMeta: metav1.TypeMeta{APIVersion: gvk.GroupVersion().String(), Kind: gvk.Kind},
	}
	if len(items) > 0 {
		list.Items = make([]dynamic.RawObject, 0, len(items))
	}
	for _, item := range items {
		list.Items = append(list.Items, *item.DeepCopy())
	}
	return list
}

func assertRawObjectEqual(t *testing.T, expected, actual *dynamic.RawObject) {
	t.Helper()
	if !rawObjectSemanticDeepEqual(expected, actual) {
		t.Fatalf("unexpected object diff (-want, +got): %s", cmp.Diff(rawObjectComparable(expected), rawObjectComparable(actual)))
	}
}

func assertRawObjectListEqual(t *testing.T, expected, actual *dynamic.RawObjectList) {
	t.Helper()
	if diff := cmp.Diff(rawObjectListComparable(expected), rawObjectListComparable(actual)); diff != "" {
		t.Fatalf("unexpected list diff (-want, +got): %s", diff)
	}
}

func assertRawObjectSliceEqual(t *testing.T, expected, actual []dynamic.RawObject) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("unexpected number of items returned, expected = %d, actual = %d", len(expected), len(actual))
	}
	for i := range expected {
		if !rawObjectSemanticDeepEqual(&expected[i], &actual[i]) {
			t.Fatalf("unexpected object at index %d diff (-want, +got): %s", i, cmp.Diff(rawObjectComparable(&expected[i]), rawObjectComparable(&actual[i])))
		}
	}
}

func rawObjectSemanticDeepEqual(lhs, rhs *dynamic.RawObject) bool {
	return equality.Semantic.DeepEqual(rawObjectComparable(lhs), rawObjectComparable(rhs))
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

func rawObjectListComparable(list *dynamic.RawObjectList) any {
	if list == nil {
		return nil
	}
	items := make([]any, len(list.Items))
	for i := range list.Items {
		items[i] = rawObjectComparable(&list.Items[i])
	}
	return map[string]any{
		"apiVersion": list.APIVersion,
		"kind":       list.Kind,
		"metadata":   list.ListMeta,
		"items":      items,
	}
}
