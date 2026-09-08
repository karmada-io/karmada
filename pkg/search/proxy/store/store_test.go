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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func Test_store_listKeyNamespace(t *testing.T) {
	// The generic registry rewrites a List key to /prefix/name when the field
	// selector is an exact metadata.name match, which for a cluster scoped
	// resource looks exactly like a namespace key (issue #7880).
	tests := []struct {
		name       string
		namespaced bool
		prefix     string
		key        string
		want       string
	}{
		{
			name:       "cluster scoped, plain list",
			namespaced: false,
			prefix:     "/v1/nodes",
			key:        "/v1/nodes",
			want:       "",
		},
		{
			name:       "cluster scoped, exact name selector",
			namespaced: false,
			prefix:     "/v1/nodes",
			key:        "/v1/nodes/node-1",
			want:       "",
		},
		{
			name:       "namespace scoped, all namespaces",
			namespaced: true,
			prefix:     "/v1/pods",
			key:        "/v1/pods",
			want:       "",
		},
		{
			name:       "namespace scoped, single namespace",
			namespaced: true,
			prefix:     "/v1/pods",
			key:        "/v1/pods/foo",
			want:       "foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &store{prefix: tt.prefix, namespaced: tt.namespaced}
			if got := s.listKeyNamespace(tt.key); got != tt.want {
				t.Errorf("listKeyNamespace(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func Test_store_ListClusterScopedByName(t *testing.T) {
	node := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Node",
		"metadata":   map[string]interface{}{"name": "node-1"},
	}}
	gvr := corev1.SchemeGroupVersion.WithResource("nodes")

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(gvr.GroupVersion().WithKind("NodeList"), &unstructured.UnstructuredList{})
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme, map[schema.GroupVersionResource]string{gvr: "NodeList"}, node)

	multiNS := NewMultiNamespace()
	multiNS.Add(metav1.NamespaceAll)
	s := newStore(gvr, false, multiNS,
		func() (dynamic.NamespaceableResourceInterface, error) { return client.Resource(gvr), nil },
		defaultVersioner, "/v1/nodes")

	// The key the generic registry builds for metadata.name=node-1.
	list := &unstructured.UnstructuredList{}
	if err := s.List(context.TODO(), "/v1/nodes/node-1", storage.ListOptions{Predicate: storage.Everything}, list); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("List() returned %d items, want 1", len(list.Items))
	}
	if got := list.Items[0].GetName(); got != "node-1" {
		t.Errorf("List() returned %q, want node-1", got)
	}

	watcher, err := s.Watch(context.TODO(), "/v1/nodes/node-1", storage.ListOptions{Predicate: storage.Everything})
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
	watcher.Stop()
}
