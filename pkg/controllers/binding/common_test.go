package binding

import (
	"reflect"
	"testing"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
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

func Test_mergeLabel(t *testing.T) {
	namespace := "fake-ns"
	bindingName := "fake-bindingName"
	rbUID := "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"

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
					UID:       types.UID(rbUID),
				},
			},
			scope: v1.NamespaceScoped,
			want: map[string]string{
				workv1alpha2.ResourceBindingUIDLabel:     rbUID,
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
					UID:  types.UID(rbUID),
				},
			},
			scope: v1.ClusterScoped,
			want: map[string]string{
				workv1alpha2.ClusterResourceBindingUIDLabel:     rbUID,
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
		name      string
		namespace string
		workload  *unstructured.Unstructured
		binding   metav1.Object
		scope     v1.ResourceScope
		want      map[string]string
	}{
		{
			name:      "NamespaceScoped",
			namespace: "test",
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
			name:      "ClusterScoped",
			namespace: "",
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
			if got := mergeAnnotations(tt.workload, tt.namespace, tt.binding, tt.scope); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeAnnotations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mergeConflictResolution(t *testing.T) {
	namespace := "fake-ns"
	workload := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": namespace,
			},
		},
	}
	workloadOverwrite := workload.DeepCopy()
	workloadOverwrite.SetAnnotations(map[string]string{workv1alpha2.ResourceConflictResolutionAnnotation: workv1alpha2.ResourceConflictResolutionOverwrite})
	workloadAbort := workload.DeepCopy()
	workloadAbort.SetAnnotations(map[string]string{workv1alpha2.ResourceConflictResolutionAnnotation: workv1alpha2.ResourceConflictResolutionAbort})
	workloadInvalid := workload.DeepCopy()
	workloadInvalid.SetAnnotations(map[string]string{workv1alpha2.ResourceConflictResolutionAnnotation: "unknown"})

	tests := []struct {
		name                        string
		workload                    *unstructured.Unstructured
		conflictResolutionInBinding policyv1alpha1.ConflictResolution
		annotations                 map[string]string
		want                        map[string]string
	}{
		{
			name:                        "EmptyInRT_OverwriteInRB",
			workload:                    &workload,
			conflictResolutionInBinding: policyv1alpha1.ConflictOverwrite,
			annotations:                 map[string]string{},
			want:                        map[string]string{workv1alpha2.ResourceConflictResolutionAnnotation: workv1alpha2.ResourceConflictResolutionOverwrite},
		},
		{
			name:                        "EmptyInRT_AbortInRB",
			workload:                    &workload,
			conflictResolutionInBinding: policyv1alpha1.ConflictAbort,
			annotations:                 map[string]string{},
			want:                        map[string]string{workv1alpha2.ResourceConflictResolutionAnnotation: workv1alpha2.ResourceConflictResolutionAbort},
		},
		{
			name:                        "OverwriteInRT_AbortInPP",
			workload:                    workloadOverwrite,
			conflictResolutionInBinding: policyv1alpha1.ConflictAbort,
			annotations:                 map[string]string{},
			want:                        map[string]string{workv1alpha2.ResourceConflictResolutionAnnotation: workv1alpha2.ResourceConflictResolutionOverwrite},
		},
		{
			name:                        "AbortInRT_OverwriteInPP",
			workload:                    workloadAbort,
			conflictResolutionInBinding: policyv1alpha1.ConflictOverwrite,
			annotations:                 map[string]string{},
			want:                        map[string]string{workv1alpha2.ResourceConflictResolutionAnnotation: workv1alpha2.ResourceConflictResolutionAbort},
		},
		{
			name:                        "InvalidInRT_OverwriteInPP",
			workload:                    workloadInvalid,
			conflictResolutionInBinding: policyv1alpha1.ConflictOverwrite,
			annotations:                 map[string]string{},
			want:                        map[string]string{workv1alpha2.ResourceConflictResolutionAnnotation: workv1alpha2.ResourceConflictResolutionOverwrite},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeConflictResolution(tt.workload, tt.conflictResolutionInBinding, tt.annotations); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeConflictResolution() = %v, want %v", got, tt.want)
			}
		})
	}
}
