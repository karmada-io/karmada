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

package mcs

import (
	"context"
	"errors"
	"fmt"
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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestGetEventHandler(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		preExisting bool
	}{
		{
			name:        "Get new event handler",
			clusterName: "cluster1",
			preExisting: false,
		},
		{
			name:        "Get existing event handler",
			clusterName: "cluster2",
			preExisting: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ServiceExportController{
				eventHandlers: sync.Map{},
				worker:        &mockAsyncWorker{},
			}

			var preExistingHandler cache.ResourceEventHandler
			if tt.preExisting {
				preExistingHandler = c.getEventHandler(tt.clusterName)
				c.eventHandlers.Store(tt.clusterName, preExistingHandler)
			}

			handler := c.getEventHandler(tt.clusterName)

			assert.NotNil(t, handler, "Event handler should not be nil")

			// If pre-existing, check if it's the same instance
			if tt.preExisting {
				storedHandler, exists := c.eventHandlers.Load(tt.clusterName)
				assert.True(t, exists, "Pre-existing handler should be stored in the map")
				assert.Equal(t, fmt.Sprintf("%p", preExistingHandler), fmt.Sprintf("%p", storedHandler), "Should return the pre-existing handler")
			}

			// Check if the handler is stored in the map
			storedHandler, exists := c.eventHandlers.Load(tt.clusterName)
			assert.True(t, exists, "Handler should be stored in the map")
			assert.Equal(t, fmt.Sprintf("%p", handler), fmt.Sprintf("%p", storedHandler), "Stored handler should be the same as returned handler")

			testObj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ServiceExport",
					"metadata": map[string]interface{}{
						"name":      "test-object",
						"namespace": "test-namespace",
					},
				},
			}

			handler.OnAdd(testObj, false)
			handler.OnUpdate(testObj, testObj)
			handler.OnDelete(testObj)

			mockWorker := c.worker.(*mockAsyncWorker)
			assert.GreaterOrEqual(t, len(mockWorker.addedItems), 1, "Worker should have received at least 1 item")

			// Print the contents of addedItems for debugging
			t.Logf("Added items: %+v", mockWorker.addedItems)
		})
	}
}

func TestGenHandlerAddFunc(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		obj         runtime.Object
		expectKey   string
		expectAdd   bool
	}{
		{
			name:        "Normal add",
			clusterName: "test-cluster",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ServiceExport",
					"metadata": map[string]interface{}{
						"name":      "test-object",
						"namespace": "test-namespace",
					},
				},
			},
			expectKey: "cluster=test-cluster, v1, kind=ServiceExport, test-namespace/test-object",
			expectAdd: true,
		},
		{
			name:        "Add with empty metadata",
			clusterName: "test-cluster",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ServiceExport",
				},
			},
			expectKey: "cluster=test-cluster, v1, kind=ServiceExport, ",
			expectAdd: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorker := &mockAsyncWorker{}
			c := &ServiceExportController{
				worker: mockWorker,
			}
			addFunc := c.genHandlerAddFunc(tt.clusterName)

			addFunc(tt.obj)

			if tt.expectAdd {
				assert.Equal(t, 1, len(mockWorker.addedItems), "Expected one item to be added")
				addedKey, ok := mockWorker.addedItems[0].(keys.FederatedKey)
				assert.True(t, ok, "Added item should be a FederatedKey")
				assert.Equal(t, tt.expectKey, addedKey.String(), "Added key does not match expected")
			} else {
				assert.Equal(t, 0, len(mockWorker.addedItems), "Expected no items to be added")
			}
		})
	}
}

func TestGenHandlerDeleteFunc(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		obj         interface{}
		expectKey   string
		expectAdd   bool
	}{
		{
			name:        "Normal delete",
			clusterName: "test-cluster",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ServiceExport",
					"metadata": map[string]interface{}{
						"name":      "test-object",
						"namespace": "test-namespace",
					},
				},
			},
			expectKey: "cluster=test-cluster, v1, kind=ServiceExport, test-namespace/test-object",
			expectAdd: true,
		},
		{
			name:        "Delete with DeletedFinalStateUnknown",
			clusterName: "test-cluster",
			obj: cache.DeletedFinalStateUnknown{
				Key: "test-key",
				Obj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ServiceExport",
						"metadata": map[string]interface{}{
							"name":      "test-object",
							"namespace": "test-namespace",
						},
					},
				},
			},
			expectKey: "cluster=test-cluster, v1, kind=ServiceExport, test-namespace/test-object",
			expectAdd: true,
		},
		{
			name:        "DeletedFinalStateUnknown with nil object",
			clusterName: "test-cluster",
			obj: cache.DeletedFinalStateUnknown{
				Key: "test-key",
				Obj: nil,
			},
			expectKey: "",
			expectAdd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorker := &mockAsyncWorker{}
			c := &ServiceExportController{
				worker: mockWorker,
			}
			deleteFunc := c.genHandlerDeleteFunc(tt.clusterName)

			deleteFunc(tt.obj)

			if tt.expectAdd {
				assert.Equal(t, 1, len(mockWorker.addedItems), "Expected one item to be added")
				addedKey, ok := mockWorker.addedItems[0].(keys.FederatedKey)
				assert.True(t, ok, "Added item should be a FederatedKey")
				assert.Equal(t, tt.expectKey, addedKey.String(), "Added key does not match expected")
			} else {
				assert.Equal(t, 0, len(mockWorker.addedItems), "Expected no items to be added")
			}
		})
	}
}

