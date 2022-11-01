package detector

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
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
			name: "same length of name",
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
			name: "different length of name",
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
			name: "someone policy name is empty",
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
		{
			name: "someone policy is nil",
			args: args{
				a: nil,
				b: &policyv1alpha1.PropagationPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ab"}},
			},
			want: &policyv1alpha1.PropagationPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ab"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getHigherPriorityPropagationPolicy(tt.args.a, tt.args.b)
			if result.Name != tt.want.Name {
				t.Errorf("getHigherPriorityPropagationPolicy() got = %v, want %v", result.Name, tt.want.Name)
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
			name: "same length of name",
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
			name: "different length of name",
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
			name: "someone policy name is empty",
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
		{
			name: "someone policy is nil",
			args: args{
				a: nil,
				b: &policyv1alpha1.ClusterPropagationPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ab"}},
			},
			want: &policyv1alpha1.ClusterPropagationPolicy{ObjectMeta: metav1.ObjectMeta{Name: "ab"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getHigherPriorityClusterPropagationPolicy(tt.args.a, tt.args.b)
			if result.Name != tt.want.Name {
				t.Errorf("getHigherPriorityClusterPropagationPolicy() got = %v, want %v", result.Name, tt.want.Name)
			}
		})
	}
}

func Test_getHighestPriorityPropagationPolicies(t *testing.T) {
	type args struct {
		policies  []*policyv1alpha1.PropagationPolicy
		resource  *unstructured.Unstructured
		objectKey keys.ClusterWideKey
	}
	tests := []struct {
		name string
		args args
		want *policyv1alpha1.PropagationPolicy
	}{
		{
			name: "empty policies",
			args: args{
				policies: []*policyv1alpha1.PropagationPolicy{},
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "nginx",
							"namespace": "test",
							"labels": map[string]interface{}{
								"app": "nginx",
							}}}},
				objectKey: keys.ClusterWideKey{Kind: "Deployment", Namespace: "test", Name: "nginx"},
			},
			want: nil,
		},
		{
			name: "mo policy match for resource",
			args: args{
				policies: []*policyv1alpha1.PropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "match-with-name", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Name:       "nginx",
									Namespace:  "test",
								}}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "match-with-label", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion:    "apps/v1",
									Kind:          "Deployment",
									Namespace:     "test",
									LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
								}}},
					},
				},
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "nginx",
							"namespace": "test01",
							"labels": map[string]interface{}{
								"app": "nginx",
							}}}},
				objectKey: keys.ClusterWideKey{Kind: "Deployment", Namespace: "test01", Name: "nginx"},
			},
			want: nil,
		},
		{
			name: "different priority policy",
			args: args{
				policies: []*policyv1alpha1.PropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "match-with-name", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Name:       "nginx",
									Namespace:  "test",
								}}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "match-with-label", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion:    "apps/v1",
									Kind:          "Deployment",
									Namespace:     "test",
									LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
								}}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "match-all", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Namespace:  "test",
								}}},
					},
				},
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "nginx",
							"namespace": "test",
							"labels": map[string]interface{}{
								"app": "nginx",
							}}}},
				objectKey: keys.ClusterWideKey{Kind: "Deployment", Namespace: "test", Name: "nginx"},
			},
			want: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "match-with-name", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "nginx",
							Namespace:  "test",
						}}},
			},
		},
		{
			name: "same priority policy",
			args: args{
				policies: []*policyv1alpha1.PropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "a-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Name:       "nginx",
									Namespace:  "test",
								}}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "b-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Name:       "nginx",
									Namespace:  "test",
								}}},
					},
				},
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "nginx",
							"namespace": "test",
							"labels": map[string]interface{}{
								"app": "nginx",
							}}}},
				objectKey: keys.ClusterWideKey{Kind: "Deployment", Namespace: "test", Name: "nginx"},
			},
			want: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "a-pp", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "nginx",
							Namespace:  "test",
						}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getHighestPriorityPropagationPolicies(tt.args.policies, tt.args.resource, tt.args.objectKey); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHighestPriorityPropagationPolicies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getHighestPriorityClusterPropagationPolicies(t *testing.T) {
	type args struct {
		policies  []*policyv1alpha1.ClusterPropagationPolicy
		resource  *unstructured.Unstructured
		objectKey keys.ClusterWideKey
	}
	tests := []struct {
		name string
		args args
		want *policyv1alpha1.ClusterPropagationPolicy
	}{
		{
			name: "empty policies",
			args: args{
				policies: []*policyv1alpha1.ClusterPropagationPolicy{},
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "nginx",
							"namespace": "test",
							"labels": map[string]interface{}{
								"app": "nginx",
							}}}},
				objectKey: keys.ClusterWideKey{Kind: "Deployment", Namespace: "test", Name: "nginx"},
			},
			want: nil,
		},
		{
			name: "mo policy match for resource",
			args: args{
				policies: []*policyv1alpha1.ClusterPropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "match-with-name", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Name:       "nginx",
									Namespace:  "test",
								}}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "match-with-label", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion:    "apps/v1",
									Kind:          "Deployment",
									Namespace:     "test",
									LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
								}}},
					},
				},
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "nginx",
							"namespace": "test01",
							"labels": map[string]interface{}{
								"app": "nginx",
							}}}},
				objectKey: keys.ClusterWideKey{Kind: "Deployment", Namespace: "test01", Name: "nginx"},
			},
			want: nil,
		},
		{
			name: "different priority policy",
			args: args{
				policies: []*policyv1alpha1.ClusterPropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "match-with-name", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Name:       "nginx",
									Namespace:  "test",
								}}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "match-with-label", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion:    "apps/v1",
									Kind:          "Deployment",
									Namespace:     "test",
									LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
								}}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "match-all", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Namespace:  "test",
								}}},
					},
				},
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "nginx",
							"namespace": "test",
							"labels": map[string]interface{}{
								"app": "nginx",
							}}}},
				objectKey: keys.ClusterWideKey{Kind: "Deployment", Namespace: "test", Name: "nginx"},
			},
			want: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "match-with-name", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "nginx",
							Namespace:  "test",
						}}},
			},
		},
		{
			name: "same priority policy",
			args: args{
				policies: []*policyv1alpha1.ClusterPropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "a-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Name:       "nginx",
									Namespace:  "test",
								}}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "b-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Name:       "nginx",
									Namespace:  "test",
								}}},
					},
				},
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "nginx",
							"namespace": "test",
							"labels": map[string]interface{}{
								"app": "nginx",
							}}}},
				objectKey: keys.ClusterWideKey{Kind: "Deployment", Namespace: "test", Name: "nginx"},
			},
			want: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "a-pp", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "nginx",
							Namespace:  "test",
						}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getHighestPriorityClusterPropagationPolicies(tt.args.policies, tt.args.resource, tt.args.objectKey); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHighestPriorityClusterPropagationPolicies() = %v, want %v", got, tt.want)
			}
		})
	}
}
