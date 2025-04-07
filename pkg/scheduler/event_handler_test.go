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

package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestResourceBindingEventFilter(t *testing.T) {
	testCases := []struct {
		name           string
		schedulerName  string
		obj            interface{}
		expectedResult bool
	}{
		{
			name:           "ResourceBinding: Matching scheduler name, no labels",
			schedulerName:  "test-scheduler",
			obj:            createResourceBinding("test-rb", "test-scheduler", nil, nil),
			expectedResult: false,
		},
		{
			name:           "ResourceBinding: Non-matching scheduler name",
			schedulerName:  "test-scheduler",
			obj:            createResourceBinding("test-rb", "other-scheduler", nil, nil),
			expectedResult: false,
		},
		{
			name:          "ResourceBinding: Matching scheduler name, with PropagationPolicyPermanentIDLabel",
			schedulerName: "test-scheduler",
			obj: createResourceBinding("test-rb", "test-scheduler", map[string]string{
				policyv1alpha1.PropagationPolicyPermanentIDLabel: "test-id",
			}, nil),
			expectedResult: true,
		},
		{
			name:          "ResourceBinding: Matching scheduler name, with ClusterPropagationPolicyPermanentIDLabel",
			schedulerName: "test-scheduler",
			obj: createResourceBinding("test-rb", "test-scheduler", map[string]string{
				policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "test-id",
			}, nil),
			expectedResult: true,
		},
		{
			name:          "ResourceBinding: Matching scheduler name, with BindingManagedByLabel",
			schedulerName: "test-scheduler",
			obj: createResourceBinding("test-rb", "test-scheduler", map[string]string{
				workv1alpha2.BindingManagedByLabel: "test-manager",
			}, nil),
			expectedResult: true,
		},
		{
			name:          "ResourceBinding: Matching scheduler name, with empty PropagationPolicyPermanentIDLabel",
			schedulerName: "test-scheduler",
			obj: createResourceBinding("test-rb", "test-scheduler", map[string]string{
				policyv1alpha1.PropagationPolicyPermanentIDLabel: "",
			}, nil),
			expectedResult: false,
		},
		{
			name:           "ClusterResourceBinding: Matching scheduler name, no labels",
			schedulerName:  "test-scheduler",
			obj:            createClusterResourceBinding("test-crb", "test-scheduler", nil, nil),
			expectedResult: false,
		},
		{
			name:           "ClusterResourceBinding: Non-matching scheduler name",
			schedulerName:  "test-scheduler",
			obj:            createClusterResourceBinding("test-crb", "other-scheduler", nil, nil),
			expectedResult: false,
		},
		{
			name:          "ClusterResourceBinding: Matching scheduler name, with ClusterPropagationPolicyPermanentIDLabel",
			schedulerName: "test-scheduler",
			obj: createClusterResourceBinding("test-crb", "test-scheduler", map[string]string{
				policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "test-id",
			}, nil),
			expectedResult: true,
		},
		{
			name:           "Nil object",
			schedulerName:  "test-scheduler",
			obj:            nil,
			expectedResult: false,
		},
		{
			name:           "Invalid object type",
			schedulerName:  "test-scheduler",
			obj:            "not-a-valid-object",
			expectedResult: false,
		},
		{
			name:          "ResourceBinding suspended",
			schedulerName: "test-scheduler",
			obj: createResourceBinding("test-rb", "test-scheduler", map[string]string{
				workv1alpha2.BindingManagedByLabel: "test-manager",
			}, &workv1alpha2.Suspension{Scheduling: ptr.To(true)}),
			expectedResult: false,
		},
		{
			name:          "ClusterResourceBinding suspended",
			schedulerName: "test-scheduler",
			obj: createClusterResourceBinding("test-crb", "test-scheduler", map[string]string{
				policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "test-id",
			}, &workv1alpha2.Suspension{Scheduling: ptr.To(true)}),
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Scheduler{
				schedulerName: tc.schedulerName,
			}
			result := s.resourceBindingEventFilter(tc.obj)
			assert.Equal(t, tc.expectedResult, result, "Test case: %s", tc.name)
		})
	}
}

