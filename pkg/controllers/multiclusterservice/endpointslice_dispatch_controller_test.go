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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

func TestUpdateEndpointSliceDispatched(t *testing.T) {
	tests := []struct {
		name              string
		mcs               *networkingv1alpha1.MultiClusterService
		status            metav1.ConditionStatus
		reason            string
		message           string
		expectedCondition metav1.Condition
	}{
		{
			name: "update status to true",
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
				},
			},
			status:  metav1.ConditionTrue,
			reason:  "EndpointSliceDispatchedSucceed",
			message: "EndpointSlice are dispatched successfully",
			expectedCondition: metav1.Condition{
				Type:    networkingv1alpha1.EndpointSliceDispatched,
				Status:  metav1.ConditionTrue,
				Reason:  "EndpointSliceDispatchedSucceed",
				Message: "EndpointSlice are dispatched successfully",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockClient)
			mockStatusWriter := new(MockStatusWriter)

			// Expectations Setup
			mockClient.On("Status").Return(mockStatusWriter)
			mockClient.On("Get", mock.Anything, mock.AnythingOfType("types.NamespacedName"), mock.AnythingOfType("*v1alpha1.MultiClusterService"), mock.Anything).
				Run(func(args mock.Arguments) {
					arg := args.Get(2).(*networkingv1alpha1.MultiClusterService)
					*arg = *tt.mcs // Copy the input MCS to the output
				}).Return(nil)

			mockStatusWriter.On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.MultiClusterService"), mock.Anything).
				Run(func(args mock.Arguments) {
					mcs := args.Get(1).(*networkingv1alpha1.MultiClusterService)
					mcs.Status.Conditions = []metav1.Condition{tt.expectedCondition}
				}).Return(nil)

			c := &EndpointsliceDispatchController{
				Client:        mockClient,
				EventRecorder: record.NewFakeRecorder(100),
			}

			err := c.updateEndpointSliceDispatched(context.Background(), tt.mcs, tt.status, tt.reason, tt.message)
			assert.NoError(t, err, "updateEndpointSliceDispatched should not return an error")

			mockClient.AssertExpectations(t)
			mockStatusWriter.AssertExpectations(t)

			assert.Len(t, tt.mcs.Status.Conditions, 1, "MCS should have one condition")
			if len(tt.mcs.Status.Conditions) > 0 {
				condition := tt.mcs.Status.Conditions[0]
				assert.Equal(t, tt.expectedCondition.Type, condition.Type)
				assert.Equal(t, tt.expectedCondition.Status, condition.Status)
				assert.Equal(t, tt.expectedCondition.Reason, condition.Reason)
				assert.Equal(t, tt.expectedCondition.Message, condition.Message)
			}
		})
	}
}

