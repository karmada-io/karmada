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
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

func TestCreateOrUpdateSecret(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		exists, same, conflict, retry bool
		owner                         string
	}{
		{name: "create"},
		{name: "unchanged", exists: true, same: true},
		{name: "orphan", exists: true},
		{name: "same cluster", exists: true, owner: "member1"},
		{name: "other cluster", exists: true, owner: "member2", conflict: true},
		{name: "other cluster unchanged", exists: true, same: true, owner: "member2", conflict: true},
		{name: "retry update conflict", exists: true, retry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			desired := makeSecret("member1")
			desired.Data = map[string][]byte{"token": []byte("new"), "caBundle": []byte("new-ca")}
			old := desired.DeepCopy()
			old.Type = corev1.SecretTypeOpaque
			old.Labels = map[string]string{"preserve": "label"}
			old.Annotations = map[string]string{"preserve": "annotation"}
			old.ResourceVersion = "1"
			if !tc.same {
				old.Data = map[string][]byte{"token": []byte("old")}
			}
			if tc.owner != "" {
				old.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: tc.owner, UID: "owner-uid"}}, clusterResourceKind)}
			}
			client := fake.NewClientset()
			if tc.exists {
				client = fake.NewClientset(old)
				var err error
				old, err = GetSecret(client, old.Namespace, old.Name)
				if err != nil {
					t.Fatal(err)
				}
			}
			updates := 0
			client.PrependReactor("update", "secrets", func(clienttesting.Action) (bool, runtime.Object, error) {
				updates++
				if tc.retry && updates == 1 {
					return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "secrets"}, desired.Name, fmt.Errorf("concurrent update"))
				}
				return false, nil, nil
			})
			got, created, err := CreateOrUpdateSecret(client, desired, "member1")
			if tc.conflict {
				if !apierrors.IsConflict(err) {
					t.Fatalf("error = %v, want conflict", err)
				}
				stored, getErr := GetSecret(client, old.Namespace, old.Name)
				if getErr != nil || !reflect.DeepEqual(stored, old) {
					t.Fatalf("conflicting secret changed: %v", getErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if created != !tc.exists {
				t.Errorf("created = %v", created)
			}
			want := desired.DeepCopy()
			if tc.exists {
				want = old.DeepCopy()
				want.Data = desired.Data
			}
			// Match fields populated by the fake API server separately from the
			// credential data and user-managed metadata being verified.
			want.TypeMeta = got.TypeMeta
			want.UID = got.UID
			want.ResourceVersion = got.ResourceVersion
			want.Generation = got.Generation
			want.ManagedFields = got.ManagedFields
			if !reflect.DeepEqual(got, want) {
				t.Errorf("secret = %#v, want %#v", got, want)
			}
			stored, err := client.CoreV1().Secrets(desired.Namespace).Get(context.TODO(), desired.Name, metav1.GetOptions{})
			if err != nil || !reflect.DeepEqual(stored, want) {
				t.Errorf("stored secret differs: %v", err)
			}
			if tc.same && updates != 0 {
				t.Error("unchanged data caused update")
			}
			if tc.retry && updates != 2 {
				t.Errorf("updates = %d, want 2", updates)
			}
		})
	}
}

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
				client: fake.NewClientset(makeSecret("test")),
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
				client: fake.NewClientset(),
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
			if got != nil {
				// remove fields injected by fake client
				got.TypeMeta = metav1.TypeMeta{}
				got.ResourceVersion = ""
				got.UID = ""
				got.Generation = 0
				got.ManagedFields = nil
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
				client:    fake.NewClientset(),
				namespace: metav1.NamespaceDefault,
				name:      "test",
			},
			wantErr: true,
		},
		{
			name: "get success",
			args: args{
				client:    fake.NewClientset(makeSecret("test")),
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
			if (err != nil) && tt.wantErr {
				return
			}

			if (err != nil) && !tt.wantErr {
				t.Fatalf("GetSecret() error = %v, wantErr %v", err, tt.wantErr)
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
				client:          fake.NewClientset(),
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
				client:          fake.NewClientset(makeSecret("test")),
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
				client:          fake.NewClientset(makeSecret("test")),
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