func TestAddCluster(t *testing.T) {
	tests := []struct {
		name                     string
		enableSchedulerEstimator bool
		obj                      interface{}
		expectedAdded            bool
		expectedClusterName      string
	}{
		{
			name:                     "valid cluster object with estimator enabled",
			enableSchedulerEstimator: true,
			obj:                      createCluster("test-cluster", 0, nil),
			expectedAdded:            true,
			expectedClusterName:      "test-cluster",
		},
		{
			name:                     "valid cluster object with estimator disabled",
			enableSchedulerEstimator: false,
			obj:                      createCluster("test-cluster-2", 0, nil),
			expectedAdded:            false,
			expectedClusterName:      "",
		},
		{
			name:                     "invalid object type",
			enableSchedulerEstimator: true,
			obj:                      &corev1.Pod{},
			expectedAdded:            false,
			expectedClusterName:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorker := &mockAsyncWorker{}
			s := &Scheduler{
				enableSchedulerEstimator: tt.enableSchedulerEstimator,
				schedulerEstimatorWorker: mockWorker,
			}

			s.addCluster(tt.obj)

			if tt.expectedAdded {
				assert.Equal(t, 1, mockWorker.addCount, "Worker Add should have been called once")
				assert.Equal(t, tt.expectedClusterName, mockWorker.lastAdded, "Incorrect cluster name added")
			} else {
				assert.Equal(t, 0, mockWorker.addCount, "Worker Add should not have been called")
				assert.Nil(t, mockWorker.lastAdded, "No cluster name should have been added")
			}

			assert.Equal(t, 0, mockWorker.enqueueCount, "Worker Enqueue should not have been called")
			assert.Nil(t, mockWorker.lastEnqueued, "No item should have been enqueued")
		})
	}
}

func TestUpdateCluster(t *testing.T) {
	tests := []struct {
		name                     string
		enableSchedulerEstimator bool
		oldObj                   interface{}
		newObj                   interface{}
		expectedEstimatorAdded   bool
		expectedReconcileAdded   int
	}{
		{
			name:                     "valid cluster update with generation change",
			enableSchedulerEstimator: true,
			oldObj:                   createCluster("test-cluster", 1, nil),
			newObj:                   createCluster("test-cluster", 2, nil),
			expectedEstimatorAdded:   true,
			expectedReconcileAdded:   2,
		},
		{
			name:                     "valid cluster update with label change",
			enableSchedulerEstimator: true,
			oldObj:                   createCluster("test-cluster", 0, map[string]string{"old": "label"}),
			newObj:                   createCluster("test-cluster", 0, map[string]string{"new": "label"}),
			expectedEstimatorAdded:   true,
			expectedReconcileAdded:   2,
		},
		{
			name:                     "valid cluster update without changes",
			enableSchedulerEstimator: true,
			oldObj:                   createCluster("test-cluster", 0, nil),
			newObj:                   createCluster("test-cluster", 0, nil),
			expectedEstimatorAdded:   true,
			expectedReconcileAdded:   0,
		},
		{
			name:                     "invalid old object type",
			enableSchedulerEstimator: true,
			oldObj:                   &corev1.Pod{},
			newObj:                   createCluster("test-cluster", 0, nil),
			expectedEstimatorAdded:   false,
			expectedReconcileAdded:   0,
		},
		{
			name:                     "invalid new object type",
			enableSchedulerEstimator: true,
			oldObj:                   createCluster("test-cluster", 0, nil),
			newObj:                   &corev1.Pod{},
			expectedEstimatorAdded:   false,
			expectedReconcileAdded:   0,
		},
		{
			name:                     "both objects invalid",
			enableSchedulerEstimator: true,
			oldObj:                   &corev1.Pod{},
			newObj:                   &corev1.Pod{},
			expectedEstimatorAdded:   false,
			expectedReconcileAdded:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimatorWorker := &mockAsyncWorker{}
			reconcileWorker := &mockAsyncWorker{}
			s := &Scheduler{
				enableSchedulerEstimator: tt.enableSchedulerEstimator,
				schedulerEstimatorWorker: estimatorWorker,
				clusterReconcileWorker:   reconcileWorker,
			}

			s.updateCluster(tt.oldObj, tt.newObj)

			// Check schedulerEstimatorWorker
			if tt.expectedEstimatorAdded {
				assert.Equal(t, 1, estimatorWorker.addCount, "Estimator worker Add should have been called once")
				if cluster, ok := tt.newObj.(*clusterv1alpha1.Cluster); ok {
					assert.Equal(t, cluster.Name, estimatorWorker.lastAdded, "Incorrect cluster name added to estimator worker")
				} else {
					t.Errorf("Expected newObj to be a Cluster, but it wasn't")
				}
			} else {
				assert.Equal(t, 0, estimatorWorker.addCount, "Estimator worker Add should not have been called")
				assert.Nil(t, estimatorWorker.lastAdded, "No cluster should have been added to estimator worker")
			}

			// Check clusterReconcileWorker
			assert.Equal(t, tt.expectedReconcileAdded, reconcileWorker.addCount, "Reconcile worker Add called unexpected number of times")

			if tt.expectedReconcileAdded > 0 {
				lastAdded, ok := reconcileWorker.lastAdded.(*clusterv1alpha1.Cluster)
				assert.True(t, ok, "Last added item is not a Cluster object")
				if ok {
					newCluster, newOk := tt.newObj.(*clusterv1alpha1.Cluster)
					assert.True(t, newOk, "newObj is not a Cluster object")
					if newOk {
						assert.Equal(t, newCluster.Name, lastAdded.Name, "Incorrect cluster added to reconcile worker")
					}
				}
			} else {
				assert.Nil(t, reconcileWorker.lastAdded, "No cluster should have been added to reconcile worker")
			}
		})
	}
}

