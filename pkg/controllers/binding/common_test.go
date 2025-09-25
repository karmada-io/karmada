/*
Copyright 2022 The Karmada Authors.

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

package binding

import (
	"reflect"
	"sort"
	"testing"
	"time"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
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
	rbID := "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"

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
					Labels: map[string]string{
						workv1alpha2.ResourceBindingPermanentIDLabel: rbID,
					},
				},
			},
			scope: v1.NamespaceScoped,
			want: map[string]string{
				workv1alpha2.ResourceBindingPermanentIDLabel: rbID,
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
					Labels: map[string]string{
						workv1alpha2.ClusterResourceBindingPermanentIDLabel: rbID,
					},
				},
			},
			scope: v1.ClusterScoped,
			want: map[string]string{
				workv1alpha2.ClusterResourceBindingPermanentIDLabel: rbID,
			},
		},
	}

	checker := func(got, want map[string]string) bool {
		for key, val := range want {
			if got[key] != val {
				return false
			}
		}
		return true
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeLabel(tt.workload, tt.binding, tt.scope); !checker(got, tt.want) {
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

func Test_shouldSuspendDispatching(t *testing.T) {
	type args struct {
		suspension    *workv1alpha2.Suspension
		targetCluster workv1alpha2.TargetCluster
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "false for nil suspension",
			args: args{},
			want: false,
		},
		{
			name: "false for nil dispatching",
			args: args{
				suspension: &workv1alpha2.Suspension{Suspension: policyv1alpha1.Suspension{Dispatching: nil}},
			},
			want: false,
		},
		{
			name: "false for not suspension",
			args: args{
				suspension: &workv1alpha2.Suspension{Suspension: policyv1alpha1.Suspension{Dispatching: ptr.To(false)}},
			},
			want: false,
		},
		{
			name: "true for suspension",
			args: args{
				suspension: &workv1alpha2.Suspension{Suspension: policyv1alpha1.Suspension{Dispatching: ptr.To(true)}},
			},
			want: true,
		},
		{
			name: "true for matching cluster",
			args: args{
				suspension:    &workv1alpha2.Suspension{Suspension: policyv1alpha1.Suspension{DispatchingOnClusters: &policyv1alpha1.SuspendClusters{ClusterNames: []string{"clusterA"}}}},
				targetCluster: workv1alpha2.TargetCluster{Name: "clusterA"},
			},
			want: true,
		},
		{
			name: "false for mismatched cluster",
			args: args{
				suspension:    &workv1alpha2.Suspension{Suspension: policyv1alpha1.Suspension{DispatchingOnClusters: &policyv1alpha1.SuspendClusters{ClusterNames: []string{"clusterB"}}}},
				targetCluster: workv1alpha2.TargetCluster{Name: "clusterA"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSuspendDispatching(tt.args.suspension, tt.args.targetCluster); got != tt.want {
				t.Errorf("shouldSuspendDispatching() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_needReviseReplicas(t *testing.T) {
	tests := []struct {
		name      string
		replicas  int32
		placement *policyv1alpha1.Placement
		want      bool
	}{
		{
			name:     "replicas is zero",
			replicas: 0,
			want:     false,
		},
		{
			name:     "replicas is greater than zero",
			replicas: 1,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needReviseReplicas(tt.replicas); got != tt.want {
				t.Errorf("needReviseReplicas() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_needReviseJobCompletions(t *testing.T) {
	tests := []struct {
		name      string
		replicas  int32
		placement *policyv1alpha1.Placement
		want      bool
	}{
		{
			name:     "replicas is zero",
			replicas: 0,
			placement: &policyv1alpha1.Placement{
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
				},
			},
			want: false,
		},
		{
			name:      "placement is nil",
			replicas:  1,
			placement: nil,
			want:      false,
		},
		{
			name:     "replica scheduling type is not divided",
			replicas: 1,
			placement: &policyv1alpha1.Placement{
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
				},
			},
			want: false,
		},
		{
			name:     "replica scheduling type is divided",
			replicas: 1,
			placement: &policyv1alpha1.Placement{
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needReviseJobCompletions(tt.replicas, tt.placement); got != tt.want {
				t.Errorf("needReviseJobCompletions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_divideReplicasByJobCompletions(t *testing.T) {
	tests := []struct {
		name     string
		workload *unstructured.Unstructured
		clusters []workv1alpha2.TargetCluster
		want     []workv1alpha2.TargetCluster
		wantErr  bool
	}{
		{
			name: "completions found",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"completions": int64(10),
					},
				},
			},
			clusters: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 5},
				{Name: "cluster2", Replicas: 5},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 5},
				{Name: "cluster2", Replicas: 5},
			},
			wantErr: false,
		},
		{
			name: "error in NestedInt64",
			workload: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": "invalid",
				},
			},
			clusters: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 5},
				{Name: "cluster2", Replicas: 5},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := divideReplicasByJobCompletions(tt.workload, tt.clusters)
			if (err != nil) != tt.wantErr {
				t.Errorf("divideReplicasByJobCompletions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			sort.Slice(got, func(i, j int) bool {
				return got[i].Name < got[j].Name
			})
			sort.Slice(tt.want, func(i, j int) bool {
				return tt.want[i].Name < tt.want[j].Name
			})

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("divideReplicasByJobCompletions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_careBindingSpecChanged(t *testing.T) {
	// Create a base ResourceBindingSpec for testing
	baseTime := metav1.Now()
	baseSpec := workv1alpha2.ResourceBindingSpec{
		Resource: workv1alpha2.ObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  "default",
			Name:       "test-deployment",
		},
		Clusters: []workv1alpha2.TargetCluster{
			{Name: "cluster1", Replicas: 1},
			{Name: "cluster2", Replicas: 2},
		},
		GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
			{
				FromCluster: "cluster3",
				PurgeMode:   policyv1alpha1.PurgeModeGracefully,
				Reason:      workv1alpha2.EvictionReasonTaintUntolerated,
				Message:     "test eviction",
				Producer:    workv1alpha2.EvictionProducerTaintManager,
			},
		},
		RequiredBy: []workv1alpha2.BindingSnapshot{
			{
				Namespace: "test-ns",
				Name:      "test-binding",
				Clusters: []workv1alpha2.TargetCluster{
					{Name: "cluster4", Replicas: 1},
				},
			},
		},
		ConflictResolution: policyv1alpha1.ConflictAbort,
		Suspension: &workv1alpha2.Suspension{
			Suspension: policyv1alpha1.Suspension{
				Dispatching: ptr.To(true),
			},
		},
		PreserveResourcesOnDeletion: ptr.To(false),
		RescheduleTriggeredAt:       &baseTime,
	}

	tests := []struct {
		name           string
		oldBindingSpec workv1alpha2.ResourceBindingSpec
		newBindingSpec workv1alpha2.ResourceBindingSpec
		want           bool
	}{
		{
			name:           "identical specs",
			oldBindingSpec: baseSpec,
			newBindingSpec: baseSpec,
			want:           false,
		},
		{
			name:           "Resource field changed - APIVersion",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.Resource.APIVersion = "apps/v2"
				return spec
			}(),
			want: true,
		},
		{
			name:           "Resource field changed - Kind",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.Resource.Kind = "StatefulSet"
				return spec
			}(),
			want: true,
		},
		{
			name:           "Resource field changed - Namespace",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.Resource.Namespace = "other-namespace"
				return spec
			}(),
			want: true,
		},
		{
			name:           "Resource field changed - Name",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.Resource.Name = "other-deployment"
				return spec
			}(),
			want: true,
		},
		{
			name:           "Clusters field changed - different cluster name",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.Clusters = []workv1alpha2.TargetCluster{
					{Name: "cluster1", Replicas: 1},
					{Name: "cluster3", Replicas: 2}, // cluster2 -> cluster3
				}
				return spec
			}(),
			want: true,
		},
		{
			name:           "Clusters field changed - different replicas",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.Clusters = []workv1alpha2.TargetCluster{
					{Name: "cluster1", Replicas: 1},
					{Name: "cluster2", Replicas: 3}, // 2 -> 3
				}
				return spec
			}(),
			want: true,
		},
		{
			name:           "Clusters field changed - different length",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.Clusters = []workv1alpha2.TargetCluster{
					{Name: "cluster1", Replicas: 1},
				}
				return spec
			}(),
			want: true,
		},
		{
			name: "Clusters field changed - empty to non-empty",
			oldBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.Clusters = []workv1alpha2.TargetCluster{}
				return spec
			}(),
			newBindingSpec: baseSpec,
			want:           true,
		},
		{
			name:           "GracefulEvictionTasks field changed - different FromCluster",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.GracefulEvictionTasks = []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "cluster4", // cluster3 -> cluster4
						PurgeMode:   policyv1alpha1.PurgeModeGracefully,
						Reason:      workv1alpha2.EvictionReasonTaintUntolerated,
						Message:     "test eviction",
						Producer:    workv1alpha2.EvictionProducerTaintManager,
					},
				}
				return spec
			}(),
			want: true,
		},
		{
			name:           "GracefulEvictionTasks field changed - different PurgeMode",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.GracefulEvictionTasks = []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "cluster3",
						PurgeMode:   policyv1alpha1.PurgeModeDirectly, // PurgeModeGracefully -> PurgeModeDirectly
						Reason:      workv1alpha2.EvictionReasonTaintUntolerated,
						Message:     "test eviction",
						Producer:    workv1alpha2.EvictionProducerTaintManager,
					},
				}
				return spec
			}(),
			want: true,
		},
		{
			name: "GracefulEvictionTasks field changed - empty to non-empty",
			oldBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.GracefulEvictionTasks = []workv1alpha2.GracefulEvictionTask{}
				return spec
			}(),
			newBindingSpec: baseSpec,
			want:           true,
		},
		{
			name:           "RequiredBy field changed - different namespace",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.RequiredBy = []workv1alpha2.BindingSnapshot{
					{
						Namespace: "other-ns", // test-ns -> other-ns
						Name:      "test-binding",
						Clusters: []workv1alpha2.TargetCluster{
							{Name: "cluster4", Replicas: 1},
						},
					},
				}
				return spec
			}(),
			want: true,
		},
		{
			name:           "RequiredBy field changed - different name",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.RequiredBy = []workv1alpha2.BindingSnapshot{
					{
						Namespace: "test-ns",
						Name:      "other-binding", // test-binding -> other-binding
						Clusters: []workv1alpha2.TargetCluster{
							{Name: "cluster4", Replicas: 1},
						},
					},
				}
				return spec
			}(),
			want: true,
		},
		{
			name:           "RequiredBy field changed - different clusters",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.RequiredBy = []workv1alpha2.BindingSnapshot{
					{
						Namespace: "test-ns",
						Name:      "test-binding",
						Clusters: []workv1alpha2.TargetCluster{
							{Name: "cluster5", Replicas: 2}, // cluster4 -> cluster5, 1 -> 2
						},
					},
				}
				return spec
			}(),
			want: true,
		},
		{
			name: "RequiredBy field changed - empty to non-empty",
			oldBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.RequiredBy = []workv1alpha2.BindingSnapshot{}
				return spec
			}(),
			newBindingSpec: baseSpec,
			want:           true,
		},
		{
			name:           "ConflictResolution field changed",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.ConflictResolution = policyv1alpha1.ConflictOverwrite // ConflictAbort -> ConflictOverwrite
				return spec
			}(),
			want: true,
		},
		{
			name: "Suspension field changed - nil to non-nil",
			oldBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.Suspension = nil
				return spec
			}(),
			newBindingSpec: baseSpec,
			want:           true,
		},
		{
			name:           "Suspension field changed - non-nil to nil",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.Suspension = nil
				return spec
			}(),
			want: true,
		},
		{
			name:           "Suspension field changed - Dispatching value",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.Suspension = &workv1alpha2.Suspension{
					Suspension: policyv1alpha1.Suspension{
						Dispatching: ptr.To(false), // true -> false
					},
				}
				return spec
			}(),
			want: true,
		},
		{
			name:           "PreserveResourcesOnDeletion field changed - false to true",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.PreserveResourcesOnDeletion = ptr.To(true) // false -> true
				return spec
			}(),
			want: true,
		},
		{
			name: "PreserveResourcesOnDeletion field changed - nil to non-nil",
			oldBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.PreserveResourcesOnDeletion = nil
				return spec
			}(),
			newBindingSpec: baseSpec,
			want:           true,
		},
		{
			name:           "PreserveResourcesOnDeletion field changed - non-nil to nil",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.PreserveResourcesOnDeletion = nil
				return spec
			}(),
			want: true,
		},
		{
			name:           "irrelevant field changed - RescheduleTriggeredAt (should not trigger)",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				newTime := metav1.NewTime(baseTime.Add(1 * time.Hour))
				spec.RescheduleTriggeredAt = &newTime
				return spec
			}(),
			want: false, // RescheduleTriggeredAt is not a field that careBindingSpecChanged cares about
		},
		{
			name:           "irrelevant field changed - Replicas (should not trigger)",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.Replicas = 10 // Added Replicas field
				return spec
			}(),
			want: false, // Replicas is not a field that careBindingSpecChanged cares about
		},
		{
			name:           "multiple relevant fields changed",
			oldBindingSpec: baseSpec,
			newBindingSpec: func() workv1alpha2.ResourceBindingSpec {
				spec := baseSpec
				spec.Resource.Name = "other-deployment"
				spec.ConflictResolution = policyv1alpha1.ConflictOverwrite
				spec.PreserveResourcesOnDeletion = ptr.To(true)
				return spec
			}(),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := careBindingSpecChanged(tt.oldBindingSpec, tt.newBindingSpec); got != tt.want {
				t.Errorf("careBindingSpecChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}
