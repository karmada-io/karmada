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

package cache

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/request"

	"github.com/karmada-io/karmada/pkg/search/proxy/framework"
	proxytest "github.com/karmada-io/karmada/pkg/search/proxy/testing"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

func TestCacheProxy_connect(t *testing.T) {
	type args struct {
		url string
	}
	type want struct {
		namespace   string
		name        string
		gvr         schema.GroupVersionResource
		getOptions  *metav1.GetOptions
		listOptions *metainternalversion.ListOptions
	}

	var actual want
	p := &Cache{
		store: &proxytest.MockStore{
			GetFunc: func(ctx context.Context, gvr schema.GroupVersionResource, name string, options *metav1.GetOptions) (runtime.Object, error) {
				actual = want{}
				actual.namespace = request.NamespaceValue(ctx)
				actual.name = name
				actual.gvr = gvr
				actual.getOptions = options
				return nil, nil
			},
			ListFunc: func(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (runtime.Object, error) {
				actual = want{}
				actual.namespace = request.NamespaceValue(ctx)
				actual.gvr = gvr
				actual.listOptions = options
				return nil, nil
			},
			WatchFunc: func(ctx context.Context, gvr schema.GroupVersionResource, options *metainternalversion.ListOptions) (watch.Interface, error) {
				actual = want{}
				actual.namespace = request.NamespaceValue(ctx)
				actual.gvr = gvr
				actual.listOptions = options
				w := newEmptyWatch()
				// avoid block in ServeHTTP
				time.AfterFunc(time.Millisecond*10, func() {
					w.Stop()
				})
				return w, nil
			},
		},
		restMapper: proxytest.RestMapper,
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "get node",
			args: args{
				url: "/api/v1/nodes/foo",
			},
			want: want{
				name:       "foo",
				gvr:        proxytest.NodeGVR,
				getOptions: &metav1.GetOptions{},
			},
		},
		{
			name: "get pod",
			args: args{
				url: "/api/v1/namespaces/default/pods/foo",
			},
			want: want{
				namespace:  "default",
				name:       "foo",
				gvr:        proxytest.PodGVR,
				getOptions: &metav1.GetOptions{},
			},
		},
		{
			name: "get pod with options",
			args: args{
				url: "/api/v1/namespaces/default/pods/foo?resourceVersion=1000",
			},
			want: want{
				namespace:  "default",
				name:       "foo",
				gvr:        proxytest.PodGVR,
				getOptions: &metav1.GetOptions{ResourceVersion: "1000"},
			},
		},
		{
			name: "list nodes",
			args: args{
				url: "/api/v1/nodes",
			},
			want: want{
				gvr:         proxytest.NodeGVR,
				listOptions: &metainternalversion.ListOptions{},
			},
		},
		{
			name: "list pod",
			args: args{
				url: "/api/v1/namespaces/default/pods",
			},
			want: want{
				namespace:   "default",
				gvr:         proxytest.PodGVR,
				listOptions: &metainternalversion.ListOptions{},
			},
		},
		{
			name: "list pod with options",
			args: args{
				url: "/api/v1/namespaces/default/pods?fieldSelector=metadata.name%3Dbar&labelSelector=app%3Dfoo&limit=500&resourceVersion=1000&container=bar",
			},
			want: want{
				namespace: "default",
				gvr:       proxytest.PodGVR,
				listOptions: &metainternalversion.ListOptions{
					LabelSelector:   asLabelSelector("app=foo"),
					FieldSelector:   fields.OneTermEqualSelector("metadata.name", "bar"),
					ResourceVersion: "1000",
					Limit:           500,
				},
			},
		},
		{
			name: "watch node",
			args: args{
				url: "/api/v1/nodes?watch=true",
			},
			want: want{
				gvr: proxytest.NodeGVR,
				listOptions: &metainternalversion.ListOptions{
					LabelSelector: labels.NewSelector(),
					FieldSelector: fields.Everything(),
					Watch:         true,
				},
			},
		},
		{
			name: "watch pod",
			args: args{
				url: "/api/v1/namespaces/default/pods?watch=true",
			},
			want: want{
				namespace: "default",
				gvr:       proxytest.PodGVR,
				listOptions: &metainternalversion.ListOptions{
					LabelSelector: labels.NewSelector(),
					FieldSelector: fields.Everything(),
					Watch:         true,
				},
			},
		},
		{
			name: "watch pod with options",
			args: args{
				url: "/api/v1/namespaces/default/pods?watch=true&fieldSelector=metadata.name%3Dbar&labelSelector=app%3Dfoo&limit=500&resourceVersion=1000&container=bar",
			},
			want: want{
				namespace: "default",
				gvr:       proxytest.PodGVR,
				listOptions: &metainternalversion.ListOptions{
					LabelSelector:   asLabelSelector("app=foo"),
					FieldSelector:   fields.OneTermEqualSelector("metadata.name", "bar"),
					ResourceVersion: "1000",
					Limit:           500,
					Watch:           true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// reset before each test
			actual = want{}

			req, err := http.NewRequest(http.MethodGet, tt.args.url, nil)
			if err != nil {
				t.Error(err)
				return
			}
			requestInfo := lifted.NewRequestInfo(req)
			req = req.WithContext(request.WithRequestInfo(req.Context(), requestInfo))
			if requestInfo.Namespace != "" {
				req = req.WithContext(request.WithNamespace(req.Context(), requestInfo.Namespace))
			}

			gvr := schema.GroupVersionResource{
				Group:    requestInfo.APIGroup,
				Version:  requestInfo.APIVersion,
				Resource: requestInfo.Resource,
			}

			h, err := p.Connect(req.Context(), framework.ProxyRequest{
				RequestInfo:          requestInfo,
				GroupVersionResource: gvr,
				ProxyPath:            "",
				Responder:            nil,
				HTTPReq:              req,
			})
			if err != nil {
				t.Error(err)
				return
			}
			h.ServeHTTP(&emptyResponseWriter{}, req)
			if tt.want.namespace != actual.namespace {
				t.Errorf("want namespace %v, get %v", tt.want.namespace, actual.namespace)
			}
			if tt.want.name != actual.name {
				t.Errorf("want name %v, get %v", tt.want.name, actual.name)
			}
			if tt.want.gvr != actual.gvr {
				t.Errorf("want gvr %v, get %v", tt.want.gvr, actual.gvr)
			}
			if !reflect.DeepEqual(tt.want.getOptions, actual.getOptions) {
				t.Errorf("getOptions diff: %v", cmp.Diff(tt.want.getOptions, actual.getOptions))
			}
			if !reflect.DeepEqual(tt.want.listOptions, actual.listOptions) {
				t.Errorf("listOptions diff: %v", diff.ObjectGoPrintSideBySide(tt.want.listOptions, actual.listOptions))
			}
		})
	}
}

type emptyResponseWriter struct{}

var _ http.ResponseWriter = &emptyResponseWriter{}
var _ http.Flusher = &emptyResponseWriter{}

func (n *emptyResponseWriter) Header() http.Header {
	return make(http.Header)
}

func (n *emptyResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (n *emptyResponseWriter) WriteHeader(int) {
}

func (n *emptyResponseWriter) Flush() {
}

type emptyWatch struct {
	ch       chan watch.Event
	isClosed bool
	lock     sync.Mutex
}

func newEmptyWatch() watch.Interface {
	w := &emptyWatch{
		ch: make(chan watch.Event),
	}

	return w
}

func (e *emptyWatch) Stop() {
	e.lock.Lock()
	defer e.lock.Unlock()

	if e.isClosed {
		return
	}
	e.isClosed = true
	close(e.ch)
}

func (e *emptyWatch) ResultChan() <-chan watch.Event {
	return e.ch
}

func asLabelSelector(s string) labels.Selector {
	selector, err := labels.Parse(s)
	if err != nil {
		panic(fmt.Sprintf("Fail to parse %s to labels: %v", s, err))
	}
	return selector
}
