/*
Copyright 2024 The Karmada Authors.

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

package detector

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

func TestClusterWideKeyFunc(t *testing.T) {
	tests := []struct {
		name      string
		object    interface{}
		expectKey keys.ClusterWideKey
		expectErr bool
	}{
		{
			name: "cluster scoped resource in core group",
			object: &corev1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "bar",
				},
			},
			expectKey: keys.ClusterWideKey{
				Version: "v1",
				Kind:    "Node",
				Name:    "bar",
			},
			expectErr: false,
		},
		{
			name: "cluster scoped resource not in core group",
			object: &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterRole",
					APIVersion: "rbac.authorization.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "bar",
				},
			},
			expectKey: keys.ClusterWideKey{
				Group:   "rbac.authorization.k8s.io",
				Version: "v1",
				Kind:    "ClusterRole",
				Name:    "bar",
			},
			expectErr: false,
		},
		{
			name: "namespace scoped resource not in core group",
			object: &rbacv1.Role{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Role",
					APIVersion: "rbac.authorization.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
			},
			expectKey: keys.ClusterWideKey{
				Group:     "rbac.authorization.k8s.io",
				Version:   "v1",
				Kind:      "Role",
				Name:      "bar",
				Namespace: "foo",
			},
			expectErr: false,
		},
		{
			name:      "nil object should be error",
			object:    nil,
			expectKey: keys.ClusterWideKey{},
			expectErr: true,
		},
		{
			name: "non apiversion and kind runtime object should be error",
			object: appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
			},
			expectKey: keys.ClusterWideKey{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := ClusterWideKeyFunc(tt.object)
			if (err != nil) != tt.expectErr {
				t.Errorf("ClusterWideKeyFunc() err = %v, wantErr %v", err, tt.expectErr)
			}
			if !reflect.DeepEqual(key, tt.expectKey) {
				t.Errorf("ClusterWideKeyFunc() = '%v', want '%v'", key, tt.expectKey)
			}
		})
	}
}

func TestResourceItemKeyFunc(t *testing.T) {
	tests := []struct {
		name      string
		object    interface{}
		expectKey keys.ClusterWideKeyWithConfig
		expectErr bool
	}{
		{
			name: "cluster scoped resource in core group, resource changed by karmada",
			object: ResourceItem{
				Obj: &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				},
				ResourceChangeByKarmada: true,
			},
			expectKey: keys.ClusterWideKeyWithConfig{
				ClusterWideKey: keys.ClusterWideKey{
					Version: "v1",
					Kind:    "Node",
					Name:    "bar",
				},
				ResourceChangeByKarmada: true,
			},
			expectErr: false,
		},
		{
			name: "cluster scoped resource not in core group, resource not changed by karmada",
			object: ResourceItem{
				Obj: &rbacv1.ClusterRole{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterRole",
						APIVersion: "rbac.authorization.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				},
				ResourceChangeByKarmada: false,
			},
			expectKey: keys.ClusterWideKeyWithConfig{
				ClusterWideKey: keys.ClusterWideKey{
					Group:   "rbac.authorization.k8s.io",
					Version: "v1",
					Kind:    "ClusterRole",
					Name:    "bar",
				},
				ResourceChangeByKarmada: false,
			},
			expectErr: false,
		},
		{
			name: "namespace scoped resource not in core group, resource changed by karmada",
			object: ResourceItem{
				Obj: &rbacv1.Role{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Role",
						APIVersion: "rbac.authorization.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
						Name:      "bar",
					},
				},
				ResourceChangeByKarmada: true,
			},
			expectKey: keys.ClusterWideKeyWithConfig{
				ClusterWideKey: keys.ClusterWideKey{
					Group:     "rbac.authorization.k8s.io",
					Version:   "v1",
					Kind:      "Role",
					Name:      "bar",
					Namespace: "foo",
				},
				ResourceChangeByKarmada: true,
			},
			expectErr: false,
		},
		{
			name: "non apiversion and kind runtime object should be error",
			object: ResourceItem{
				Obj: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo",
						Name:      "bar",
					},
				},
				ResourceChangeByKarmada: true,
			},
			expectKey: keys.ClusterWideKeyWithConfig{ResourceChangeByKarmada: true},
			expectErr: true,
		},
		{
			name: "not type of resourceitem should be error",
			object: &rbacv1.Role{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Role",
					APIVersion: "rbac.authorization.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
			},
			expectKey: keys.ClusterWideKeyWithConfig{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := ResourceItemKeyFunc(tt.object)
			if (err != nil) != tt.expectErr {
				t.Errorf("ResourceItemKeyFunc() err = %v, wantErr %v", err, tt.expectErr)
			}
			if !reflect.DeepEqual(key, tt.expectKey) {
				t.Errorf("ResourceItemKeyFunc() = '%v', want '%v'", key, tt.expectKey)
			}
		})
	}
}
