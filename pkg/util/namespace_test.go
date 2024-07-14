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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"
)

func TestIsNamespaceExist(t *testing.T) {
	type args struct {
		client    *fake.Clientset
		namespace string
		reactor   coretesting.ReactionFunc
	}

	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "query namespace error",
			args: args{
				client:    fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
				namespace: metav1.NamespaceDefault,
				reactor: func(_ coretesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, &corev1.Namespace{}, errors.New("failed to get namespace")
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "namespace not exists",
			args: args{
				client:    fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default-1"}}),
				namespace: metav1.NamespaceDefault,
				reactor:   nil,
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "namespace already exists",
			args: args{
				client:    fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
				namespace: metav1.NamespaceDefault,
				reactor:   nil,
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.reactor != nil {
				tt.args.client.PrependReactor("get", "namespaces", tt.args.reactor)
			}

			got, err := IsNamespaceExist(tt.args.client, tt.args.namespace)
			if (err == nil && tt.wantErr == true) || (err != nil && tt.wantErr == false) {
				t.Errorf("IsNamespaceExist() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsNamespaceExist() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateNamespace(t *testing.T) {
	type args struct {
		client       *fake.Clientset
		namespaceObj *corev1.Namespace
		reactor      coretesting.ReactionFunc
	}
	tests := []struct {
		name    string
		args    args
		want    *corev1.Namespace
		wantErr bool
	}{
		{
			name: "success create namespace",
			args: args{
				client:       fake.NewSimpleClientset(),
				namespaceObj: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
				reactor:      nil,
			},
			want:    &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			wantErr: false,
		},
		{
			name: "namespace already exists",
			args: args{
				client:       fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
				namespaceObj: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			},
			want:    &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			wantErr: false,
		},
		{
			name: "create namespace error",
			args: args{
				client:       fake.NewSimpleClientset(),
				namespaceObj: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
				reactor: func(_ coretesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("failed to create namespace")
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.reactor != nil {
				tt.args.client.PrependReactor("create", "namespaces", tt.args.reactor)
			}
			got, err := CreateNamespace(tt.args.client, tt.args.namespaceObj)
			if (err == nil && tt.wantErr == true) || (err != nil && tt.wantErr == false) {
				t.Errorf("CreateNamespace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateNamespace() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteNamespace(t *testing.T) {
	type args struct {
		client    *fake.Clientset
		namespace string
		reactor   coretesting.ReactionFunc
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "namespace not exists",
			args: args{
				client:    fake.NewSimpleClientset(),
				namespace: "default",
				reactor:   nil,
			},
			wantErr: false,
		},
		{
			name: "delete namespace error",
			args: args{
				client:    fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
				namespace: "default",
				reactor: func(_ coretesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("failed to delete namespaces")
				},
			},
			wantErr: true,
		},
		{
			name: "success delete namespace",
			args: args{
				client:    fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
				namespace: "default",
				reactor:   nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.reactor != nil {
				tt.args.client.PrependReactor("delete", "namespaces", tt.args.reactor)
			}

			err := DeleteNamespace(tt.args.client, tt.args.namespace)
			if (err == nil && tt.wantErr == true) || (err != nil && tt.wantErr == false) {
				t.Errorf("DeleteNamespace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnsureNamespaceExist(t *testing.T) {
	type args struct {
		client        *fake.Clientset
		namespace     string
		dryRun        bool
		reactorGet    coretesting.ReactionFunc
		reactorCreate coretesting.ReactionFunc
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "dry run",
			args: args{
				client:    fake.NewSimpleClientset(),
				namespace: "default",
				dryRun:    true,
			},
			wantErr: false,
		},
		{
			name: "namespace already exists",
			args: args{
				client:    fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
				namespace: "default",
				reactorCreate: func(_ coretesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, apierrors.NewAlreadyExists(schema.ParseGroupResource("namespaces"), "default")
				},
			},
			wantErr: false,
		},
		{
			name: "check namespace exists error",
			args: args{
				client:    fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
				namespace: metav1.NamespaceDefault,
				reactorGet: func(_ coretesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("failed to get namespace")
				},
			},
			wantErr: true,
		},
		{
			name: "create namespace error",
			args: args{
				client:    fake.NewSimpleClientset(),
				namespace: metav1.NamespaceDefault,
				reactorCreate: func(_ coretesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("failed to create namespace")
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.reactorGet != nil {
				tt.args.client.PrependReactor("get", "namespaces", tt.args.reactorGet)
			}

			if tt.args.reactorCreate != nil {
				tt.args.client.PrependReactor("create", "namespaces", tt.args.reactorCreate)
			}

			_, err := EnsureNamespaceExist(tt.args.client, tt.args.namespace, tt.args.dryRun)
			if (err == nil && tt.wantErr == true) || (err != nil && tt.wantErr == false) {
				t.Errorf("EnsureNamespaceExist() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestEnsureNamespaceExistWithLabels(t *testing.T) {
	type args struct {
		client        *fake.Clientset
		namespace     string
		dryRun        bool
		labels        map[string]string
		reactorGet    coretesting.ReactionFunc
		reactorCreate coretesting.ReactionFunc
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "dry run",
			args: args{
				client:    fake.NewSimpleClientset(),
				namespace: "default",
				dryRun:    true,
				labels:    map[string]string{"testkey": "testvalue"},
			},
			wantErr: false,
		},
		{
			name: "namespace already exists",
			args: args{
				client:    fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
				namespace: "default",
				labels:    map[string]string{"testkey": "testvalue"},
				reactorCreate: func(_ coretesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, apierrors.NewAlreadyExists(schema.ParseGroupResource("namespaces"), "default")
				},
			},
			wantErr: false,
		},
		{
			name: "check namespace exists error",
			args: args{
				client:    fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}),
				namespace: metav1.NamespaceDefault,
				labels:    map[string]string{"testkey": "testvalue"},
				reactorGet: func(_ coretesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("failed to get namespace")
				},
			},
			wantErr: true,
		},
		{
			name: "create namespace error",
			args: args{
				client:    fake.NewSimpleClientset(),
				namespace: metav1.NamespaceDefault,
				labels:    map[string]string{"testkey": "testvalue"},
				reactorCreate: func(_ coretesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("failed to create namespace")
				},
			},
			wantErr: true,
		},
		{
			name: "create namespace with nil labels",
			args: args{
				client:    fake.NewSimpleClientset(),
				namespace: metav1.NamespaceDefault,
				labels:    nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.reactorGet != nil {
				tt.args.client.PrependReactor("get", "namespaces", tt.args.reactorGet)
			}

			if tt.args.reactorCreate != nil {
				tt.args.client.PrependReactor("create", "namespaces", tt.args.reactorCreate)
			}

			_, err := EnsureNamespaceExistWithLabels(tt.args.client, tt.args.namespace, tt.args.dryRun, tt.args.labels)
			if (err == nil && tt.wantErr == true) || (err != nil && tt.wantErr == false) {
				t.Errorf("EnsureNamespaceExistWithLabels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
