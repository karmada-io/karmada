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

package proxy

import (
	"context"
	"encoding/pem"
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
				secretGetter: func(_ context.Context, _ string, name string) (*corev1.Secret, error) {
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
						APIEndpoint:                 s.URL,
						ImpersonatorSecretRef:       &clusterapis.LocalSecretReference{Namespace: "ns", Name: "secret"},
						InsecureSkipTLSVerification: true,
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
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1/xxx", nil)
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

func TestGetTLSConfigForCluster(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer s.Close()
	validCA := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.Certificate().Raw})

	newCluster := func() *clusterapis.Cluster {
		return &clusterapis.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec: clusterapis.ClusterSpec{
				SecretRef: &clusterapis.LocalSecretReference{Namespace: "ns", Name: "secret"},
			},
		}
	}
	secretGetterWith := func(data map[string][]byte) SecretGetterFunc {
		return func(context.Context, string, string) (*corev1.Secret, error) {
			return &corev1.Secret{Data: data}, nil
		}
	}

	tests := []struct {
		name         string
		data         map[string][]byte
		wantErr      bool
		wantSubjects int
	}{
		{
			name:         "valid CA bundle",
			data:         map[string][]byte{clusterapis.SecretCADataKey: validCA},
			wantSubjects: 1,
		},
		{
			name:         "CA bundle not set",
			data:         map[string][]byte{},
			wantSubjects: 0,
		},
		{
			name:    "CA bundle without a valid PEM certificate",
			data:    map[string][]byte{clusterapis.SecretCADataKey: []byte("not a certificate")},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTLSConfigForCluster(context.TODO(), newCluster(), secretGetterWith(tt.data))
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetTLSConfigForCluster() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			//nolint:staticcheck // Subjects is fine for counting the certificates added to a custom pool.
			if n := len(got.RootCAs.Subjects()); n != tt.wantSubjects {
				t.Errorf("RootCAs has %d certificates, want %d", n, tt.wantSubjects)
			}
		})
	}
}
