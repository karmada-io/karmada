/*
Copyright 2024 The Karmada Authors.

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

package core

import (
	"context"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/estimator/client"
)

func TestNewSchedulingResultHelper(t *testing.T) {
	binding := &workv1alpha2.ResourceBinding{
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 3},
				{Name: "cluster2", Replicas: 2},
			},
		},
		Status: workv1alpha2.ResourceBindingStatus{
			AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "cluster1",
					Status:      &runtime.RawExtension{Raw: []byte(`{"readyReplicas": 2}`)},
				},
				{
					ClusterName: "cluster2",
					Status:      &runtime.RawExtension{Raw: []byte(`{"readyReplicas": 1}`)},
				},
			},
		},
	}

	helper := NewSchedulingResultHelper(binding)

	expected := &SchedulingResultHelper{
		ResourceBinding: binding,
		TargetClusters: []*TargetClusterWrapper{
			{ClusterName: "cluster1", Spec: 3, Ready: 2},
			{ClusterName: "cluster2", Spec: 2, Ready: 1},
		},
	}

	if !reflect.DeepEqual(helper, expected) {
		t.Errorf("NewSchedulingResultHelper() = %v, want %v", helper, expected)
	}
}

func TestSchedulingResultHelper_GetUndesiredClusters(t *testing.T) {
	helper := &SchedulingResultHelper{
		TargetClusters: []*TargetClusterWrapper{
			{ClusterName: "cluster1", Spec: 3, Ready: 2},
			{ClusterName: "cluster2", Spec: 2, Ready: 2},
			{ClusterName: "cluster3", Spec: 4, Ready: 1},
		},
	}

	clusters, names := helper.GetUndesiredClusters()

	expectedClusters := []*TargetClusterWrapper{
		{ClusterName: "cluster1", Spec: 3, Ready: 2},
		{ClusterName: "cluster3", Spec: 4, Ready: 1},
	}
	expectedNames := []string{"cluster1", "cluster3"}

	if !reflect.DeepEqual(clusters, expectedClusters) {
		t.Errorf("GetUndesiredClusters() clusters = %v, want %v", clusters, expectedClusters)
	}
	if !reflect.DeepEqual(names, expectedNames) {
		t.Errorf("GetUndesiredClusters() names = %v, want %v", names, expectedNames)
	}
}

func TestSchedulingResultHelper_FillUnschedulableReplicas(t *testing.T) {
	mockEstimator := &mockUnschedulableReplicaEstimator{}
	client.GetUnschedulableReplicaEstimators()["mock"] = mockEstimator
	defer delete(client.GetUnschedulableReplicaEstimators(), "mock")

	helper := &SchedulingResultHelper{
		ResourceBinding: &workv1alpha2.ResourceBinding{
			Spec: workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
					Namespace:  "default",
				},
			},
		},
		TargetClusters: []*TargetClusterWrapper{
			{ClusterName: "cluster1", Spec: 3, Ready: 2},
			{ClusterName: "cluster2", Spec: 2, Ready: 1},
		},
	}

	helper.FillUnschedulableReplicas(time.Minute)

	expected := []*TargetClusterWrapper{
		{ClusterName: "cluster1", Spec: 3, Ready: 2, Unschedulable: 1},
		{ClusterName: "cluster2", Spec: 2, Ready: 1, Unschedulable: 1},
	}

	if !reflect.DeepEqual(helper.TargetClusters, expected) {
		t.Errorf("FillUnschedulableReplicas() = %v, want %v", helper.TargetClusters, expected)
	}
}

func TestGetReadyReplicas(t *testing.T) {
	binding := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-binding",
			Namespace: "default",
		},
		Status: workv1alpha2.ResourceBindingStatus{
			AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "cluster1",
					Status:      &runtime.RawExtension{Raw: []byte(`{"readyReplicas": 2}`)},
				},
				{
					ClusterName: "cluster2",
					Status:      &runtime.RawExtension{Raw: []byte(`{"readyReplicas": 3}`)},
				},
				{
					ClusterName: "cluster3",
					Status:      &runtime.RawExtension{Raw: []byte(`{"someOtherField": 1}`)},
				},
			},
		},
	}

	result := getReadyReplicas(binding)

	expected := map[string]int32{
		"cluster1": 2,
		"cluster2": 3,
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("getReadyReplicas() = %v, want %v", result, expected)
	}
}

// mockUnschedulableReplicaEstimator is a mock implementation of the UnschedulableReplicaEstimator interface
type mockUnschedulableReplicaEstimator struct{}

func (m *mockUnschedulableReplicaEstimator) GetUnschedulableReplicas(_ context.Context, _ []string, _ *workv1alpha2.ObjectReference, _ time.Duration) ([]workv1alpha2.TargetCluster, error) {
	return []workv1alpha2.TargetCluster{
		{Name: "cluster1", Replicas: 1},
		{Name: "cluster2", Replicas: 1},
	}, nil
}
