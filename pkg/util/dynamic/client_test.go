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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	restclient "k8s.io/client-go/rest"
	restclientwatch "k8s.io/client-go/rest/watch"
	"k8s.io/client-go/util/watchlist"
)

func getJSON(version, kind, name string) []byte {
	return fmt.Appendf(nil, `{"apiVersion": %q, "kind": %q, "metadata": {"name": %q}}`, version, kind, name)
}

func getListJSON(version, kind string, items ...[]byte) []byte {
	json := fmt.Sprintf(`{"apiVersion": %q, "kind": %q, "items": [%s]}`,
		version, kind, bytes.Join(items, []byte(",")))
	return []byte(json)
}

func getObject(version, kind, name string) *RawObject {
	obj, err := NewRawObject(getJSON(version, kind, name))
	if err != nil {
		panic(err)
	}
	return obj
}

func getCompactObject(version, kind, name string) *RawObject {
	var compact bytes.Buffer
	if err := json.Compact(&compact, getJSON(version, kind, name)); err != nil {
		panic(err)
	}
	obj, err := NewRawObject(compact.Bytes())
	if err != nil {
		panic(err)
	}
	return obj
}

func getObjectFromJSON(b []byte) *RawObject {
	obj, _ := NewRawObject(b) // can ignore parse error because the comparison will fail
	return obj
}

func rawObjectSemanticDeepEqual(lhs, rhs *RawObject) bool {
	return reflect.DeepEqual(rawObjectComparable(lhs), rawObjectComparable(rhs))
}

