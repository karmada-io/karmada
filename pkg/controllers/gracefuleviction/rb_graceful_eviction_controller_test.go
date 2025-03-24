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
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
)

func TestRBGracefulEvictionController_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	err := workv1alpha2.Install(scheme)
	require.NoError(t, err, "Failed to add workv1alpha2 to scheme")

	now := metav1.Now()

	testCases := []struct {
		name            string
		binding         *workv1alpha2.ResourceBinding
		expectedResult  controllerruntime.Result
		expectedError   bool
		expectedRequeue bool
		notFound        bool
	}{
		{
			name: "binding with no graceful eviction tasks",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
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
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-binding",
					Namespace:  "default",
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
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-binding",
					Namespace:         "default",
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
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-existent-binding",
					Namespace: "default",
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

			c := &RBGracefulEvictionController{
				Client:                  client,
				EventRecorder:           record.NewFakeRecorder(10),
				RateLimiterOptions:      ratelimiterflag.Options{},
				GracefulEvictionTimeout: 5 * time.Minute,
			}

			result, err := c.Reconcile(context.TODO(), controllerruntime.Request{
				NamespacedName: types.NamespacedName{
					Namespace: tc.binding.Namespace,
					Name:      tc.binding.Name,
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
				updatedBinding := &workv1alpha2.ResourceBinding{}
				err = client.Get(context.TODO(), types.NamespacedName{Namespace: tc.binding.Namespace, Name: tc.binding.Name}, updatedBinding)
				assert.NoError(t, err)
			}
		})
	}
}

func TestRBGracefulEvictionController_syncBinding(t *testing.T) {
	scheme := runtime.NewScheme()
	err := workv1alpha2.Install(scheme)
	if err != nil {
		t.Fatalf("Failed to add workv1alpha2 to scheme: %v", err)
	}

	now := metav1.Now()

	testCases := []struct {
		name                string
		binding             *workv1alpha2.ResourceBinding
		expectedRetryAfter  time.Duration
		expectedEvictionLen int
		expectedError       bool
	}{
		{
			name: "no eviction tasks",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{},
				},
			},
			expectedRetryAfter:  0,
			expectedEvictionLen: 0,
			expectedError:       false,
		},
		{
			name: "active eviction task",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster:       "cluster1",
							CreationTimestamp: &now,
						},
					},
				},
			},
			expectedRetryAfter:  0,
			expectedEvictionLen: 0,
			expectedError:       false,
		},
		{
			name: "expired eviction task",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
						{
							FromCluster:       "cluster1",
							CreationTimestamp: &metav1.Time{Time: now.Add(-10 * time.Minute)},
						},
					},
				},
			},
			expectedRetryAfter:  0,
			expectedEvictionLen: 0,
			expectedError:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fake client with the binding object
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.binding).Build()
			c := &RBGracefulEvictionController{
				Client:                  client,
				EventRecorder:           &record.FakeRecorder{},
				RateLimiterOptions:      ratelimiterflag.Options{},
				GracefulEvictionTimeout: 5 * time.Minute,
			}

			retryAfter, err := c.syncBinding(context.TODO(), tc.binding)

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedRetryAfter, retryAfter)

			// Check the updated binding
			updatedBinding := &workv1alpha2.ResourceBinding{}
			err = client.Get(context.TODO(), types.NamespacedName{Namespace: tc.binding.Namespace, Name: tc.binding.Name}, updatedBinding)
			assert.NoError(t, err)

			assert.Equal(t, tc.expectedEvictionLen, len(updatedBinding.Spec.GracefulEvictionTasks))
		})
	}
}
