package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	utiltest "github.com/karmada-io/karmada/pkg/util/testing"
)

func TestProxyREST_Connect(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/proxy" {
			_, _ = io.WriteString(rw, "ok")
		} else {
			_, _ = io.WriteString(rw, "bad request: "+req.URL.Path)
		}
	}))
	defer s.Close()

	type fields struct {
		secret        *corev1.Secret
		kubeClient    kubernetes.Interface
		clusterGetter func(ctx context.Context, name string) (*clusterapis.Cluster, error)
	}
	type args struct {
		id      string
		options runtime.Object
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "options is invalid",
			fields: fields{
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "ns"},
					Data:       map[string][]byte{clusterapis.SecretTokenKey: []byte("token")},
				},
				kubeClient: fake.NewSimpleClientset(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "ns"},
					Data:       map[string][]byte{clusterapis.SecretTokenKey: []byte("token")},
				}),
				clusterGetter: func(_ context.Context, name string) (*clusterapis.Cluster, error) {
					return &clusterapis.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: name},
						Spec: clusterapis.ClusterSpec{
							APIEndpoint:                 s.URL,
							ImpersonatorSecretRef:       &clusterapis.LocalSecretReference{Namespace: "ns", Name: "secret"},
							InsecureSkipTLSVerification: true,
						},
					}, nil
				},
			},
			args: args{
				id:      "cluster",
				options: &corev1.Pod{},
			},
			wantErr: true,
			want:    "",
		},
		{
			name: "cluster not found",
			fields: fields{
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "ns"},
					Data:       map[string][]byte{clusterapis.SecretTokenKey: []byte("token")},
				},
				kubeClient: fake.NewSimpleClientset(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "ns"},
					Data:       map[string][]byte{clusterapis.SecretTokenKey: []byte("token")},
				}),
				clusterGetter: func(_ context.Context, name string) (*clusterapis.Cluster, error) {
					return nil, apierrors.NewNotFound(clusterapis.Resource("clusters"), name)
				},
			},
			args: args{
				id:      "cluster",
				options: &clusterapis.ClusterProxyOptions{Path: "/proxy"},
			},
			wantErr: true,
			want:    "",
		},
		{
			name: "proxy success without secret cache",
			fields: fields{
				secret: &corev1.Secret{},
				kubeClient: fake.NewSimpleClientset(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "ns"},
					Data:       map[string][]byte{clusterapis.SecretTokenKey: []byte("token")},
				}),
				clusterGetter: func(_ context.Context, name string) (*clusterapis.Cluster, error) {
					return &clusterapis.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: name},
						Spec: clusterapis.ClusterSpec{
							APIEndpoint:                 s.URL,
							ImpersonatorSecretRef:       &clusterapis.LocalSecretReference{Namespace: "ns", Name: "secret"},
							InsecureSkipTLSVerification: true,
						},
					}, nil
				},
			},
			args: args{
				id:      "cluster",
				options: &clusterapis.ClusterProxyOptions{Path: "/proxy"},
			},
			wantErr: false,
			want:    "ok",
		},
		{
			name: "proxy success with secret cache",
			fields: fields{
				secret: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "ns"},
					Data:       map[string][]byte{clusterapis.SecretTokenKey: []byte("token")},
				},
				kubeClient: fake.NewSimpleClientset(),
				clusterGetter: func(_ context.Context, name string) (*clusterapis.Cluster, error) {
					return &clusterapis.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: name},
						Spec: clusterapis.ClusterSpec{
							APIEndpoint:                 s.URL,
							ImpersonatorSecretRef:       &clusterapis.LocalSecretReference{Namespace: "ns", Name: "secret"},
							InsecureSkipTLSVerification: true,
						},
					}, nil
				},
			},
			args: args{
				id:      "cluster",
				options: &clusterapis.ClusterProxyOptions{Path: "/proxy"},
			},
			wantErr: false,
			want:    "ok",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stopCh := make(chan struct{})
			defer close(stopCh)

			req, err := http.NewRequestWithContext(request.WithUser(request.NewContext(), &user.DefaultInfo{}), http.MethodGet, "http://127.0.0.1/xxx", nil)
			if err != nil {
				t.Fatal(err)
			}
			resp := httptest.NewRecorder()

			kubeFactory := informers.NewSharedInformerFactory(fake.NewSimpleClientset(tt.fields.secret), 0)
			r := &ProxyREST{
				secretLister:  kubeFactory.Core().V1().Secrets().Lister(),
				kubeClient:    tt.fields.kubeClient,
				clusterGetter: tt.fields.clusterGetter,
			}

			kubeFactory.Start(stopCh)
			kubeFactory.WaitForCacheSync(stopCh)

			h, err := r.Connect(req.Context(), tt.args.id, tt.args.options, utiltest.NewResponder(resp))
			if (err != nil) != tt.wantErr {
				t.Errorf("Connect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			h.ServeHTTP(resp, req)
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Error(err)
				return
			}
			if got := string(body); got != tt.want {
				t.Errorf("Connect() got = %v, want %v", got, tt.want)
			}
		})
	}
}