func rawObjectComparable(obj *RawObject) any {
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

func rawObjectListSemanticDeepEqual(lhs, rhs *RawObjectList) bool {
	return reflect.DeepEqual(rawObjectListComparable(lhs), rawObjectListComparable(rhs))
}

func rawObjectListComparable(list *RawObjectList) any {
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

func watchEventSemanticDeepEqual(lhs, rhs watch.Event) bool {
	if lhs.Type != rhs.Type {
		return false
	}
	lhsObj, lhsOK := lhs.Object.(*RawObject)
	rhsObj, rhsOK := rhs.Object.(*RawObject)
	if lhsOK || rhsOK {
		return lhsOK && rhsOK && rawObjectSemanticDeepEqual(lhsObj, rhsObj)
	}
	return reflect.DeepEqual(lhs.Object, rhs.Object)
}

func getClientServer(h func(http.ResponseWriter, *http.Request)) (Interface, *httptest.Server, error) {
	srv := httptest.NewServer(http.HandlerFunc(h))
	cl, err := NewForConfig(&restclient.Config{
		Host: srv.URL,
	})
	if err != nil {
		srv.Close()
		return nil, nil, err
	}
	return cl, srv, nil
}

func TestDoesClientSupportWatchListSemantics(t *testing.T) {
	target, err := NewForConfig(&restclient.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if watchlist.DoesClientNotSupportWatchListSemantics(target) {
		t.Fatalf("Dynamic client should support WatchList semantics")
	}
}

func TestList(t *testing.T) {
	tcs := []struct {
		name      string
		namespace string
		path      string
		resp      []byte
		want      *RawObjectList
	}{
		{
			name: "normal_list",
			path: "/apis/gtest/vtest/rtest",
			resp: getListJSON("vTest", "rTestList",
				getJSON("vTest", "rTest", "item1"),
				getJSON("vTest", "rTest", "item2")),
			want: &RawObjectList{
				TypeMeta: metav1.TypeMeta{APIVersion: "vTest", Kind: "rTestList"},
				Items: []RawObject{
					*getObject("vTest", "rTest", "item1"),
					*getObject("vTest", "rTest", "item2"),
				},
			},
		},
		{
			name:      "namespaced_list",
			namespace: "nstest",
			path:      "/apis/gtest/vtest/namespaces/nstest/rtest",
			resp: getListJSON("vTest", "rTestList",
				getJSON("vTest", "rTest", "item1"),
				getJSON("vTest", "rTest", "item2")),
			want: &RawObjectList{
				TypeMeta: metav1.TypeMeta{APIVersion: "vTest", Kind: "rTestList"},
				Items: []RawObject{
					*getObject("vTest", "rTest", "item1"),
					*getObject("vTest", "rTest", "item2"),
				},
			},
		},
	}
	for _, tc := range tcs {
		resource := schema.GroupVersionResource{Group: "gtest", Version: "vtest", Resource: "rtest"}
		cl, srv, err := getClientServer(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Errorf("List(%q) got HTTP method %s. wanted GET", tc.name, r.Method)
			}

			if r.URL.Path != tc.path {
				t.Errorf("List(%q) got path %s. wanted %s", tc.name, r.URL.Path, tc.path)
			}

			w.Header().Set("Content-Type", runtime.ContentTypeJSON)
			_, _ = w.Write(tc.resp)
		})
		if err != nil {
			t.Errorf("unexpected error when creating client: %v", err)
			continue
		}
		defer srv.Close()

		got, err := cl.Resource(resource).Namespace(tc.namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			t.Errorf("unexpected error when listing %q: %v", tc.name, err)
			continue
		}

		if !rawObjectListSemanticDeepEqual(got, tc.want) {
			t.Errorf("List(%q) want: %v\ngot: %v", tc.name, tc.want, got)
		}
	}
}

func TestGet(t *testing.T) {
	tcs := []struct {
		resource    string
		subresource []string
		namespace   string
		name        string
		path        string
		resp        []byte
		want        *RawObject
	}{
		{
			resource: "rtest",
			name:     "normal_get",
			path:     "/apis/gtest/vtest/rtest/normal_get",
			resp:     getJSON("vTest", "rTest", "normal_get"),
			want:     getObject("vTest", "rTest", "normal_get"),
		},
		{
			resource:  "rtest",
			namespace: "nstest",
			name:      "namespaced_get",
			path:      "/apis/gtest/vtest/namespaces/nstest/rtest/namespaced_get",
			resp:      getJSON("vTest", "rTest", "namespaced_get"),
			want:      getObject("vTest", "rTest", "namespaced_get"),
		},
		{
			resource:    "rtest",
			subresource: []string{"srtest"},
			name:        "normal_subresource_get",
			path:        "/apis/gtest/vtest/rtest/normal_subresource_get/srtest",
			resp:        getJSON("vTest", "srTest", "normal_subresource_get"),
			want:        getObject("vTest", "srTest", "normal_subresource_get"),
		},
		{
			resource:    "rtest",
			subresource: []string{"srtest"},
			namespace:   "nstest",
			name:        "namespaced_subresource_get",
			path:        "/apis/gtest/vtest/namespaces/nstest/rtest/namespaced_subresource_get/srtest",
			resp:        getJSON("vTest", "srTest", "namespaced_subresource_get"),
			want:        getObject("vTest", "srTest", "namespaced_subresource_get"),
		},
		{
			resource:    "rtest",
			subresource: []string{"srtest"},
			namespace:   "nstest",
			name:        "namespaced_subresource_get_list",
			path:        "/apis/gtest/vtest/namespaces/nstest/rtest/namespaced_subresource_get_list/srtest",
			resp:        getListJSON("vTest", "srTest", getJSON("vTest", "srTest", "a1")),
			want:        getObjectFromJSON(getListJSON("vTest", "srTest", getJSON("vTest", "srTest", "a1"))),
		},
	}
	for _, tc := range tcs {
		resource := schema.GroupVersionResource{Group: "gtest", Version: "vtest", Resource: tc.resource}
		cl, srv, err := getClientServer(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Errorf("Get(%q) got HTTP method %s. wanted GET", tc.name, r.Method)
			}

			if r.URL.Path != tc.path {
				t.Errorf("Get(%q) got path %s. wanted %s", tc.name, r.URL.Path, tc.path)
			}

			w.Header().Set("Content-Type", runtime.ContentTypeJSON)
			_, _ = w.Write(tc.resp)
		})
		if err != nil {
			t.Errorf("unexpected error when creating client: %v", err)
			continue
		}
		defer srv.Close()

		got, err := cl.Resource(resource).Namespace(tc.namespace).Get(context.TODO(), tc.name, metav1.GetOptions{}, tc.subresource...)
		if err != nil {
			t.Errorf("unexpected error when getting %q: %v", tc.name, err)
			continue
		}

		if !rawObjectSemanticDeepEqual(got, tc.want) {
			t.Errorf("Get(%q) want: %v\ngot: %v", tc.name, tc.want, got)
		}
	}
}

