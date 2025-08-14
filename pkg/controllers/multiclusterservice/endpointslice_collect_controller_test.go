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

package multiclusterservice

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

func TestGetEventHandler(t *testing.T) {
	testCases := []struct {
		name            string
		clusterName     string
		existingHandler bool
	}{
		{
			name:            "New handler",
			clusterName:     "cluster1",
			existingHandler: false,
		},
		{
			name:            "Existing handler",
			clusterName:     "cluster2",
			existingHandler: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := &EndpointSliceCollectController{
				eventHandlers: sync.Map{},
				worker:        &mockAsyncWorker{},
			}
			if tc.existingHandler {
				controller.eventHandlers.Store(tc.clusterName, &mockResourceEventHandler{})
			}
			handler := controller.getEventHandler(tc.clusterName)
			assert.NotNil(t, handler, "Handler should not be nil")
			storedHandler, exists := controller.eventHandlers.Load(tc.clusterName)
			assert.True(t, exists, "Handler should be stored in eventHandlers")
			assert.Equal(t, handler, storedHandler, "Stored handler should match returned handler")
			if !tc.existingHandler {
				assert.IsType(t, &cache.ResourceEventHandlerFuncs{}, handler, "New handler should be of type *cache.ResourceEventHandlerFuncs")
			} else {
				assert.IsType(t, &mockResourceEventHandler{}, handler, "Existing handler should be of type *mockResourceEventHandler")
			}
		})
	}
}

func TestGenHandlerFuncs(t *testing.T) {
	clusterName := "test-cluster"
	testObj := createTestEndpointSlice("test-object", "test-namespace")

	t.Run("AddFunc", func(t *testing.T) {
		mockWorker := &mockAsyncWorker{}
		controller := &EndpointSliceCollectController{
			worker: mockWorker,
		}
		addFunc := controller.genHandlerAddFunc(clusterName)
		addFunc(testObj)
		assert.Equal(t, 1, mockWorker.addCount, "Add function should be called once")
	})

	t.Run("UpdateFunc", func(t *testing.T) {
		mockWorker := &mockAsyncWorker{}
		controller := &EndpointSliceCollectController{
			worker: mockWorker,
		}
		updateFunc := controller.genHandlerUpdateFunc(clusterName)
		newObj := createTestEndpointSlice("test-object", "test-namespace")
		newObj.SetLabels(map[string]string{"new-label": "new-value"})

		updateFunc(testObj, newObj)
		assert.Equal(t, 1, mockWorker.addCount, "Update function should be called once when objects are different")

		updateFunc(testObj, testObj)
		assert.Equal(t, 1, mockWorker.addCount, "Update function should not be called when objects are the same")
	})

	t.Run("DeleteFunc", func(t *testing.T) {
		mockWorker := &mockAsyncWorker{}
		controller := &EndpointSliceCollectController{
			worker: mockWorker,
		}
		deleteFunc := controller.genHandlerDeleteFunc(clusterName)
		deleteFunc(testObj)
		assert.Equal(t, 1, mockWorker.addCount, "Delete function should be called once")

		deletedObj := cache.DeletedFinalStateUnknown{Obj: testObj}
		deleteFunc(deletedObj)
		assert.Equal(t, 2, mockWorker.addCount, "Delete function should be called for DeletedFinalStateUnknown")
	})
}