func TestDeleteCluster(t *testing.T) {
	tests := []struct {
		name                     string
		enableSchedulerEstimator bool
		obj                      interface{}
		expectedAdded            bool
		expectedClusterName      string
	}{
		{
			name:                     "valid cluster object with estimator enabled",
			enableSchedulerEstimator: true,
			obj:                      createCluster("test-cluster", 0, nil),
			expectedAdded:            true,
			expectedClusterName:      "test-cluster",
		},
		{
			name:                     "valid cluster object with estimator disabled",
			enableSchedulerEstimator: false,
			obj:                      createCluster("test-cluster", 0, nil),
			expectedAdded:            false,
			expectedClusterName:      "",
		},
		{
			name:                     "deleted final state unknown with valid cluster",
			enableSchedulerEstimator: true,
			obj: cache.DeletedFinalStateUnknown{
				Key: "test-cluster",
				Obj: createCluster("test-cluster", 0, nil),
			},
			expectedAdded:       true,
			expectedClusterName: "test-cluster",
		},
		{
			name:                     "deleted final state unknown with invalid object",
			enableSchedulerEstimator: true,
			obj: cache.DeletedFinalStateUnknown{
				Key: "test-pod",
				Obj: &corev1.Pod{},
			},
			expectedAdded:       false,
			expectedClusterName: "",
		},
		{
			name:                     "invalid object type",
			enableSchedulerEstimator: true,
			obj:                      &corev1.Pod{},
			expectedAdded:            false,
			expectedClusterName:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker := &mockAsyncWorker{}
			s := &Scheduler{
				enableSchedulerEstimator: tt.enableSchedulerEstimator,
				schedulerEstimatorWorker: worker,
			}

			s.deleteCluster(tt.obj)

			if tt.expectedAdded {
				assert.Equal(t, 1, worker.addCount, "Worker Add should have been called once")
				assert.Equal(t, tt.expectedClusterName, worker.lastAdded, "Incorrect cluster name added to worker")
			} else {
				assert.Equal(t, 0, worker.addCount, "Worker Add should not have been called")
				assert.Nil(t, worker.lastAdded, "No cluster name should have been added")
			}
		})
	}
}

func TestSchedulerNameFilter(t *testing.T) {
	tests := []struct {
		name                     string
		schedulerNameFromOptions string
		schedulerName            string
		expected                 bool
	}{
		{
			name:                     "matching scheduler names",
			schedulerNameFromOptions: "test-scheduler",
			schedulerName:            "test-scheduler",
			expected:                 true,
		},
		{
			name:                     "non-matching scheduler names",
			schedulerNameFromOptions: "test-scheduler",
			schedulerName:            "other-scheduler",
			expected:                 false,
		},
		{
			name:                     "empty scheduler name defaults to DefaultScheduler",
			schedulerNameFromOptions: DefaultScheduler,
			schedulerName:            "",
			expected:                 true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := schedulerNameFilter(tt.schedulerNameFromOptions, tt.schedulerName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper functions

func createCluster(name string, generation int64, labels map[string]string) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Generation: generation,
			Labels:     labels,
		},
	}
}

func createResourceBinding(name, schedulerName string, labels map[string]string, suspension *workv1alpha2.Suspension) *workv1alpha2.ResourceBinding {
	return &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			SchedulerName: schedulerName,
			Suspension:    suspension,
		},
	}
}

func createClusterResourceBinding(name, schedulerName string, labels map[string]string, suspension *workv1alpha2.Suspension) *workv1alpha2.ClusterResourceBinding {
	return &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			SchedulerName: schedulerName,
			Suspension:    suspension,
		},
	}
}

// Mock Implementations

// mockAsyncWorker is a mock implementation of util.AsyncWorker
type mockAsyncWorker struct {
	addCount     int
	enqueueCount int
	lastAdded    interface{}
	lastEnqueued interface{}
}

func (m *mockAsyncWorker) Add(item interface{}) {
	m.addCount++
	m.lastAdded = item
}

func (m *mockAsyncWorker) Enqueue(item interface{}) {
	m.enqueueCount++
	m.lastEnqueued = item
}

func (m *mockAsyncWorker) AddAfter(_ interface{}, _ time.Duration) {}

func (m *mockAsyncWorker) Run(_ context.Context, _ int) {}
