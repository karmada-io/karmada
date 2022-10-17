package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"
)

func Test_karmadaProxy(t *testing.T) {
	var gotRequest *http.Request
	s := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
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
			p, err := newKarmadaProxy(restConfig)
			if err != nil {
				t.Error(err)
				return
			}

			response := httptest.NewRecorder()
			h, err := p.connect(context.TODO(), podGVR, tt.args.path, newTestResponder(response))
			if err != nil {
				t.Error(err)
				return
			}

			request, err := http.NewRequest("GET", "http://localhost", nil)
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

type testResponder struct {
	resp *httptest.ResponseRecorder
}

func newTestResponder(response *httptest.ResponseRecorder) *testResponder {
	return &testResponder{
		resp: response,
	}
}

func (f *testResponder) Object(statusCode int, obj runtime.Object) {
	f.resp.Code = statusCode

	if obj != nil {
		err := json.NewEncoder(f.resp).Encode(obj)
		if err != nil {
			f.Error(err)
		}
	}
}

func (f *testResponder) Error(err error) {
	_, _ = f.resp.WriteString(err.Error())
}
