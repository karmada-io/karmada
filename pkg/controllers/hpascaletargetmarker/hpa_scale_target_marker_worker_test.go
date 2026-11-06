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

package hpascaletargetmarker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/restmapper"
	coretesting "k8s.io/client-go/testing"

	"github.com/karmada-io/karmada/pkg/util"
)

// fakeCachedDiscoveryClient implements CachedDiscoveryInterface
type fakeCachedDiscoveryClient struct {
	*fakediscovery.FakeDiscovery
}

func (d *fakeCachedDiscoveryClient) Fresh() bool {
	return true
}

func (d *fakeCachedDiscoveryClient) Invalidate() {
	// Do nothing
}

// newFakeCachedDiscoveryClient creates a new fake CachedDiscoveryInterface
func newFakeCachedDiscoveryClient() discovery.CachedDiscoveryInterface {
	return &fakeCachedDiscoveryClient{
		FakeDiscovery: &fakediscovery.FakeDiscovery{
			Fake: &coretesting.Fake{},
		},
	}
}

// Helper function to set up common test resources
func setupTestResources(_ *testing.T) (*fake.FakeDynamicClient, *restmapper.DeferredDiscoveryRESTMapper) {
	fakeDiscovery := newFakeCachedDiscoveryClient()
	fakeDiscovery.(*fakeCachedDiscoveryClient).Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment"},
			},
		},
	}

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, &unstructured.Unstructured{})
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(fakeDiscovery)

	return dynamicClient, restMapper
}

func TestReconcileScaleRef(t *testing.T) {
	tests := []struct {
		name        string
		key         util.QueueKey
		setupMocks  func(*fake.FakeDynamicClient)
		expectError bool
	}{
		{
			name: "Add label event",
			key: labelEvent{
				kind: addLabelEvent,
				hpa: &autoscalingv2.HorizontalPodAutoscaler{
					Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Delete label event",
			key: labelEvent{
				kind: deleteLabelEvent,
				hpa: &autoscalingv2.HorizontalPodAutoscaler{
					Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "Invalid key",
			key:         "invalid-key",
			expectError: false,
		},
		{
			name: "Error in add label",
			key: labelEvent{
				kind: addLabelEvent,
				hpa: &autoscalingv2.HorizontalPodAutoscaler{
					Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "non-existent",
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dynamicClient, restMapper := setupTestResources(t)

			if tt.setupMocks != nil {
				tt.setupMocks(dynamicClient)
			}

			marker := &HpaScaleTargetMarker{
				DynamicClient: dynamicClient,
				RESTMapper:    restMapper,
			}

			err := marker.reconcileScaleRef(tt.key)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAddHPALabelToScaleRef(t *testing.T) {
	tests := []struct {
		name        string
		hpa         *autoscalingv2.HorizontalPodAutoscaler
		scaleRef    *unstructured.Unstructured
		expectError bool
	}{
		{
			name: "Successfully add label",
			hpa: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hpa",
					Namespace: "default",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
					},
				},
			},
			scaleRef: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dynamicClient, restMapper := setupTestResources(t)

			// Add the scale reference to the client if it exists
			if tt.scaleRef != nil {
				dynamicClient = fake.NewSimpleDynamicClient(scheme.Scheme, tt.scaleRef)
			}

			marker := &HpaScaleTargetMarker{
				DynamicClient: dynamicClient,
				RESTMapper:    restMapper,
			}

			err := marker.addHPALabelToScaleRef(context.TODO(), tt.hpa)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify that the label was added
				updatedScaleRef, err := dynamicClient.Resource(schema.GroupVersionResource{
					Group:    "apps",
					Version:  "v1",
					Resource: "deployments",
				}).Namespace("default").Get(context.TODO(), "test-deployment", metav1.GetOptions{})
				require.NoError(t, err)
				labels := updatedScaleRef.GetLabels()
				assert.Contains(t, labels, util.RetainReplicasLabel)
				assert.Equal(t, util.RetainReplicasValue, labels[util.RetainReplicasLabel])
			}
		})
	}
}

func TestDeleteHPALabelFromScaleRef(t *testing.T) {
	tests := []struct {
		name        string
		hpa         *autoscalingv2.HorizontalPodAutoscaler
		scaleRef    *unstructured.Unstructured
		expectError bool
	}{
		{
			name: "Successfully delete label",
			hpa: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hpa",
					Namespace: "default",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
					},
				},
			},
			scaleRef: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
						"labels": map[string]interface{}{
							util.RetainReplicasLabel: util.RetainReplicasValue,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Label already removed",
			hpa: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hpa",
					Namespace: "default",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
					},
				},
			},
			scaleRef: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			expectError: false,
		},
		{
			name: "Scale reference not found",
			hpa: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hpa",
					Namespace: "default",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "non-existent-deployment",
					},
				},
			},
			scaleRef:    nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dynamicClient, restMapper := setupTestResources(t)

			// Add the scale reference to the client if it exists
			if tt.scaleRef != nil {
				dynamicClient = fake.NewSimpleDynamicClient(scheme.Scheme, tt.scaleRef)
			}

			marker := &HpaScaleTargetMarker{
				DynamicClient: dynamicClient,
				RESTMapper:    restMapper,
			}

			err := marker.deleteHPALabelFromScaleRef(context.TODO(), tt.hpa)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if tt.scaleRef != nil {
					// Verify that the label was removed
					updatedScaleRef, err := dynamicClient.Resource(schema.GroupVersionResource{
						Group:    "apps",
						Version:  "v1",
						Resource: "deployments",
					}).Namespace("default").Get(context.TODO(), tt.hpa.Spec.ScaleTargetRef.Name, metav1.GetOptions{})

					if err == nil {
						labels := updatedScaleRef.GetLabels()
						assert.NotContains(t, labels, util.RetainReplicasLabel)
					} else {
						assert.True(t, tt.scaleRef == nil, "Error getting updated scale ref: %v", err)
					}
				}
			}
		})
	}
}
