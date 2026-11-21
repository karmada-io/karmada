/*
Copyright 2023 The Karmada Authors.

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

package store

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func Test_filterNS(t *testing.T) {
	type args struct {
		cached  *MultiNamespace
		request string
	}
	tests := []struct {
		name             string
		args             args
		wantReqNS        string
		wantObjFilter    bool
		wantShortCircuit bool
	}{
		{
			name: "Cache all namespaces, and request NamespaceAll",
			args: args{
				cached:  &MultiNamespace{allNamespaces: true},
				request: metav1.NamespaceAll,
			},
			wantReqNS:        metav1.NamespaceAll,
			wantObjFilter:    false,
			wantShortCircuit: false,
		},
		{
			name: "Cache all namespaces, and request foo ns",
			args: args{
				cached:  &MultiNamespace{allNamespaces: true},
				request: "foo",
			},
			wantReqNS:        "foo",
			wantObjFilter:    false,
			wantShortCircuit: false,
		},
		{
			name: "Cache foo namespace, and request all namespaces",
			args: args{
				cached:  &MultiNamespace{namespaces: sets.New[string]("foo")},
				request: metav1.NamespaceAll,
			},
			wantReqNS:        "foo",
			wantObjFilter:    false,
			wantShortCircuit: false,
		},
		{
			name: "Cache foo namespace, and request foo ns",
			args: args{
				cached:  &MultiNamespace{namespaces: sets.New[string]("foo")},
				request: "foo",
			},
			wantReqNS:        "foo",
			wantObjFilter:    false,
			wantShortCircuit: false,
		},
		{
			name: "Cache foo namespace, and request bar ns",
			args: args{
				cached:  &MultiNamespace{namespaces: sets.New[string]("foo")},
				request: "bar",
			},
			wantReqNS:        "",
			wantObjFilter:    false,
			wantShortCircuit: true,
		},
		{
			name: "Cache foo,bar namespaces, and request all namespaces",
			args: args{
				cached:  &MultiNamespace{namespaces: sets.New[string]("foo", "bar")},
				request: metav1.NamespaceAll,
			},
			wantReqNS:        metav1.NamespaceAll,
			wantObjFilter:    true,
			wantShortCircuit: false,
		},
		{
			name: "Cache foo,bar namespaces, and request foo namespace",
			args: args{
				cached:  &MultiNamespace{namespaces: sets.New[string]("foo", "bar")},
				request: "foo",
			},
			wantReqNS:        "foo",
			wantObjFilter:    false,
			wantShortCircuit: false,
		},
		{
			name: "Cache foo,bar namespaces, and request baz namespace",
			args: args{
				cached:  &MultiNamespace{namespaces: sets.New[string]("foo", "bar")},
				request: "baz",
			},
			wantReqNS:        "",
			wantObjFilter:    false,
			wantShortCircuit: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReqNS, gotObjFilter, gotShortCircuit := filterNS(tt.args.cached, tt.args.request)
			if gotReqNS != tt.wantReqNS {
				t.Errorf("filterNS() gotReqNS = %v, want %v", gotReqNS, tt.wantReqNS)
			}
			if (gotObjFilter != nil) != tt.wantObjFilter {
				t.Errorf("filterNS() gotObjFilter %v, want %v", gotObjFilter != nil, tt.wantObjFilter)
			}
			if gotShortCircuit != tt.wantShortCircuit {
				t.Errorf("filterNS() gotShortCircuit = %v, want %v", gotShortCircuit, tt.wantShortCircuit)
			}
		})
	}
}

func newTestStore(t *testing.T, gvr schema.GroupVersionResource, kind string, namespaced bool, objs ...runtime.Object) *store {
	t.Helper()
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{gvr: kind + "List"}, objs...)
	newClientFunc := func() (dynamic.NamespaceableResourceInterface, error) {
		return client.Resource(gvr), nil
	}
	return newStore(gvr, namespaced, &MultiNamespace{allNamespaces: true}, newClientFunc, storage.APIObjectVersioner{}, "/prefix")
}

func newTestObject(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	return obj
}

func Test_store_List(t *testing.T) {
	nodeGVR := schema.GroupVersionResource{Version: "v1", Resource: "nodes"}
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}

	tests := []struct {
		name       string
		gvr        schema.GroupVersionResource
		kind       string
		namespaced bool
		objs       []runtime.Object
		key        string
		wantNames  []string
	}{
		{
			name:       "cluster scope resource, list all",
			gvr:        nodeGVR,
			kind:       "Node",
			namespaced: false,
			objs:       []runtime.Object{newTestObject("v1", "Node", "", "node-1"), newTestObject("v1", "Node", "", "node-2")},
			key:        "/prefix",
			wantNames:  []string{"node-1", "node-2"},
		},
		{
			name:       "cluster scope resource, key with the name is not treated as a namespace",
			gvr:        nodeGVR,
			kind:       "Node",
			namespaced: false,
			objs:       []runtime.Object{newTestObject("v1", "Node", "", "node-1"), newTestObject("v1", "Node", "", "node-2")},
			key:        "/prefix/node-1",
			wantNames:  []string{"node-1", "node-2"},
		},
		{
			name:       "namespace scope resource, key with the namespace lists in that namespace",
			gvr:        podGVR,
			kind:       "Pod",
			namespaced: true,
			objs:       []runtime.Object{newTestObject("v1", "Pod", "foo", "pod-1"), newTestObject("v1", "Pod", "bar", "pod-2")},
			key:        "/prefix/foo",
			wantNames:  []string{"pod-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t, tt.gvr, tt.kind, tt.namespaced, tt.objs...)
			list := &unstructured.UnstructuredList{}
			opts := storage.ListOptions{Predicate: storage.SelectionPredicate{Label: labels.Everything(), Field: fields.Everything()}}
			if err := s.List(context.TODO(), tt.key, opts, list); err != nil {
				t.Fatalf("List() error = %v", err)
			}
			got := sets.New[string]()
			for _, item := range list.Items {
				got.Insert(item.GetName())
			}
			if !got.Equal(sets.New(tt.wantNames...)) {
				t.Errorf("List() = %v, want %v", sets.List(got), tt.wantNames)
			}
		})
	}
}

func Test_store_namespaceFromListKey(t *testing.T) {
	tests := []struct {
		name       string
		namespaced bool
		key        string
		want       string
	}{
		{name: "cluster scope, key is prefix", namespaced: false, key: "/prefix", want: ""},
		{name: "cluster scope, key is prefix and name", namespaced: false, key: "/prefix/node-1", want: ""},
		{name: "namespace scope, key is prefix", namespaced: true, key: "/prefix", want: ""},
		{name: "namespace scope, key is prefix and namespace", namespaced: true, key: "/prefix/foo", want: "foo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &store{prefix: "/prefix", namespaced: tt.namespaced}
			if got := s.namespaceFromListKey(tt.key); got != tt.want {
				t.Errorf("namespaceFromListKey() = %q, want %q", got, tt.want)
			}
		})
	}
}
