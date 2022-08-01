package binding

import (
	"reflect"
	"testing"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func Test_mergeTargetClusters(t *testing.T) {
	tests := []struct {
		name                      string
		targetClusters            []workv1alpha2.TargetCluster
		requiredByBindingSnapshot []workv1alpha2.BindingSnapshot
		want                      []workv1alpha2.TargetCluster
	}{
		{
			name: "the same cluster",
			targetClusters: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
			},
			requiredByBindingSnapshot: []workv1alpha2.BindingSnapshot{
				{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "foo",
							Replicas: 1,
						},
					},
				},
			},
			want: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
			},
		},
		{
			name: "different clusters",
			targetClusters: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
			},
			requiredByBindingSnapshot: []workv1alpha2.BindingSnapshot{
				{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "bar",
							Replicas: 1,
						},
					},
				},
			},
			want: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
				{
					Name:     "bar",
					Replicas: 1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeTargetClusters(tt.targetClusters, tt.requiredByBindingSnapshot); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeTargetClusters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_transScheduleResultToMap(t *testing.T) {
	tests := []struct {
		name           string
		scheduleResult []workv1alpha2.TargetCluster
		want           map[string]int64
	}{
		{
			name: "one cluster",
			scheduleResult: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
			},
			want: map[string]int64{
				"foo": 1,
			},
		},
		{
			name: "different clusters",
			scheduleResult: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
				{
					Name:     "bar",
					Replicas: 2,
				},
			},
			want: map[string]int64{
				"foo": 1,
				"bar": 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transScheduleResultToMap(tt.scheduleResult); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("transScheduleResultToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getReplicaInfos(t *testing.T) {
	tests := []struct {
		name           string
		targetClusters []workv1alpha2.TargetCluster
		wantBool       bool
		wantRes        map[string]int64
	}{
		{
			name: "a cluster with replicas",
			targetClusters: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
			},
			wantBool: true,
			wantRes: map[string]int64{
				"foo": 1,
			},
		},
		{
			name: "none of the clusters have replicas",
			targetClusters: []workv1alpha2.TargetCluster{
				{
					Name: "foo",
				},
			},
			wantBool: false,
			wantRes:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := getReplicaInfos(tt.targetClusters)
			if got != tt.wantBool {
				t.Errorf("getReplicaInfos() got = %v, want %v", got, tt.wantBool)
			}
			if !reflect.DeepEqual(got1, tt.wantRes) {
				t.Errorf("getReplicaInfos() got1 = %v, want %v", got1, tt.wantRes)
			}
		})
	}
}

func Test_mergeLabel(t *testing.T) {
	namespace := "fake-ns"
	bindingName := "fake-bindingName"

	tests := []struct {
		name          string
		workload      *unstructured.Unstructured
		workNamespace string
		binding       metav1.Object
		scope         v1.ResourceScope
		want          map[string]string
	}{
		{
			name: "NamespaceScoped",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "demo-deployment",
						"namespace": namespace,
					},
				},
			},
			workNamespace: namespace,
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: namespace,
				},
			},
			scope: v1.NamespaceScoped,
			want: map[string]string{
				workv1alpha2.ResourceBindingReferenceKey: names.GenerateBindingReferenceKey(namespace, bindingName),
			},
		},
		{
			name: "ClusterScoped",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Namespace",
					"metadata": map[string]interface{}{
						"name": "demo-ns",
					},
				},
			},
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: bindingName,
				},
			},
			scope: v1.ClusterScoped,
			want: map[string]string{
				workv1alpha2.ClusterResourceBindingReferenceKey: names.GenerateBindingReferenceKey("", bindingName),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeLabel(tt.workload, tt.workNamespace, tt.binding, tt.scope); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mergeAnnotations(t *testing.T) {
	namespace := "fake-ns"
	bindingName := "fake-bindingName"

	tests := []struct {
		name     string
		workload *unstructured.Unstructured
		binding  metav1.Object
		scope    v1.ResourceScope
		want     map[string]string
	}{
		{
			name: "NamespaceScoped",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "demo-deployment",
						"namespace": namespace,
					},
				},
			},
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: namespace,
				},
			},
			scope: v1.NamespaceScoped,
			want: map[string]string{
				workv1alpha2.ResourceBindingNamespaceAnnotationKey: namespace,
				workv1alpha2.ResourceBindingNameAnnotationKey:      bindingName,
			},
		},
		{
			name: "ClusterScoped",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Namespace",
					"metadata": map[string]interface{}{
						"name": "demo-ns",
					},
				},
			},
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: bindingName,
				},
			},
			scope: v1.ClusterScoped,
			want: map[string]string{
				workv1alpha2.ClusterResourceBindingAnnotationKey: bindingName,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeAnnotations(tt.workload, tt.binding, tt.scope); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeAnnotations() = %v, want %v", got, tt.want)
			}
		})
	}
}
