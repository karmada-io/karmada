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

package util

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

// copy from go/src/net/http/internal/testcert/testcert.go
var testCA = []byte(`-----BEGIN CERTIFICATE-----
MIIDSDCCAjCgAwIBAgIQEP/md970HysdBTpuzDOf0DANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEAxcl69ROJdxjN+MJZnbFrYxyQooADCsJ6VDkuMyNQIix/Hk15Nk/u
FyBX1Me++aEpGmY3RIY4fUvELqT/srvAHsTXwVVSttMcY8pcAFmXSqo3x4MuUTG/
jCX3Vftj0r3EM5M8ImY1rzA/jqTTLJg00rD+DmuDABcqQvoXw/RV8w1yTRi5BPoH
DFD/AWTt/YgMvk1l2Yq/xI8VbMUIpjBoGXxWsSevQ5i2s1mk9/yZzu0Ysp1tTlzD
qOPa4ysFjBitdXiwfxjxtv5nXqOCP5rheKO0sWLk0fetMp1OV5JSJMAJw6c2ZMkl
U2WMqAEpRjdE/vHfIuNg+yGaRRqI07NZRQIDAQABo4GXMIGUMA4GA1UdDwEB/wQE
AwICpDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MB0GA1Ud
DgQWBBQR5QIzmacmw78ZI1C4MXw7Q0wJ1jA9BgNVHREENjA0ggtleGFtcGxlLmNv
bYINKi5leGFtcGxlLmNvbYcEfwAAAYcQAAAAAAAAAAAAAAAAAAAAATANBgkqhkiG
9w0BAQsFAAOCAQEACrRNgiioUDzxQftd0fwOa6iRRcPampZRDtuaF68yNHoNWbOu
LUwc05eOWxRq3iABGSk2xg+FXM3DDeW4HhAhCFptq7jbVZ+4Jj6HeJG9mYRatAxR
Y/dEpa0D0EHhDxxVg6UzKOXB355n0IetGE/aWvyTV9SiDs6QsaC57Q9qq1/mitx5
2GFBoapol9L5FxCc77bztzK8CpLujkBi25Vk6GAFbl27opLfpyxkM+rX/T6MXCPO
6/YBacNZ7ff1/57Etg4i5mNA6ubCpuc4Gi9oYqCNNohftr2lkJr7REdDR6OW0lsL
rF7r4gUnKeC7mYIH1zypY7laskopiLFAfe96Kg==
-----END CERTIFICATE-----`)

func TestNewClusterScaleClientSet(t *testing.T) {
	type args struct {
		clusterName string
		client      client.Client
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		errMsg  string
	}{
		{
			name: "cluster not found",
			args: args{
				clusterName: "test",
				client:      fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
			},
			wantErr: true,
			errMsg:  "clusters.cluster.karmada.io \"test\" not found",
		},
		{
			name: "APIEndpoint is empty",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).
					WithObjects(newCluster("test")).Build(),
			},
			wantErr: true,
			errMsg:  "the api endpoint of cluster test is empty",
		},
		{
			name: "SecretRef is empty",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).
					WithObjects(withAPIEndPoint(newCluster("test"), "https://127.0.0.1")).Build(),
			},
			wantErr: true,
			errMsg:  "cluster test does not have a secret",
		},
		{
			name: "Secret not found",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "default", Name: "secret1"},
						},
					}).Build(),
			},
			wantErr: true,
			errMsg:  "secrets \"secret1\" not found",
		},
		{
			name: "token not found",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
					}).Build(),
			},
			wantErr: true,
			errMsg:  "the secret for cluster test is missing a non-empty value for \"token\"",
		},
		{
			name: "valid configuration",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
					}).Build(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClusterScaleClientSet(tt.args.clusterName, tt.args.client)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.args.clusterName, got.ClusterName)
				assert.NotNil(t, got.KubeClient)
				assert.NotNil(t, got.ScaleClient)
			}
		})
	}
}

func TestNewClusterClientSetForAgent(t *testing.T) {
	type args struct {
		clusterName  string
		client       client.Client
		clientOption *ClientOption
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid configuration",
			args: args{
				clusterName:  "test-agent",
				client:       fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
				clientOption: &ClientOption{QPS: 100, Burst: 200},
			},
			wantErr: false,
		},
	}

	// Store the original GetConfig function
	originalGetConfig := controllerruntime.GetConfig
	// Defer its restoration
	defer func() { controllerruntime.GetConfig = originalGetConfig }()

	// Mock the GetConfig function
	controllerruntime.GetConfig = func() (*rest.Config, error) {
		return &rest.Config{
			Host: "https://fake.example.com",
		}, nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClusterClientSetForAgent(tt.args.clusterName, tt.args.client, tt.args.clientOption)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.args.clusterName, got.ClusterName)
				assert.NotNil(t, got.KubeClient)
			}
		})
	}
}

func TestNewClusterDynamicClientSetForAgent(t *testing.T) {
	type args struct {
		clusterName string
		client      client.Client
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid configuration",
			args: args{
				clusterName: "test-agent-dynamic",
				client:      fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
			},
			wantErr: false,
		},
	}

	// Store the original GetConfig function
	originalGetConfig := controllerruntime.GetConfig
	// Defer its restoration
	defer func() { controllerruntime.GetConfig = originalGetConfig }()

	// Mock the GetConfig function
	controllerruntime.GetConfig = func() (*rest.Config, error) {
		return &rest.Config{
			Host: "https://fake.example.com",
		}, nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClusterDynamicClientSetForAgent(tt.args.clusterName, tt.args.client)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.args.clusterName, got.ClusterName)
				assert.NotNil(t, got.DynamicClientSet)
			}
		})
	}
}