func TestNewClusterFunc(t *testing.T) {
	tests := []struct {
		name           string
		existingObjs   []client.Object
		inputObj       client.Object
		expectedResult []reconcile.Request
	}{
		{
			name: "new cluster, matching MCS",
			existingObjs: []client.Object{
				&networkingv1alpha1.MultiClusterService{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-mcs",
						Namespace: "default",
					},
					Spec: networkingv1alpha1.MultiClusterServiceSpec{
						ConsumerClusters: []networkingv1alpha1.ClusterSelector{
							{Name: "cluster1"},
						},
					},
				},
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-work",
						Namespace: "karmada-es-cluster1",
						Labels: map[string]string{
							util.MultiClusterServiceNameLabel:      "test-mcs",
							util.MultiClusterServiceNamespaceLabel: "default",
						},
					},
				},
			},
			inputObj: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
			},
			expectedResult: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: "karmada-es-cluster1", Name: "test-work"}},
			},
		},
		{
			name: "new cluster, no matching MCS",
			existingObjs: []client.Object{
				&networkingv1alpha1.MultiClusterService{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-mcs",
						Namespace: "default",
					},
					Spec: networkingv1alpha1.MultiClusterServiceSpec{
						ConsumerClusters: []networkingv1alpha1.ClusterSelector{
							{Name: "cluster2"},
						},
					},
				},
			},
			inputObj: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
			},
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := setupController(tt.existingObjs...)
			result := c.newClusterFunc()(context.Background(), tt.inputObj)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetClusterEndpointSliceWorks(t *testing.T) {
	tests := []struct {
		name          string
		existingObjs  []client.Object
		mcsNamespace  string
		mcsName       string
		expectedWorks int
		expectedError bool
		listError     error
	}{
		{
			name: "find matching works",
			existingObjs: []client.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work1",
						Namespace: "karmada-es-cluster1",
						Labels: map[string]string{
							util.MultiClusterServiceNameLabel:      "test-mcs",
							util.MultiClusterServiceNamespaceLabel: "default",
						},
					},
				},
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work2",
						Namespace: "karmada-es-cluster2",
						Labels: map[string]string{
							util.MultiClusterServiceNameLabel:      "test-mcs",
							util.MultiClusterServiceNamespaceLabel: "default",
						},
					},
				},
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work3",
						Namespace: "karmada-es-cluster3",
						Labels: map[string]string{
							util.MultiClusterServiceNameLabel:      "other-mcs",
							util.MultiClusterServiceNamespaceLabel: "default",
						},
					},
				},
			},
			mcsNamespace:  "default",
			mcsName:       "test-mcs",
			expectedWorks: 2,
			expectedError: false,
		},
		{
			name: "no matching works",
			existingObjs: []client.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work1",
						Namespace: "karmada-es-cluster1",
						Labels: map[string]string{
							util.MultiClusterServiceNameLabel:      "other-mcs",
							util.MultiClusterServiceNamespaceLabel: "default",
						},
					},
				},
			},
			mcsNamespace:  "default",
			mcsName:       "test-mcs",
			expectedWorks: 0,
			expectedError: false,
		},
		{
			name: "works in different namespace",
			existingObjs: []client.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work1",
						Namespace: "karmada-es-cluster1",
						Labels: map[string]string{
							util.MultiClusterServiceNameLabel:      "test-mcs",
							util.MultiClusterServiceNamespaceLabel: "test-namespace",
						},
					},
				},
			},
			mcsNamespace:  "test-namespace",
			mcsName:       "test-mcs",
			expectedWorks: 1,
			expectedError: false,
		},
		{
			name:          "list error",
			existingObjs:  []client.Object{},
			mcsNamespace:  "default",
			mcsName:       "test-mcs",
			expectedWorks: 0,
			expectedError: true,
			listError:     errors.New("fake list error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := setupController(tt.existingObjs...)
			if tt.listError != nil {
				c.Client = &fakeClient{Client: c.Client, listError: tt.listError}
			}
			works, err := c.getClusterEndpointSliceWorks(context.Background(), tt.mcsNamespace, tt.mcsName)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, works)
			} else {
				assert.NoError(t, err)
				assert.Len(t, works, tt.expectedWorks)
				for _, work := range works {
					assert.Equal(t, tt.mcsName, work.Labels[util.MultiClusterServiceNameLabel])
					assert.Equal(t, tt.mcsNamespace, work.Labels[util.MultiClusterServiceNamespaceLabel])
				}
			}
		})
	}
}

