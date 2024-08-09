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

package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestGenerateFullyAppliedCondition(t *testing.T) {
	spec := workv1alpha2.ResourceBindingSpec{
		Clusters: []workv1alpha2.TargetCluster{
			{Name: "cluster1"},
			{Name: "cluster2"},
		},
	}
	statuses := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "cluster1", Applied: true, Health: workv1alpha2.ResourceHealthy},
		{ClusterName: "cluster2", Applied: true, Health: workv1alpha2.ResourceUnknown},
	}

	expectedTrue := metav1.ConditionTrue
	expectedFalse := metav1.ConditionFalse

	resultTrue := generateFullyAppliedCondition(spec, statuses)
	if resultTrue.Status != expectedTrue {
		t.Errorf("generateFullyAppliedCondition with fully applied statuses returned %v, expected %v", resultTrue, expectedTrue)
	}

	resultFalse := generateFullyAppliedCondition(spec, statuses[:1])
	if resultFalse.Status != expectedFalse {
		t.Errorf("generateFullyAppliedCondition with partially applied statuses returned %v, expected %v", resultFalse, expectedFalse)
	}
}

func TestWorksFullyApplied(t *testing.T) {
	type args struct {
		aggregatedStatuses []workv1alpha2.AggregatedStatusItem
		targetClusters     sets.Set[string]
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "no cluster",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
				},
				targetClusters: nil,
			},
			want: false,
		},
		{
			name: "no aggregatedStatuses",
			args: args{
				aggregatedStatuses: nil,
				targetClusters:     sets.New("member1"),
			},
			want: false,
		},
		{
			name: "cluster size is not equal to aggregatedStatuses",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
				},
				targetClusters: sets.New("member1", "member2"),
			},
			want: false,
		},
		{
			name: "aggregatedStatuses is equal to clusterNames and all applied",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
					{
						ClusterName: "member2",
						Applied:     true,
					},
				},
				targetClusters: sets.New("member1", "member2"),
			},
			want: true,
		},
		{
			name: "aggregatedStatuses is equal to clusterNames but not all applied",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
					{
						ClusterName: "member2",
						Applied:     false,
					},
				},
				targetClusters: sets.New("member1", "member2"),
			},
			want: false,
		},
		{
			name: "target clusters not match expected status",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
				},
				targetClusters: sets.New("member2"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := worksFullyApplied(tt.args.aggregatedStatuses, tt.args.targetClusters); got != tt.want {
				t.Errorf("worksFullyApplied() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetManifestIndex(t *testing.T) {
	manifest1 := workv1alpha1.Manifest{
		RawExtension: runtime.RawExtension{
			Raw: []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"test-service","namespace":"test-namespace"}}`),
		},
	}
	manifest2 := workv1alpha1.Manifest{
		RawExtension: runtime.RawExtension{
			Raw: []byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test-deployment","namespace":"test-namespace"}}`),
		},
	}
	manifests := []workv1alpha1.Manifest{manifest1, manifest2}

	service := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      "test-service",
				"namespace": "test-namespace",
			},
		},
	}

	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-namespace",
			},
		},
	}

	t.Run("Service", func(t *testing.T) {
		index, err := GetManifestIndex(manifests, service)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if index != 0 {
			t.Errorf("expected index 0, got %d", index)
		}
	})

	t.Run("Deployment", func(t *testing.T) {
		index, err := GetManifestIndex(manifests, deployment)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if index != 1 {
			t.Errorf("expected index 1, got %d", index)
		}
	})

	t.Run("No match", func(t *testing.T) {
		_, err := GetManifestIndex(manifests, &unstructured.Unstructured{})
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})
}

func TestEqualIdentifier(t *testing.T) {
	testCases := []struct {
		name           string
		target         *workv1alpha1.ResourceIdentifier
		ordinal        int
		workload       workv1alpha2.ObjectReference
		expectedOutput bool
	}{
		{
			name: "identifiers are equal",
			target: &workv1alpha1.ResourceIdentifier{
				Ordinal:   0,
				Group:     "apps",
				Version:   "v1",
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "test-deployment",
			},
			ordinal: 0,
			workload: workv1alpha2.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "default",
				Name:       "test-deployment",
			},
			expectedOutput: true,
		},
		{
			name: "identifiers are not equal",
			target: &workv1alpha1.ResourceIdentifier{
				Ordinal:   1,
				Group:     "apps",
				Version:   "v1",
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "test-deployment",
			},
			ordinal: 0,
			workload: workv1alpha2.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "default",
				Name:       "test-deployment",
			},
			expectedOutput: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			equal, err := equalIdentifier(tc.target, tc.ordinal, tc.workload)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if equal != tc.expectedOutput {
				t.Errorf("expected %v, got %v", tc.expectedOutput, equal)
			}
		})
	}
}

func TestIsResourceApplied(t *testing.T) {
	// Create a WorkStatus struct with a WorkApplied condition set to True
	workStatus := &workv1alpha1.WorkStatus{
		Conditions: []metav1.Condition{
			{
				Type:   workv1alpha1.WorkApplied,
				Status: metav1.ConditionTrue,
			},
		},
	}

	// Call IsResourceApplied and assert that it returns true
	assert.True(t, IsResourceApplied(workStatus))
}
