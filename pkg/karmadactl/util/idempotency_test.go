package util

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"
)

func TestCreateOrUpdateSecret(t *testing.T) {
	type args struct {
		client kubernetes.Interface
		secret *corev1.Secret
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "create success",
			args: args{
				client: fake.NewSimpleClientset(),
				secret: makeSecret("test"),
			},
			wantErr: false,
		},
		{
			name: "update success",
			args: args{
				client: fake.NewSimpleClientset(makeSecret("test")),
				secret: makeSecret("test"),
			},
			wantErr: false,
		},
		{
			name: "create error",
			args: args{
				client: func() kubernetes.Interface {
					c := fake.NewSimpleClientset()
					c.PrependReactor("create", "*", func(action coretesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("create secret error")
					})
					return c
				}(),
				secret: makeSecret("test"),
			},
			wantErr: true,
		},
		{
			name: "update error",
			args: args{
				client: func() kubernetes.Interface {
					c := fake.NewSimpleClientset(makeSecret("test"))
					c.PrependReactor("update", "*", func(action coretesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("update secret error")
					})
					return c
				}(),
				secret: makeSecret("test"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CreateOrUpdateSecret(tt.args.client, tt.args.secret); (err != nil) != tt.wantErr {
				t.Errorf("CreateOrUpdateSecret() error = %v, wantErr %v", err, tt.wantErr)
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