func TestNewClusterClientSet(t *testing.T) {
	type args struct {
		clusterName  string
		client       client.Client
		clientOption *ClientOption
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "cluster not found",
			args: args{
				clusterName:  "test",
				client:       fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
				clientOption: nil,
			},
			wantErr: true,
		},
		{
			name: "APIEndpoint is empty",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).
					WithObjects(newCluster("test")).Build(),
				clientOption: nil,
			},
			wantErr: true,
		},
		{
			name: "SecretRef is empty",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).
					WithObjects(withAPIEndPoint(newCluster("test"), "https://127.0.0.1")).Build(),
				clientOption: nil,
			},
			wantErr: true,
		},
		{
			name: "Secret not found",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "default", Name: "secret1"},
						},
					}).Build(),
				clientOption: nil,
			},
			wantErr: true,
		},
		{
			name: "token not found",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
					}).Build(),
				clientOption: nil,
			},
			wantErr: true,
		},
		{
			name: "CA data is set",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
					}).Build(),
				clientOption: &ClientOption{QPS: 100, Burst: 200},
			},
			wantErr: false,
		},
		{
			name: "skip TLS verification",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint:                 "https://127.0.0.1",
							SecretRef:                   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
							InsecureSkipTLSVerification: true,
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token")},
					}).Build(),
				clientOption: &ClientOption{QPS: 100, Burst: 200},
			},
			wantErr: false,
		},
		{
			name: "ProxyURL is error",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
							ProxyURL:    "://",
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA}}).Build(),
				clientOption: &ClientOption{QPS: 100, Burst: 200},
			},
			wantErr: true,
		},
		{
			name: "ProxyURL is set",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
							ProxyURL:    "http://1.1.1.1",
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
					}).Build(),
				clientOption: &ClientOption{QPS: 100, Burst: 200},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClusterClientSet(tt.args.clusterName, tt.args.client, tt.args.clientOption)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.args.clusterName, got.ClusterName)
				assert.NotNil(t, got.KubeClient)
			}
		})
	}
}

func TestNewClusterClientSet_ClientWorks(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Add("Content-Type", "application/json")
		_, _ = io.WriteString(rw, `
{
    "apiVersion": "v1",
    "kind": "Node",
    "metadata": {
        "name": "foo"
    }
}`)
	}))
	defer s.Close()

	const clusterName = "test"
	hostClient := fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
		&clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName},
			Spec: clusterv1alpha1.ClusterSpec{
				APIEndpoint: s.URL,
				SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
			Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
		}).Build()

	clusterClient, err := NewClusterClientSet(clusterName, hostClient, nil)
	assert.NoError(t, err)
	assert.NotNil(t, clusterClient)

	got, err := clusterClient.KubeClient.CoreV1().Nodes().Get(context.TODO(), "foo", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, got)

	want := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	assert.Equal(t, want, got)
}

func TestNewClusterDynamicClientSet(t *testing.T) {
	type args struct {
		clusterName string
		client      client.Client
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "cluster not found",
			args: args{
				clusterName: "test",
				client:      fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
			},
			wantErr: true,
		},
		{
			name: "APIEndpoint is empty",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).
					WithObjects(newCluster("test")).Build(),
			},
			wantErr: true,
		},
		{
			name: "SecretRef is empty",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).
					WithObjects(withAPIEndPoint(newCluster("test"), "https://127.0.0.1")).Build(),
			},
			wantErr: true,
		},
		{
			name: "Secret not found",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "default", Name: "secret1"},
						},
					}).Build(),
			},
			wantErr: true,
		},
		{
			name: "token not found",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
					}).Build(),
			},
			wantErr: true,
		},
		{
			name: "CA data is set",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
					}).Build(),
			},
			wantErr: false,
		},
		{
			name: "skip TLS verification",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
					}).Build(),
			},
			wantErr: false,
		},
		{
			name: "ProxyURL is error",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
							ProxyURL:    "://",
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
					}).Build(),
			},
			wantErr: true,
		},
		{
			name: "ProxyURL is set",
			args: args{
				clusterName: "test",
				client: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "test"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
							ProxyURL:    "http://1.1.1.1",
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
					}).Build(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClusterDynamicClientSet(tt.args.clusterName, tt.args.client)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.args.clusterName, got.ClusterName)
				assert.NotNil(t, got.DynamicClientSet)
			}
		})
	}
}

func TestNewClusterDynamicClientSet_ClientWorks(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Add("Content-Type", "application/json")
		_, _ = io.WriteString(rw, `
{
    "apiVersion": "v1",
    "kind": "Node",
    "metadata": {
        "name": "foo"
    }
}`)
	}))
	defer s.Close()

	const clusterName = "test"
	hostClient := fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
		&clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName},
			Spec: clusterv1alpha1.ClusterSpec{
				APIEndpoint: s.URL,
				SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
			Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
		}).Build()

	clusterClient, err := NewClusterDynamicClientSet(clusterName, hostClient)
	assert.NoError(t, err)
	assert.NotNil(t, clusterClient)

	nodeGVR := corev1.SchemeGroupVersion.WithResource("nodes")
	got, err := clusterClient.DynamicClientSet.Resource(nodeGVR).Get(context.TODO(), "foo", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, got)

	want := &unstructured.Unstructured{}
	want.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
	want.SetName("foo")

	assert.Equal(t, want, got)
}
