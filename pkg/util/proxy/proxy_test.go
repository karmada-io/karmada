package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	utiltest "github.com/karmada-io/karmada/pkg/util/testing"
)

func TestConnectCluster(t *testing.T) {
	const (
		testToken = "token"
		testGroup = "group"
		testUser  = "user"
	)

	s := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/proxy" ||
			req.Header.Get("Authorization") != "bearer "+testToken ||
			req.Header.Get("Impersonate-Group") != testGroup ||
			req.Header.Get("Impersonate-User") != testUser {
			t.Errorf("bad request: %v, %v", req.URL.Path, req.Header)
			return
		}
		fmt.Fprintf(rw, "ok")
	}))
	defer s.Close()

	type args struct {
		ctx          context.Context
		cluster      *clusterapis.Cluster
		secretGetter func(context.Context, string, string) (*corev1.Secret, error)
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "apiEndpoint is empty",
			args: args{
				cluster: &clusterapis.Cluster{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cluster"},
					Spec:       clusterapis.ClusterSpec{},
				},
				secretGetter: nil,
			},
			wantErr: true,
			want:    "",
		},
		{
			name: "apiEndpoint is invalid",
			args: args{
				cluster: &clusterapis.Cluster{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cluster"},
					Spec:       clusterapis.ClusterSpec{APIEndpoint: "h :/ invalid"},
				},
				secretGetter: nil,
			},
			wantErr: true,
			want:    "",
		},
		{
			name: "ProxyURL is invalid",
			args: args{
				cluster: &clusterapis.Cluster{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cluster"},
					Spec: clusterapis.ClusterSpec{
						APIEndpoint: s.URL,
						ProxyURL:    "h :/ invalid",
					},
				},
				secretGetter: nil,
			},
			wantErr: true,
			want:    "",
		},
		{
			name: "ImpersonatorSecretRef is nil",
			args: args{
				cluster: &clusterapis.Cluster{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cluster"},
					Spec: clusterapis.ClusterSpec{
						APIEndpoint: s.URL,
						ProxyURL:    "http://proxy",
					},
				},
				secretGetter: nil,
			},
			wantErr: true,
			want:    "",
		},
		{
			name: "secret not found",
			args: args{
				cluster: &clusterapis.Cluster{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cluster"},
					Spec: clusterapis.ClusterSpec{
						APIEndpoint:           s.URL,
						ImpersonatorSecretRef: &clusterapis.LocalSecretReference{Namespace: "ns", Name: "secret"},
					},
				},
				secretGetter: func(_ context.Context, ns string, name string) (*corev1.Secret, error) {
					return nil, apierrors.NewNotFound(corev1.Resource("secrets"), name)
				},
			},
			wantErr: true,
			want:    "",
		},
		{
			name: "SecretTokenKey not found",
			args: args{
				cluster: &clusterapis.Cluster{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cluster"},
					Spec: clusterapis.ClusterSpec{
						APIEndpoint:           s.URL,
						ImpersonatorSecretRef: &clusterapis.LocalSecretReference{Namespace: "ns", Name: "secret"},
					},
				},
				secretGetter: func(_ context.Context, ns string, name string) (*corev1.Secret, error) {
					return &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
						Data:       map[string][]byte{},
					}, nil
				},
			},
			wantErr: true,
			want:    "",
		},
		{
			name: "no user found for request",
			args: args{
				ctx: context.TODO(),
				cluster: &clusterapis.Cluster{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cluster"},
					Spec: clusterapis.ClusterSpec{
						APIEndpoint:           s.URL,
						ImpersonatorSecretRef: &clusterapis.LocalSecretReference{Namespace: "ns", Name: "secret"},
					},
				},
				secretGetter: func(_ context.Context, ns string, name string) (*corev1.Secret, error) {
					return &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
						Data:       map[string][]byte{clusterapis.SecretTokenKey: []byte(testToken)},
					}, nil
				},
			},
			wantErr: false,
			want:    "Internal Server Error: \"\": no user found for request\n",
		},
		{
			name: "proxy success",
			args: args{
				ctx: request.WithUser(request.NewContext(), &user.DefaultInfo{Name: testUser, Groups: []string{testGroup, user.AllAuthenticated, user.AllUnauthenticated}}),
				cluster: &clusterapis.Cluster{
					ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cluster"},
					Spec: clusterapis.ClusterSpec{
						APIEndpoint:           s.URL,
						ImpersonatorSecretRef: &clusterapis.LocalSecretReference{Namespace: "ns", Name: "secret"},
					},
				},
				secretGetter: func(_ context.Context, ns string, name string) (*corev1.Secret, error) {
					return &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
						Data:       map[string][]byte{clusterapis.SecretTokenKey: []byte(testToken)},
					}, nil
				},
			},
			wantErr: false,
			want:    "ok",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.args.ctx
			if ctx == nil {
				ctx = context.TODO()
			}
			req, err := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1/xxx", nil)
			if err != nil {
				t.Fatal(err)
			}

			resp := httptest.NewRecorder()

			h, err := ConnectCluster(context.TODO(), tt.args.cluster, "proxy", tt.args.secretGetter, utiltest.NewResponder(resp))
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectCluster() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			h.ServeHTTP(resp, req)
			if t.Failed() {
				return
			}

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
