package utils

import (
	"reflect"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterRoleFromRules(t *testing.T) {
	type args struct {
		name        string
		rules       []rbacv1.PolicyRule
		annotations map[string]string
		labels      map[string]string
	}
	tests := []struct {
		name string
		args args
		want *rbacv1.ClusterRole
	}{
		{
			name: "get clusterrole from rules",
			args: args{
				name: "foo",
				rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"cluster.karmada.io"},
						Resources: []string{"clusters/proxy"},
						Verbs:     []string{"*"},
					},
				},
				annotations: map[string]string{"foo": "bar"},
				labels:      map[string]string{"foo": "bar"},
			},
			want: &rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "foo",
					Annotations: map[string]string{"foo": "bar"},
					Labels:      map[string]string{"foo": "bar"},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"cluster.karmada.io"},
						Resources: []string{"clusters/proxy"},
						Verbs:     []string{"*"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClusterRoleFromRules(tt.args.name, tt.args.rules, tt.args.annotations, tt.args.labels); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClusterRoleFromRules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterRoleBindingFromSubjects(t *testing.T) {
	type args struct {
		clusterRoleBindingName string
		clusterRoleName        string
		sub                    []rbacv1.Subject
		labels                 map[string]string
	}
	tests := []struct {
		name string
		args args
		want *rbacv1.ClusterRoleBinding
	}{
		{
			name: "get clusterrolebinding from sub",
			args: args{
				clusterRoleBindingName: "foo",
				clusterRoleName:        "bar",
				sub: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Namespace: "karmada",
						Name:      "foo",
					},
				},
				labels: map[string]string{"foo": "bar"},
			},
			want: &rbacv1.ClusterRoleBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRoleBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   "foo",
					Labels: map[string]string{"foo": "bar"},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     "bar",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Namespace: "karmada",
						Name:      "foo",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClusterRoleBindingFromSubjects(tt.args.clusterRoleBindingName, tt.args.clusterRoleName, tt.args.sub, tt.args.labels); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClusterRoleBindingFromSubjects() = %v, want %v", got, tt.want)
			}
		})
	}
}
