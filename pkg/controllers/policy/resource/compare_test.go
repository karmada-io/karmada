package resource

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func Test_GetHigherPriorityPropagationPolicy(t *testing.T) {
	type args struct {
		a *policyv1alpha1.PropagationPolicy
		b *policyv1alpha1.PropagationPolicy
	}
	tests := []struct {
		name string
		args args
		want *policyv1alpha1.PropagationPolicy
	}{
		{
			name: "Test 1",
			args: args{
				a: &policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "A",
					},
				},
				b: &policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "B",
					},
				},
			},
			want: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "A",
				},
			},
		},
		{
			name: "Test 2",
			args: args{
				a: &policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc",
					},
				},
				b: &policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ab",
					},
				},
			},
			want: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ab",
				},
			},
		},
		{
			name: "Test 3",
			args: args{
				a: &policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "",
					},
				},
				b: &policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ab",
					},
				},
			},
			want: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetHigherPriorityPropagationPolicy(tt.args.a, tt.args.b)
			if result.Name != tt.want.Name {
				t.Errorf("GetHigherPriorityPropagationPolicy() got = %v, want %v", result.Name, tt.want.Name)
			}
		})
	}
}

func Test_GetHigherPriorityClusterPropagationPolicy(t *testing.T) {
	type args struct {
		a *policyv1alpha1.ClusterPropagationPolicy
		b *policyv1alpha1.ClusterPropagationPolicy
	}
	tests := []struct {
		name string
		args args
		want *policyv1alpha1.ClusterPropagationPolicy
	}{
		{
			name: "Test 1",
			args: args{
				a: &policyv1alpha1.ClusterPropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "A",
					},
				},
				b: &policyv1alpha1.ClusterPropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "B",
					},
				},
			},
			want: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "A",
				},
			},
		},
		{
			name: "Test 2",
			args: args{
				a: &policyv1alpha1.ClusterPropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "abc",
					},
				},
				b: &policyv1alpha1.ClusterPropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ab",
					},
				},
			},
			want: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ab",
				},
			},
		},
		{
			name: "Test 3",
			args: args{
				a: &policyv1alpha1.ClusterPropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "",
					},
				},
				b: &policyv1alpha1.ClusterPropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ab",
					},
				},
			},
			want: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetHigherPriorityClusterPropagationPolicy(tt.args.a, tt.args.b)
			if result.Name != tt.want.Name {
				t.Errorf("GetHigherPriorityClusterPropagationPolicy() got = %v, want %v", result.Name, tt.want.Name)
			}
		})
	}
}