func TestDelete(t *testing.T) {
	background := metav1.DeletePropagationBackground
	uid := types.UID("uid")

	statusOK := &metav1.Status{
		TypeMeta: metav1.TypeMeta{Kind: "Status"},
		Status:   metav1.StatusSuccess,
	}
	tcs := []struct {
		subresource   []string
		namespace     string
		name          string
		path          string
		deleteOptions metav1.DeleteOptions
	}{
		{
			name: "normal_delete",
			path: "/apis/gtest/vtest/rtest/normal_delete",
		},
		{
			namespace: "nstest",
			name:      "namespaced_delete",
			path:      "/apis/gtest/vtest/namespaces/nstest/rtest/namespaced_delete",
		},
		{
			subresource: []string{"srtest"},
			name:        "normal_delete",
			path:        "/apis/gtest/vtest/rtest/normal_delete/srtest",
		},
		{
			subresource: []string{"srtest"},
			namespace:   "nstest",
			name:        "namespaced_delete",
			path:        "/apis/gtest/vtest/namespaces/nstest/rtest/namespaced_delete/srtest",
		},
		{
			namespace:     "nstest",
			name:          "namespaced_delete_with_options",
			path:          "/apis/gtest/vtest/namespaces/nstest/rtest/namespaced_delete_with_options",
			deleteOptions: metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}, PropagationPolicy: &background},
		},
	}
	for _, tc := range tcs {
		resource := schema.GroupVersionResource{Group: "gtest", Version: "vtest", Resource: "rtest"}
		cl, srv, err := getClientServer(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "DELETE" {
				t.Errorf("Delete(%q) got HTTP method %s. wanted DELETE", tc.name, r.Method)
			}

			if r.URL.Path != tc.path {
				t.Errorf("Delete(%q) got path %s. wanted %s", tc.name, r.URL.Path, tc.path)
			}

			content := r.Header.Get("Content-Type")
			if content != runtime.ContentTypeJSON {
				t.Errorf("Delete(%q) got Content-Type %s. wanted %s", tc.name, content, runtime.ContentTypeJSON)
			}

			w.Header().Set("Content-Type", runtime.ContentTypeJSON)
			_ = unstructured.UnstructuredJSONScheme.Encode(statusOK, w)
		})
		if err != nil {
			t.Errorf("unexpected error when creating client: %v", err)
			continue
		}
		defer srv.Close()

		err = cl.Resource(resource).Namespace(tc.namespace).Delete(context.TODO(), tc.name, tc.deleteOptions, tc.subresource...)
		if err != nil {
			t.Errorf("unexpected error when deleting %q: %v", tc.name, err)
			continue
		}
	}
}

