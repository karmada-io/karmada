package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	karmadainformers "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	"github.com/karmada-io/karmada/pkg/search/proxy/store"
)

func TestModifyRequest(t *testing.T) {
	newObjectFunc := func(annotations map[string]string, resourceVersion string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("v1")
		obj.SetKind("Pod")
		obj.SetAnnotations(annotations)
		obj.SetResourceVersion(resourceVersion)
		return obj
	}

	type args struct {
		body    interface{}
		cluster string
	}
	type want struct {
		body interface{}
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Empty body",
			args: args{
				body: nil,
			},
			want: want{
				body: nil,
			},
		},
		{
			name: "Body with nil annotations",
			args: args{
				body: newObjectFunc(nil, ""),
			},
			want: want{
				body: newObjectFunc(nil, ""),
			},
		},
		{
			name: "Body with empty annotations",
			args: args{
				body: newObjectFunc(map[string]string{}, ""),
			},
			want: want{
				body: newObjectFunc(map[string]string{}, ""),
			},
		},
		{
			name: "Body with cache source annotation",
			args: args{
				body: newObjectFunc(map[string]string{clusterv1alpha1.CacheSourceAnnotationKey: "bar"}, ""),
			},
			want: want{
				body: newObjectFunc(map[string]string{}, ""),
			},
		},
		{
			name: "Body with single cluster resource version",
			args: args{
				body:    newObjectFunc(nil, "1234"),
				cluster: "cluster1",
			},
			want: want{
				body: newObjectFunc(nil, "1234"),
			},
		},
		{
			name: "Body with multi cluster resource version",
			args: args{
				body:    newObjectFunc(nil, store.BuildMultiClusterResourceVersion(map[string]string{"cluster1": "1234", "cluster2": "5678"})),
				cluster: "cluster1",
			},
			want: want{
				body: newObjectFunc(nil, "1234"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.args.body != nil {
				buf := bytes.NewBuffer(nil)
				err := json.NewEncoder(buf).Encode(tt.args.body)
				if err != nil {
					t.Error(err)
					return
				}
				body = buf
			}
			req, _ := http.NewRequest("PUT", "/api/v1/namespaces/default/pods/foo", body)
			err := modifyRequest(req, tt.args.cluster)
			if err != nil {
				t.Error(err)
				return
			}

			var get runtime.Object
			if req.ContentLength != 0 {
				data, err := ioutil.ReadAll(req.Body)
				if err != nil {
					t.Error(err)
					return
				}

				if int64(len(data)) != req.ContentLength {
					t.Errorf("expect contentLength %v, but got %v", len(data), req.ContentLength)
					return
				}

				get, _, err = unstructured.UnstructuredJSONScheme.Decode(data, nil, nil)
				if err != nil {
					t.Error(err)
					return
				}
			}

			if !reflect.DeepEqual(tt.want.body, get) {
				t.Errorf("get body diff: %v", cmp.Diff(tt.want.body, get))
			}
		})
	}
}

