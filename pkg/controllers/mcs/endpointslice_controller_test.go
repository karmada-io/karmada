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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestRemoveFinalizer(t *testing.T) {
	testScheme := runtime.NewScheme()
	err := scheme.AddToScheme(testScheme)
	require.NoError(t, err)
	err = workv1alpha1.Install(testScheme)
	require.NoError(t, err)

	testCases := []struct {
		name            string
		work            *workv1alpha1.Work
		expectFinalizer bool
	}{
		{
			name: "Remove existing finalizer",
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-work",
					Namespace:  "default",
					Finalizers: []string{util.EndpointSliceControllerFinalizer},
				},
			},
			expectFinalizer: false,
		},
		{
			name: "No finalizer to remove",
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "default",
				},
			},
			expectFinalizer: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(tc.work).Build()
			controller := &EndpointSliceController{Client: fakeClient}

			err := controller.removeFinalizer(context.Background(), tc.work)
			require.NoError(t, err)

			updatedWork := &workv1alpha1.Work{}
			err = fakeClient.Get(context.Background(), client.ObjectKey{Namespace: tc.work.Namespace, Name: tc.work.Name}, updatedWork)
			require.NoError(t, err)

			hasFinalizer := containsFinalizer(updatedWork.Finalizers, util.EndpointSliceControllerFinalizer)
			assert.Equal(t, tc.expectFinalizer, hasFinalizer)
		})
	}
}

func TestDeriveEndpointSlice(t *testing.T) {
	testCases := []struct {
		name         string
		original     *discoveryv1.EndpointSlice
		migratedFrom string
		expected     *discoveryv1.EndpointSlice
	}{
		{
			name: "Basic derivation",
			original: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "original-slice",
					Namespace: "default",
					Labels: map[string]string{
						"key": "value",
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"192.168.0.1"},
					},
				},
			},
			migratedFrom: "cluster1",
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.GenerateEndpointSliceName("original-slice", "cluster1"),
					Namespace: "default",
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"192.168.0.1"},
					},
				},
			},
		},
		{
			name: "Derivation with empty original name",
			original: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "kube-system",
				},
			},
			migratedFrom: "cluster2",
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.GenerateEndpointSliceName("", "cluster2"),
					Namespace: "kube-system",
				},
			},
		},
		{
			name: "Derivation with non-empty labels and annotations",
			original: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "original-slice",
					Namespace:   "default",
					Labels:      map[string]string{"key": "value"},
					Annotations: map[string]string{"anno": "value"},
				},
				AddressType: discoveryv1.AddressTypeIPv6,
			},
			migratedFrom: "cluster3",
			expected: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.GenerateEndpointSliceName("original-slice", "cluster3"),
					Namespace: "default",
				},
				AddressType: discoveryv1.AddressTypeIPv6,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := deriveEndpointSlice(tc.original, tc.migratedFrom)
			assert.Equal(t, tc.expected.Name, result.Name)
			assert.Equal(t, tc.expected.Namespace, result.Namespace)
			assert.Equal(t, tc.expected.AddressType, result.AddressType)
			assert.Equal(t, tc.expected.Endpoints, result.Endpoints)
			assert.Empty(t, result.Labels)
			assert.Empty(t, result.Annotations)
		})
	}
}

func TestCollectEndpointSliceFromWork(t *testing.T) {
	// Set up the scheme
	testScheme := runtime.NewScheme()
	err := scheme.AddToScheme(testScheme)
	require.NoError(t, err)
	err = workv1alpha1.Install(testScheme)
	require.NoError(t, err)
	err = discoveryv1.AddToScheme(testScheme)
	require.NoError(t, err)

	testCases := []struct {
		name           string
		work           *workv1alpha1.Work
		expectedSlices int
		expectedError  bool
	}{
		{
			name: "Collect single EndpointSlice",
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "karmada-es-member-cluster1",
					Labels: map[string]string{
						util.ServiceNameLabel:             "test-service",
						workv1alpha2.WorkPermanentIDLabel: "test-id",
					},
				},
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{
						Manifests: []workv1alpha1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: mustMarshal(t, &discoveryv1.EndpointSlice{
										TypeMeta: metav1.TypeMeta{
											Kind:       "EndpointSlice",
											APIVersion: "discovery.k8s.io/v1",
										},
										ObjectMeta: metav1.ObjectMeta{
											Name:      "test-es",
											Namespace: "default",
										},
										AddressType: discoveryv1.AddressTypeIPv4,
										Endpoints: []discoveryv1.Endpoint{
											{
												Addresses: []string{"192.168.0.1"},
											},
										},
									}),
								},
							},
						},
					},
				},
			},
			expectedSlices: 1,
			expectedError:  false,
		},
		{
			name: "Invalid manifest",
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-work",
					Namespace: "karmada-es-member-cluster1",
					Labels: map[string]string{
						util.ServiceNameLabel:             "test-service",
						workv1alpha2.WorkPermanentIDLabel: "test-id",
					},
				},
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{
						Manifests: []workv1alpha1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: []byte("invalid json"),
								},
							},
						},
					},
				},
			},
			expectedSlices: 0,
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(tc.work).Build()
			controller := &EndpointSliceController{Client: fakeClient}

			err := controller.collectEndpointSliceFromWork(context.Background(), tc.work)

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				endpointSliceList := &discoveryv1.EndpointSliceList{}
				labelSelector := labels.SelectorFromSet(labels.Set{
					workv1alpha2.WorkPermanentIDLabel: tc.work.Labels[workv1alpha2.WorkPermanentIDLabel],
				})
				err = fakeClient.List(context.Background(), endpointSliceList, &client.ListOptions{
					LabelSelector: labelSelector,
				})
				require.NoError(t, err)

				assert.Len(t, endpointSliceList.Items, tc.expectedSlices)

				if tc.expectedSlices > 0 && len(endpointSliceList.Items) > 0 {
					endpointSlice := endpointSliceList.Items[0]
					assert.Equal(t, names.GenerateDerivedServiceName(tc.work.Labels[util.ServiceNameLabel]), endpointSlice.Labels[discoveryv1.LabelServiceName])
					assert.Equal(t, tc.work.Namespace, endpointSlice.Annotations[workv1alpha2.WorkNamespaceAnnotation])
					assert.Equal(t, tc.work.Name, endpointSlice.Annotations[workv1alpha2.WorkNameAnnotation])
				}
			}
		})
	}
}

//Helper functions

func mustMarshal(t *testing.T, obj interface{}) []byte {
	data, err := json.Marshal(obj)
	require.NoError(t, err)
	return data
}

func containsFinalizer(finalizers []string, finalizer string) bool {
	for _, f := range finalizers {
		if f == finalizer {
			return true
		}
	}
	return false
}