func TestDeleteCollection(t *testing.T) {
	statusOK := &metav1.Status{
		TypeMeta: metav1.TypeMeta{Kind: "Status"},
		Status:   metav1.StatusSuccess,
	}
	tcs := []struct {
		namespace string
		name      string
		path      string
	}{
		{
			name: "normal_delete_collection",
			path: "/apis/gtest/vtest/rtest",
		},
		{
			namespace: "nstest",
			name:      "namespaced_delete_collection",
			path:      "/apis/gtest/vtest/namespaces/nstest/rtest",
		},
	}
	for _, tc := range tcs {
		resource := schema.GroupVersionResource{Group: "gtest", Version: "vtest", Resource: "rtest"}
		cl, srv, err := getClientServer(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "DELETE" {
				t.Errorf("DeleteCollection(%q) got HTTP method %s. wanted DELETE", tc.name, r.Method)
			}

			if r.URL.Path != tc.path {
				t.Errorf("DeleteCollection(%q) got path %s. wanted %s", tc.name, r.URL.Path, tc.path)
			}

			content := r.Header.Get("Content-Type")
			if content != runtime.ContentTypeJSON {
				t.Errorf("DeleteCollection(%q) got Content-Type %s. wanted %s", tc.name, content, runtime.ContentTypeJSON)
			}

			w.Header().Set("Content-Type", runtime.ContentTypeJSON)
			_ = unstructured.UnstructuredJSONScheme.Encode(statusOK, w)
		})
		if err != nil {
			t.Errorf("unexpected error when creating client: %v", err)
			continue
		}
		defer srv.Close()

		err = cl.Resource(resource).Namespace(tc.namespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{})
		if err != nil {
			t.Errorf("unexpected error when deleting collection %q: %v", tc.name, err)
			continue
		}
	}
}

func TestCreate(t *testing.T) {
	tcs := []struct {
		resource    string
		subresource []string
		name        string
		namespace   string
		obj         *RawObject
		path        string
	}{
		{
			resource: "rtest",
			name:     "normal_create",
			path:     "/apis/gtest/vtest/rtest",
			obj:      getObject("gtest/vTest", "rTest", "normal_create"),
		},
		{
			resource:  "rtest",
			name:      "namespaced_create",
			namespace: "nstest",
			path:      "/apis/gtest/vtest/namespaces/nstest/rtest",
			obj:       getObject("gtest/vTest", "rTest", "namespaced_create"),
		},
		{
			resource:    "rtest",
			subresource: []string{"srtest"},
			name:        "normal_subresource_create",
			path:        "/apis/gtest/vtest/rtest/normal_subresource_create/srtest",
			obj:         getObject("vTest", "srTest", "normal_subresource_create"),
		},
		{
			resource:    "rtest/",
			subresource: []string{"srtest"},
			name:        "namespaced_subresource_create",
			namespace:   "nstest",
			path:        "/apis/gtest/vtest/namespaces/nstest/rtest/namespaced_subresource_create/srtest",
			obj:         getObject("vTest", "srTest", "namespaced_subresource_create"),
		},
	}
	for _, tc := range tcs {
		resource := schema.GroupVersionResource{Group: "gtest", Version: "vtest", Resource: tc.resource}
		cl, srv, err := getClientServer(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("Create(%q) got HTTP method %s. wanted POST", tc.name, r.Method)
			}

			if r.URL.Path != tc.path {
				t.Errorf("Create(%q) got path %s. wanted %s", tc.name, r.URL.Path, tc.path)
			}

			content := r.Header.Get("Content-Type")
			if content != runtime.ContentTypeJSON {
				t.Errorf("Create(%q) got Content-Type %s. wanted %s", tc.name, content, runtime.ContentTypeJSON)
			}

			w.Header().Set("Content-Type", runtime.ContentTypeJSON)
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("Create(%q) unexpected error reading body: %v", tc.name, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			_, _ = w.Write(data)
		})
		if err != nil {
			t.Errorf("unexpected error when creating client: %v", err)
			continue
		}
		defer srv.Close()

		got, err := cl.Resource(resource).Namespace(tc.namespace).Create(context.TODO(), tc.obj, metav1.CreateOptions{}, tc.subresource...)
		if err != nil {
			t.Errorf("unexpected error when creating %q: %v", tc.name, err)
			continue
		}

		if !rawObjectSemanticDeepEqual(got, tc.obj) {
			t.Errorf("Create(%q) want: %v\ngot: %v", tc.name, tc.obj, got)
		}
	}
}