func Test_clusterProxy_connect(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(rw, "ok")
	}))

	reqCtx := request.WithUser(context.TODO(), &user.DefaultInfo{})

	type fields struct {
		store    store.Cache
		secrets  []runtime.Object
		clusters []runtime.Object
	}
	type args struct {
		ctx     context.Context
		request *http.Request
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
			name:   "missing requestInfo",
			fields: fields{},
			args: args{
				ctx: context.TODO(),
			},
			want: want{
				err: errors.New("missing requestInfo"),
			},
		},
		{
			name:   "create not supported",
			fields: fields{},
			args: args{
				ctx: request.WithRequestInfo(context.TODO(), &request.RequestInfo{Verb: "create"}),
			},
			want: want{
				err: apierrors.NewMethodNotSupported(podGVR.GroupResource(), "create"),
			},
		},
		{
			name: "get cache error",
			fields: fields{
				store: &cacheFuncs{
					GetResourceFromCacheFunc: func(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error) {
						return nil, "", errors.New("test error")
					},
				},
			},
			args: args{
				ctx: request.WithRequestInfo(context.TODO(), &request.RequestInfo{Verb: "get"}),
			},
			want: want{
				err: errors.New("test error"),
			},
		},
		{
			name: "cluster not found",
			fields: fields{
				store: &cacheFuncs{
					GetResourceFromCacheFunc: func(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error) {
						return nil, "cluster1", nil
					},
				},
			},
			args: args{
				ctx: request.WithRequestInfo(context.TODO(), &request.RequestInfo{Verb: "get"}),
			},
			want: want{
				err: apierrors.NewNotFound(clusterGVR.GroupResource(), "cluster1"),
			},
		},
		{
			name: "API endpoint of cluster cluster1 should not be empty",
			fields: fields{
				store: &cacheFuncs{
					GetResourceFromCacheFunc: func(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error) {
						return nil, "cluster1", nil
					},
				},
				clusters: []runtime.Object{&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Spec:       clusterv1alpha1.ClusterSpec{},
				}},
			},
			args: args{
				ctx: request.WithRequestInfo(context.TODO(), &request.RequestInfo{Verb: "get"}),
			},
			want: want{
				err: errors.New("API endpoint of cluster cluster1 should not be empty"),
			},
		},
		{
			name: "impersonatorSecretRef is nil",
			fields: fields{
				store: &cacheFuncs{
					GetResourceFromCacheFunc: func(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error) {
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
				ctx: request.WithRequestInfo(context.TODO(), &request.RequestInfo{Verb: "get"}),
			},
			want: want{
				err: errors.New("the impersonatorSecretRef of cluster cluster1 is nil"),
			},
		},
		{
			name: "secret not found",
			fields: fields{
				store: &cacheFuncs{
					GetResourceFromCacheFunc: func(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error) {
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
				ctx: request.WithRequestInfo(context.TODO(), &request.RequestInfo{Verb: "get"}),
			},
			want: want{
				err: apierrors.NewNotFound(secretGVR.GroupResource(), "secret"),
			},
		},
		{
			name: "response ok",
			fields: fields{
				store: &cacheFuncs{
					GetResourceFromCacheFunc: func(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error) {
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
				ctx:     request.WithRequestInfo(context.TODO(), &request.RequestInfo{Verb: "get"}),
				request: makeRequest(reqCtx, "GET", "/test", nil),
			},
			want: want{
				err:  nil,
				body: "ok",
			},
		},
		{
			name: "update error",
			fields: fields{
				store: &cacheFuncs{
					GetResourceFromCacheFunc: func(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error) {
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
				ctx: request.WithRequestInfo(context.TODO(), &request.RequestInfo{Verb: "update"}),
				request: (&http.Request{
					Method:        "PUT",
					URL:           &url.URL{Scheme: "https", Host: "localhost", Path: "/test"},
					Body:          ioutil.NopCloser(&alwaysErrorReader{}),
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
			stopCh := make(chan struct{})
			defer close(stopCh)
			kubeFactory := informers.NewSharedInformerFactory(fake.NewSimpleClientset(tt.fields.secrets...), 0)
			karmadaFactory := karmadainformers.NewSharedInformerFactory(karmadafake.NewSimpleClientset(tt.fields.clusters...), 0)

			c := &clusterProxy{
				store:         tt.fields.store,
				clusterLister: karmadaFactory.Cluster().V1alpha1().Clusters().Lister(),
				secretLister:  kubeFactory.Core().V1().Secrets().Lister(),
			}

			kubeFactory.Start(stopCh)
			karmadaFactory.Start(stopCh)
			kubeFactory.WaitForCacheSync(stopCh)
			karmadaFactory.WaitForCacheSync(stopCh)

			response := httptest.NewRecorder()

			h, err := c.connect(tt.args.ctx, podGVR, "/proxy", newTestResponder(response))
			if !errorEquals(err, tt.want.err) {
				t.Errorf("connect() error = %v, want %v", err, tt.want.err)
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
