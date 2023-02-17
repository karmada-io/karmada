package utils

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"reflect"
	"testing"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestCreateBasic(t *testing.T) {
	type args struct {
		serverURL   string
		userName    string
		clusterName string
		caCert      []byte
	}
	tests := []struct {
		name string
		args args
		want *clientcmdapi.Config
	}{
		{
			name: "create config from args",
			args: args{
				serverURL:   "https://127.0.0.1:6443",
				userName:    "admin",
				clusterName: "local",
				caCert:      []byte{'c', 'a', 'c', 'e', 'r', 't'},
			},
			want: &clientcmdapi.Config{
				Clusters: map[string]*clientcmdapi.Cluster{
					"local": {
						Server:                   "https://127.0.0.1:6443",
						CertificateAuthorityData: []byte{'c', 'a', 'c', 'e', 'r', 't'},
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"local": {
						Cluster:  "local",
						AuthInfo: "admin",
					},
				},
				AuthInfos:      map[string]*clientcmdapi.AuthInfo{},
				CurrentContext: "local",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreateBasic(tt.args.serverURL, tt.args.userName, tt.args.clusterName, tt.args.caCert); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateBasic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateWithCerts(t *testing.T) {
	type args struct {
		serverURL   string
		userName    string
		clusterName string
		caCert      []byte
		clientKey   []byte
		clientCert  []byte
	}
	tests := []struct {
		name string
		args args
		want *clientcmdapi.Config
	}{
		{
			name: "create config from args with certs",
			args: args{
				serverURL:   "https://127.0.0.1:6443",
				userName:    "admin",
				clusterName: "local",
				caCert:      []byte{'c', 'a', 'c', 'e', 'r', 't'},
				clientKey:   []byte{'c', 'l', 'i', 'e', 'n', 't', 'k', 'e', 'y'},
				clientCert:  []byte{'c', 'l', 'i', 'e', 'n', 't', 'c', 'e', 'r', 't'},
			},
			want: &clientcmdapi.Config{
				Clusters: map[string]*clientcmdapi.Cluster{
					"local": {
						Server:                   "https://127.0.0.1:6443",
						CertificateAuthorityData: []byte{'c', 'a', 'c', 'e', 'r', 't'},
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"local": {
						Cluster:  "local",
						AuthInfo: "admin",
					},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"admin": {
						ClientKeyData:         []byte{'c', 'l', 'i', 'e', 'n', 't', 'k', 'e', 'y'},
						ClientCertificateData: []byte{'c', 'l', 'i', 'e', 'n', 't', 'c', 'e', 'r', 't'},
					},
				},
				CurrentContext: "local",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreateWithCerts(tt.args.serverURL, tt.args.userName, tt.args.clusterName, tt.args.caCert, tt.args.clientKey, tt.args.clientCert); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateWithCerts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func randString() string {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}

func TestWriteKubeConfigFromSpec(t *testing.T) {
	type args struct {
		serverURL      string
		userName       string
		clusterName    string
		kubeconfigPath string
		kubeconfigName string
		caCert         []byte
		clientKey      []byte
		clientCert     []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Test Write KubeConfig From Spec",
			args: args{
				serverURL:      "https://127.0.0.1:6443",
				userName:       "admin",
				clusterName:    "local",
				kubeconfigPath: "temp-kubeconfig-" + randString(),
				kubeconfigName: "karmada-test.kubeconfig",
				caCert:         []byte{'c', 'a', 'c', 'e', 'r', 't'},
				clientKey:      []byte{'c', 'l', 'i', 'e', 'n', 't', 'k', 'e', 'y'},
				clientCert:     []byte{'c', 'l', 'i', 'e', 'n', 't', 'c', 'e', 'r', 't'},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		// create config path
		if err := os.Mkdir(tt.args.kubeconfigPath, 0755); err != nil {
			t.Fatal(err)
		}
		t.Run(tt.name, func(t *testing.T) {
			if err := WriteKubeConfigFromSpec(tt.args.serverURL, tt.args.userName, tt.args.clusterName, tt.args.kubeconfigPath, tt.args.kubeconfigName, tt.args.caCert, tt.args.clientKey, tt.args.clientCert); (err != nil) != tt.wantErr {
				t.Errorf("WriteKubeConfigFromSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
		// cleanup test data
		os.RemoveAll(tt.args.kubeconfigPath)
	}
}