func TestNewMultiClusterServiceFunc(t *testing.T) {
	tests := []struct {
		name           string
		existingObjs   []client.Object
		inputObj       client.Object
		expectedResult []reconcile.Request
	}{
		{
			name: "MCS with matching works",
			existingObjs: []client.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-work-1",
						Namespace: "karmada-es-cluster1",
						Labels: map[string]string{
							util.MultiClusterServiceNameLabel:      "test-mcs",
							util.MultiClusterServiceNamespaceLabel: "default",
						},
					},
				},
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-work-2",
						Namespace: "karmada-es-cluster2",
						Labels: map[string]string{
							util.MultiClusterServiceNameLabel:      "test-mcs",
							util.MultiClusterServiceNamespaceLabel: "default",
						},
						Annotations: map[string]string{
							util.EndpointSliceProvisionClusterAnnotation: "cluster2",
						},
					},
				},
			},
			inputObj: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
				},
			},
			expectedResult: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: "karmada-es-cluster1", Name: "test-work-1"}},
			},
		},
		{
			name: "MCS with no matching works",
			existingObjs: []client.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-work",
						Namespace: "karmada-es-cluster1",
						Labels: map[string]string{
							util.MultiClusterServiceNameLabel:      "other-mcs",
							util.MultiClusterServiceNamespaceLabel: "default",
						},
					},
				},
			},
			inputObj: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
				},
			},
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := setupController(tt.existingObjs...)
			result := c.newMultiClusterServiceFunc()(context.Background(), tt.inputObj)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestCleanOrphanDispatchedEndpointSlice(t *testing.T) {
	tests := []struct {
		name            string
		existingObjs    []client.Object
		mcs             *networkingv1alpha1.MultiClusterService
		expectedDeletes int
		expectedError   bool
	}{
		{
			name: "clean orphan works",
			existingObjs: []client.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work1",
						Namespace: "karmada-es-cluster1",
						Labels: map[string]string{
							util.MultiClusterServiceNameLabel:      "test-mcs",
							util.MultiClusterServiceNamespaceLabel: "default",
						},
						Annotations: map[string]string{
							util.EndpointSliceProvisionClusterAnnotation: "provider",
						},
					},
				},
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work2",
						Namespace: "karmada-es-cluster2",
						Labels: map[string]string{
							util.MultiClusterServiceNameLabel:      "test-mcs",
							util.MultiClusterServiceNamespaceLabel: "default",
						},
						Annotations: map[string]string{
							util.EndpointSliceProvisionClusterAnnotation: "provider",
						},
					},
				},
			},
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
				},
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "cluster1"},
					},
				},
			},
			expectedDeletes: 1,
			expectedError:   false,
		},
		{
			name: "no orphan works",
			existingObjs: []client.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work1",
						Namespace: "karmada-es-cluster1",
						Labels: map[string]string{
							util.MultiClusterServiceNameLabel:      "test-mcs",
							util.MultiClusterServiceNamespaceLabel: "default",
						},
						Annotations: map[string]string{
							util.EndpointSliceProvisionClusterAnnotation: "provider",
						},
					},
				},
			},
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
				},
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "cluster1"},
					},
				},
			},
			expectedDeletes: 0,
			expectedError:   false,
		},
		{
			name: "work without provision cluster annotation",
			existingObjs: []client.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work1",
						Namespace: "karmada-es-cluster1",
						Labels: map[string]string{
							util.MultiClusterServiceNameLabel:      "test-mcs",
							util.MultiClusterServiceNamespaceLabel: "default",
						},
					},
				},
			},
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
				},
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "cluster2"},
					},
				},
			},
			expectedDeletes: 0,
			expectedError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupSchemeEndpointDispatch()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.existingObjs...).Build()
			c := &EndpointsliceDispatchController{
				Client: fakeClient,
			}
			err := c.cleanOrphanDispatchedEndpointSlice(context.Background(), tt.mcs)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Check if the expected number of works were deleted
				remainingWorks := &workv1alpha1.WorkList{}
				err = fakeClient.List(context.Background(), remainingWorks, &client.ListOptions{})
				assert.NoError(t, err)
				assert.Len(t, remainingWorks.Items, len(tt.existingObjs)-tt.expectedDeletes)
			}
		})
	}
}