func TestUpdate(t *testing.T) {
	tcs := []struct {
		resource    string
		subresource []string
		name        string
		namespace   string
		obj         *RawObject
		path        string
	}{
		{
			resource: "rtest",
			name:     "normal_update",
			path:     "/apis/gtest/vtest/rtest/normal_update",
			obj:      getObject("gtest/vTest", "rTest", "normal_update"),
		},
		{
			resource:  "rtest",
			name:      "namespaced_update",
			namespace: "nstest",
			path:      "/apis/gtest/vtest/namespaces/nstest/rtest/namespaced_update",
			obj:       getObject("gtest/vTest", "rTest", "namespaced_update"),
		},
		{
			resource:    "rtest",
			subresource: []string{"srtest"},
			name:        "normal_subresource_update",
			path:        "/apis/gtest/vtest/rtest/normal_update/srtest",
			obj:         getObject("gtest/vTest", "srTest", "normal_update"),
		},
		{
			resource:    "rtest",
			subresource: []string{"srtest"},
			name:        "namespaced_subresource_update",
			namespace:   "nstest",
			path:        "/apis/gtest/vtest/namespaces/nstest/rtest/namespaced_update/srtest",
			obj:         getObject("gtest/vTest", "srTest", "namespaced_update"),
		},
	}
	for _, tc := range tcs {
		resource := schema.GroupVersionResource{Group: "gtest", Version: "vtest", Resource: tc.resource}
		cl, srv, err := getClientServer(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "PUT" {
				t.Errorf("Update(%q) got HTTP method %s. wanted PUT", tc.name, r.Method)
			}

			if r.URL.Path != tc.path {
				t.Errorf("Update(%q) got path %s. wanted %s", tc.name, r.URL.Path, tc.path)
			}

			content := r.Header.Get("Content-Type")
			if content != runtime.ContentTypeJSON {
				t.Errorf("Uppdate(%q) got Content-Type %s. wanted %s", tc.name, content, runtime.ContentTypeJSON)
			}

			w.Header().Set("Content-Type", runtime.ContentTypeJSON)
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("Update(%q) unexpected error reading body: %v", tc.name, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			_, _ = w.Write(data)
		})
		if err != nil {
			t.Errorf("unexpected error when creating client: %v", err)
			continue
		}
		defer srv.Close()

		got, err := cl.Resource(resource).Namespace(tc.namespace).Update(context.TODO(), tc.obj, metav1.UpdateOptions{}, tc.subresource...)
		if err != nil {
			t.Errorf("unexpected error when updating %q: %v", tc.name, err)
			continue
		}

		if !rawObjectSemanticDeepEqual(got, tc.obj) {
			t.Errorf("Update(%q) want: %v\ngot: %v", tc.name, tc.obj, got)
		}
	}
}

func TestWatch(t *testing.T) {
	tcs := []struct {
		name      string
		namespace string
		events    []watch.Event
		path      string
		query     string
	}{
		{
			name:  "normal_watch",
			path:  "/apis/gtest/vtest/rtest",
			query: "watch=true",
			events: []watch.Event{
				{Type: watch.Added, Object: getCompactObject("gtest/vTest", "rTest", "normal_watch")},
				{Type: watch.Modified, Object: getCompactObject("gtest/vTest", "rTest", "normal_watch")},
				{Type: watch.Deleted, Object: getCompactObject("gtest/vTest", "rTest", "normal_watch")},
			},
		},
		{
			name:      "namespaced_watch",
			namespace: "nstest",
			path:      "/apis/gtest/vtest/namespaces/nstest/rtest",
			query:     "watch=true",
			events: []watch.Event{
				{Type: watch.Added, Object: getCompactObject("gtest/vTest", "rTest", "namespaced_watch")},
				{Type: watch.Modified, Object: getCompactObject("gtest/vTest", "rTest", "namespaced_watch")},
				{Type: watch.Deleted, Object: getCompactObject("gtest/vTest", "rTest", "namespaced_watch")},
			},
		},
	}
	for _, tc := range tcs {
		resource := schema.GroupVersionResource{Group: "gtest", Version: "vtest", Resource: "rtest"}
		cl, srv, err := getClientServer(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Errorf("Watch(%q) got HTTP method %s. wanted GET", tc.name, r.Method)
			}

			if r.URL.Path != tc.path {
				t.Errorf("Watch(%q) got path %s. wanted %s", tc.name, r.URL.Path, tc.path)
			}
			if r.URL.RawQuery != tc.query {
				t.Errorf("Watch(%q) got query %s. wanted %s", tc.name, r.URL.RawQuery, tc.query)
			}

			w.Header().Set("Content-Type", "application/json")

			enc := restclientwatch.NewEncoder(streaming.NewEncoder(w, unstructured.UnstructuredJSONScheme), unstructured.UnstructuredJSONScheme)
			for _, e := range tc.events {
				_ = enc.Encode(&e)
			}
		})
		if err != nil {
			t.Errorf("unexpected error when creating client: %v", err)
			continue
		}
		defer srv.Close()

		watcher, err := cl.Resource(resource).Namespace(tc.namespace).Watch(context.TODO(), metav1.ListOptions{})
		if err != nil {
			t.Errorf("unexpected error when watching %q: %v", tc.name, err)
			continue
		}

		for _, want := range tc.events {
			got := <-watcher.ResultChan()
			if !watchEventSemanticDeepEqual(got, want) {
				t.Errorf("Watch(%q) want: %v\ngot: %v", tc.name, want, got)
			}
		}
	}
}

