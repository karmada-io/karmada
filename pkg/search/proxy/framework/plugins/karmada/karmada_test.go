/*
Copyright 2022 The Karmada Authors.

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

package karmada

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	restclient "k8s.io/client-go/rest"

	"github.com/karmada-io/karmada/pkg/search/proxy/framework"
	pluginruntime "github.com/karmada-io/karmada/pkg/search/proxy/framework/runtime"
	utiltest "github.com/karmada-io/karmada/pkg/util/testing"
)

func Test_karmadaProxy(t *testing.T) {
	var gotRequest *http.Request
	s := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		gotRequest = req
	}))
	defer s.Close()

	type args struct {
		host string
		path string
	}

	type want struct {
		path          string
		requestGroups []string
		wantGroups    []string
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "proxy to /proxy",
			args: args{
				host: s.URL,
				path: "proxy",
			},
			want: want{
				path:          "/proxy",
				requestGroups: []string{"team-a", "system:authenticated"},
				wantGroups:    []string{"team-a"},
			},
		},
		{
			name: "proxy to /api/proxy",
			args: args{
				host: s.URL + "/api",
				path: "proxy",
			},
			want: want{
				path:          "/api/proxy",
				requestGroups: []string{"team-a", "system:authenticated"},
				wantGroups:    []string{"team-a"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRequest = nil
			restConfig := &restclient.Config{
				Host: tt.args.host,
				TLSClientConfig: restclient.TLSClientConfig{
					Insecure: true,
				},
				Timeout: time.Second * 1,
			}
			p, err := New(pluginruntime.PluginDependency{RestConfig: restConfig})
			if err != nil {
				t.Error(err)
				return
			}

			response := httptest.NewRecorder()
			h, err := p.Connect(context.TODO(), framework.ProxyRequest{
				ProxyPath: tt.args.path,
				Responder: utiltest.NewResponder(response),
			})
			if err != nil {
				t.Error(err)
				return
			}

			httpRequest, err := http.NewRequest(http.MethodGet, "http://localhost", nil)
			if err != nil {
				t.Error(err)
				return
			}
			requester := &user.DefaultInfo{Name: "test-user", Groups: tt.want.requestGroups}
			httpRequest = httpRequest.WithContext(request.WithUser(httpRequest.Context(), requester))
			h.ServeHTTP(response, httpRequest)

			if t.Failed() {
				return
			}

			if gotRequest == nil {
				t.Error("got request nil")
				return
			}

			if gotRequest.URL.Path != tt.want.path {
				t.Errorf("path got = %v, want = %v", gotRequest.URL.Path, tt.want.path)
				return
			}

			if got := gotRequest.Header.Get(authenticationv1.ImpersonateUserHeader); got != requester.GetName() {
				t.Errorf("impersonate user header got = %v, want = %v", got, requester.GetName())
			}

			if got := gotRequest.Header.Values(authenticationv1.ImpersonateGroupHeader); !reflect.DeepEqual(got, tt.want.wantGroups) {
				t.Errorf("impersonate group header got = %v, want = %v", got, tt.want.wantGroups)
			}
		})
	}
}

func Test_karmadaProxy_NoUser(t *testing.T) {
	restConfig := &restclient.Config{Host: "http://localhost", Timeout: time.Second}
	p, err := New(pluginruntime.PluginDependency{RestConfig: restConfig})
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	h, err := p.Connect(context.TODO(), framework.ProxyRequest{
		ProxyPath: "proxy",
		Responder: utiltest.NewResponder(response),
	})
	if err != nil {
		t.Fatal(err)
	}

	httpRequest, err := http.NewRequest(http.MethodGet, "http://localhost", nil)
	if err != nil {
		t.Fatal(err)
	}
	h.ServeHTTP(response, httpRequest)

	if response.Code != http.StatusInternalServerError {
		t.Errorf("status code got = %v, want = %v", response.Code, http.StatusInternalServerError)
	}
}
