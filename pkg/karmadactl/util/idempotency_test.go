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
	"errors"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"
)

var errorAction = func(coretesting.Action) (handled bool, ret runtime.Object, err error) {
	return true, nil, fmt.Errorf("always error")
}

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
					c.PrependReactor("create", "*", func(coretesting.Action) (handled bool, ret runtime.Object, err error) {
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
					c.PrependReactor("update", "*", func(coretesting.Action) (handled bool, ret runtime.Object, err error) {
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

func TestCreateOrUpdateClusterRole(t *testing.T) {
	type args struct {
		client      kubernetes.Interface
		clusterRole *rbacv1.ClusterRole
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "not exist and create",
			args: args{
				client:      fake.NewSimpleClientset(),
				clusterRole: makeClusterRole("test"),
			},
			wantErr: false,
		},
		{
			name: "already exist and update",
			args: args{
				client:      fake.NewSimpleClientset(makeClusterRole("test")),
				clusterRole: makeClusterRole("test"),
			},
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
				clusterRole: makeClusterRole("test"),
			},
			wantErr: true,
		},
		{
			name: "update error",
			args: args{
				client: func() kubernetes.Interface {
					c := fake.NewSimpleClientset(makeClusterRole("test"))
					c.PrependReactor("update", "*", errorAction)
					return c
				}(),
				clusterRole: makeClusterRole("test"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CreateOrUpdateClusterRole(tt.args.client, tt.args.clusterRole); (err != nil) != tt.wantErr {
				t.Errorf("CreateOrUpdateClusterRole() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateOrUpdateClusterRoleBinding(t *testing.T) {
	type args struct {
		client             kubernetes.Interface
		clusterRoleBinding *rbacv1.ClusterRoleBinding
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "not exist and create",
			args: args{
				client:             fake.NewSimpleClientset(),
				clusterRoleBinding: makeClusterRoleBinding("test"),
			},
			wantErr: false,
		},
		{
			name: "already exist and update",
			args: args{
				client:             fake.NewSimpleClientset(makeClusterRole("test")),
				clusterRoleBinding: makeClusterRoleBinding("test"),
			},
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
				clusterRoleBinding: makeClusterRoleBinding("test"),
			},
			wantErr: true,
		},
		{
			name: "update error",
			args: args{
				client: func() kubernetes.Interface {
					c := fake.NewSimpleClientset(makeClusterRoleBinding("test"))
					c.PrependReactor("update", "*", errorAction)
					return c
				}(),
				clusterRoleBinding: makeClusterRoleBinding("test"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CreateOrUpdateClusterRoleBinding(tt.args.client, tt.args.clusterRoleBinding); (err != nil) != tt.wantErr {
				t.Errorf("CreateOrUpdateClusterRoleBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func makeClusterRole(name string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func makeClusterRoleBinding(name string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}