func TestGenHandlerUpdateFunc(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		oldObj      runtime.Object
		newObj      runtime.Object
		expectKey   string
		expectAdd   bool
	}{
		{
			name:        "Objects are different",
			clusterName: "test-cluster",
			oldObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ServiceExport",
					"metadata": map[string]interface{}{
						"name":      "test-object",
						"namespace": "test-namespace",
					},
				},
			},
			newObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ServiceExport",
					"metadata": map[string]interface{}{
						"name":      "test-object",
						"namespace": "test-namespace",
						"labels":    map[string]interface{}{"new": "label"},
					},
				},
			},
			expectKey: "cluster=test-cluster, v1, kind=ServiceExport, test-namespace/test-object",
			expectAdd: true,
		},
		{
			name:        "Objects are identical",
			clusterName: "test-cluster",
			oldObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ServiceExport",
					"metadata": map[string]interface{}{
						"name":      "test-object",
						"namespace": "test-namespace",
					},
				},
			},
			newObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ServiceExport",
					"metadata": map[string]interface{}{
						"name":      "test-object",
						"namespace": "test-namespace",
					},
				},
			},
			expectKey: "",
			expectAdd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorker := &mockAsyncWorker{}
			c := &ServiceExportController{
				worker: mockWorker,
			}
			updateFunc := c.genHandlerUpdateFunc(tt.clusterName)

			updateFunc(tt.oldObj, tt.newObj)

			if tt.expectAdd {
				assert.Equal(t, 1, len(mockWorker.addedItems), "Expected one item to be added")
				addedKey, ok := mockWorker.addedItems[0].(keys.FederatedKey)
				assert.True(t, ok, "Added item should be a FederatedKey")
				assert.Equal(t, tt.expectKey, addedKey.String(), "Added key does not match expected")
			} else {
				assert.Equal(t, 0, len(mockWorker.addedItems), "Expected no items to be added")
			}
		})
	}
}

func TestRemoveOrphanWork(t *testing.T) {
	tests := []struct {
		name              string
		endpointSlices    []runtime.Object
		existingWorks     []runtime.Object
		serviceExportKey  keys.FederatedKey
		expectedWorkNames []string
	}{
		{
			name: "no works removed",
			endpointSlices: []runtime.Object{
				newUnstructuredEndpointSlice("eps1", "default", "svc1"),
			},
			existingWorks: []runtime.Object{
				newWork("work-eps1", "default", "svc1"),
				newWork("work-eps2", "default", "svc1"),
			},
			serviceExportKey: keys.FederatedKey{
				Cluster: "member1",
				ClusterWideKey: keys.ClusterWideKey{
					Namespace: "default",
					Name:      "svc1",
				},
			},
			expectedWorkNames: []string{"work-eps1", "work-eps2"},
		},
		{
			name:           "no existing works",
			endpointSlices: []runtime.Object{},
			existingWorks:  []runtime.Object{},
			serviceExportKey: keys.FederatedKey{
				Cluster: "member1",
				ClusterWideKey: keys.ClusterWideKey{
					Namespace: "default",
					Name:      "svc1",
				},
			},
			expectedWorkNames: []string{},
		},
		{
			name: "work with different service",
			endpointSlices: []runtime.Object{
				newUnstructuredEndpointSlice("eps1", "default", "svc1"),
			},
			existingWorks: []runtime.Object{
				newWork("work-eps1", "default", "svc1"),
				newWork("work-eps2", "default", "svc2"),
			},
			serviceExportKey: keys.FederatedKey{
				Cluster: "member1",
				ClusterWideKey: keys.ClusterWideKey{
					Namespace: "default",
					Name:      "svc1",
				},
			},
			expectedWorkNames: []string{"work-eps1", "work-eps2"},
		},
		{
			name: "multiple services and endpoint slices",
			endpointSlices: []runtime.Object{
				newUnstructuredEndpointSlice("eps1", "default", "svc1"),
				newUnstructuredEndpointSlice("eps2", "default", "svc1"),
				newUnstructuredEndpointSlice("eps3", "default", "svc2"),
			},
			existingWorks: []runtime.Object{
				newWork("work-eps1", "default", "svc1"),
				newWork("work-eps2", "default", "svc1"),
				newWork("work-eps3", "default", "svc1"),
				newWork("work-eps4", "default", "svc2"),
			},
			serviceExportKey: keys.FederatedKey{
				Cluster: "member1",
				ClusterWideKey: keys.ClusterWideKey{
					Namespace: "default",
					Name:      "svc1",
				},
			},
			expectedWorkNames: []string{"work-eps1", "work-eps2", "work-eps3", "work-eps4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := setupScheme()
			require.NoError(t, err)

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(tt.existingWorks...).Build()
			stopCh := make(<-chan struct{})
			controller := &ServiceExportController{
				Client:          fakeClient,
				InformerManager: genericmanager.NewMultiClusterInformerManager(stopCh),
			}

			dynamicClient := dynamicfake.NewSimpleDynamicClient(s, tt.endpointSlices...)
			controller.InformerManager.ForCluster(tt.serviceExportKey.Cluster, dynamicClient, 0).ForResource(endpointSliceGVR, nil)

			err = controller.removeOrphanWork(context.TODO(), tt.endpointSlices, tt.serviceExportKey)
			assert.NoError(t, err)

			workList := &workv1alpha1.WorkList{}
			err = fakeClient.List(context.TODO(), workList, &client.ListOptions{
				Namespace: names.GenerateExecutionSpaceName(tt.serviceExportKey.Cluster),
			})
			assert.NoError(t, err)

			actualWorkNames := []string{}
			for _, work := range workList.Items {
				actualWorkNames = append(actualWorkNames, work.Name)
			}
			assert.ElementsMatch(t, tt.expectedWorkNames, actualWorkNames)
		})
	}
}