func TestPatch(t *testing.T) {
	tcs := []struct {
		resource    string
		subresource []string
		name        string
		namespace   string
		patch       []byte
		want        *RawObject
		path        string
	}{
		{
			resource: "rtest",
			name:     "normal_patch",
			path:     "/apis/gtest/vtest/rtest/normal_patch",
			patch:    getJSON("gtest/vTest", "rTest", "normal_patch"),
			want:     getObject("gtest/vTest", "rTest", "normal_patch"),
		},
		{
			resource:  "rtest",
			name:      "namespaced_patch",
			namespace: "nstest",
			path:      "/apis/gtest/vtest/namespaces/nstest/rtest/namespaced_patch",
			patch:     getJSON("gtest/vTest", "rTest", "namespaced_patch"),
			want:      getObject("gtest/vTest", "rTest", "namespaced_patch"),
		},
		{
			resource:    "rtest",
			subresource: []string{"srtest"},
			name:        "normal_subresource_patch",
			path:        "/apis/gtest/vtest/rtest/normal_subresource_patch/srtest",
			patch:       getJSON("gtest/vTest", "srTest", "normal_subresource_patch"),
			want:        getObject("gtest/vTest", "srTest", "normal_subresource_patch"),
		},
		{
			resource:    "rtest",
			subresource: []string{"srtest"},
			name:        "namespaced_subresource_patch",
			namespace:   "nstest",
			path:        "/apis/gtest/vtest/namespaces/nstest/rtest/namespaced_subresource_patch/srtest",
			patch:       getJSON("gtest/vTest", "srTest", "namespaced_subresource_patch"),
			want:        getObject("gtest/vTest", "srTest", "namespaced_subresource_patch"),
		},
	}
	for _, tc := range tcs {
		resource := schema.GroupVersionResource{Group: "gtest", Version: "vtest", Resource: tc.resource}
		cl, srv, err := getClientServer(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "PATCH" {
				t.Errorf("Patch(%q) got HTTP method %s. wanted PATCH", tc.name, r.Method)
			}

			if r.URL.Path != tc.path {
				t.Errorf("Patch(%q) got path %s. wanted %s", tc.name, r.URL.Path, tc.path)
			}

			content := r.Header.Get("Content-Type")
			if content != string(types.StrategicMergePatchType) {
				t.Errorf("Patch(%q) got Content-Type %s. wanted %s", tc.name, content, types.StrategicMergePatchType)
			}

			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("Patch(%q) unexpected error reading body: %v", tc.name, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(data)
		})
		if err != nil {
			t.Errorf("unexpected error when creating client: %v", err)
			continue
		}
		defer srv.Close()

		got, err := cl.Resource(resource).Namespace(tc.namespace).Patch(context.TODO(), tc.name, types.StrategicMergePatchType, tc.patch, metav1.PatchOptions{}, tc.subresource...)
		if err != nil {
			t.Errorf("unexpected error when patching %q: %v", tc.name, err)
			continue
		}

		if !rawObjectSemanticDeepEqual(got, tc.want) {
			t.Errorf("Patch(%q) want: %v\ngot: %v", tc.name, tc.want, got)
		}
	}
}

