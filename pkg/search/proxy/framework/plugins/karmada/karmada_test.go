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
	"testing"
	"time"

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
		path string
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
				path: "/proxy",
			},
		},
		{
			name: "proxy to /api/proxy",
			args: args{
				host: s.URL + "/api",
				path: "proxy",
			},
			want: want{
				path: "/api/proxy",
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

			request, err := http.NewRequest(http.MethodGet, "http://localhost", nil)
			if err != nil {
				t.Error(err)
				return
			}
			h.ServeHTTP(response, request)

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
		})
	}
}
