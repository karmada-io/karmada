/*
Copyright 2021 The Karmada Authors.

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

package keys

import (
	"fmt"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	secretObj = &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "bar",
		},
	}
	nodeObj = &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "bar",
		},
	}
	roleObj = &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "bar",
		},
	}
	clusterRoleObj = &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "bar",
		},
	}
	deploymentObj = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "bar",
		},
	}
	podWithEmptyGroup = &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "bar",
		},
	}

	podWithEmptyKind = &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "bar",
		},
	}

	secretWithEmptyNamespace = &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "",
			Name:      "bar",
		},
	}
)

func TestClusterWideKeyFunc(t *testing.T) {
	tests := []struct {
		name         string
		object       interface{}
		expectErr    bool
		expectKeyStr string
	}{
		{
			name:         "namespace scoped resource in core group",
			object:       secretObj,
			expectErr:    false,
			expectKeyStr: "v1, kind=Secret, foo/bar",
		},
		{
			name:         "cluster scoped resource in core group",
			object:       nodeObj,
			expectErr:    false,
			expectKeyStr: "v1, kind=Node, foo/bar",
		},
		{
			name:         "namespace scoped resource not in core group",
			object:       roleObj,
			expectErr:    false,
			expectKeyStr: "v1, kind=Role, foo/bar",
		},
		{
			name:         "cluster scoped resource not in core group",
			object:       clusterRoleObj,
			expectErr:    false,
			expectKeyStr: "v1, kind=Role, foo/bar",
		},
		{
			name:      "non runtime object should be error",
			object:    "non-runtime-object",
			expectErr: true,
		},
		{
			name:      "nil object should be error",
			object:    nil,
			expectErr: true,
		},
		{
			name:      "non APIVersion and kind runtime object should be error",
			object:    deploymentObj,
			expectErr: true,
		},
		{
			name:      "resource with empty group",
			object:    podWithEmptyGroup,
			expectErr: true,
		},
		{
			name:      "resource with empty kind",
			object:    podWithEmptyKind,
			expectErr: true,
		},
		{
			name:         "resource with empty namespace",
			object:       secretWithEmptyNamespace,
			expectErr:    false,
			expectKeyStr: "v1, kind=Secret, bar",
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			key, err := ClusterWideKeyFunc(tc.object)
			if err != nil {
				if tc.expectErr == false {
					t.Fatalf("not expect error but error happed: %v", err)
				}

				return
			}

			if key.String() != tc.expectKeyStr {
				t.Fatalf("expect key string: %s, but got: %s", tc.expectKeyStr, key.String())
			}
		})
	}
}

func TestFederatedKeyFunc(t *testing.T) {
	tests := []struct {
		name         string
		object       interface{}
		cluster      string
		expectErr    bool
		expectKeyStr string
	}{
		{
			name:         "namespace scoped resource in core group",
			object:       secretObj,
			cluster:      "member1",
			expectErr:    false,
			expectKeyStr: "cluster=member1, v1, kind=Secret, foo/bar",
		},
		{
			name:         "cluster scoped resource in core group",
			object:       nodeObj,
			cluster:      "member1",
			expectErr:    false,
			expectKeyStr: "cluster=member1, v1, kind=Node, foo/bar",
		},
		{
			name:         "namespace scoped resource not in core group",
			object:       roleObj,
			cluster:      "member1",
			expectErr:    false,
			expectKeyStr: "cluster=member1, v1, kind=Role, foo/bar",
		},
		{
			name:         "cluster scoped resource not in core group",
			object:       clusterRoleObj,
			cluster:      "member1",
			expectErr:    false,
			expectKeyStr: "cluster=member1, v1, kind=Role, foo/bar",
		},
		{
			name:      "non runtime object should be error",
			object:    "non-runtime-object",
			cluster:   "member1",
			expectErr: true,
		},
		{
			name:      "nil object should be error",
			object:    nil,
			cluster:   "member1",
			expectErr: true,
		},
		{
			name:      "empty cluster name should be error",
			object:    clusterRoleObj,
			cluster:   "",
			expectErr: true,
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			key, err := FederatedKeyFunc(tc.cluster, tc.object)
			if err != nil {
				if tc.expectErr == false {
					t.Fatalf("not expect error but got: %v", err)
				}
				return
			}

			if key.String() != tc.expectKeyStr {
				t.Fatalf("expect key string: %s, but got: %s", tc.expectKeyStr, key.String())
			}
		})
	}
}

func ExampleClusterWideKey_String() {
	key := ClusterWideKey{
		Group:     "apps",
		Version:   "v1",
		Kind:      "Namespace",
		Namespace: "default",
		Name:      "foo",
	}
	pKey := &key
	fmt.Printf("%s\n", key)
	fmt.Printf("%v\n", key)
	fmt.Printf("%s\n", key.String())
	fmt.Printf("%s\n", pKey)
	fmt.Printf("%v\n", pKey)
	fmt.Printf("%s\n", pKey.String())
	// Output:
	// apps/v1, kind=Namespace, default/foo
	// apps/v1, kind=Namespace, default/foo
	// apps/v1, kind=Namespace, default/foo
	// apps/v1, kind=Namespace, default/foo
	// apps/v1, kind=Namespace, default/foo
	// apps/v1, kind=Namespace, default/foo
}

func ExampleFederatedKey_String() {
	key := FederatedKey{
		Cluster: "karmada",
		ClusterWideKey: ClusterWideKey{
			Group:     "apps",
			Version:   "v1",
			Kind:      "Namespace",
			Namespace: "default",
			Name:      "foo"},
	}
	pKey := &key
	fmt.Printf("%s\n", key)
	fmt.Printf("%v\n", key)
	fmt.Printf("%s\n", key.String())
	fmt.Printf("%s\n", pKey)
	fmt.Printf("%v\n", pKey)
	fmt.Printf("%s\n", pKey.String())
	// Output:
	// cluster=karmada, apps/v1, kind=Namespace, default/foo
	// cluster=karmada, apps/v1, kind=Namespace, default/foo
	// cluster=karmada, apps/v1, kind=Namespace, default/foo
	// cluster=karmada, apps/v1, kind=Namespace, default/foo
	// cluster=karmada, apps/v1, kind=Namespace, default/foo
	// cluster=karmada, apps/v1, kind=Namespace, default/foo
}

func TestNamespacedKeyFunc(t *testing.T) {
	type args struct {
		obj interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    NamespacedKey
		wantErr bool
	}{
		{
			name: "obj is deployment",
			args: args{
				obj: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
						Name:      "bar",
					},
				}},
			want: NamespacedKey{
				Namespace: "foo",
				Name:      "bar",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NamespacedKeyFunc(tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("NamespacedKeyFunc() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NamespacedKeyFunc() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNamespacedKey_String(t *testing.T) {
	type fields struct {
		Namespace string
		Name      string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "obj is deployment",
			fields: fields{
				Namespace: "foo",
				Name:      "bar",
			},
			want: fmt.Sprintf("namespace=%s, name=%s", "foo", "bar"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := NamespacedKey{
				Namespace: tt.fields.Namespace,
				Name:      tt.fields.Name,
			}
			if got := k.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
