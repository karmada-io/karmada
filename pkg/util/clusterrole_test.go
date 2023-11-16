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
	"reflect"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEnsureClusterRoleBindingExist(t *testing.T) {
	type args struct {
		client             kubernetes.Interface
		clusterRoleBinding *rbacv1.ClusterRoleBinding
		dryRun             bool
	}
	tests := []struct {
		name    string
		args    args
		want    *rbacv1.ClusterRoleBinding
		wantErr bool
	}{
		{
			name: "dry run",
			args: args{
				client:             fake.NewSimpleClientset(),
				clusterRoleBinding: makeClusterRoleBinding("test"),
				dryRun:             true,
			},
			want:    makeClusterRoleBinding("test"),
			wantErr: false,
		},
		{
			name: "already exist",
			args: args{
				client:             fake.NewSimpleClientset(makeClusterRoleBinding("test")),
				clusterRoleBinding: makeClusterRoleBinding("test"),
				dryRun:             false,
			},
			want:    makeClusterRoleBinding("test"),
			wantErr: false,
		},
		{
			name: "not exist",
			args: args{
				client:             fake.NewSimpleClientset(),
				clusterRoleBinding: makeClusterRoleBinding("test"),
				dryRun:             false,
			},
			want:    makeClusterRoleBinding("test"),
			wantErr: false,
		},
		{
			name: "get error",
			args: args{
				client:             alwaysErrorKubeClient,
				clusterRoleBinding: makeClusterRoleBinding("test"),
				dryRun:             false,
			},
			want:    nil,
			wantErr: true,
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
				dryRun:             false,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EnsureClusterRoleBindingExist(tt.args.client, tt.args.clusterRoleBinding, tt.args.dryRun)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureClusterRoleBindingExist() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EnsureClusterRoleBindingExist() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureClusterRoleExist(t *testing.T) {
	type args struct {
		client      kubernetes.Interface
		clusterRole *rbacv1.ClusterRole
		dryRun      bool
	}
	tests := []struct {
		name    string
		args    args
		want    *rbacv1.ClusterRole
		wantErr bool
	}{
		{
			name: "dry run",
			args: args{
				client:      fake.NewSimpleClientset(),
				clusterRole: makeClusterRole("test"),
				dryRun:      true,
			},
			want:    makeClusterRole("test"),
			wantErr: false,
		},
		{
			name: "already exist",
			args: args{
				client:      fake.NewSimpleClientset(makeClusterRole("test")),
				clusterRole: makeClusterRole("test"),
				dryRun:      false,
			},
			want:    makeClusterRole("test"),
			wantErr: false,
		},
		{
			name: "not exist",
			args: args{
				client:      fake.NewSimpleClientset(),
				clusterRole: makeClusterRole("test"),
				dryRun:      false,
			},
			want:    makeClusterRole("test"),
			wantErr: false,
		},
		{
			name: "get error",
			args: args{
				client:      alwaysErrorKubeClient,
				clusterRole: makeClusterRole("test"),
				dryRun:      false,
			},
			want:    nil,
			wantErr: true,
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
				dryRun:      false,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EnsureClusterRoleExist(tt.args.client, tt.args.clusterRole, tt.args.dryRun)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureClusterRoleExist() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EnsureClusterRoleExist() got = %v, want %v", got, tt.want)
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