func TestReportEndpointSliceWithEndpointSliceCreateOrUpdate(t *testing.T) {
	tests := []struct {
		name                string
		clusterName         string
		endpointSlice       *unstructured.Unstructured
		serviceExportExists bool
		serviceExportError  error
		expectedError       bool
		expectedReported    bool
	}{
		{
			name:        "ServiceExport exists, EndpointSlice should be reported",
			clusterName: "member1",
			endpointSlice: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "discovery.k8s.io/v1",
					"kind":       "EndpointSlice",
					"metadata": map[string]interface{}{
						"name":      "eps1",
						"namespace": "default",
						"labels": map[string]interface{}{
							discoveryv1.LabelServiceName: "svc1",
						},
					},
				},
			},
			serviceExportExists: true,
			expectedReported:    true,
		},
		{
			name:        "ServiceExport does not exist, EndpointSlice should not be reported",
			clusterName: "member1",
			endpointSlice: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "discovery.k8s.io/v1",
					"kind":       "EndpointSlice",
					"metadata": map[string]interface{}{
						"name":      "eps1",
						"namespace": "default",
						"labels": map[string]interface{}{
							discoveryv1.LabelServiceName: "svc1",
						},
					},
				},
			},
			serviceExportExists: false,
			expectedReported:    false,
		},
		{
			name:        "Error getting ServiceExport",
			clusterName: "member1",
			endpointSlice: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "discovery.k8s.io/v1",
					"kind":       "EndpointSlice",
					"metadata": map[string]interface{}{
						"name":      "eps1",
						"namespace": "default",
						"labels": map[string]interface{}{
							discoveryv1.LabelServiceName: "svc1",
						},
					},
				},
			},
			serviceExportExists: false,
			serviceExportError:  fmt.Errorf("internal error"),
			expectedError:       true,
			expectedReported:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := setupScheme()
			require.NoError(t, err)

			fakeClient := fake.NewClientBuilder().WithScheme(s).Build()

			mockGenericLister := &mockGenericLister{
				serviceExportExists: tt.serviceExportExists,
				serviceExportError:  tt.serviceExportError,
			}

			mockSingleClusterManager := &mockSingleClusterManager{
				lister: mockGenericLister,
			}

			mockInformerManager := &mockInformerManager{
				singleClusterManager: mockSingleClusterManager,
			}

			controller := &ServiceExportController{
				Client:          fakeClient,
				InformerManager: mockInformerManager,
			}

			err = controller.reportEndpointSliceWithEndpointSliceCreateOrUpdate(context.TODO(), tt.clusterName, tt.endpointSlice)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			workName := names.GenerateWorkName(tt.endpointSlice.GetKind(), tt.endpointSlice.GetName(), tt.endpointSlice.GetNamespace())
			work := &workv1alpha1.Work{}
			err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: workName, Namespace: names.GenerateExecutionSpaceName(tt.clusterName)}, work)

			if tt.expectedReported {
				assert.NoError(t, err)
				assert.Equal(t, workName, work.Name)
				assert.Equal(t, names.GenerateExecutionSpaceName(tt.clusterName), work.Namespace)
			} else {
				assert.True(t, apierrors.IsNotFound(err))
			}
		})
	}
}