func TestGetEndpointSliceWorkMeta(t *testing.T) {
	testCases := []struct {
		name          string
		existingWork  *workv1alpha1.Work
		endpointSlice *unstructured.Unstructured
		expectedMeta  metav1.ObjectMeta
		expectedError bool
	}{
		{
			name:          "New work for EndpointSlice",
			endpointSlice: createEndpointSliceForTest("test-eps", "default", "test-service", false),
			expectedMeta: metav1.ObjectMeta{
				Name:      "endpointslice-test-eps-default",
				Namespace: "test-cluster",
				Labels: map[string]string{
					util.MultiClusterServiceNamespaceLabel: "default",
					util.MultiClusterServiceNameLabel:      "test-service",
					util.EndpointSliceWorkManagedByLabel:   util.MultiClusterServiceKind,
				},
				Finalizers: []string{util.MCSEndpointSliceDispatchControllerFinalizer},
			},
		},
		{
			name:          "Existing work for EndpointSlice without finalizers",
			existingWork:  createExistingWork("endpointslice-test-eps-default", "test-cluster", "ExistingController"),
			endpointSlice: createEndpointSliceForTest("test-eps", "default", "test-service", false),
			expectedMeta: metav1.ObjectMeta{
				Name:      "endpointslice-test-eps-default",
				Namespace: "test-cluster",
				Labels: map[string]string{
					util.MultiClusterServiceNamespaceLabel: "default",
					util.MultiClusterServiceNameLabel:      "test-service",
					util.EndpointSliceWorkManagedByLabel:   "ExistingController.MultiClusterService",
				},
				Finalizers: []string{util.MCSEndpointSliceDispatchControllerFinalizer},
			},
		},
		{
			name:          "Existing work with existing finalizers",
			existingWork:  createExistingWorkWithFinalizers("endpointslice-test-eps-default", "test-cluster", "ExistingController", []string{"existing.finalizer", "another.finalizer"}),
			endpointSlice: createEndpointSliceForTest("test-eps", "default", "test-service", false),
			expectedMeta: metav1.ObjectMeta{
				Name:      "endpointslice-test-eps-default",
				Namespace: "test-cluster",
				Labels: map[string]string{
					util.MultiClusterServiceNamespaceLabel: "default",
					util.MultiClusterServiceNameLabel:      "test-service",
					util.EndpointSliceWorkManagedByLabel:   "ExistingController.MultiClusterService",
				},
				Finalizers: []string{"another.finalizer", "existing.finalizer", util.MCSEndpointSliceDispatchControllerFinalizer},
			},
		},
		{
			name:          "Existing work with duplicate finalizer",
			existingWork:  createExistingWorkWithFinalizers("endpointslice-test-eps-default", "test-cluster", "ExistingController", []string{util.MCSEndpointSliceDispatchControllerFinalizer, "another.finalizer"}),
			endpointSlice: createEndpointSliceForTest("test-eps", "default", "test-service", false),
			expectedMeta: metav1.ObjectMeta{
				Name:      "endpointslice-test-eps-default",
				Namespace: "test-cluster",
				Labels: map[string]string{
					util.MultiClusterServiceNamespaceLabel: "default",
					util.MultiClusterServiceNameLabel:      "test-service",
					util.EndpointSliceWorkManagedByLabel:   "ExistingController.MultiClusterService",
				},
				Finalizers: []string{"another.finalizer", util.MCSEndpointSliceDispatchControllerFinalizer},
			},
		},
		{
			name:          "Existing work without labels",
			existingWork:  createExistingWorkWithoutLabels("endpointslice-test-eps-default", "test-cluster", []string{"existing.finalizer"}),
			endpointSlice: createEndpointSliceForTest("test-eps", "default", "test-service", false),
			expectedMeta: metav1.ObjectMeta{
				Name:      "endpointslice-test-eps-default",
				Namespace: "test-cluster",
				Labels: map[string]string{
					util.MultiClusterServiceNamespaceLabel: "default",
					util.MultiClusterServiceNameLabel:      "test-service",
					util.EndpointSliceWorkManagedByLabel:   util.MultiClusterServiceKind,
				},
				Finalizers: []string{"existing.finalizer", util.MCSEndpointSliceDispatchControllerFinalizer},
			},
		},
		{
			name:          "Nil EndpointSlice",
			endpointSlice: nil,
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := createFakeClient(tc.existingWork)
			testFunc := func() (metav1.ObjectMeta, error) {
				return getEndpointSliceWorkMeta(context.TODO(), fakeClient, "test-cluster", "endpointslice-test-eps-default", tc.endpointSlice)
			}
			if tc.expectedError {
				assert.Panics(t, func() {
					_, err := testFunc()
					require.Error(t, err)
				}, "Expected a panic for nil EndpointSlice")
			} else {
				meta, err := testFunc()
				require.NoError(t, err)
				assert.Equal(t, tc.expectedMeta.Name, meta.Name)
				assert.Equal(t, tc.expectedMeta.Namespace, meta.Namespace)

				assert.Equal(t, tc.expectedMeta.Finalizers, meta.Finalizers,
					"Finalizers do not match. Expected: %v, Got: %v", tc.expectedMeta.Finalizers, meta.Finalizers)

				assert.True(t, compareLabels(meta.Labels, tc.expectedMeta.Labels),
					"Labels do not match. Expected: %v, Got: %v", tc.expectedMeta.Labels, meta.Labels)
			}
		})
	}
}

