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

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	schedulercache "github.com/karmada-io/karmada/pkg/scheduler/cache"
	internalqueue "github.com/karmada-io/karmada/pkg/scheduler/internal/queue"
)

func TestResourceBindingEventFilter(t *testing.T) {
	testCases := []struct {
		name           string
		schedulerName  string
		obj            any
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
			}, &workv1alpha2.Suspension{Scheduling: new(true)}),
			expectedResult: false,
		},
		{
			name:          "ClusterResourceBinding suspended",
			schedulerName: "test-scheduler",
			obj: createClusterResourceBinding("test-crb", "test-scheduler", map[string]string{
				policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "test-id",
			}, &workv1alpha2.Suspension{Scheduling: new(true)}),
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
		name                         string
		enableSchedulerEstimator     bool
		obj                          any
		priorityQueue                internalqueue.SchedulingQueue
		expectedEstimatorAdded       bool
		expectedEstimatorClusterName string
		expectedMoveAllToActive      bool
	}{
		{
			name:                         "valid cluster, estimator enabled, no priority queue",
			enableSchedulerEstimator:     true,
			obj:                          createCluster("test-cluster", 0, nil),
			priorityQueue:                nil,
			expectedEstimatorAdded:       true,
			expectedEstimatorClusterName: "test-cluster",
			expectedMoveAllToActive:      false,
		},
		{
			name:                         "valid cluster, estimator enabled, with priority queue",
			enableSchedulerEstimator:     true,
			obj:                          createCluster("test-cluster", 0, nil),
			priorityQueue:                &mockSchedulingQueue{},
			expectedEstimatorAdded:       true,
			expectedEstimatorClusterName: "test-cluster",
			expectedMoveAllToActive:      true,
		},
		{
			name:                     "valid cluster, estimator disabled, with priority queue",
			enableSchedulerEstimator: false,
			obj:                      createCluster("test-cluster-2", 0, nil),
			priorityQueue:            &mockSchedulingQueue{},
			expectedEstimatorAdded:   false,
			expectedMoveAllToActive:  true,
		},
		{
			name:                     "invalid object type",
			enableSchedulerEstimator: true,
			obj:                      &corev1.Pod{},
			expectedEstimatorAdded:   false,
			expectedMoveAllToActive:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimatorWorker := &mockAsyncWorker{}
			s := &Scheduler{
				enableSchedulerEstimator: tt.enableSchedulerEstimator,
				schedulerEstimatorWorker: estimatorWorker,
				priorityQueue:            tt.priorityQueue,
			}

			s.addCluster(tt.obj)

			if tt.expectedEstimatorAdded {
				assert.Equal(t, 1, estimatorWorker.addCount, "Estimator worker Add should have been called once")
				assert.Equal(t, tt.expectedEstimatorClusterName, estimatorWorker.lastAdded, "Incorrect cluster name added to estimator worker")
			} else {
				assert.Equal(t, 0, estimatorWorker.addCount, "Estimator worker Add should not have been called")
			}

			if tt.priorityQueue != nil {
				mock := tt.priorityQueue.(*mockSchedulingQueue)
				assert.Equal(t, tt.expectedMoveAllToActive, mock.moveAllToActiveCalled, "MoveAllToActive called state mismatch")
			}
		})
	}
}