func TestReportEndpointSlice(t *testing.T) {
	tests := []struct {
		name          string
		clusterName   string
		endpointSlice *unstructured.Unstructured
		existingWork  *workv1alpha1.Work
		expectedWork  *workv1alpha1.Work
		expectedError bool
	}{
		{
			name:        "create new work for endpointslice",
			clusterName: "member1",
			endpointSlice: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "discovery.k8s.io/v1",
					"kind":       "EndpointSlice",
					"metadata": map[string]interface{}{
						"name":      "eps1",
						"namespace": "default",
						"labels": map[string]interface{}{
							discoveryv1.LabelServiceName: "svc1",
						},
					},
				},
			},
			existingWork: nil,
			expectedWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.GenerateWorkName("EndpointSlice", "eps1", "default"),
					Namespace: names.GenerateExecutionSpaceName("member1"),
					Labels: map[string]string{
						util.ServiceNamespaceLabel:           "default",
						util.ServiceNameLabel:                "svc1",
						util.PropagationInstruction:          util.PropagationInstructionSuppressed,
						util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
					},
					Finalizers: []string{util.EndpointSliceControllerFinalizer},
				},
			},
		},
		{
			name:        "update existing work for endpointslice",
			clusterName: "member1",
			endpointSlice: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "discovery.k8s.io/v1",
					"kind":       "EndpointSlice",
					"metadata": map[string]interface{}{
						"name":      "eps1",
						"namespace": "default",
						"labels": map[string]interface{}{
							discoveryv1.LabelServiceName: "svc1",
						},
					},
				},
			},
			existingWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.GenerateWorkName("EndpointSlice", "eps1", "default"),
					Namespace: names.GenerateExecutionSpaceName("member1"),
					Labels: map[string]string{
						util.ServiceNamespaceLabel:           "default",
						util.ServiceNameLabel:                "svc1",
						util.PropagationInstruction:          util.PropagationInstructionSuppressed,
						util.EndpointSliceWorkManagedByLabel: "OtherController",
					},
				},
			},
			expectedWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.GenerateWorkName("EndpointSlice", "eps1", "default"),
					Namespace: names.GenerateExecutionSpaceName("member1"),
					Labels: map[string]string{
						util.ServiceNamespaceLabel:           "default",
						util.ServiceNameLabel:                "svc1",
						util.PropagationInstruction:          util.PropagationInstructionSuppressed,
						util.EndpointSliceWorkManagedByLabel: "OtherController.ServiceExport",
					},
					Finalizers: []string{util.EndpointSliceControllerFinalizer},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := setupScheme()
			require.NoError(t, err)

			fakeClient := fake.NewClientBuilder().WithScheme(s).Build()

			if tt.existingWork != nil {
				err := fakeClient.Create(context.TODO(), tt.existingWork)
				require.NoError(t, err)
			}

			err = reportEndpointSlice(context.TODO(), fakeClient, tt.endpointSlice, tt.clusterName)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				workName := names.GenerateWorkName(tt.endpointSlice.GetKind(), tt.endpointSlice.GetName(), tt.endpointSlice.GetNamespace())
				work := &workv1alpha1.Work{}
				err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: workName, Namespace: names.GenerateExecutionSpaceName(tt.clusterName)}, work)
				require.NoError(t, err)

				assert.Equal(t, tt.expectedWork.Name, work.Name)
				assert.Equal(t, tt.expectedWork.Namespace, work.Namespace)

				assert.Equal(t, len(tt.expectedWork.Labels), len(work.Labels), "Number of labels doesn't match")
				for key, expectedValue := range tt.expectedWork.Labels {
					actualValue, exists := work.Labels[key]
					assert.True(t, exists, "Expected label %s not found", key)
					if key == util.EndpointSliceWorkManagedByLabel {
						// Check if actualValue contains all expected components
						expectedComponents := strings.Split(expectedValue, ".")
						for _, component := range expectedComponents {
							assert.Contains(t, actualValue, component, "Label %s is missing expected component %s", key, component)
						}
					} else {
						assert.Equal(t, expectedValue, actualValue, "Label %s doesn't match", key)
					}
				}

				assert.Equal(t, tt.expectedWork.Finalizers, work.Finalizers)

				require.Equal(t, 1, len(work.Spec.Workload.Manifests))
				manifestObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&work.Spec.Workload.Manifests[0])
				require.NoError(t, err)
				assert.Equal(t, tt.endpointSlice.Object, manifestObj)

				// Verify that the Work's resourceVersion has been updated (for update case)
				if tt.existingWork != nil {
					assert.NotEqual(t, tt.existingWork.ResourceVersion, work.ResourceVersion)
				}
			}
		})
	}
}

