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

package helper

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestUpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name          string
		setupObj      *corev1.Pod
		mutateStatus  func(*corev1.Pod)
		statusError   bool
		expectedOp    controllerutil.OperationResult
		expectedError string
	}{
		{
			name: "successful status update",
			setupObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			mutateStatus: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodRunning
			},
			statusError:   false,
			expectedOp:    controllerutil.OperationResultUpdatedStatusOnly,
			expectedError: "",
		},
		{
			name: "status update error",
			setupObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			mutateStatus: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodRunning
			},
			statusError:   true,
			expectedOp:    controllerutil.OperationResultNone,
			expectedError: "Internal error occurred: status update failed",
		},
		{
			name: "no changes needed",
			setupObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			mutateStatus: func(_ *corev1.Pod) {
				// No changes to status
			},
			statusError:   false,
			expectedOp:    controllerutil.OperationResultNone,
			expectedError: "",
		},
		{
			name:     "object not found",
			setupObj: nil, // Not create the object
			mutateStatus: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodRunning
			},
			statusError:   false,
			expectedOp:    controllerutil.OperationResultNone,
			expectedError: "not found",
		},
		{
			name: "mutation error",
			setupObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			mutateStatus: func(pod *corev1.Pod) {
				// Simulate mutation error by changing name
				pod.Name = "different-name"
			},
			statusError:   false,
			expectedOp:    controllerutil.OperationResultNone,
			expectedError: "cannot mutate object name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.setupObj != nil {
				clientBuilder = clientBuilder.WithObjects(tt.setupObj)
			}
			fakeClient := clientBuilder.Build()

			var client client.Client
			if tt.statusError {
				client = &mockClient{
					Client:      fakeClient,
					shouldError: true,
				}
			} else {
				client = fakeClient
			}

			// Create a new object for update
			obj := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			}

			// Run the update
			op, err := UpdateStatus(context.TODO(), client, obj, func() error {
				if tt.mutateStatus != nil {
					tt.mutateStatus(obj)
				}
				return nil
			})

			// Check error
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			// Check operation result
			assert.Equal(t, tt.expectedOp, op)

			// If successful update, verify the status was actually changed
			if tt.expectedOp == controllerutil.OperationResultUpdatedStatusOnly {
				updatedPod := &corev1.Pod{}
				err := client.Get(context.TODO(), types.NamespacedName{Name: "test-pod", Namespace: "default"}, updatedPod)
				assert.NoError(t, err)
				assert.Equal(t, corev1.PodRunning, updatedPod.Status.Phase)
			}
		})
	}
}

func TestMutate(t *testing.T) {
	tests := []struct {
		name          string
		key           types.NamespacedName
		obj           *corev1.Pod
		mutateFn      controllerutil.MutateFn
		expectedError string
	}{
		{
			name: "successful mutation with doing nothing",
			key: types.NamespacedName{
				Name:      "test-pod",
				Namespace: "default",
			},
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			mutateFn: func() error {
				return nil
			},
			expectedError: "",
		},
		{
			name: "mutation function error",
			key: types.NamespacedName{
				Name:      "test-pod",
				Namespace: "default",
			},
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			mutateFn: func() error {
				return fmt.Errorf("mutation failed")
			},
			expectedError: "mutation failed",
		},
		{
			name: "attempt to mutate object name",
			key: types.NamespacedName{
				Name:      "test-pod",
				Namespace: "default",
			},
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			mutateFn:      nil, // Will be set in the test
			expectedError: "MutateFn cannot mutate object name and/or object namespace",
		},
		{
			name: "attempt to mutate object namespace",
			key: types.NamespacedName{
				Name:      "test-pod",
				Namespace: "default",
			},
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			mutateFn:      nil, // Will be set in the test
			expectedError: "MutateFn cannot mutate object name and/or object namespace",
		},
		{
			name: "attempt to mutate both name and namespace",
			key: types.NamespacedName{
				Name:      "test-pod",
				Namespace: "default",
			},
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			mutateFn:      nil, // Will be set in the test
			expectedError: "MutateFn cannot mutate object name and/or object namespace",
		},
		{
			name: "successful mutation with labels and annotations",
			key: types.NamespacedName{
				Name:      "test-pod",
				Namespace: "default",
			},
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			mutateFn:      nil, // Will be set in the test
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create local copy of the test object
			testObj := tt.obj.DeepCopy()

			// Set up mutation functions that operate on the local object
			switch tt.name {
			case "attempt to mutate object name":
				tt.mutateFn = func() error {
					testObj.SetName("modified-pod")
					return nil
				}
			case "attempt to mutate object namespace":
				tt.mutateFn = func() error {
					testObj.SetNamespace("new-namespace")
					return nil
				}
			case "attempt to mutate both name and namespace":
				tt.mutateFn = func() error {
					testObj.SetName("modified-pod")
					testObj.SetNamespace("new-namespace")
					return nil
				}
			case "successful mutation with labels and annotations":
				tt.mutateFn = func() error {
					testObj.SetLabels(map[string]string{"key": "value"})
					testObj.SetAnnotations(map[string]string{"note": "test"})
					return nil
				}
			}

			err := mutate(tt.mutateFn, tt.key, testObj)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				return
			}
			assert.NoError(t, err)

			// Verify the object key hasn't changed for successful mutations
			newKey := types.NamespacedName{
				Name:      testObj.GetName(),
				Namespace: testObj.GetNamespace(),
			}

			assert.Equal(t, tt.key, newKey)
			// For successful mutations, verify other metadata changes were applied
			if testObj.GetLabels() != nil {
				assert.Equal(t, "value", testObj.GetLabels()["key"])
			}
			if testObj.GetAnnotations() != nil {
				assert.Equal(t, "test", testObj.GetAnnotations()["note"])
			}
		})
	}
}

// Mock Implementations

// mockStatusWriter is a mock implementation of client.StatusWriter that returns an error
type mockStatusWriter struct {
	client.StatusWriter
	shouldError bool
}

func (m *mockStatusWriter) Update(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
	if m.shouldError {
		return apierrors.NewInternalError(fmt.Errorf("status update failed"))
	}
	return nil
}

// mockClient wraps the fake client and returns our mock status writer
type mockClient struct {
	client.Client
	shouldError bool
}

func (m *mockClient) Status() client.StatusWriter {
	return &mockStatusWriter{shouldError: m.shouldError}
}
