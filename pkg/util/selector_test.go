package util

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func TestResourceMatches(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": "default",
				"labels": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}

	misMatch := policyv1alpha1.ResourceSelector{APIVersion: "v1", Kind: "Pod", Name: "unmatched"}
	matchAll := policyv1alpha1.ResourceSelector{APIVersion: "v1", Kind: "Pod"}
	matchLabels := policyv1alpha1.ResourceSelector{
		APIVersion: "v1", Kind: "Pod",
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
	}
	matchName := policyv1alpha1.ResourceSelector{APIVersion: "v1", Kind: "Pod", Name: "test"}

	if p := ResourceSelectorPriority(resource, misMatch); p != PriorityMisMatch {
		t.Errorf("misMatch shall not be %v", p)
		return
	}
	if p := ResourceSelectorPriority(resource, matchAll); p != PriorityMatchAll {
		t.Errorf("matchAll shall not be %v", p)
		return
	}
	if p := ResourceSelectorPriority(resource, matchLabels); p != PriorityMatchLabelSelector {
		t.Errorf("matchLabels shall not be %v", p)
		return
	}
	if p := ResourceSelectorPriority(resource, matchName); p != PriorityMatchName {
		t.Errorf("matchName shall not be %v", p)
		return
	}

	type args struct {
		rs policyv1alpha1.ResourceSelector
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "miss match",
			args: args{
				rs: misMatch,
			},
			want: false,
		},
		{
			name: "match all",
			args: args{
				rs: matchAll,
			},
			want: true,
		},
		{
			name: "match labels",
			args: args{
				rs: matchLabels,
			},
			want: true,
		},
		{
			name: "match name",
			args: args{
				rs: matchName,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResourceMatches(resource, tt.args.rs); got != tt.want {
				t.Errorf("ResourceMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceSelectorPriority(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": "default",
				"labels": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}

	type args struct {
		rs policyv1alpha1.ResourceSelector
	}
	tests := []struct {
		name string
		args args
		want ImplicitPriority
	}{
		{
			name: "APIVersion not matched",
			args: args{
				rs: policyv1alpha1.ResourceSelector{
					APIVersion: "unmatched",
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "default",
				},
			},
			want: PriorityMisMatch,
		},
		{
			name: "Kind not matched",
			args: args{
				rs: policyv1alpha1.ResourceSelector{
					APIVersion: "v1",
					Kind:       "Unmatched",
					Name:       "foo",
					Namespace:  "default",
				},
			},
			want: PriorityMisMatch,
		},
		{
			name: "namespace not matched",
			args: args{
				rs: policyv1alpha1.ResourceSelector{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "Unmatched",
				},
			},
			want: PriorityMisMatch,
		},
		{
			name: "namespace unset and name matched",
			args: args{
				rs: policyv1alpha1.ResourceSelector{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "test",
				},
			},
			want: PriorityMatchName,
		},
		{
			name: "[case 1] name not matched",
			args: args{
				rs: policyv1alpha1.ResourceSelector{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "unmatched",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			want: PriorityMisMatch,
		},
		{
			name: "[case 1] name matched, labels not matched and ignore",
			args: args{
				rs: policyv1alpha1.ResourceSelector{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "test",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "unmatched",
						},
					},
				},
			},
			want: PriorityMatchName,
		},
		{
			name: "[case 2] name not matched, labels unset",
			args: args{
				rs: policyv1alpha1.ResourceSelector{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "unmatched",
				},
			},
			want: PriorityMisMatch,
		},
		{
			name: "[case 2] name matched, labels unset",
			args: args{
				rs: policyv1alpha1.ResourceSelector{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "test",
				},
			},
			want: PriorityMatchName,
		},
		{
			name: "[case 3] name unset, labels not matched",
			args: args{
				rs: policyv1alpha1.ResourceSelector{
					APIVersion: "v1",
					Kind:       "Pod",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "unmatched",
						},
					},
				},
			},
			want: PriorityMisMatch,
		},
		{
			name: "[case 3] name unset, labels matched",
			args: args{
				rs: policyv1alpha1.ResourceSelector{
					APIVersion: "v1",
					Kind:       "Pod",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			want: PriorityMatchLabelSelector,
		},
		{
			name: "[case 4] name and labels unset, match all",
			args: args{
				rs: policyv1alpha1.ResourceSelector{
					APIVersion: "v1",
					Kind:       "Pod",
				},
			},
			want: PriorityMatchAll,
		},
		{
			name: "[case 4] labels error",
			args: args{
				rs: policyv1alpha1.ResourceSelector{
					APIVersion: "v1",
					Kind:       "Pod",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"": "bar",
						},
					},
				},
			},
			want: PriorityMisMatch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResourceSelectorPriority(resource, tt.args.rs); got != tt.want {
				t.Errorf("ResourceSelectorPriority() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClusterMatches(t *testing.T) {
	cluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster1",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Spec: clusterv1alpha1.ClusterSpec{
			Zone: "zone1",
		},
	}

	type args struct {
		affinity policyv1alpha1.ClusterAffinity
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "cluster excluded",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					ExcludeClusters: []string{cluster.Name},
				},
			},
			want: false,
		},
		{
			name: "[case 1] labels not matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "unmatched"},
					},
					ClusterNames: []string{},
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: ZoneField, Operator: corev1.NodeSelectorOpExists},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "[case 1] clusterNames not matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					ClusterNames: []string{"unmatched"},
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: ZoneField, Operator: corev1.NodeSelectorOpExists},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "[case 1] fields not matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					ClusterNames: []string{cluster.Name},
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: ProviderField, Operator: corev1.NodeSelectorOpExists},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "[case 1] matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					ClusterNames: []string{cluster.Name},
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: ZoneField, Operator: corev1.NodeSelectorOpExists},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "[case 2] labels not matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "unmatched"},
					},
				},
			},
			want: false,
		},
		{
			name: "[case 2] labels matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
				},
			},
			want: true,
		},
		{
			name: "[case 3] labels not matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "unmatched"},
					},
					ClusterNames: []string{cluster.Name},
				},
			},
			want: false,
		},
		{
			name: "[case 3] clusterNames not matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					ClusterNames: []string{"unmatched"},
				},
			},
			want: false,
		},
		{
			name: "[case 3] matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					ClusterNames: []string{cluster.Name},
				},
			},
			want: true,
		},
		{
			name: "[case 4] labels not matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "unmatched"},
					},
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: ZoneField, Operator: corev1.NodeSelectorOpExists},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "[case 4] fields not matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: ProviderField, Operator: corev1.NodeSelectorOpExists},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "[case 4] matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: ZoneField, Operator: corev1.NodeSelectorOpExists},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "[case 5] clusterNames matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{"unmatched"},
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: ZoneField, Operator: corev1.NodeSelectorOpExists},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "[case 5] fields matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{cluster.Name},
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: ProviderField, Operator: corev1.NodeSelectorOpExists},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "[case 5] matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{cluster.Name},
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: ZoneField, Operator: corev1.NodeSelectorOpExists},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "[case 6] clusterNames not matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{"unmatched"},
				},
			},
			want: false,
		},
		{
			name: "[case 6] matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{cluster.Name},
				},
			},
			want: true,
		},
		{
			name: "[case 7] fields not matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: ProviderField, Operator: corev1.NodeSelectorOpExists},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "[case 7] matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: ZoneField, Operator: corev1.NodeSelectorOpExists},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "[case 8] matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{},
			},
			want: true,
		},
		{
			name: "selector error",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"": "bar"},
					},
				},
			},
			want: false,
		},
		{
			name: "fields matched",
			args: args{
				affinity: policyv1alpha1.ClusterAffinity{
					FieldSelector: &policyv1alpha1.FieldSelector{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: ""},
						},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClusterMatches(cluster, tt.args.affinity); got != tt.want {
				t.Errorf("ClusterMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceMatchSelectors(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": "default",
				"labels": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}
	matched := policyv1alpha1.ResourceSelector{APIVersion: "v1", Kind: "Pod"}
	unmatched := policyv1alpha1.ResourceSelector{APIVersion: "v1", Kind: "Unmatched"}

	if !ResourceMatches(resource, matched) {
		t.Error("matched shall be matched")
		return
	}
	if ResourceMatches(resource, unmatched) {
		t.Error("unmatched shall not be matched")
		return
	}

	type args struct {
		selectors []policyv1alpha1.ResourceSelector
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "empty",
			args: args{
				selectors: nil,
			},
			want: false,
		},
		{
			name: "selectors are [matched, unmatched]",
			args: args{
				selectors: []policyv1alpha1.ResourceSelector{matched, unmatched},
			},
			want: true,
		},
		{
			name: "selectors are [unmatched, matched]",
			args: args{
				selectors: []policyv1alpha1.ResourceSelector{unmatched, matched},
			},
			want: true,
		},
		{
			name: "selectors are [unmatched, unmatched]",
			args: args{
				selectors: []policyv1alpha1.ResourceSelector{unmatched, unmatched},
			},
			want: false,
		},
		{
			name: "selectors are [matched, matched]",
			args: args{
				selectors: []policyv1alpha1.ResourceSelector{matched, matched},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResourceMatchSelectors(resource, tt.args.selectors...); got != tt.want {
				t.Errorf("ResourceMatchSelectors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceMatchSelectorsPriority(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": "default",
				"labels": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}

	p1 := policyv1alpha1.ResourceSelector{APIVersion: "v1", Kind: "Pod"}
	p3 := policyv1alpha1.ResourceSelector{APIVersion: "v1", Kind: "Pod", Name: "test"}

	if p := ResourceSelectorPriority(resource, p1); p != PriorityMatchAll {
		t.Errorf("p1 shall not be %v", p)
		return
	}
	if p := ResourceSelectorPriority(resource, p3); p != PriorityMatchName {
		t.Errorf("p3 shall not be %v", p)
		return
	}

	type args struct {
		selectors []policyv1alpha1.ResourceSelector
	}
	tests := []struct {
		name string
		args args
		want ImplicitPriority
	}{
		{
			name: "empty",
			args: args{
				selectors: nil,
			},
			want: 0,
		},
		{
			name: "priority of selectors are [1, 1]",
			args: args{
				selectors: []policyv1alpha1.ResourceSelector{p1, p1},
			},
			want: 1,
		},
		{
			name: "priority of selectors are [1, 3]",
			args: args{
				selectors: []policyv1alpha1.ResourceSelector{p1, p3},
			},
			want: 3,
		},
		{
			name: "priority of selectors are [3, 1]",
			args: args{
				selectors: []policyv1alpha1.ResourceSelector{p3, p1},
			},
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResourceMatchSelectorsPriority(resource, tt.args.selectors...); got != tt.want {
				t.Errorf("ResourceMatchSelectorsPriority() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_extractClusterFields(t *testing.T) {
	type args struct {
		cluster *clusterv1alpha1.Cluster
	}
	tests := []struct {
		name string
		args args
		want labels.Set
	}{
		{
			name: "empty",
			args: args{
				cluster: &clusterv1alpha1.Cluster{},
			},
			want: labels.Set{},
		},
		{
			name: "provider is set",
			args: args{
				cluster: &clusterv1alpha1.Cluster{
					Spec: clusterv1alpha1.ClusterSpec{
						Provider: "foo",
					},
				},
			},
			want: labels.Set{
				ProviderField: "foo",
			},
		},
		{
			name: "region is set",
			args: args{
				cluster: &clusterv1alpha1.Cluster{
					Spec: clusterv1alpha1.ClusterSpec{
						Region: "foo",
					},
				},
			},
			want: labels.Set{
				RegionField: "foo",
			},
		},
		{
			name: "zone is set",
			args: args{
				cluster: &clusterv1alpha1.Cluster{
					Spec: clusterv1alpha1.ClusterSpec{
						Zone: "foo",
					},
				},
			},
			want: labels.Set{
				ZoneField: "foo",
			},
		},
		{
			name: "all are set",
			args: args{
				cluster: &clusterv1alpha1.Cluster{
					Spec: clusterv1alpha1.ClusterSpec{
						Provider: "foo",
						Region:   "bar",
						Zone:     "baz",
					},
				},
			},
			want: labels.Set{
				ProviderField: "foo",
				RegionField:   "bar",
				ZoneField:     "baz",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractClusterFields(tt.args.cluster); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractClusterFields() = %v, want %v", got, tt.want)
			}
		})
	}
}
