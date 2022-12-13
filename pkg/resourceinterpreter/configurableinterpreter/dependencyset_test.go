package configurableinterpreter

import (
	"reflect"
	"testing"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

func Test_dependencySet_list(t *testing.T) {
	tests := []struct {
		name string
		s    dependencySet
		want []configv1alpha1.DependentObjectReference
	}{
		{
			name: "empty set",
			s:    newDependencySet(),
			want: []configv1alpha1.DependentObjectReference{},
		},
		{
			name: "new with items",
			s: newDependencySet([]configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Kind: "Secret", Namespace: "foo", Name: "foo"},
			}...),
			want: []configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Kind: "Secret", Namespace: "foo", Name: "foo"},
			},
		},
		{
			name: "insert with different items",
			s: newDependencySet([]configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Kind: "Secret", Namespace: "foo", Name: "foo"},
			}...).insert(
				[]configv1alpha1.DependentObjectReference{
					{APIVersion: "v1", Kind: "Configmap", Namespace: "foo", Name: "foo"},
				}...),
			want: []configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Kind: "Configmap", Namespace: "foo", Name: "foo"},
				{APIVersion: "v1", Kind: "Secret", Namespace: "foo", Name: "foo"},
			},
		},
		{
			name: "insert with same items",
			s: newDependencySet([]configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Kind: "Secret", Namespace: "foo", Name: "foo"},
			}...).insert(
				[]configv1alpha1.DependentObjectReference{
					{APIVersion: "v1", Kind: "Configmap", Namespace: "foo", Name: "foo"},
				}...).insert(
				[]configv1alpha1.DependentObjectReference{
					{APIVersion: "v1", Kind: "Secret", Namespace: "foo", Name: "foo"},
					{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "foo", Name: "foo"},
				}...),
			want: []configv1alpha1.DependentObjectReference{
				{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "foo", Name: "foo"},
				{APIVersion: "v1", Kind: "Configmap", Namespace: "foo", Name: "foo"},
				{APIVersion: "v1", Kind: "Secret", Namespace: "foo", Name: "foo"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.list(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dependencySet.list() = %v, want %v", got, tt.want)
			}
		})
	}
}