func TestGetEndpointSliceWorkMeta(t *testing.T) {
	tests := []struct {
		name          string
		existingWork  *workv1alpha1.Work
		endpointSlice *unstructured.Unstructured
		expectedMeta  metav1.ObjectMeta
		expectedError bool
	}{
		{
			name: "new work",
			endpointSlice: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-eps",
						"namespace": "default",
						"labels": map[string]interface{}{
							"kubernetes.io/service-name": "test-service",
						},
					},
				},
			},
			expectedMeta: metav1.ObjectMeta{
				Name:       "endpointslice-test-eps-default",
				Namespace:  "member-1",
				Finalizers: []string{util.EndpointSliceControllerFinalizer},
				Labels: map[string]string{
					util.ServiceNamespaceLabel:           "default",
					util.ServiceNameLabel:                "test-service",
					util.PropagationInstruction:          util.PropagationInstructionSuppressed,
					util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
				},
			},
		},
		{
			name: "existing work with additional labels",
			existingWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "endpointslice-test-eps-default",
					Namespace: "member-1",
					Labels: map[string]string{
						"existing-label":                     "value",
						util.EndpointSliceWorkManagedByLabel: "ExistingController",
					},
				},
			},
			endpointSlice: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-eps",
						"namespace": "default",
						"labels": map[string]interface{}{
							"kubernetes.io/service-name": "test-service",
						},
					},
				},
			},
			expectedMeta: metav1.ObjectMeta{
				Name:       "endpointslice-test-eps-default",
				Namespace:  "member-1",
				Finalizers: []string{util.EndpointSliceControllerFinalizer},
				Labels: map[string]string{
					util.ServiceNamespaceLabel:           "default",
					util.ServiceNameLabel:                "test-service",
					util.PropagationInstruction:          util.PropagationInstructionSuppressed,
					util.EndpointSliceWorkManagedByLabel: "ExistingController.ServiceExport",
					"existing-label":                     "value",
				},
			},
		},
		{
			name:          "empty endpointSlice",
			endpointSlice: &unstructured.Unstructured{},
			expectedMeta: metav1.ObjectMeta{
				Name:       "endpointslice-test-eps-default",
				Namespace:  "member-1",
				Finalizers: []string{util.EndpointSliceControllerFinalizer},
				Labels: map[string]string{
					util.ServiceNamespaceLabel:           "",
					util.ServiceNameLabel:                "",
					util.PropagationInstruction:          util.PropagationInstructionSuppressed,
					util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
				},
			},
		},
		{
			name: "missing required labels in endpointSlice",
			endpointSlice: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-eps",
						"namespace": "default",
					},
				},
			},
			expectedMeta: metav1.ObjectMeta{
				Name:       "endpointslice-test-eps-default",
				Namespace:  "member-1",
				Finalizers: []string{util.EndpointSliceControllerFinalizer},
				Labels: map[string]string{
					util.ServiceNamespaceLabel:           "default",
					util.ServiceNameLabel:                "",
					util.PropagationInstruction:          util.PropagationInstructionSuppressed,
					util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			s := scheme.Scheme
			s.AddKnownTypes(workv1alpha1.SchemeGroupVersion, &workv1alpha1.Work{})

			objs := []runtime.Object{}
			if tt.existingWork != nil {
				objs = append(objs, tt.existingWork)
			}

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()

			gotMeta, err := getEndpointSliceWorkMeta(ctx, fakeClient, "member-1", "endpointslice-test-eps-default", tt.endpointSlice)

			if (err != nil) != tt.expectedError {
				t.Errorf("getEndpointSliceWorkMeta() error = %v, expectedError %v", err, tt.expectedError)
				return
			}

			assert.Equal(t, tt.expectedMeta.Name, gotMeta.Name, "Name should match")
			assert.Equal(t, tt.expectedMeta.Namespace, gotMeta.Namespace, "Namespace should match")
			assert.Equal(t, tt.expectedMeta.Finalizers, gotMeta.Finalizers, "Finalizers should match")

			assert.Equal(t, len(tt.expectedMeta.Labels), len(gotMeta.Labels), "Number of labels should match")

			for k, v := range tt.expectedMeta.Labels {
				if k != util.EndpointSliceWorkManagedByLabel {
					assert.Equal(t, v, gotMeta.Labels[k], "Label mismatch for key %s", k)
				} else {
					expectedControllers := sets.New(strings.Split(v, ".")...)
					gotControllers := sets.New(strings.Split(gotMeta.Labels[k], ".")...)
					assert.True(t, expectedControllers.Equal(gotControllers),
						"EndpointSliceWorkManagedByLabel should contain the same controllers, regardless of order")
				}
			}
		})
	}
}

func TestCleanupWorkWithServiceExportDelete(t *testing.T) {
	tests := []struct {
		name             string
		serviceExportKey keys.FederatedKey
		existingWorks    []runtime.Object
		expectedDeleted  int
		expectedError    bool
		deleteError      error
	}{
		{
			name: "delete works associated with service export",
			serviceExportKey: keys.FederatedKey{
				Cluster: "cluster1",
				ClusterWideKey: keys.ClusterWideKey{
					Namespace: "default",
					Name:      "test-service",
				},
			},
			existingWorks: []runtime.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work1",
						Namespace: names.GenerateExecutionSpaceName("cluster1"),
						Labels: map[string]string{
							util.ServiceNamespaceLabel:           "default",
							util.ServiceNameLabel:                "test-service",
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
							util.PropagationInstruction:          util.PropagationInstructionSuppressed,
						},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{RawExtension: runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"test-service","namespace":"default"}}`)}},
							},
						},
					},
				},
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work2",
						Namespace: names.GenerateExecutionSpaceName("cluster1"),
						Labels: map[string]string{
							util.ServiceNamespaceLabel:           "default",
							util.ServiceNameLabel:                "test-service",
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
							util.PropagationInstruction:          util.PropagationInstructionSuppressed,
						},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{RawExtension: runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"test-service","namespace":"default"}}`)}},
							},
						},
					},
				},
			},
			expectedDeleted: 2,
			expectedError:   false,
		},
		{
			name: "no works to delete",
			serviceExportKey: keys.FederatedKey{
				Cluster: "cluster1",
				ClusterWideKey: keys.ClusterWideKey{
					Namespace: "default",
					Name:      "non-existent-service",
				},
			},
			existingWorks:   []runtime.Object{},
			expectedDeleted: 0,
			expectedError:   false,
		},
		{
			name: "partial deletion due to error",
			serviceExportKey: keys.FederatedKey{
				Cluster: "cluster1",
				ClusterWideKey: keys.ClusterWideKey{
					Namespace: "default",
					Name:      "test-service",
				},
			},
			existingWorks: []runtime.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work1",
						Namespace: names.GenerateExecutionSpaceName("cluster1"),
						Labels: map[string]string{
							util.ServiceNamespaceLabel:           "default",
							util.ServiceNameLabel:                "test-service",
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
							util.PropagationInstruction:          util.PropagationInstructionSuppressed,
						},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{RawExtension: runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"test-service","namespace":"default"}}`)}},
							},
						},
					},
				},
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work2",
						Namespace: names.GenerateExecutionSpaceName("cluster1"),
						Labels: map[string]string{
							util.ServiceNamespaceLabel:           "default",
							util.ServiceNameLabel:                "test-service",
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
							util.PropagationInstruction:          util.PropagationInstructionSuppressed,
						},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{RawExtension: runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"test-service","namespace":"default"}}`)}},
							},
						},
					},
				},
			},
			expectedDeleted: 1,
			expectedError:   true,
			deleteError:     errors.New("failed to delete work2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			s.AddKnownTypes(workv1alpha1.SchemeGroupVersion, &workv1alpha1.Work{}, &workv1alpha1.WorkList{})
			s.AddKnownTypes(mcsv1alpha1.SchemeGroupVersion, &mcsv1alpha1.ServiceExport{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(tt.existingWorks...).Build()

			serviceExport := &mcsv1alpha1.ServiceExport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.serviceExportKey.Name,
					Namespace: tt.serviceExportKey.Namespace,
				},
			}
			err := fakeClient.Create(context.TODO(), serviceExport)
			assert.NoError(t, err)

			var testClient client.Client = fakeClient
			if tt.deleteError != nil {
				testClient = &fakeClientWithErrors{
					Client:      fakeClient,
					deleteError: tt.deleteError,
				}
			}

			err = cleanupWorkWithServiceExportDelete(context.TODO(), testClient, tt.serviceExportKey)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			workList := &workv1alpha1.WorkList{}
			err = fakeClient.List(context.TODO(), workList, &client.ListOptions{
				Namespace: names.GenerateExecutionSpaceName(tt.serviceExportKey.Cluster),
				LabelSelector: labels.SelectorFromSet(labels.Set{
					util.ServiceNamespaceLabel: tt.serviceExportKey.Namespace,
					util.ServiceNameLabel:      tt.serviceExportKey.Name,
				}),
			})
			assert.NoError(t, err)

			deletedCount := len(tt.existingWorks) - len(workList.Items)
			assert.Equal(t, tt.expectedDeleted, deletedCount)

			// For partial deletion, we expect one work to remain
			if tt.name == "partial deletion due to error" {
				assert.Equal(t, 1, len(workList.Items))
				assert.Equal(t, "work2", workList.Items[0].Name)
			} else {
				// Verify that the remaining works are not the ones that should have been deleted
				for _, work := range workList.Items {
					assert.NotEqual(t, tt.serviceExportKey.Name, work.Labels[util.ServiceNameLabel])
					assert.NotEqual(t, tt.serviceExportKey.Namespace, work.Labels[util.ServiceNamespaceLabel])
				}
			}
		})
	}
}

