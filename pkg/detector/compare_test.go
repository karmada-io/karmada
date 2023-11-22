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
			name: "different implicit priority policy",
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
			name: "same implicit priority policy",
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
		{
			name: "one policy with implicit priority, one policy with explicit priority 1",
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
							Priority: func() *int32 {
								p := int32(1)
								return &p
							}(),
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
				ObjectMeta: metav1.ObjectMeta{Name: "b-pp", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: func() *int32 {
						p := int32(1)
						return &p
					}(),
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
			name: "one policy with explicit priority 1(name match), one policy with explicit priority 2(label selector match)",
			args: args{
				policies: []*policyv1alpha1.PropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "a-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							Priority: func() *int32 {
								p := int32(1)
								return &p
							}(),
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
							Priority: func() *int32 {
								p := int32(2)
								return &p
							}(),
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
							"namespace": "test",
							"labels": map[string]interface{}{
								"app": "nginx",
							}}}},
				objectKey: keys.ClusterWideKey{Kind: "Deployment", Namespace: "test", Name: "nginx"},
			},
			want: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "b-pp", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: func() *int32 {
						p := int32(2)
						return &p
					}(),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						}}},
			},
		},
		{
			name: "two policies with explicit priority 1(name match), select the one with lower alphabetical order",
			args: args{
				policies: []*policyv1alpha1.PropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "a-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							Priority: func() *int32 {
								p := int32(1)
								return &p
							}(),
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
							Priority: func() *int32 {
								p := int32(1)
								return &p
							}(),
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Namespace:  "test",
									Name:       "nginx",
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
					Priority: func() *int32 {
						p := int32(1)
						return &p
					}(),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Namespace:  "test",
							Name:       "nginx",
						}}},
			},
		},
		{
			name: "one policy with explicit priority 1(name match), one policy with explicit priority 1(label selector match)",
			args: args{
				policies: []*policyv1alpha1.PropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "a-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							Priority: func() *int32 {
								p := int32(1)
								return &p
							}(),
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion:    "apps/v1",
									Kind:          "Deployment",
									Namespace:     "test",
									LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
								}}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "b-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							Priority: func() *int32 {
								p := int32(1)
								return &p
							}(),
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Namespace:  "test",
									Name:       "nginx",
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
				ObjectMeta: metav1.ObjectMeta{Name: "b-pp", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: func() *int32 {
						p := int32(1)
						return &p
					}(),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Namespace:  "test",
							Name:       "nginx",
						}}},
			},
		},
		{
			name: "one policy with explicit priority -1(name match), one policy with implicit priority(label selector match)",
			args: args{
				policies: []*policyv1alpha1.PropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "a-pp", Namespace: "test"},
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
						ObjectMeta: metav1.ObjectMeta{Name: "b-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							Priority: func() *int32 {
								p := int32(-1)
								return &p
							}(),
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Namespace:  "test",
									Name:       "nginx",
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
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getHighestPriorityPropagationPolicy(tt.args.policies, tt.args.resource, tt.args.objectKey); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHighestPriorityPropagationPolicy() = %v, want %v", got, tt.want)
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
			name: "different implicit priority policy",
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
			name: "same implicit priority policy",
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
		{
			name: "one policy with implicit priority, one policy with explicit priority 1",
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
							Priority: func() *int32 {
								p := int32(1)
								return &p
							}(),
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
				ObjectMeta: metav1.ObjectMeta{Name: "b-pp", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: func() *int32 {
						p := int32(1)
						return &p
					}(),
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
			name: "one policy with explicit priority 1(name match), one policy with explicit priority 2(label selector match)",
			args: args{
				policies: []*policyv1alpha1.ClusterPropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "a-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							Priority: func() *int32 {
								p := int32(1)
								return &p
							}(),
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
							Priority: func() *int32 {
								p := int32(2)
								return &p
							}(),
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
							"namespace": "test",
							"labels": map[string]interface{}{
								"app": "nginx",
							}}}},
				objectKey: keys.ClusterWideKey{Kind: "Deployment", Namespace: "test", Name: "nginx"},
			},
			want: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "b-pp", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: func() *int32 {
						p := int32(2)
						return &p
					}(),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						}}},
			},
		},
		{
			name: "two policies with explicit priority 1(name match), select the one with lower alphabetical order",
			args: args{
				policies: []*policyv1alpha1.ClusterPropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "a-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							Priority: func() *int32 {
								p := int32(1)
								return &p
							}(),
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
							Priority: func() *int32 {
								p := int32(1)
								return &p
							}(),
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Namespace:  "test",
									Name:       "nginx",
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
					Priority: func() *int32 {
						p := int32(1)
						return &p
					}(),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Namespace:  "test",
							Name:       "nginx",
						}}},
			},
		},
		{
			name: "one policy with explicit priority 1(name match), one policy with explicit priority 1(label selector match)",
			args: args{
				policies: []*policyv1alpha1.ClusterPropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "a-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							Priority: func() *int32 {
								p := int32(1)
								return &p
							}(),
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion:    "apps/v1",
									Kind:          "Deployment",
									Namespace:     "test",
									LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
								}}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "b-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							Priority: func() *int32 {
								p := int32(1)
								return &p
							}(),
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Namespace:  "test",
									Name:       "nginx",
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
				ObjectMeta: metav1.ObjectMeta{Name: "b-pp", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: func() *int32 {
						p := int32(1)
						return &p
					}(),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Namespace:  "test",
							Name:       "nginx",
						}}},
			},
		},
		{
			name: "one policy with explicit priority -1(name match), one policy with implicit priority(label selector match)",
			args: args{
				policies: []*policyv1alpha1.ClusterPropagationPolicy{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "a-pp", Namespace: "test"},
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
						ObjectMeta: metav1.ObjectMeta{Name: "b-pp", Namespace: "test"},
						Spec: policyv1alpha1.PropagationSpec{
							Priority: func() *int32 {
								p := int32(-1)
								return &p
							}(),
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Namespace:  "test",
									Name:       "nginx",
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
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getHighestPriorityClusterPropagationPolicy(tt.args.policies, tt.args.resource, tt.args.objectKey); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHighestPriorityPropagationPolicies() = %v, want %v", got, tt.want)
			}
		})
	}
}