func TestEnsureEndpointSliceWork(t *testing.T) {
	tests := []struct {
		name            string
		mcs             *networkingv1alpha1.MultiClusterService
		work            *workv1alpha1.Work
		providerCluster string
		consumerCluster string
		expectedError   bool
		expectedWork    *workv1alpha1.Work
	}{
		{
			name: "create new work",
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
				},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "karmada-es-provider",
				},
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{
						Manifests: []workv1alpha1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: []byte(`{
                                        "apiVersion": "discovery.k8s.io/v1",
                                        "kind": "EndpointSlice",
                                        "metadata": {
                                            "name": "test-eps"
                                        },
                                        "endpoints": [
                                            {
                                                "addresses": ["10.0.0.1"]
                                            }
                                        ],
                                        "ports": [
                                            {
                                                "port": 80
                                            }
                                        ]
                                    }`),
								},
							},
						},
					},
				},
			},
			providerCluster: "provider",
			consumerCluster: "consumer",
			expectedError:   false,
			expectedWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-work",
					Namespace:  "karmada-es-consumer",
					Finalizers: []string{util.ExecutionControllerFinalizer},
					Annotations: map[string]string{
						util.EndpointSliceProvisionClusterAnnotation: "provider",
					},
					Labels: map[string]string{
						util.MultiClusterServiceNameLabel:      "test-mcs",
						util.MultiClusterServiceNamespaceLabel: "default",
					},
				},
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{
						Manifests: []workv1alpha1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: []byte(`{
                                        "apiVersion": "discovery.k8s.io/v1",
                                        "kind": "EndpointSlice",
                                        "metadata": {
                                            "name": "provider-test-eps",
                                            "labels": {
                                                "kubernetes.io/service-name": "test-mcs",
                                                "endpointslice.kubernetes.io/managed-by": "endpointslice-dispatch-controller.karmada.io",
                                                "karmada.io/managed": "true"
                                            },
                                            "annotations": {
                                                "endpointslice.karmada.io/provision-cluster": "provider",
                                                "work.karmada.io/name": "test-work",
                                                "work.karmada.io/namespace": "karmada-es-consumer",
                                                "resourcetemplate.karmada.io/uid": "",
                                                "resourcetemplate.karmada.io/managed-annotations": "endpointslice.karmada.io/provision-cluster,resourcetemplate.karmada.io/managed-annotations,resourcetemplate.karmada.io/managed-labels,resourcetemplate.karmada.io/uid,work.karmada.io/name,work.karmada.io/namespace",
                                                "resourcetemplate.karmada.io/managed-labels":"endpointslice.kubernetes.io/managed-by,karmada.io/managed,kubernetes.io/service-name"
                                            }
                                        },
                                        "endpoints": [
                                            {
                                                "addresses": ["10.0.0.1"]
                                            }
                                        ],
                                        "ports": [
                                            {
                                                "port": 80
                                            }
                                        ]
                                    }`),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "empty manifest",
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
				},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "karmada-es-provider",
				},
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{
						Manifests: []workv1alpha1.Manifest{},
					},
				},
			},
			providerCluster: "provider",
			consumerCluster: "consumer",
			expectedError:   false,
			expectedWork:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupSchemeEndpointDispatch()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			c := &EndpointsliceDispatchController{
				Client: fakeClient,
			}

			err := c.ensureEndpointSliceWork(context.Background(), tt.mcs, tt.work, tt.providerCluster, tt.consumerCluster)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if tt.expectedWork != nil {
					createdWork := &workv1alpha1.Work{}
					err = fakeClient.Get(context.Background(), types.NamespacedName{
						Name:      tt.expectedWork.Name,
						Namespace: tt.expectedWork.Namespace,
					}, createdWork)
					assert.NoError(t, err)

					assert.Equal(t, tt.expectedWork.ObjectMeta.Name, createdWork.ObjectMeta.Name)
					assert.Equal(t, tt.expectedWork.ObjectMeta.Namespace, createdWork.ObjectMeta.Namespace)
					assert.Equal(t, tt.expectedWork.ObjectMeta.Finalizers, createdWork.ObjectMeta.Finalizers)
					assert.Equal(t, tt.expectedWork.ObjectMeta.Annotations, createdWork.ObjectMeta.Annotations)
					assert.Equal(t, tt.expectedWork.ObjectMeta.Labels, createdWork.ObjectMeta.Labels)

					// Comparing manifests
					assert.Equal(t, len(tt.expectedWork.Spec.Workload.Manifests), len(createdWork.Spec.Workload.Manifests))
					if len(tt.expectedWork.Spec.Workload.Manifests) > 0 {
						expectedManifest := &unstructured.Unstructured{}
						createdManifest := &unstructured.Unstructured{}

						err = expectedManifest.UnmarshalJSON(tt.expectedWork.Spec.Workload.Manifests[0].Raw)
						assert.NoError(t, err)
						err = createdManifest.UnmarshalJSON(createdWork.Spec.Workload.Manifests[0].Raw)
						assert.NoError(t, err)

						assert.Equal(t, expectedManifest.GetName(), createdManifest.GetName())
						assert.Equal(t, expectedManifest.GetLabels(), createdManifest.GetLabels())
						assert.Equal(t, expectedManifest.GetAnnotations(), createdManifest.GetAnnotations())
					}
				} else {
					workList := &workv1alpha1.WorkList{}
					err = fakeClient.List(context.Background(), workList)
					assert.NoError(t, err)
					assert.Empty(t, workList.Items)
				}
			}
		})
	}
}

