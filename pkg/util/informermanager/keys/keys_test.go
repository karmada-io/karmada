package keys

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterWideKeyFunc(t *testing.T) {
	tests := []struct {
		name         string
		object       interface{}
		expectErr    bool
		expectKeyStr string
	}{
		{
			name: "namespace scoped resource in core group",
			object: &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
			},
			expectErr:    false,
			expectKeyStr: "v1, kind=Secret, foo/bar",
		},
		{
			name: "cluster scoped resource in core group",
			object: &v1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
			},
			expectErr:    false,
			expectKeyStr: "v1, kind=Node, foo/bar",
		},
		{
			name: "namespace scoped resource not in core group",
			object: &rbacv1.Role{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Role",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
			},
			expectErr:    false,
			expectKeyStr: "v1, kind=Role, foo/bar",
		},
		{
			name: "cluster scoped resource not in core group",
			object: &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Role",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
			},
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
