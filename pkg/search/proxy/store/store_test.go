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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
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