func TestCleanupWorkWithEndpointSliceDelete(t *testing.T) {
	tests := []struct {
		name              string
		endpointSliceKey  keys.FederatedKey
		existingWork      *workv1alpha1.Work
		expectedWorkExist bool
		expectedError     bool
		clientError       error
	}{
		{
			name: "delete existing work",
			endpointSliceKey: keys.FederatedKey{
				Cluster: "cluster1",
				ClusterWideKey: keys.ClusterWideKey{
					Kind:      "EndpointSlice",
					Namespace: "default",
					Name:      "test-eps",
				},
			},
			existingWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.GenerateWorkName("EndpointSlice", "test-eps", "default"),
					Namespace: names.GenerateExecutionSpaceName("cluster1"),
					Labels: map[string]string{
						util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
						util.ServiceNamespaceLabel:           "default",
						util.ServiceNameLabel:                "test-service",
						util.PropagationInstruction:          util.PropagationInstructionSuppressed,
					},
				},
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{
						Manifests: []workv1alpha1.Manifest{
							{RawExtension: runtime.RawExtension{Raw: []byte(`{"apiVersion":"discovery.k8s.io/v1","kind":"EndpointSlice"}`)}},
						},
					},
				},
			},
			expectedWorkExist: false,
			expectedError:     false,
		},
		{
			name: "work does not exist",
			endpointSliceKey: keys.FederatedKey{
				Cluster: "cluster1",
				ClusterWideKey: keys.ClusterWideKey{
					Kind:      "EndpointSlice",
					Namespace: "default",
					Name:      "non-existent-eps",
				},
			},
			existingWork:      nil,
			expectedWorkExist: false,
			expectedError:     false,
		},
		{
			name: "error getting work",
			endpointSliceKey: keys.FederatedKey{
				Cluster: "cluster1",
				ClusterWideKey: keys.ClusterWideKey{
					Kind:      "EndpointSlice",
					Namespace: "default",
					Name:      "test-eps",
				},
			},
			existingWork:      nil,
			expectedWorkExist: false,
			expectedError:     true,
			clientError:       errors.New("failed to get work"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			s.AddKnownTypes(workv1alpha1.SchemeGroupVersion, &workv1alpha1.Work{})

			objs := []runtime.Object{}
			if tt.existingWork != nil {
				objs = append(objs, tt.existingWork)
			}

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()

			var testClient client.Client = fakeClient
			if tt.clientError != nil {
				testClient = &fakeClientWithErrors{
					Client:   fakeClient,
					getError: tt.clientError,
				}
			}

			err := cleanupWorkWithEndpointSliceDelete(context.TODO(), testClient, tt.endpointSliceKey)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			work := &workv1alpha1.Work{}
			err = fakeClient.Get(context.TODO(), client.ObjectKey{
				Namespace: names.GenerateExecutionSpaceName(tt.endpointSliceKey.Cluster),
				Name:      names.GenerateWorkName(tt.endpointSliceKey.Kind, tt.endpointSliceKey.Name, tt.endpointSliceKey.Namespace),
			}, work)
			if tt.expectedWorkExist {
				assert.NoError(t, err)
			} else {
				assert.True(t, apierrors.IsNotFound(err))
			}
		})
	}
}

