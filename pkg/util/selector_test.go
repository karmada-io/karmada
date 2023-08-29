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
			Zones:    []string{"zone1", "zone2", "zone3"},
			Region:   "region1",
			Provider: "provider1",
		},
	}

	tests := []struct {
		name     string
		affinity policyv1alpha1.ClusterAffinity
		want     bool
	}{
		{
			name: "test cluster excluded and names",
			affinity: policyv1alpha1.ClusterAffinity{
				ExcludeClusters: []string{cluster.Name},
				ClusterNames:    []string{cluster.Name},
			},
			want: false,
		},
		{
			name: "test cluster excluded and label selector",
			affinity: policyv1alpha1.ClusterAffinity{
				ExcludeClusters: []string{cluster.Name},
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
			},
			want: false,
		},
		{
			name: "test cluster excluded and field selector",
			affinity: policyv1alpha1.ClusterAffinity{
				ExcludeClusters: []string{cluster.Name},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ZoneField, Operator: corev1.NodeSelectorOpIn, Values: cluster.Spec.Zones},
					},
				},
			},
			want: false,
		},
		{
			name: "test cluster names",
			affinity: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
			},
			want: true,
		},
		{
			name: "test cluster names not matched",
			affinity: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{"cluster2"},
			},
			want: false,
		},
		{
			name: "test cluster names and label selector",
			affinity: policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				ClusterNames: []string{cluster.Name},
			},
			want: true,
		},
		{
			name: "test cluster names and label selector not matched",
			affinity: policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "unmatched"},
				},
				ClusterNames: []string{cluster.Name},
			},
			want: false,
		},
		{
			name: "test cluster names and field selector(zone & region)",
			affinity: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ZoneField, Operator: corev1.NodeSelectorOpIn, Values: cluster.Spec.Zones},
						{Key: RegionField, Operator: corev1.NodeSelectorOpIn, Values: []string{cluster.Spec.Region}},
					},
				},
			},
			want: true,
		},
		{
			name: "test cluster names and field selector(provider)",
			affinity: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ProviderField, Operator: corev1.NodeSelectorOpIn, Values: []string{cluster.Spec.Provider}},
					},
				},
			},
			want: true,
		},
		{
			name: "test cluster names and field selector(region)",
			affinity: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: RegionField, Operator: corev1.NodeSelectorOpIn, Values: []string{cluster.Spec.Region}},
					},
				},
			},
			want: true,
		},
		{
			name: "test cluster names and field selector(zone not in)",
			affinity: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ZoneField, Operator: corev1.NodeSelectorOpNotIn, Values: cluster.Spec.Zones},
					},
				},
			},
			want: false,
		},
		{
			name: "test cluster names and field selector(provider not in)",
			affinity: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ProviderField, Operator: corev1.NodeSelectorOpNotIn, Values: []string{cluster.Spec.Provider}},
					},
				},
			},
			want: false,
		},
		{
			name: "test cluster names and field selector(region not in)",
			affinity: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: RegionField, Operator: corev1.NodeSelectorOpNotIn, Values: []string{cluster.Spec.Region}},
					},
				},
			},
			want: false,
		},
		{
			name: "test label selector and field selector(zone)",
			affinity: policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ZoneField, Operator: corev1.NodeSelectorOpIn, Values: cluster.Spec.Zones},
					},
				},
			},
			want: true,
		},
		{
			name: "test label selector and field selector(provider)",
			affinity: policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ProviderField, Operator: corev1.NodeSelectorOpIn, Values: []string{cluster.Spec.Provider}},
					},
				},
			},
			want: true,
		},
		{
			name: "test label selector and field selector(region)",
			affinity: policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: RegionField, Operator: corev1.NodeSelectorOpIn, Values: []string{cluster.Spec.Region}},
					},
				},
			},
			want: true,
		},
		{
			name: "test label selector and field selector(zone not in)",
			affinity: policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ZoneField, Operator: corev1.NodeSelectorOpNotIn, Values: cluster.Spec.Zones},
					},
				},
			},
			want: false,
		},
		{
			name: "test label selector and field selector(provider)",
			affinity: policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ProviderField, Operator: corev1.NodeSelectorOpNotIn, Values: []string{cluster.Spec.Provider}},
					},
				},
			},
			want: false,
		},
		{
			name: "test label selector and field selector(region)",
			affinity: policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: RegionField, Operator: corev1.NodeSelectorOpNotIn, Values: []string{cluster.Spec.Region}},
					},
				},
			},
			want: false,
		},
		{
			name: "test label selector",
			affinity: policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
			},
			want: true,
		},
		{
			name: "test label selector not matched",
			affinity: policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "unmatched"},
				},
			},
			want: false,
		},
		{
			name:     "test empty cluster affinity matched",
			affinity: policyv1alpha1.ClusterAffinity{},
			want:     true,
		},
		{
			name: "test label selector error",
			affinity: policyv1alpha1.ClusterAffinity{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"": "bar"},
				},
			},
			want: false,
		},
		{
			name: "test field selector error",
			affinity: policyv1alpha1.ClusterAffinity{
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ""},
					},
				},
			},
			want: false,
		},
		{
			name: "test field selector zone matched",
			affinity: policyv1alpha1.ClusterAffinity{
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ZoneField, Operator: corev1.NodeSelectorOpIn, Values: cluster.Spec.Zones},
					},
				},
			},
			want: true,
		},
		{
			name: "test field selector zone not matched",
			affinity: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ZoneField, Operator: corev1.NodeSelectorOpNotIn, Values: cluster.Spec.Zones},
					},
				},
			},
			want: false,
		},
		{
			name: "test field selector zone not matched",
			affinity: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ZoneField, Operator: corev1.NodeSelectorOpIn, Values: []string{"zone2"}},
					},
				},
			},
			want: false,
		},
		{
			name: "test field selector region matched",
			affinity: policyv1alpha1.ClusterAffinity{
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: RegionField, Operator: corev1.NodeSelectorOpIn, Values: []string{cluster.Spec.Region}},
					},
				},
			},
			want: true,
		},
		{
			name: "test field selector region not matched",
			affinity: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: RegionField, Operator: corev1.NodeSelectorOpNotIn, Values: []string{cluster.Spec.Region}},
					},
				},
			},
			want: false,
		},
		{
			name: "test field selector region not matched",
			affinity: policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{cluster.Name},
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: RegionField, Operator: corev1.NodeSelectorOpIn, Values: []string{"region2"}},
					},
				},
			},
			want: false,
		},
		{
			name: "test field selector provider matched",
			affinity: policyv1alpha1.ClusterAffinity{
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ProviderField, Operator: corev1.NodeSelectorOpIn, Values: []string{cluster.Spec.Provider}},
					},
				},
			},
			want: true,
		},
		{
			name: "test field selector provider not matched",
			affinity: policyv1alpha1.ClusterAffinity{
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ProviderField, Operator: corev1.NodeSelectorOpNotIn, Values: []string{cluster.Spec.Provider}},
					},
				},
			},
			want: false,
		},
		{
			name: "test field selector provider not matched",
			affinity: policyv1alpha1.ClusterAffinity{
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ProviderField, Operator: corev1.NodeSelectorOpIn, Values: []string{"provider2"}},
					},
				},
			},
			want: false,
		},
		{
			name: "test field selector other key not matched",
			affinity: policyv1alpha1.ClusterAffinity{
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: "metadata.name", Operator: corev1.NodeSelectorOpIn, Values: []string{cluster.Name}},
					},
				},
			},
			want: false,
		},
		{
			name: "test field selector cluster names and label selector",
			affinity: policyv1alpha1.ClusterAffinity{
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ZoneField, Operator: corev1.NodeSelectorOpIn, Values: cluster.Spec.Zones},
					},
				},
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				ClusterNames: []string{cluster.Name},
			},
			want: true,
		},
		{
			name: "test field selector cluster names and label selector not matched",
			affinity: policyv1alpha1.ClusterAffinity{
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ZoneField, Operator: corev1.NodeSelectorOpIn, Values: cluster.Spec.Zones},
					},
				},
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				ClusterNames: []string{"cluster2"},
			},
			want: false,
		},
		{
			name: "test field selector cluster names and label selector not matched",
			affinity: policyv1alpha1.ClusterAffinity{
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ZoneField, Operator: corev1.NodeSelectorOpNotIn, Values: cluster.Spec.Zones},
					},
				},
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				ClusterNames: []string{cluster.Name},
			},
			want: false,
		},
		{
			name: "test field selector cluster names and label selector not matched",
			affinity: policyv1alpha1.ClusterAffinity{
				FieldSelector: &policyv1alpha1.FieldSelector{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: ZoneField, Operator: corev1.NodeSelectorOpIn, Values: cluster.Spec.Zones},
					},
				},
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "Unmatched"},
				},
				ClusterNames: []string{cluster.Name},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClusterMatches(cluster, tt.affinity); got != tt.want {
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
			name: "all are set",
			args: args{
				cluster: &clusterv1alpha1.Cluster{
					Spec: clusterv1alpha1.ClusterSpec{
						Provider: "foo",
						Region:   "bar",
					},
				},
			},
			want: labels.Set{
				ProviderField: "foo",
				RegionField:   "bar",
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

func Test_matchZones(t *testing.T) {
	tests := []struct {
		name                string
		zoneMatchExpression *corev1.NodeSelectorRequirement
		zones               []string
		matched             bool
	}{
		{
			name: "empty zones for In operator",
			zoneMatchExpression: &corev1.NodeSelectorRequirement{
				Key:      ZoneField,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"foo"},
			},
			zones:   nil,
			matched: false,
		},
		{
			name: "partial zones for In operator",
			zoneMatchExpression: &corev1.NodeSelectorRequirement{
				Key:      ZoneField,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"foo"},
			},
			zones:   []string{"foo", "bar"},
			matched: false,
		},
		{
			name: "all zones for In operator",
			zoneMatchExpression: &corev1.NodeSelectorRequirement{
				Key:      ZoneField,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"foo", "bar"},
			},
			zones:   []string{"foo", "bar"},
			matched: true,
		},
		{
			name: "empty zones for NotIn operator",
			zoneMatchExpression: &corev1.NodeSelectorRequirement{
				Key:      ZoneField,
				Operator: corev1.NodeSelectorOpNotIn,
				Values:   []string{"foo"},
			},
			zones:   nil,
			matched: true,
		},
		{
			name: "partial zones for NotIn operator",
			zoneMatchExpression: &corev1.NodeSelectorRequirement{
				Key:      ZoneField,
				Operator: corev1.NodeSelectorOpNotIn,
				Values:   []string{"foo"},
			},
			zones:   []string{"foo", "bar"},
			matched: false,
		},
		{
			name: "empty zones for Exists operator",
			zoneMatchExpression: &corev1.NodeSelectorRequirement{
				Key:      ZoneField,
				Operator: corev1.NodeSelectorOpExists,
				Values:   nil,
			},
			zones:   nil,
			matched: false,
		},
		{
			name: "empty zones for DoesNotExist operator",
			zoneMatchExpression: &corev1.NodeSelectorRequirement{
				Key:      ZoneField,
				Operator: corev1.NodeSelectorOpDoesNotExist,
				Values:   nil,
			},
			zones:   nil,
			matched: true,
		},
		{
			name: "unknown operator",
			zoneMatchExpression: &corev1.NodeSelectorRequirement{
				Key:      ZoneField,
				Operator: corev1.NodeSelectorOpGt,
				Values:   nil,
			},
			zones:   []string{"foo"},
			matched: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchZones(tt.zoneMatchExpression, tt.zones); got != tt.matched {
				t.Errorf("matchZones() got %v, but expected %v", got, tt.matched)
			}
		})
	}
}
