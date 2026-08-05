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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
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
			c := &CRBGracefulEvictionController{
				Client:                  client,
				EventRecorder:           record.NewFakeRecorder(10),
				RateLimiterOptions:      ratelimiterflag.Options{},
				GracefulEvictionTimeout: 5 * time.Minute,
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
			assert.Equal(t, tc.expectedResult, result)
			if tc.expectedRequeue {
				assert.True(t, result.RequeueAfter > 0, "Expected requeue, but got no requeue")
			} else {
				assert.Zero(t, result.RequeueAfter, "Expected no requeue, but got requeue")
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
	now := metav1.Now()
	timeout := 5 * time.Minute

	s := runtime.NewScheme()
	err := workv1alpha2.Install(s)
	assert.NoError(t, err, "Failed to add workv1alpha2 to scheme")

	tests := []struct {
		name                string
		binding             *workv1alpha2.ClusterResourceBinding
		expectedRetryAfter  time.Duration
		expectedEvictionLen int
		expectedError       bool
	}{
		{
			name: "no tasks",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
					},
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
					},
				},
			},
			expectedRetryAfter:  0,
			expectedEvictionLen: 0,
			expectedError:       false,
		},
		{
			name: "task not expired",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
					},
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
					},
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster:       "cluster1",
							CreationTimestamp: &metav1.Time{Time: now.Add(-2 * time.Minute)},
						},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status:      createRawExtension("Bound"),
						},
					},
				},
			},
			expectedRetryAfter:  3 * time.Minute,
			expectedEvictionLen: 1,
			expectedError:       false,
		},
		{
			name: "task expired",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
					},
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
					},
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster:       "cluster1",
							CreationTimestamp: &metav1.Time{Time: now.Add(-6 * time.Minute)},
						},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{
							ClusterName: "cluster1",
							Status:      createRawExtension("Bound"),
						},
					},
				},
			},
			expectedRetryAfter:  0,
			expectedEvictionLen: 0,
			expectedError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake client with the binding object
			client := fake.NewClientBuilder().WithScheme(s).WithObjects(tt.binding).Build()
			c := &CRBGracefulEvictionController{
				Client:                  client,
				EventRecorder:           record.NewFakeRecorder(10),
				GracefulEvictionTimeout: timeout,
			}

			retryAfter, err := c.syncBinding(context.Background(), types.NamespacedName{Name: tt.binding.Name})

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Tolerance is intentionally loose: the assertion compares a wall-clock
			// duration computed at test setup against one computed inside the
			// controller, and the gap is dominated by fake-client setup latency
			// rather than by the controller's logic.
			assert.True(t, almostEqual(retryAfter, tt.expectedRetryAfter, time.Second),
				"Expected retry after %v, but got %v", tt.expectedRetryAfter, retryAfter)

			// Check the updated binding
			updatedBinding := &workv1alpha2.ClusterResourceBinding{}
			err = client.Get(context.Background(), types.NamespacedName{Name: tt.binding.Name}, updatedBinding)
			assert.NoError(t, err, "Failed to get updated binding")

			actualEvictionLen := len(updatedBinding.Spec.GracefulEvictionTasks)
			assert.Equal(t, tt.expectedEvictionLen, actualEvictionLen,
				"Expected %d eviction tasks, but got %d", tt.expectedEvictionLen, actualEvictionLen)
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

func TestCRBGracefulEvictionController_syncBinding_retryOnConflict(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, workv1alpha2.Install(scheme))

	now := metav1.Now()
	crb := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "conflict-crb"},
		Spec: workv1alpha2.ResourceBindingSpec{
			Clusters: []workv1alpha2.TargetCluster{{Name: "member1"}},
			GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
				{
					FromCluster:       "member1",
					CreationTimestamp: &metav1.Time{Time: now.Add(-10 * time.Minute)}, // expired ⇒ patched out
				},
			},
		},
	}

	gr := schema.GroupResource{Group: "work.karmada.io", Resource: "clusterresourcebindings"}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(crb).
		WithInterceptorFuncs(conflictPatchInterceptor(gr, crb.Name, 2)).
		Build()

	c := &CRBGracefulEvictionController{
		Client:                  fakeClient,
		EventRecorder:           record.NewFakeRecorder(10),
		GracefulEvictionTimeout: 5 * time.Minute,
	}

	if _, err := c.syncBinding(context.Background(), types.NamespacedName{Name: crb.Name}); err != nil {
		t.Fatalf("syncBinding returned error after transient conflicts: %v", err)
	}

	got := &workv1alpha2.ClusterResourceBinding{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: crb.Name}, got); err != nil {
		t.Fatalf("failed to refetch cluster binding: %v", err)
	}
	if len(got.Spec.GracefulEvictionTasks) != 0 {
		t.Errorf("expected expired eviction task to be removed after retry-on-conflict, still have %d task(s)", len(got.Spec.GracefulEvictionTasks))
	}
}

func TestCRBGracefulEvictionController_syncBinding_notFoundSurfacedToReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, workv1alpha2.Install(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	c := &CRBGracefulEvictionController{
		Client:                  fakeClient,
		EventRecorder:           record.NewFakeRecorder(10),
		GracefulEvictionTimeout: 5 * time.Minute,
	}

	_, err := c.syncBinding(context.Background(), types.NamespacedName{Name: "missing"})
	if err == nil || !apierrors.IsNotFound(err) {
		t.Fatalf("expected NotFound error so Reconcile can short-circuit, got %v", err)
	}
}
