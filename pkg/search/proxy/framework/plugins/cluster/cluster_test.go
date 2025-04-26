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

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	karmadainformers "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	"github.com/karmada-io/karmada/pkg/search/proxy/framework"
	"github.com/karmada-io/karmada/pkg/search/proxy/store"
	proxytest "github.com/karmada-io/karmada/pkg/search/proxy/testing"
	utiltest "github.com/karmada-io/karmada/pkg/util/testing"
)

func TestModifyRequest(t *testing.T) {
	const (
		contentTypeJSON     = "application/json"
		contentTypeProtobuf = "application/vnd.kubernetes.protobuf"
	)

	makePod := func(annotations map[string]string, resourceVersion string) *corev1.Pod {
		return &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Annotations:     annotations,
				ResourceVersion: resourceVersion,
			},
		}
	}

	jsonEncode := func(t *testing.T, obj runtime.Object) []byte {
		buf := &bytes.Buffer{}
		require.NoError(t, json.NewEncoder(buf).Encode(obj))
		return buf.Bytes()
	}

	protobufEncode := func(t *testing.T, obj runtime.Object) []byte {
		buf := &bytes.Buffer{}
		encoder, err := runtime.NewClientNegotiator(scheme.Codecs.WithoutConversion(), corev1.SchemeGroupVersion).Encoder(contentTypeProtobuf, nil)
		require.NoError(t, err)
		require.NoError(t, encoder.Encode(obj, buf))
		return buf.Bytes()
	}

	makeRequest := func(t *testing.T, contentType string, obj runtime.Object) *http.Request {
		var body io.Reader
		if obj != nil {
			switch contentType {
			case contentTypeJSON:
				body = bytes.NewBuffer(jsonEncode(t, obj))
			case contentTypeProtobuf:
				body = bytes.NewBuffer(protobufEncode(t, obj))
			default:
				t.Fatalf("unknown content-type %s", contentType)
			}
		}

		req, err := http.NewRequest(http.MethodPut, "", body)
		require.NoError(t, err)
		req.Header.Add("Content-Type", contentType)
		return req
	}

	type args struct {
		reqFunc     func(t *testing.T) *http.Request
		requestInfo framework.ProxyRequest
		cluster     string
	}
	type want struct {
		bodyFunc func(t *testing.T) []byte
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Empty body",
			args: args{
				reqFunc: func(t *testing.T) *http.Request {
					return makeRequest(t, contentTypeJSON, nil)
				},
				requestInfo: framework.ProxyRequest{
					RestMapper: &mockRestMapper{},
				},
			},
			want: want{
				bodyFunc: func(*testing.T) []byte {
					return nil
				},
			},
		},
		{
			name: "Body with nil annotations",
			args: args{
				reqFunc: func(t *testing.T) *http.Request {
					return makeRequest(t, contentTypeJSON, makePod(nil, ""))
				},
				requestInfo: framework.ProxyRequest{
					RestMapper:           &mockRestMapper{},
					GroupVersionResource: proxytest.PodGVR,
				},
			},
			want: want{
				bodyFunc: func(t *testing.T) []byte {
					return jsonEncode(t, makePod(nil, ""))
				},
			},
		},
		{
			name: "Body with empty annotations",
			args: args{
				reqFunc: func(t *testing.T) *http.Request {
					return makeRequest(t, contentTypeJSON, makePod(map[string]string{}, ""))
				},
				requestInfo: framework.ProxyRequest{
					RestMapper:           &mockRestMapper{},
					GroupVersionResource: proxytest.PodGVR,
				},
			},
			want: want{
				bodyFunc: func(t *testing.T) []byte {
					return jsonEncode(t, makePod(map[string]string{}, ""))
				},
			},
		},
		{
			name: "Body with cache source annotation",
			args: args{
				reqFunc: func(t *testing.T) *http.Request {
					obj := makePod(map[string]string{clusterv1alpha1.CacheSourceAnnotationKey: "bar"}, "")
					return makeRequest(t, contentTypeJSON, obj)
				},
				requestInfo: framework.ProxyRequest{
					RestMapper:           &mockRestMapper{},
					GroupVersionResource: proxytest.PodGVR,
				},
			},
			want: want{
				bodyFunc: func(t *testing.T) []byte {
					return jsonEncode(t, makePod(nil, ""))
				},
			},
		},
		{
			name: "Body with single cluster resource version",
			args: args{
				reqFunc: func(t *testing.T) *http.Request {
					return makeRequest(t, contentTypeJSON, makePod(nil, "1234"))
				},
				requestInfo: framework.ProxyRequest{
					RestMapper:           &mockRestMapper{},
					GroupVersionResource: proxytest.PodGVR,
				},
				cluster: "cluster1",
			},
			want: want{
				bodyFunc: func(t *testing.T) []byte {
					return jsonEncode(t, makePod(nil, "1234"))
				},
			},
		},
		{
			name: "Body with multi cluster resource version",
			args: args{
				reqFunc: func(t *testing.T) *http.Request {
					obj := makePod(nil, store.BuildMultiClusterResourceVersion(map[string]string{"cluster1": "1234", "cluster2": "5678"}))
					return makeRequest(t, contentTypeJSON, obj)
				},
				requestInfo: framework.ProxyRequest{
					RestMapper:           &mockRestMapper{},
					GroupVersionResource: proxytest.PodGVR,
				},
				cluster: "cluster1",
			},
			want: want{
				bodyFunc: func(t *testing.T) []byte {
					return jsonEncode(t, makePod(nil, "1234"))
				},
			},
		},
		{
			name: "pod with protobuf",
			args: args{
				reqFunc: func(t *testing.T) *http.Request {
					obj := makePod(map[string]string{clusterv1alpha1.CacheSourceAnnotationKey: "cluster1"}, "1234")
					return makeRequest(t, contentTypeProtobuf, obj)
				},
				requestInfo: framework.ProxyRequest{
					RestMapper:           &mockRestMapper{},
					GroupVersionResource: proxytest.PodGVR,
				},
				cluster: "cluster1",
			},
			want: want{
				bodyFunc: func(t *testing.T) []byte {
					return protobufEncode(t, makePod(nil, "1234"))
				},
			},
		},
		{
			name: "crd request",
			args: args{
				reqFunc: func(t *testing.T) *http.Request {
					obj := &unstructured.Unstructured{}
					obj.SetAPIVersion("testing.com/v1")
					obj.SetKind("TestCRD")
					obj.SetAnnotations(map[string]string{clusterv1alpha1.CacheSourceAnnotationKey: "cluster1"})
					return makeRequest(t, contentTypeJSON, obj)
				},
				requestInfo: framework.ProxyRequest{
					RestMapper:           &mockRestMapper{},
					GroupVersionResource: schema.GroupVersionResource{Group: "testing.com", Version: "v1", Resource: "testcrds"},
				},
				cluster: "cluster1",
			},
			want: want{
				bodyFunc: func(t *testing.T) []byte {
					obj := &unstructured.Unstructured{}
					obj.SetAPIVersion("testing.com/v1")
					obj.SetKind("TestCRD")
					obj.SetAnnotations(map[string]string{})
					return jsonEncode(t, obj)
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			req := tt.args.reqFunc(t)

			require.NoError(t, modifyRequest(req, tt.args.requestInfo, tt.args.cluster))
			var got []byte
			if req.Body != nil {
				got, err = io.ReadAll(req.Body)
				require.NoError(t, err)
				require.Equal(t, int(req.ContentLength), len(got), "ContentLength")
			}

			w := tt.want.bodyFunc(t)
			require.Equal(t, w, got)
		})
	}
}