func TestCleanEndpointSliceWork(t *testing.T) {
	tests := []struct {
		name           string
		existingWork   *workv1alpha1.Work
		expectedResult *workv1alpha1.Work
		expectedError  bool
	}{
		{
			name: "work managed by multiple controllers",
			existingWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "member-1",
					Labels: map[string]string{
						util.ServiceNameLabel:                "test-service",
						util.ServiceNamespaceLabel:           "default",
						util.EndpointSliceWorkManagedByLabel: "ServiceExport.OtherController",
					},
				},
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{
						Manifests: []workv1alpha1.Manifest{
							{RawExtension: runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"Service"}`)}},
						},
					},
				},
			},
			expectedResult: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "member-1",
					Labels: map[string]string{
						util.EndpointSliceWorkManagedByLabel: "OtherController",
					},
				},
			},
		},
		{
			name: "work managed only by ServiceExport",
			existingWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "member-1",
					Labels: map[string]string{
						util.ServiceNameLabel:                "test-service",
						util.ServiceNamespaceLabel:           "default",
						util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
					},
				},
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{
						Manifests: []workv1alpha1.Manifest{
							{RawExtension: runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"Service"}`)}},
						},
					},
				},
			},
			expectedResult: nil, // Work should be deleted
		},
		{
			name: "work managed by ServiceExport and multiple other controllers",
			existingWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "member-1",
					Labels: map[string]string{
						util.ServiceNameLabel:                "test-service",
						util.ServiceNamespaceLabel:           "default",
						util.EndpointSliceWorkManagedByLabel: "ServiceExport.OtherController1.OtherController2",
					},
				},
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{
						Manifests: []workv1alpha1.Manifest{
							{RawExtension: runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"Service"}`)}},
						},
					},
				},
			},
			expectedResult: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "member-1",
					Labels: map[string]string{
						util.EndpointSliceWorkManagedByLabel: "OtherController1.OtherController2",
					},
				},
			},
		},
		{
			name: "work with malformed controller label",
			existingWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "member-1",
					Labels: map[string]string{
						util.ServiceNameLabel:                "test-service",
						util.ServiceNamespaceLabel:           "default",
						util.EndpointSliceWorkManagedByLabel: "ServiceExport..",
					},
				},
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{
						Manifests: []workv1alpha1.Manifest{
							{RawExtension: runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"Service"}`)}},
						},
					},
				},
			},
			expectedResult: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "member-1",
					Labels: map[string]string{
						util.EndpointSliceWorkManagedByLabel: "",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			s := scheme.Scheme
			s.AddKnownTypes(workv1alpha1.SchemeGroupVersion, &workv1alpha1.Work{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(tt.existingWork).Build()

			err := cleanEndpointSliceWork(ctx, fakeClient, tt.existingWork)

			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none")
			} else {
				assert.NoError(t, err, "Expected no error but got one")
			}

			if tt.expectedResult == nil {
				// Check if the work was deleted
				updatedWork := &workv1alpha1.Work{}
				err = fakeClient.Get(ctx, client.ObjectKey{Namespace: tt.existingWork.Namespace, Name: tt.existingWork.Name}, updatedWork)
				assert.True(t, apierrors.IsNotFound(err), "Expected work to be deleted, but it still exists")
			} else {
				// Check if the work was updated correctly
				updatedWork := &workv1alpha1.Work{}
				err = fakeClient.Get(ctx, client.ObjectKey{Namespace: tt.existingWork.Namespace, Name: tt.existingWork.Name}, updatedWork)
				assert.NoError(t, err, "Failed to get updated work")

				assert.Equal(t, len(tt.expectedResult.Labels), len(updatedWork.Labels), "Number of labels should match")

				for key, expectedValue := range tt.expectedResult.Labels {
					actualValue, exists := updatedWork.Labels[key]
					assert.True(t, exists, "Expected label %s to exist", key)
					assert.Equal(t, sortControllers(expectedValue), sortControllers(actualValue), "Label %s should match expected value", key)
				}

				assert.NotContains(t, updatedWork.Labels, util.ServiceNameLabel, "ServiceNameLabel should be removed")
				assert.NotContains(t, updatedWork.Labels, util.ServiceNamespaceLabel, "ServiceNamespaceLabel should be removed")

				managedBy, hasManagedBy := updatedWork.Labels[util.EndpointSliceWorkManagedByLabel]
				if hasManagedBy {
					assert.NotContains(t, managedBy, util.ServiceExportKind, "ManagedBy label should not contain ServiceExport")
				}
			}
		})
	}
}

// Helper function to setup scheme
func setupScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := workv1alpha1.Install(scheme); err != nil {
		return nil, err
	}
	if err := discoveryv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := mcsv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	return scheme, nil
}

// Helper function to create a new unstructured EndpointSlice
func newUnstructuredEndpointSlice(name, namespace, serviceName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "discovery.k8s.io/v1",
			"kind":       "EndpointSlice",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					discoveryv1.LabelServiceName: serviceName,
				},
			},
		},
	}
}

// Helper function to create a new Work object
func newWork(name, namespace, serviceName string) *workv1alpha1.Work {
	return &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: names.GenerateExecutionSpaceName("member1"),
			Labels: map[string]string{
				util.ServiceNamespaceLabel:           namespace,
				util.ServiceNameLabel:                serviceName,
				util.PropagationInstruction:          util.PropagationInstructionSuppressed,
				util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
			},
		},
	}
}