//nolint:gocyclo
func TestInvalidSegments(t *testing.T) {
	name := "bad/name"
	namespace := "bad/namespace"
	resource := schema.GroupVersionResource{Group: "gtest", Version: "vtest", Resource: "rtest"}
	obj := getObject("vtest", "vkind", name)
	cl, err := NewForConfig(&restclient.Config{
		Host: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	_, err = cl.Resource(resource).Namespace(namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid namespace") {
		t.Fatalf("Expected `invalid namespace` error, got: %v", err)
	}

	_, err = cl.Resource(resource).Update(context.TODO(), obj, metav1.UpdateOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid resource name") {
		t.Fatalf("Expected `invalid resource name` error, got: %v", err)
	}
	_, err = cl.Resource(resource).Namespace(namespace).Update(context.TODO(), obj, metav1.UpdateOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid namespace") {
		t.Fatalf("Expected `invalid namespace` error, got: %v", err)
	}

	_, err = cl.Resource(resource).UpdateStatus(context.TODO(), obj, metav1.UpdateOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid resource name") {
		t.Fatalf("Expected `invalid resource name` error, got: %v", err)
	}
	_, err = cl.Resource(resource).Namespace(namespace).UpdateStatus(context.TODO(), obj, metav1.UpdateOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid namespace") {
		t.Fatalf("Expected `invalid namespace` error, got: %v", err)
	}

	err = cl.Resource(resource).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid resource name") {
		t.Fatalf("Expected `invalid resource name` error, got: %v", err)
	}
	err = cl.Resource(resource).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid namespace") {
		t.Fatalf("Expected `invalid namespace` error, got: %v", err)
	}

	err = cl.Resource(resource).Namespace(namespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid namespace") {
		t.Fatalf("Expected `invalid namespace` error, got: %v", err)
	}

	_, err = cl.Resource(resource).Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid resource name") {
		t.Fatalf("Expected `invalid resource name` error, got: %v", err)
	}
	_, err = cl.Resource(resource).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid namespace") {
		t.Fatalf("Expected `invalid namespace` error, got: %v", err)
	}

	_, err = cl.Resource(resource).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid namespace") {
		t.Fatalf("Expected `invalid namespace` error, got: %v", err)
	}

	_, err = cl.Resource(resource).Namespace(namespace).Watch(context.TODO(), metav1.ListOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid namespace") {
		t.Fatalf("Expected `invalid namespace` error, got: %v", err)
	}

	_, err = cl.Resource(resource).Patch(context.TODO(), name, types.StrategicMergePatchType, []byte("{}"), metav1.PatchOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid resource name") {
		t.Fatalf("Expected `invalid resource name` error, got: %v", err)
	}
	_, err = cl.Resource(resource).Namespace(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, []byte("{}"), metav1.PatchOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid namespace") {
		t.Fatalf("Expected `invalid namespace` error, got: %v", err)
	}

	_, err = cl.Resource(resource).Apply(context.TODO(), name, obj, metav1.ApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid resource name") {
		t.Fatalf("Expected `invalid resource name` error, got: %v", err)
	}
	_, err = cl.Resource(resource).Namespace(namespace).Apply(context.TODO(), name, obj, metav1.ApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid namespace") {
		t.Fatalf("Expected `invalid namespace` error, got: %v", err)
	}

	_, err = cl.Resource(resource).ApplyStatus(context.TODO(), name, obj, metav1.ApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid resource name") {
		t.Fatalf("Expected `invalid resource name` error, got: %v", err)
	}
	_, err = cl.Resource(resource).Namespace(namespace).ApplyStatus(context.TODO(), name, obj, metav1.ApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid namespace") {
		t.Fatalf("Expected `invalid namespace` error, got: %v", err)
	}
}
