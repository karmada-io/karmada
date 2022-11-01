package util

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateSecret(t *testing.T) {
	type args struct {
		client kubernetes.Interface
		secret *corev1.Secret
	}
	tests := []struct {
		name    string
		args    args
		want    *corev1.Secret
		wantErr bool
	}{
		{
			name: "already exist",
			args: args{
				client: fake.NewSimpleClientset(makeSecret("test")),
				secret: makeSecret("test"),
			},
			want:    makeSecret("test"),
			wantErr: false,
		},
		{
			name: "create error",
			args: args{
				client: alwaysErrorKubeClient,
				secret: makeSecret("test"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "create success",
			args: args{
				client: fake.NewSimpleClientset(),
				secret: makeSecret("test"),
			},
			want:    makeSecret("test"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateSecret(tt.args.client, tt.args.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateSecret() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSecret(t *testing.T) {
	type args struct {
		client    kubernetes.Interface
		namespace string
		name      string
	}
	tests := []struct {
		name    string
		args    args
		want    *corev1.Secret
		wantErr bool
	}{
		{
			name: "not found error",
			args: args{
				client:    fake.NewSimpleClientset(),
				namespace: metav1.NamespaceDefault,
				name:      "test",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "get success",
			args: args{
				client:    fake.NewSimpleClientset(makeSecret("test")),
				namespace: metav1.NamespaceDefault,
				name:      "test",
			},
			want:    makeSecret("test"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetSecret(tt.args.client, tt.args.namespace, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSecret() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPatchSecret(t *testing.T) {
	type args struct {
		client          kubernetes.Interface
		namespace       string
		name            string
		pt              types.PatchType
		patchSecretBody *corev1.Secret
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "not found error",
			args: args{
				client:          fake.NewSimpleClientset(),
				namespace:       metav1.NamespaceDefault,
				name:            "test",
				pt:              types.MergePatchType,
				patchSecretBody: makeSecret("test"),
			},
			wantErr: true,
		},
		{
			name: "patchType is not supported",
			args: args{
				client:          fake.NewSimpleClientset(makeSecret("test")),
				namespace:       metav1.NamespaceDefault,
				name:            "test",
				pt:              "",
				patchSecretBody: makeSecret("test"),
			},
			wantErr: true,
		},
		{
			name: "patch success",
			args: args{
				client:          fake.NewSimpleClientset(makeSecret("test")),
				namespace:       metav1.NamespaceDefault,
				name:            "test",
				pt:              types.MergePatchType,
				patchSecretBody: makeSecret("test"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := PatchSecret(tt.args.client, tt.args.namespace, tt.args.name, tt.args.pt, tt.args.patchSecretBody); (err != nil) != tt.wantErr {
				t.Errorf("PatchSecret() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func makeSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      name,
		},
	}
}