func TestCleanupEndpointSliceFromConsumerClusters(t *testing.T) {
	tests := []struct {
		name         string
		existingObjs []client.Object
		inputWork    *workv1alpha1.Work
		expectedErr  bool
	}{
		{
			name: "cleanup works in consumer clusters",
			existingObjs: []client.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-work-1",
						Namespace: "karmada-es-cluster1",
						Annotations: map[string]string{
							util.EndpointSliceProvisionClusterAnnotation: "cluster1",
						},
					},
				},
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-work-2",
						Namespace: "karmada-es-cluster2",
						Annotations: map[string]string{
							util.EndpointSliceProvisionClusterAnnotation: "cluster1",
						},
					},
				},
			},
			inputWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "karmada-es-cluster1",
					Finalizers: []string{
						util.MCSEndpointSliceDispatchControllerFinalizer,
					},
				},
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupSchemeEndpointDispatch()
			c := &EndpointsliceDispatchController{
				Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(append(tt.existingObjs, tt.inputWork)...).Build(),
			}

			err := c.cleanupEndpointSliceFromConsumerClusters(context.Background(), tt.inputWork)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Check if works are deleted
				for _, obj := range tt.existingObjs {
					work := obj.(*workv1alpha1.Work)
					err := c.Client.Get(context.Background(), types.NamespacedName{Namespace: work.Namespace, Name: work.Name}, &workv1alpha1.Work{})
					assert.True(t, client.IgnoreNotFound(err) == nil)
				}

				// Check if the finalizer is removed
				updatedWork := &workv1alpha1.Work{}
				err := c.Client.Get(context.Background(), types.NamespacedName{Namespace: tt.inputWork.Namespace, Name: tt.inputWork.Name}, updatedWork)
				assert.NoError(t, err)
				assert.NotContains(t, updatedWork.Finalizers, util.MCSEndpointSliceDispatchControllerFinalizer)
			}
		})
	}
}

// Helper Functions

// Helper function to create and configure a runtime scheme for the controller
func setupSchemeEndpointDispatch() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = networkingv1alpha1.Install(scheme)
	_ = workv1alpha1.Install(scheme)
	_ = clusterv1alpha1.Install(scheme)
	_ = discoveryv1.AddToScheme(scheme)
	return scheme
}

// Helper function to create a new EndpointsliceDispatchController with a fake client for testing
func setupController(objs ...client.Object) *EndpointsliceDispatchController {
	scheme := setupSchemeEndpointDispatch()
	return &EndpointsliceDispatchController{
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build(),
		EventRecorder: record.NewFakeRecorder(100),
	}
}

// Mock implementations

// MockClient is a mock of client.Client interface
type MockClient struct {
	mock.Mock
}

func (m *MockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	args := m.Called(ctx, key, obj, opts)
	return args.Error(0)
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := m.Called(ctx, list, opts)
	return args.Error(0)
}

func (m *MockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := m.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

func (m *MockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockClient) Status() client.StatusWriter {
	args := m.Called()
	return args.Get(0).(client.StatusWriter)
}

func (m *MockClient) SubResource(subResource string) client.SubResourceClient {
	args := m.Called(subResource)
	return args.Get(0).(client.SubResourceClient)
}

func (m *MockClient) Scheme() *runtime.Scheme {
	args := m.Called()
	return args.Get(0).(*runtime.Scheme)
}

func (m *MockClient) RESTMapper() meta.RESTMapper {
	args := m.Called()
	return args.Get(0).(meta.RESTMapper)
}

func (m *MockClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	args := m.Called(obj)
	return args.Get(0).(schema.GroupVersionKind), args.Error(1)
}

func (m *MockClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	args := m.Called(obj)
	return args.Bool(0), args.Error(1)
}

// MockStatusWriter is a mock of client.StatusWriter interface
type MockStatusWriter struct {
	mock.Mock
}

func (m *MockStatusWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	args := m.Called(ctx, obj, subResource, opts)
	return args.Error(0)
}

func (m *MockStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	args := m.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

// Custom fake client that can simulate list errors
type fakeClient struct {
	client.Client
	listError error
}

func (f *fakeClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if f.listError != nil {
		return f.listError
	}
	return f.Client.List(ctx, list, opts...)
}
