package util

import (
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
)

func TestCreateServiceAccount(t *testing.T) {
	type args struct {
		client kubernetes.Interface
		sa     *corev1.ServiceAccount
	}
	tests := []struct {
		name    string
		args    args
		want    *corev1.ServiceAccount
		wantErr bool
	}{
		{
			name: "already exist",
			args: args{
				client: fake.NewSimpleClientset(makeServiceAccount("test")),
				sa:     makeServiceAccount("test"),
			},
			want:    makeServiceAccount("test"),
			wantErr: false,
		},
		{
			name: "create error",
			args: args{
				client: alwaysErrorKubeClient,
				sa:     makeServiceAccount("test"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "create success",
			args: args{
				client: fake.NewSimpleClientset(),
				sa:     makeServiceAccount("test"),
			},
			want:    makeServiceAccount("test"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateServiceAccount(tt.args.client, tt.args.sa)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateServiceAccount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateServiceAccount() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteServiceAccount(t *testing.T) {
	type args struct {
		client    kubernetes.Interface
		namespace string
		name      string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "not found",
			args: args{
				client:    fake.NewSimpleClientset(),
				namespace: metav1.NamespaceDefault,
				name:      "test",
			},
			wantErr: false,
		},
		{
			name: "error",
			args: args{
				client:    alwaysErrorKubeClient,
				namespace: metav1.NamespaceDefault,
				name:      "test",
			},
			wantErr: true,
		},
		{
			name: "success",
			args: args{
				client:    fake.NewSimpleClientset(makeServiceAccount("test")),
				namespace: metav1.NamespaceDefault,
				name:      "test",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DeleteServiceAccount(tt.args.client, tt.args.namespace, tt.args.name); (err != nil) != tt.wantErr {
				t.Errorf("DeleteServiceAccount() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnsureServiceAccountExist(t *testing.T) {
	type args struct {
		client            kubernetes.Interface
		serviceAccountObj *corev1.ServiceAccount
		dryRun            bool
	}
	tests := []struct {
		name    string
		args    args
		want    *corev1.ServiceAccount
		wantErr bool
	}{
		{
			name: "dry run",
			args: args{
				client:            fake.NewSimpleClientset(),
				serviceAccountObj: makeServiceAccount("test"),
				dryRun:            true,
			},
			want:    makeServiceAccount("test"),
			wantErr: false,
		},
		{
			name: "exist",
			args: args{
				client:            fake.NewSimpleClientset(makeServiceAccount("test")),
				serviceAccountObj: makeServiceAccount("test"),
				dryRun:            false,
			},
			want:    makeServiceAccount("test"),
			wantErr: false,
		},
		{
			name: "not exist",
			args: args{
				client:            fake.NewSimpleClientset(),
				serviceAccountObj: makeServiceAccount("test"),
				dryRun:            false,
			},
			want:    makeServiceAccount("test"),
			wantErr: false,
		},
		{
			name: "create error",
			args: args{
				client: func() kubernetes.Interface {
					c := fake.NewSimpleClientset()
					c.PrependReactor("create", "*", errorAction)
					return c
				}(),
				serviceAccountObj: makeServiceAccount("test"),
				dryRun:            false,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EnsureServiceAccountExist(tt.args.client, tt.args.serviceAccountObj, tt.args.dryRun)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureServiceAccountExist() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EnsureServiceAccountExist() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWaitForServiceAccountSecretCreation(t *testing.T) {
	sa := makeServiceAccount("test")
	saVisitCount, secretVisitCount := 0, 0

	// ROUND1: get serviceAccount returns not found
	// ROUND2: get secret not found
	//         create secret
	//         token is empty
	// ROUND3: token exist
	injectSecretToken := func(client *fake.Clientset, secret *corev1.Secret) error {
		secret = secret.DeepCopy()
		secret.Data = map[string][]byte{
			"token": []byte("test token"),
		}
		return client.Tracker().Update(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, secret, metav1.NamespaceDefault)
	}

	client := fake.NewSimpleClientset(makeServiceAccount("test"))
	client.PrependReactor("get", "serviceaccounts", func(action kubetesting.Action) (bool, runtime.Object, error) {
		saVisitCount++
		if saVisitCount == 1 {
			return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), "test")
		}
		return false, nil, nil
	})
	client.PrependReactor("get", "secrets", func(action kubetesting.Action) (bool, runtime.Object, error) {
		secretVisitCount++
		if secretVisitCount == 2 {
			s, err := client.Tracker().Get(action.GetResource(), action.GetNamespace(), "test")
			if err != nil {
				return true, nil, fmt.Errorf("secret shall be found: %v", err)
			}

			// We inject token here, so that next round will see the token
			if err := injectSecretToken(client, s.(*corev1.Secret)); err != nil {
				return true, nil, err
			}
			return true, s, nil
		}
		return false, nil, nil
	})

	got, err := WaitForServiceAccountSecretCreation(client, sa)
	if err != nil {
		t.Error(err)
		return
	}

	want := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: sa.Namespace,
			Name:      sa.Name,
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: sa.Name,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{
			"token": []byte("test token"),
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WaitForServiceAccountSecretCreation() got = %v, want %v", got, want)
	}
}

var (
	alwaysErrorKubeClient kubernetes.Interface
	errorAction           = func(kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("always error")
	}
)

func init() {
	c := fake.NewSimpleClientset()
	c.PrependReactor("*", "*", errorAction)
	alwaysErrorKubeClient = c
}

func makeServiceAccount(name string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
	}
}