func TestCleanProviderClustersEndpointSliceWork(t *testing.T) {
	testCases := []struct {
		name           string
		existingWork   *workv1alpha1.Work
		expectedWork   *workv1alpha1.Work
		expectedDelete bool
	}{
		{
			name: "Work managed by multiple controllers",
			existingWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "test-cluster",
					Labels: map[string]string{
						util.MultiClusterServiceNameLabel:      "test-service",
						util.MultiClusterServiceNamespaceLabel: "default",
						util.EndpointSliceWorkManagedByLabel:   "MultiClusterService.OtherController",
					},
				},
			},
			expectedWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "test-cluster",
					Labels: map[string]string{
						util.EndpointSliceWorkManagedByLabel: "OtherController",
					},
				},
			},
			expectedDelete: false,
		},
		{
			name: "Work managed only by MultiClusterService",
			existingWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "test-cluster",
					Labels: map[string]string{
						util.MultiClusterServiceNameLabel:      "test-service",
						util.MultiClusterServiceNamespaceLabel: "default",
						util.EndpointSliceWorkManagedByLabel:   "MultiClusterService",
					},
				},
			},
			expectedWork:   nil,
			expectedDelete: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := setupSchemeEndpointCollect()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.existingWork).Build()
			err := cleanProviderClustersEndpointSliceWork(context.TODO(), fakeClient, tc.existingWork)
			assert.NoError(t, err, "Unexpected error in cleanProviderClustersEndpointSliceWork")

			if tc.expectedDelete {
				err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: tc.existingWork.Name, Namespace: tc.existingWork.Namespace}, &workv1alpha1.Work{})
				assert.Error(t, err, "Expected Work to be deleted, but it still exists")
				assert.True(t, apierrors.IsNotFound(err), "Expected NotFound error, got %v", err)
			} else {
				updatedWork := &workv1alpha1.Work{}
				err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: tc.existingWork.Name, Namespace: tc.existingWork.Namespace}, updatedWork)
				assert.NoError(t, err, "Failed to get updated Work")
				assert.True(t, compareLabels(updatedWork.Labels, tc.expectedWork.Labels),
					"Labels mismatch. Expected %v, but got %v", tc.expectedWork.Labels, updatedWork.Labels)
			}
		})
	}
}

// Helper Functions

// Helper function to set up a scheme for EndpointSlice collection tests
func setupSchemeEndpointCollect() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = workv1alpha1.Install(scheme)
	_ = discoveryv1.AddToScheme(scheme)
	return scheme
}

// Helper function to create a test EndpointSlice
func createTestEndpointSlice(name, namespace string) *unstructured.Unstructured {
	endpointSlice := &discoveryv1.EndpointSlice{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "discovery.k8s.io/v1",
			Kind:       "EndpointSlice",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	unstructuredObj, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(endpointSlice)
	return &unstructured.Unstructured{Object: unstructuredObj}
}

// Helper function to create an EndpointSlice for testing with specific properties
func createEndpointSliceForTest(name, namespace, serviceName string, isManaged bool) *unstructured.Unstructured {
	labels := map[string]interface{}{
		discoveryv1.LabelServiceName: serviceName,
	}
	if isManaged {
		labels[discoveryv1.LabelManagedBy] = util.EndpointSliceDispatchControllerLabelValue
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "discovery.k8s.io/v1",
			"kind":       "EndpointSlice",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels":    labels,
			},
		},
	}
}

// Helper function to create an existing Work resource for testing
func createExistingWork(name, namespace, managedBy string) *workv1alpha1.Work {
	return &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				util.EndpointSliceWorkManagedByLabel: managedBy,
			},
		},
	}
}

// Helper function to create an existing Work resource for testing with specific finalizers
func createExistingWorkWithFinalizers(name, namespace, managedBy string, finalizers []string) *workv1alpha1.Work {
	return &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				util.EndpointSliceWorkManagedByLabel: managedBy,
			},
			Finalizers: finalizers,
		},
	}
}

// Helper function to create an existing Work resource for testing without labels
func createExistingWorkWithoutLabels(name, namespace string, finalizers []string) *workv1alpha1.Work {
	return &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Finalizers: finalizers,
		},
	}
}

// Helper function to create a fake client with an optional existing Work
func createFakeClient(existingWork *workv1alpha1.Work) client.Client {
	scheme := setupSchemeEndpointCollect()
	objs := []client.Object{}
	if existingWork != nil {
		objs = append(objs, existingWork)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

// Helper function to compare two label maps, considering special handling for EndpointSliceWorkManagedByLabel
func compareLabels(actual, expected map[string]string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for k, v := range expected {
		actualV, exists := actual[k]
		if !exists {
			return false
		}
		if k == util.EndpointSliceWorkManagedByLabel {
			actualParts := strings.Split(actualV, ".")
			expectedParts := strings.Split(v, ".")
			sort.Strings(actualParts)
			sort.Strings(expectedParts)
			if !reflect.DeepEqual(actualParts, expectedParts) {
				return false
			}
		} else if actualV != v {
			return false
		}
	}
	return true
}

// Mock implementations

type mockAsyncWorker struct {
	addCount int
}

func (m *mockAsyncWorker) Add(_ interface{}) {
	m.addCount++
}

func (m *mockAsyncWorker) AddAfter(_ interface{}, _ time.Duration) {}

func (m *mockAsyncWorker) Enqueue(_ interface{}) {}

func (m *mockAsyncWorker) Run(_ context.Context, _ int) {}

type mockResourceEventHandler struct{}

func (m *mockResourceEventHandler) OnAdd(_ interface{}, _ bool) {}

func (m *mockResourceEventHandler) OnUpdate(_, _ interface{}) {}

func (m *mockResourceEventHandler) OnDelete(_ interface{}) {}