func Test_clusterProxy_connect(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(rw, "ok")
	}))

	reqCtx := request.WithUser(context.TODO(), &user.DefaultInfo{})

	type fields struct {
		store    store.Store
		secrets  []runtime.Object
		clusters []runtime.Object
	}
	type args struct {
		requestInfo *request.RequestInfo
		request     *http.Request
	}
	type want struct {
		err  error
		body string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   want
	}{
		{
			name:   "create not supported",
			fields: fields{},
			args: args{
				requestInfo: &request.RequestInfo{Verb: "create"},
			},
			want: want{
				err: apierrors.NewMethodNotSupported(proxytest.PodGVR.GroupResource(), "create"),
			},
		},
		{
			name: "get cache error",
			fields: fields{
				store: &proxytest.MockStore{
					GetResourceFromCacheFunc: func(_ context.Context, _ schema.GroupVersionResource, _, _ string) (runtime.Object, string, error) {
						return nil, "", errors.New("test error")
					},
				},
			},
			args: args{
				requestInfo: &request.RequestInfo{Verb: "get"},
			},
			want: want{
				err: errors.New("test error"),
			},
		},
		{
			name: "cluster not found",
			fields: fields{
				store: &proxytest.MockStore{
					GetResourceFromCacheFunc: func(_ context.Context, _ schema.GroupVersionResource, _, _ string) (runtime.Object, string, error) {
						return nil, "cluster1", nil
					},
				},
			},
			args: args{
				requestInfo: &request.RequestInfo{Verb: "get"},
			},
			want: want{
				err: apierrors.NewNotFound(proxytest.ClusterGVR.GroupResource(), "cluster1"),
			},
		},
		{
			name: "API endpoint of cluster cluster1 should not be empty",
			fields: fields{
				store: &proxytest.MockStore{
					GetResourceFromCacheFunc: func(_ context.Context, _ schema.GroupVersionResource, _, _ string) (runtime.Object, string, error) {
						return nil, "cluster1", nil
					},
				},
				clusters: []runtime.Object{&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Spec:       clusterv1alpha1.ClusterSpec{},
				}},
			},
			args: args{
				requestInfo: &request.RequestInfo{Verb: "get"},
			},
			want: want{
				err: errors.New("API endpoint of cluster cluster1 should not be empty"),
			},
		},
		{
			name: "impersonatorSecretRef is nil",
			fields: fields{
				store: &proxytest.MockStore{
					GetResourceFromCacheFunc: func(_ context.Context, _ schema.GroupVersionResource, _, _ string) (runtime.Object, string, error) {
						return nil, "cluster1", nil
					},
				},
				clusters: []runtime.Object{&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Spec: clusterv1alpha1.ClusterSpec{
						APIEndpoint: s.URL,
					},
				}},
			},
			args: args{
				requestInfo: &request.RequestInfo{Verb: "get"},
			},
			want: want{
				err: errors.New("the impersonatorSecretRef of cluster cluster1 is nil"),
			},
		},
		{
			name: "secret not found",
			fields: fields{
				store: &proxytest.MockStore{
					GetResourceFromCacheFunc: func(_ context.Context, _ schema.GroupVersionResource, _, _ string) (runtime.Object, string, error) {
						return nil, "cluster1", nil
					},
				},
				clusters: []runtime.Object{&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Spec: clusterv1alpha1.ClusterSpec{
						APIEndpoint: s.URL,
						ImpersonatorSecretRef: &clusterv1alpha1.LocalSecretReference{
							Namespace: "default",
							Name:      "secret",
						},
					},
				}},
			},
			args: args{
				requestInfo: &request.RequestInfo{Verb: "get"},
			},
			want: want{
				err: apierrors.NewNotFound(proxytest.SecretGVR.GroupResource(), "secret"),
			},
		},
		{
			name: "response ok",
			fields: fields{
				store: &proxytest.MockStore{
					GetResourceFromCacheFunc: func(_ context.Context, _ schema.GroupVersionResource, _, _ string) (runtime.Object, string, error) {
						return nil, "cluster1", nil
					},
				},
				clusters: []runtime.Object{&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Spec: clusterv1alpha1.ClusterSpec{
						APIEndpoint: s.URL,
						ImpersonatorSecretRef: &clusterv1alpha1.LocalSecretReference{
							Namespace: "default",
							Name:      "secret",
						},
					},
				}},
				secrets: []runtime.Object{&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "secret",
					},
					Data: map[string][]byte{
						clusterapis.SecretTokenKey: []byte("token"),
					},
				}},
			},
			args: args{
				requestInfo: &request.RequestInfo{Verb: "get"},
				request:     makeRequest(reqCtx, "GET", "/test", nil),
			},
			want: want{
				err:  nil,
				body: "ok",
			},
		},
		{
			name: "update error",
			fields: fields{
				store: &proxytest.MockStore{
					GetResourceFromCacheFunc: func(_ context.Context, _ schema.GroupVersionResource, _, _ string) (runtime.Object, string, error) {
						return nil, "cluster1", nil
					},
				},
				clusters: []runtime.Object{&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Spec: clusterv1alpha1.ClusterSpec{
						APIEndpoint: s.URL,
						ImpersonatorSecretRef: &clusterv1alpha1.LocalSecretReference{
							Namespace: "default",
							Name:      "secret",
						},
					},
				}},
				secrets: []runtime.Object{&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "secret",
					},
					Data: map[string][]byte{
						clusterapis.SecretTokenKey: []byte("token"),
					},
				}},
			},
			args: args{
				requestInfo: &request.RequestInfo{Verb: "update"},
				request: (&http.Request{
					Method:        http.MethodPut,
					URL:           &url.URL{Scheme: "https", Host: "localhost", Path: "/test"},
					Body:          io.NopCloser(&alwaysErrorReader{}),
					ContentLength: 10,
					Header:        make(http.Header),
				}).WithContext(reqCtx),
			},
			want: want{
				err:  nil,
				body: io.ErrUnexpectedEOF.Error(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			kubeFactory := informers.NewSharedInformerFactory(fake.NewSimpleClientset(tt.fields.secrets...), 0)
			karmadaFactory := karmadainformers.NewSharedInformerFactory(karmadafake.NewSimpleClientset(tt.fields.clusters...), 0)

			c := &Cluster{
				store:         tt.fields.store,
				clusterLister: karmadaFactory.Cluster().V1alpha1().Clusters().Lister(),
				secretLister:  kubeFactory.Core().V1().Secrets().Lister(),
			}

			kubeFactory.Start(ctx.Done())
			karmadaFactory.Start(ctx.Done())
			kubeFactory.WaitForCacheSync(ctx.Done())
			karmadaFactory.WaitForCacheSync(ctx.Done())

			response := httptest.NewRecorder()

			h, err := c.Connect(context.TODO(), framework.ProxyRequest{
				RequestInfo:          tt.args.requestInfo,
				GroupVersionResource: proxytest.PodGVR,
				ProxyPath:            "/proxy",
				Responder:            utiltest.NewResponder(response),
				HTTPReq:              tt.args.request,
			})
			if !proxytest.ErrorMessageEquals(err, tt.want.err) {
				t.Errorf("Connect() error = %v, want %v", err, tt.want.err)
				return
			}
			if err != nil {
				return
			}
			if h == nil {
				t.Error("got handler nil")
			}

			h.ServeHTTP(response, tt.args.request)
			body := response.Body.String()
			if body != tt.want.body {
				t.Errorf("got body = %v, want %v", body, tt.want.body)
			}
		})
	}
}

func makeRequest(ctx context.Context, method, url string, body io.Reader) *http.Request {
	if ctx == nil {
		ctx = context.TODO()
	}
	r, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		panic(err)
	}
	return r
}

type alwaysErrorReader struct{}

func (alwaysErrorReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

type mockRestMapper struct {
	meta.RESTMapper
}

func (mockRestMapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	switch resource {
	case corev1.SchemeGroupVersion.WithResource("pods"):
		return resource.GroupVersion().WithKind("Pod"), nil
	case corev1.SchemeGroupVersion.WithResource("nodes"):
		return resource.GroupVersion().WithKind("Node"), nil
	default:
		return schema.GroupVersionKind{}, fmt.Errorf("unknown resource %s", resource)
	}
}
