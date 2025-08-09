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

package gracefuleviction

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	plugins "github.com/karmada-io/karmada/pkg/controllers/gracefuleviction/evictplugins"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
)

func TestCRBGracefulEvictionController_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	err := workv1alpha2.Install(scheme)
	assert.NoError(t, err, "Failed to add workv1alpha2 to scheme")
	now := metav1.Now()
	testCases := []struct {
		name            string
		binding         *workv1alpha2.ClusterResourceBinding
		expectedResult  controllerruntime.Result
		expectedError   bool
		expectedRequeue bool
		notFound        bool
	}{
		{
			name: "binding with no graceful eviction tasks",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{},
				},
			},
			expectedResult:  controllerruntime.Result{},
			expectedError:   false,
			expectedRequeue: false,
		},
		{
			name: "binding with active graceful eviction tasks",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-binding",
					Generation: 1,
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster:       "cluster1",
							CreationTimestamp: &now,
						},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedGeneration: 1,
				},
			},
			expectedResult:  controllerruntime.Result{},
			expectedError:   false,
			expectedRequeue: false,
		},
		{
			name: "binding marked for deletion",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-binding",
					DeletionTimestamp: &now,
					Finalizers:        []string{"test-finalizer"},
				},
			},
			expectedResult:  controllerruntime.Result{},
			expectedError:   false,
			expectedRequeue: false,
		},
		{
			name: "binding not found",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "non-existent-binding",
				},
			},
			expectedResult:  controllerruntime.Result{},
			expectedError:   false,
			expectedRequeue: false,
			notFound:        true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fake client with or without the binding object
			var client client.Client
			if tc.notFound {
				client = fake.NewClientBuilder().WithScheme(scheme).Build()
			} else {
				client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.binding).Build()
			}

			pluginDenyMgr, _ := plugins.NewManager([]string{"mock-deny"})

			c := &CRBGracefulEvictionController{
				Client:                  client,
				EventRecorder:           record.NewFakeRecorder(10),
				RateLimiterOptions:      ratelimiterflag.Options{},
				GracefulEvictionTimeout: 5 * time.Minute,
				PluginManager:           pluginDenyMgr,
			}
			result, err := c.Reconcile(context.TODO(), controllerruntime.Request{
				NamespacedName: types.NamespacedName{
					Name: tc.binding.Name,
				},
			})
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Assert on the requeue behavior
			if tc.expectedRequeue {
				assert.True(t, result.RequeueAfter > 0, "Expected requeue, but got no requeue for test: %s", tc.name)
			} else {
				assert.Zero(t, result.RequeueAfter, "Expected no requeue, but got requeue for test: %s", tc.name)
			}
			// Verify the binding was updated, unless it's the "not found" case
			if !tc.notFound {
				updatedBinding := &workv1alpha2.ClusterResourceBinding{}
				err = client.Get(context.TODO(), types.NamespacedName{Name: tc.binding.Name}, updatedBinding)
				assert.NoError(t, err)
			}
		})
	}
}

func TestCRBGracefulEvictionController_syncBinding(t *testing.T) {
	scheme := runtime.NewScheme()
	err := workv1alpha2.Install(scheme)
	require.NoError(t, err, "Failed to add workv1alpha2 to scheme")

	now := metav1.Now()
	timeout := 5 * time.Minute

	pluginAllowMgr, _ := plugins.NewManager([]string{"mock-allow"})
	pluginDenyMgr, _ := plugins.NewManager([]string{"mock-deny"})

	testCases := []struct {
		name                string
		binding             *workv1alpha2.ClusterResourceBinding
		pluginMgr           *plugins.Manager
		expectedKeptCluster []string
		expectRequeue       bool
		expectedError       bool
	}{
		{
			name: "no eviction tasks",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "test-binding"},
				Spec:       workv1alpha2.ResourceBindingSpec{GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{}},
			},
			pluginMgr:           pluginDenyMgr,
			expectedKeptCluster: []string{},
			expectRequeue:       false,
			expectedError:       false,
		},
		{
			name: "task not timed out and plugin denies, should keep task and requeue",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "test-binding", Generation: 1},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "member2"},
					},
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{{FromCluster: "member1", CreationTimestamp: &now}},
				},
				Status: workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 1},
			},
			pluginMgr:           pluginDenyMgr,
			expectedKeptCluster: []string{"member1"},
			expectRequeue:       true,
			expectedError:       false,
		},
		{
			name: "task timed out, should be removed and not requeue",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "test-binding", Generation: 1},
				Spec: workv1alpha2.ResourceBindingSpec{
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{{FromCluster: "member1", CreationTimestamp: &metav1.Time{Time: now.Add(-10 * time.Minute)}}},
				},
				Status: workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 1},
			},
			pluginMgr:           pluginDenyMgr,
			expectedKeptCluster: []string{},
			expectRequeue:       false,
			expectedError:       false,
		},
		{
			name: "task not timed out, but plugin allows, should be removed and not requeue",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "test-binding", Generation: 1},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "member2"},
					},
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{{FromCluster: "member1", CreationTimestamp: &now}},
				},
				Status: workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 1},
			},
			pluginMgr:           pluginAllowMgr,
			expectedKeptCluster: []string{},
			expectRequeue:       false,
			expectedError:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.binding.DeepCopy()).Build()

			c := &CRBGracefulEvictionController{
				Client:                  fakeClient,
				EventRecorder:           record.NewFakeRecorder(10),
				GracefulEvictionTimeout: timeout,
				PluginManager:           tc.pluginMgr,
			}

			retryAfter, err := c.syncBinding(context.TODO(), tc.binding.DeepCopy())

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tc.expectRequeue {
				assert.True(t, retryAfter > 0, "Expected requeue but got none")
			} else {
				assert.True(t, retryAfter == 0, "Expected no requeue but got one")
			}

			updatedBinding := &workv1alpha2.ClusterResourceBinding{}
			err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: tc.binding.Name}, updatedBinding)
			assert.NoError(t, err)

			keptClusterNames := getClusterNamesFromTasks(updatedBinding.Spec.GracefulEvictionTasks)
			assert.Equal(t, tc.expectedKeptCluster, keptClusterNames)
		})
	}
}

// Helper function to create a RawExtension from a status string
func createRawExtension(status string) *runtime.RawExtension {
	raw, _ := json.Marshal(status)
	return &runtime.RawExtension{Raw: raw}
}

// Helper function to compare two time.Duration values with a tolerance
func almostEqual(a, b time.Duration, tolerance time.Duration) bool {
	diff := a - b
	return math.Abs(float64(diff)) < float64(tolerance)
}

func getClusterNamesFromTasks(tasks []workv1alpha2.GracefulEvictionTask) []string {
	names := make([]string, len(tasks))
	for i, task := range tasks {
		names[i] = task.FromCluster
	}
	sort.Strings(names)
	return names
}