func TestUpdateCluster(t *testing.T) {
	tests := []struct {
		name                     string
		enableSchedulerEstimator bool
		oldObj                   any
		newObj                   any
		priorityQueue            internalqueue.SchedulingQueue
		expectedEstimatorAdded   bool
		expectedReconcileAdded   int
		expectedMoveAllToActive  bool
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
		{
			name:                     "cluster update with ResourceSummary change, priorityQueue nil",
			enableSchedulerEstimator: true,
			oldObj:                   createCluster("test-cluster", 0, nil),
			newObj: createClusterWithStatus("test-cluster", 0, nil, clusterv1alpha1.ClusterStatus{
				ResourceSummary: &clusterv1alpha1.ResourceSummary{},
			}),
			expectedEstimatorAdded: true,
			expectedReconcileAdded: 0,
		},
		{
			name:                     "cluster update with ResourceSummary change, with priority queue",
			enableSchedulerEstimator: true,
			oldObj:                   createCluster("test-cluster", 0, nil),
			newObj: createClusterWithStatus("test-cluster", 0, nil, clusterv1alpha1.ClusterStatus{
				ResourceSummary: &clusterv1alpha1.ResourceSummary{},
			}),
			priorityQueue:           &mockSchedulingQueue{},
			expectedEstimatorAdded:  true,
			expectedReconcileAdded:  0,
			expectedMoveAllToActive: true,
		},
		{
			name:                     "ResourceSummary and Ready condition change simultaneously — single merged case fires",
			enableSchedulerEstimator: false,
			oldObj:                   createCluster("test-cluster", 0, nil),
			newObj: createClusterWithStatus("test-cluster", 0, nil, clusterv1alpha1.ClusterStatus{
				ResourceSummary: &clusterv1alpha1.ResourceSummary{},
				Conditions: []metav1.Condition{
					{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
				},
			}),
			priorityQueue:           &mockSchedulingQueue{},
			expectedEstimatorAdded:  false,
			expectedReconcileAdded:  0,
			expectedMoveAllToActive: true,
		},
		{
			name:                     "generation and status change simultaneously — generation case takes precedence",
			enableSchedulerEstimator: false,
			oldObj:                   createCluster("test-cluster", 1, nil),
			newObj: createClusterWithStatus("test-cluster", 2, nil, clusterv1alpha1.ClusterStatus{
				Conditions: []metav1.Condition{
					{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
				},
			}),
			expectedEstimatorAdded: false,
			// Generation case fires and adds both old+new → 2, not 3
			expectedReconcileAdded: 2,
		},
		{
			name:                     "DeletionTimestamp and status change simultaneously — deletion case takes precedence",
			enableSchedulerEstimator: false,
			oldObj:                   createCluster("test-cluster", 0, nil),
			newObj: func() *clusterv1alpha1.Cluster {
				c := createClusterWithStatus("test-cluster", 0, nil, clusterv1alpha1.ClusterStatus{
					Conditions: []metav1.Condition{
						{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
					},
				})
				now := metav1.Now()
				c.DeletionTimestamp = &now
				return c
			}(),
			expectedEstimatorAdded: false,
			// Deletion case fires → only 1 add, status case is skipped
			expectedReconcileAdded: 1,
		},
		{
			name:                     "identical non-nil ResourceSummary — DeepEqual true, no reconcile triggered",
			enableSchedulerEstimator: false,
			oldObj: createClusterWithStatus("test-cluster", 0, nil, clusterv1alpha1.ClusterStatus{
				ResourceSummary: &clusterv1alpha1.ResourceSummary{},
			}),
			newObj: createClusterWithStatus("test-cluster", 0, nil, clusterv1alpha1.ClusterStatus{
				ResourceSummary: &clusterv1alpha1.ResourceSummary{},
			}),
			priorityQueue:          &mockSchedulingQueue{},
			expectedEstimatorAdded: false,
			expectedReconcileAdded: 0,
		},
		{
			name:                     "cluster update with Ready condition transition, with priority queue",
			enableSchedulerEstimator: true,
			oldObj:                   createCluster("test-cluster", 0, nil),
			newObj: createClusterWithStatus("test-cluster", 0, nil, clusterv1alpha1.ClusterStatus{
				Conditions: []metav1.Condition{
					{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
				},
			}),
			priorityQueue:           &mockSchedulingQueue{},
			expectedEstimatorAdded:  true,
			expectedReconcileAdded:  0,
			expectedMoveAllToActive: true,
		},
		{
			name:                     "APIEnablements diff while CompleteAPIEnablements is True triggers requeue",
			enableSchedulerEstimator: true,
			oldObj:                   createCluster("test-cluster", 0, nil),
			newObj: createClusterWithStatus("test-cluster", 0, nil, clusterv1alpha1.ClusterStatus{
				APIEnablements: []clusterv1alpha1.APIEnablement{
					{GroupVersion: "apps/v1"},
				},
				Conditions: []metav1.Condition{
					{Type: clusterv1alpha1.ClusterConditionCompleteAPIEnablements, Status: metav1.ConditionTrue},
				},
			}),
			priorityQueue:           &mockSchedulingQueue{},
			expectedEstimatorAdded:  true,
			expectedReconcileAdded:  0,
			expectedMoveAllToActive: true,
		},
		{
			name:                     "APIEnablements diff while CompleteAPIEnablements is False is ignored",
			enableSchedulerEstimator: true,
			oldObj:                   createCluster("test-cluster", 0, nil),
			newObj: createClusterWithStatus("test-cluster", 0, nil, clusterv1alpha1.ClusterStatus{
				APIEnablements: []clusterv1alpha1.APIEnablement{
					{GroupVersion: "apps/v1"},
				},
				Conditions: []metav1.Condition{
					{Type: clusterv1alpha1.ClusterConditionCompleteAPIEnablements, Status: metav1.ConditionFalse},
				},
			}),
			priorityQueue:           &mockSchedulingQueue{},
			expectedEstimatorAdded:  true,
			expectedReconcileAdded:  0,
			expectedMoveAllToActive: false,
		},
		{
			name:                     "cluster update with non-relevant condition change is ignored",
			enableSchedulerEstimator: true,
			oldObj:                   createCluster("test-cluster", 0, nil),
			newObj: createClusterWithStatus("test-cluster", 0, nil, clusterv1alpha1.ClusterStatus{
				Conditions: []metav1.Condition{
					{Type: "SomeOtherCondition", Status: metav1.ConditionTrue},
				},
			}),
			priorityQueue:           &mockSchedulingQueue{},
			expectedEstimatorAdded:  true,
			expectedReconcileAdded:  0,
			expectedMoveAllToActive: false,
		},
		{
			name:                     "cluster update with raw APIEnablements diff but no CompleteAPIEnablements condition is ignored",
			enableSchedulerEstimator: true,
			oldObj:                   createCluster("test-cluster", 0, nil),
			newObj: createClusterWithStatus("test-cluster", 0, nil, clusterv1alpha1.ClusterStatus{
				APIEnablements: []clusterv1alpha1.APIEnablement{
					{GroupVersion: "apps/v1"},
				},
			}),
			priorityQueue:           &mockSchedulingQueue{},
			expectedEstimatorAdded:  true,
			expectedReconcileAdded:  0,
			expectedMoveAllToActive: false,
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
				priorityQueue:            tt.priorityQueue,
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

			// Check MoveAllToActive
			if tt.priorityQueue != nil {
				mock := tt.priorityQueue.(*mockSchedulingQueue)
				assert.Equal(t, tt.expectedMoveAllToActive, mock.moveAllToActiveCalled, "MoveAllToActive called state mismatch")
			}
		})
	}
}

func TestDeleteCluster(t *testing.T) {
	tests := []struct {
		name                     string
		enableSchedulerEstimator bool
		obj                      any
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

func createClusterWithStatus(name string, generation int64, labels map[string]string, status clusterv1alpha1.ClusterStatus) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Generation: generation,
			Labels:     labels,
		},
		Status: status,
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
	lastAdded    any
	lastEnqueued any
}

func (m *mockAsyncWorker) Add(item any) {
	m.addCount++
	m.lastAdded = item
}

func (m *mockAsyncWorker) Enqueue(item any) {
	m.enqueueCount++
	m.lastEnqueued = item
}

func (m *mockAsyncWorker) AddAfter(_ any, _ time.Duration) {}

func (m *mockAsyncWorker) Run(_ context.Context, _ int) {}

func TestNewlyHealthyClusters(t *testing.T) {
	tests := []struct {
		name      string
		oldStatus []workv1alpha2.AggregatedStatusItem
		newStatus []workv1alpha2.AggregatedStatusItem
		want      []string
	}{
		{
			name:      "no status entries: nothing released",
			oldStatus: nil,
			newStatus: nil,
			want:      nil,
		},
		{
			name: "cluster newly becomes Healthy",
			oldStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster1", Health: workv1alpha2.ResourceUnknown},
			},
			newStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster1", Health: workv1alpha2.ResourceHealthy},
			},
			want: []string{"cluster1"},
		},
		{
			name: "cluster already Healthy: not returned again",
			oldStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster1", Health: workv1alpha2.ResourceHealthy},
			},
			newStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster1", Health: workv1alpha2.ResourceHealthy},
			},
			want: nil,
		},
		{
			name: "cluster transitions from Healthy to Unhealthy: not returned",
			oldStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster1", Health: workv1alpha2.ResourceHealthy},
			},
			newStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster1", Health: workv1alpha2.ResourceUnhealthy},
			},
			want: nil,
		},
		{
			name: "one new Healthy cluster among multiple",
			oldStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster1", Health: workv1alpha2.ResourceHealthy},
				{ClusterName: "cluster2", Health: workv1alpha2.ResourceUnknown},
			},
			newStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster1", Health: workv1alpha2.ResourceHealthy},
				{ClusterName: "cluster2", Health: workv1alpha2.ResourceHealthy},
			},
			want: []string{"cluster2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newlyHealthyClusters(tt.oldStatus, tt.newStatus)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOnResourceBindingUpdate_ReleasesAssumptionOnHealthy(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.SchedulingOvercommitProtection, true)()
	components := []workv1alpha2.Component{
		{Name: "jobmanager", Replicas: 1},
		{Name: "taskmanager", Replicas: 2},
	}
	oldBinding := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-binding", Namespace: "default", Generation: 1,
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Components: components,
			Clusters:   []workv1alpha2.TargetCluster{{Name: "cluster1"}},
		},
		Status: workv1alpha2.ResourceBindingStatus{
			AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster1", Health: workv1alpha2.ResourceUnknown},
			},
		},
	}
	// Only status changed (AggregatedStatus); generation stays the same.
	newBinding := oldBinding.DeepCopy()
	newBinding.Status.AggregatedStatus = []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "cluster1", Health: workv1alpha2.ResourceHealthy},
	}

	sc := schedulercache.NewCache(nil, nil, 0)
	bindingKey := "default/test-binding"
	sc.AssigningResourceBindings().Assume(bindingKey, "cluster1", schedulercache.AssumedWorkload{
		Namespace:  "default",
		Components: components,
	})

	s := &Scheduler{schedulerCache: sc}
	s.onResourceBindingUpdate(oldBinding, newBinding)

	assert.Empty(t, sc.AssigningResourceBindings().GetAssumedWorkloads("cluster1"),
		"assumption should be released when workload becomes Healthy")
}

func TestReleaseHealthyClusterAssumptions_SkipsNonWorkload(t *testing.T) {
	// Non-workload binding: no Replicas, no ReplicaRequirements, no Components.
	oldBinding := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "cm-binding", Namespace: "default", Generation: 1},
		Spec:       workv1alpha2.ResourceBindingSpec{},
		Status: workv1alpha2.ResourceBindingStatus{
			AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster1", Health: workv1alpha2.ResourceUnknown},
			},
		},
	}
	newBinding := oldBinding.DeepCopy()
	newBinding.Status.AggregatedStatus = []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "cluster1", Health: workv1alpha2.ResourceHealthy},
	}

	sc := schedulercache.NewCache(nil, nil, 0)
	// Manually plant an entry to verify it is NOT released.
	sc.AssigningResourceBindings().Assume("default/cm-binding", "cluster1", schedulercache.AssumedWorkload{
		Namespace: "default",
	})

	s := &Scheduler{schedulerCache: sc}
	s.releaseHealthyClusterAssumptions(oldBinding, newBinding)

	assert.Len(t, sc.AssigningResourceBindings().GetAssumedWorkloads("cluster1"), 1,
		"non-workload assumption should NOT be released")
}