// Helper function to sort controllers in a label
func sortControllers(controllers string) string {
	if controllers == "" {
		return ""
	}
	split := strings.Split(controllers, ".")
	sort.Strings(split)
	return strings.Join(split, ".")
}

// mockInformerManager simulates the behavior of an InformerManager
type mockInformerManager struct {
	singleClusterManager genericmanager.SingleClusterInformerManager
}

func (m *mockInformerManager) GetSingleClusterManager(_ string) genericmanager.SingleClusterInformerManager {
	return m.singleClusterManager
}

func (m *mockInformerManager) ForCluster(_ string, _ dynamic.Interface, _ time.Duration) genericmanager.SingleClusterInformerManager {
	return nil
}

func (m *mockInformerManager) Start(_ string) {}

func (m *mockInformerManager) Stop(_ string) {}

func (m *mockInformerManager) WaitForCacheSync(_ string) map[schema.GroupVersionResource]bool {
	return nil
}

func (m *mockInformerManager) WaitForCacheSyncWithTimeout(_ string, _ time.Duration) map[schema.GroupVersionResource]bool {
	return nil
}

func (m *mockInformerManager) IsManagerExist(_ string) bool {
	return false
}

// mockAsyncWorker is a mock implementation of util.AsyncWorker
type mockAsyncWorker struct {
	addedItems      []interface{}
	addedAfterItems []interface{}
	enqueuedItems   []interface{}
}

func (m *mockAsyncWorker) Add(item interface{}) {
	m.addedItems = append(m.addedItems, item)
}

// AddAfter simulates adding an item to the worker queue after a specified duration
func (m *mockAsyncWorker) AddAfter(item interface{}, _ time.Duration) {
	m.addedAfterItems = append(m.addedAfterItems, item)
}

// Enqueue simulates enqueueing an object in the worker
func (m *mockAsyncWorker) Enqueue(obj interface{}) {
	m.enqueuedItems = append(m.enqueuedItems, obj)
}

func (m *mockAsyncWorker) Run(_ int, _ <-chan struct{}) {
	// Implementation not needed for the test
}

// mockSingleClusterManager simulates the behavior of a SingleClusterManager
type mockSingleClusterManager struct {
	lister cache.GenericLister
}

func (m *mockSingleClusterManager) ForResource(_ schema.GroupVersionResource, _ cache.ResourceEventHandler) {
	// Implementation not needed for the test
}

func (m *mockSingleClusterManager) IsInformerSynced(_ schema.GroupVersionResource) bool {
	return false
}

func (m *mockSingleClusterManager) IsHandlerExist(_ schema.GroupVersionResource, _ cache.ResourceEventHandler) bool {
	return false
}

func (m *mockSingleClusterManager) Lister(_ schema.GroupVersionResource) cache.GenericLister {
	return m.lister
}

func (m *mockSingleClusterManager) Start() {}

func (m *mockSingleClusterManager) Stop() {}

func (m *mockSingleClusterManager) WaitForCacheSync() map[schema.GroupVersionResource]bool {
	return nil
}

func (m *mockSingleClusterManager) WaitForCacheSyncWithTimeout(_ time.Duration) map[schema.GroupVersionResource]bool {
	return nil
}

func (m *mockSingleClusterManager) Context() context.Context {
	return context.Background()
}

func (m *mockSingleClusterManager) GetClient() dynamic.Interface {
	return nil
}

// mockGenericLister simulates the behavior of a GenericLister
type mockGenericLister struct {
	serviceExportExists bool
	serviceExportError  error
}

func (m *mockGenericLister) List(_ labels.Selector) ([]runtime.Object, error) {
	return nil, nil
}

func (m *mockGenericLister) Get(_ string) (runtime.Object, error) {
	return nil, nil
}

// ByNamespace returns a mockGenericNamespaceLister for the given namespace
func (m *mockGenericLister) ByNamespace(_ string) cache.GenericNamespaceLister {
	return &mockGenericNamespaceLister{
		serviceExportExists: m.serviceExportExists,
		serviceExportError:  m.serviceExportError,
	}
}

// mockGenericNamespaceLister simulates the behavior of a GenericNamespaceLister
type mockGenericNamespaceLister struct {
	serviceExportExists bool
	serviceExportError  error
}

func (m *mockGenericNamespaceLister) List(_ labels.Selector) ([]runtime.Object, error) {
	return nil, nil
}

// Get simulates retrieving a ServiceExport object by name
func (m *mockGenericNamespaceLister) Get(name string) (runtime.Object, error) {
	if m.serviceExportError != nil {
		return nil, m.serviceExportError
	}
	if m.serviceExportExists {
		return &unstructured.Unstructured{}, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: "mcs.karmada.io", Resource: "serviceexports"}, name)
}

// fakeClientWithErrors is a mock client that can return specified errors for testing error scenarios
type fakeClientWithErrors struct {
	client.Client
	deleteError error
	getError    error
}

// Delete simulates deleting an object, potentially returning a specified error
func (f *fakeClientWithErrors) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if f.deleteError != nil && obj.GetName() == "work2" {
		return f.deleteError
	}
	return f.Client.Delete(ctx, obj, opts...)
}

// Get simulates retrieving an object, potentially returning a specified error
func (f *fakeClientWithErrors) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if f.getError != nil {
		return f.getError
	}
	return f.Client.Get(ctx, key, obj, opts...)
}
