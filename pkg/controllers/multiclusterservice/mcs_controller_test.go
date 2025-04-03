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
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestHandleMultiClusterServiceDelete(t *testing.T) {
	tests := []struct {
		name                       string
		mcs                        *networkingv1alpha1.MultiClusterService
		existingService            *corev1.Service
		existingResourceBinding    *workv1alpha2.ResourceBinding
		expectedServiceLabels      map[string]string
		expectedServiceAnnotations map[string]string
		expectedRBLabels           map[string]string
		expectedRBAnnotations      map[string]string
	}{
		{
			name: "Delete MCS and clean up Service and ResourceBinding",
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-mcs",
					Namespace:  "default",
					Finalizers: []string{util.MCSControllerFinalizer},
				},
			},
			existingService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						util.ResourceTemplateClaimedByLabel:                    util.MultiClusterServiceKind,
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
					Annotations: map[string]string{
						networkingv1alpha1.MultiClusterServiceNameAnnotation:      "test-mcs",
						networkingv1alpha1.MultiClusterServiceNamespaceAnnotation: "default",
					},
				},
			},
			existingResourceBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						workv1alpha2.BindingManagedByLabel:                     util.MultiClusterServiceKind,
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
					Annotations: map[string]string{
						networkingv1alpha1.MultiClusterServiceNameAnnotation:      "test-mcs",
						networkingv1alpha1.MultiClusterServiceNamespaceAnnotation: "default",
					},
				},
			},
			expectedServiceLabels: nil,
			expectedServiceAnnotations: map[string]string{
				networkingv1alpha1.MultiClusterServiceNameAnnotation:      "test-mcs",
				networkingv1alpha1.MultiClusterServiceNamespaceAnnotation: "default",
			},
			expectedRBLabels: map[string]string{
				workv1alpha2.BindingManagedByLabel:                     util.MultiClusterServiceKind,
				networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
			},
			expectedRBAnnotations: map[string]string{
				networkingv1alpha1.MultiClusterServiceNameAnnotation:      "test-mcs",
				networkingv1alpha1.MultiClusterServiceNamespaceAnnotation: "default",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := newFakeController(tt.mcs, tt.existingService, tt.existingResourceBinding)

			_, err := controller.handleMultiClusterServiceDelete(context.Background(), tt.mcs)
			assert.NoError(t, err)

			updatedService := &corev1.Service{}
			err = controller.Client.Get(context.Background(), types.NamespacedName{Namespace: tt.mcs.Namespace, Name: tt.mcs.Name}, updatedService)
			assert.NoError(t, err)

			updatedRB := &workv1alpha2.ResourceBinding{}
			err = controller.Client.Get(context.Background(), types.NamespacedName{Namespace: tt.mcs.Namespace, Name: "service-" + tt.mcs.Name}, updatedRB)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedServiceLabels, updatedService.Labels)
			assert.Equal(t, tt.expectedServiceAnnotations, updatedService.Annotations)
			assert.Equal(t, tt.expectedRBLabels, updatedRB.Labels)
			assert.Equal(t, tt.expectedRBAnnotations, updatedRB.Annotations)

			updatedMCS := &networkingv1alpha1.MultiClusterService{}
			err = controller.Client.Get(context.Background(), types.NamespacedName{Namespace: tt.mcs.Namespace, Name: tt.mcs.Name}, updatedMCS)
			assert.NoError(t, err)
			assert.NotContains(t, updatedMCS.Finalizers, util.MCSControllerFinalizer)
		})
	}
}

func TestRetrieveMultiClusterService(t *testing.T) {
	tests := []struct {
		name             string
		mcs              *networkingv1alpha1.MultiClusterService
		existingWorks    []*workv1alpha1.Work
		providerClusters sets.Set[string]
		clusters         []*clusterv1alpha1.Cluster
		expectedWorks    int
	}{
		{
			name: "Remove work for non-provider cluster",
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
				},
			},
			existingWorks: []*workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      names.GenerateWorkName("MultiClusterService", "test-mcs", "default"),
						Namespace: names.GenerateExecutionSpaceName("cluster1"),
						Labels: map[string]string{
							networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
						},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{Raw: []byte(`{"apiVersion":"networking.karmada.io/v1alpha1","kind":"MultiClusterService"}`)},
								},
							},
						},
					},
				},
			},
			providerClusters: sets.New("cluster2"),
			clusters: []*clusterv1alpha1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
						},
					},
				},
			},
			expectedWorks: 0,
		},
		{
			name: "Keep work for provider cluster",
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
				},
			},
			existingWorks: []*workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      names.GenerateWorkName("MultiClusterService", "test-mcs", "default"),
						Namespace: names.GenerateExecutionSpaceName("cluster1"),
						Labels: map[string]string{
							networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
						},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{Raw: []byte(`{"apiVersion":"networking.karmada.io/v1alpha1","kind":"MultiClusterService"}`)},
								},
							},
						},
					},
				},
			},
			providerClusters: sets.New("cluster1"),
			clusters: []*clusterv1alpha1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
						},
					},
				},
			},
			expectedWorks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []runtime.Object{tt.mcs}
			objs = append(objs, toRuntimeObjects(tt.existingWorks)...)
			objs = append(objs, toRuntimeObjects(tt.clusters)...)

			controller := newFakeController(objs...)

			err := controller.retrieveMultiClusterService(context.Background(), tt.mcs, tt.providerClusters)
			assert.NoError(t, err)

			workList := &workv1alpha1.WorkList{}
			err = controller.Client.List(context.Background(), workList)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedWorks, len(workList.Items))
		})
	}
}

func TestPropagateMultiClusterService(t *testing.T) {
	tests := []struct {
		name             string
		mcs              *networkingv1alpha1.MultiClusterService
		providerClusters sets.Set[string]
		clusters         []*clusterv1alpha1.Cluster
		expectedWorks    int
	}{
		{
			name: "Propagate to one ready cluster",
			mcs: &networkingv1alpha1.MultiClusterService{
				TypeMeta: metav1.TypeMeta{
					Kind:       "MultiClusterService",
					APIVersion: networkingv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
				},
			},
			providerClusters: sets.New("cluster1"),
			clusters: []*clusterv1alpha1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
						},
						APIEnablements: []clusterv1alpha1.APIEnablement{
							{
								GroupVersion: "discovery.k8s.io/v1",
								Resources: []clusterv1alpha1.APIResource{
									{Kind: "EndpointSlice"},
								},
							},
						},
					},
				},
			},
			expectedWorks: 1,
		},
		{
			name: "No propagation to unready cluster",
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
				},
			},
			providerClusters: sets.New("cluster1"),
			clusters: []*clusterv1alpha1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionFalse},
						},
					},
				},
			},
			expectedWorks: 0,
		},
		{
			name: "No propagation to cluster without EndpointSlice support",
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
				},
			},
			providerClusters: sets.New("cluster1"),
			clusters: []*clusterv1alpha1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
						},
						APIEnablements: []clusterv1alpha1.APIEnablement{
							{
								GroupVersion: "v1",
								Resources: []clusterv1alpha1.APIResource{
									{Kind: "Pod"},
								},
							},
						},
					},
				},
			},
			expectedWorks: 0,
		},
		{
			name: "Propagate to multiple ready clusters",
			mcs: &networkingv1alpha1.MultiClusterService{
				TypeMeta: metav1.TypeMeta{
					Kind:       "MultiClusterService",
					APIVersion: networkingv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
				},
			},
			providerClusters: sets.New("cluster1", "cluster2"),
			clusters: []*clusterv1alpha1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
						},
						APIEnablements: []clusterv1alpha1.APIEnablement{
							{
								GroupVersion: "discovery.k8s.io/v1",
								Resources: []clusterv1alpha1.APIResource{
									{Kind: "EndpointSlice"},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster2"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
						},
						APIEnablements: []clusterv1alpha1.APIEnablement{
							{
								GroupVersion: "discovery.k8s.io/v1",
								Resources: []clusterv1alpha1.APIResource{
									{Kind: "EndpointSlice"},
								},
							},
						},
					},
				},
			},
			expectedWorks: 2,
		},
		{
			name: "Mixed cluster readiness and API support",
			mcs: &networkingv1alpha1.MultiClusterService{
				TypeMeta: metav1.TypeMeta{
					Kind:       "MultiClusterService",
					APIVersion: networkingv1alpha1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
				},
			},
			providerClusters: sets.New("cluster1", "cluster2", "cluster3"),
			clusters: []*clusterv1alpha1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
						},
						APIEnablements: []clusterv1alpha1.APIEnablement{
							{
								GroupVersion: "discovery.k8s.io/v1",
								Resources: []clusterv1alpha1.APIResource{
									{Kind: "EndpointSlice"},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster2"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionFalse},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster3"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
						},
						APIEnablements: []clusterv1alpha1.APIEnablement{
							{
								GroupVersion: "v1",
								Resources: []clusterv1alpha1.APIResource{
									{Kind: "Pod"},
								},
							},
						},
					},
				},
			},
			expectedWorks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []runtime.Object{tt.mcs}
			objs = append(objs, toRuntimeObjects(tt.clusters)...)

			controller := newFakeController(objs...)

			err := controller.propagateMultiClusterService(context.Background(), tt.mcs, tt.providerClusters)
			assert.NoError(t, err)

			workList := &workv1alpha1.WorkList{}
			err = controller.Client.List(context.Background(), workList)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedWorks, len(workList.Items))

			if tt.expectedWorks > 0 {
				for _, work := range workList.Items {
					assert.Equal(t, names.GenerateWorkName(tt.mcs.Kind, tt.mcs.Name, tt.mcs.Namespace), work.Name)
					clusterName, err := names.GetClusterName(work.Namespace)
					assert.NoError(t, err)
					assert.Contains(t, tt.providerClusters, clusterName)
					assert.Equal(t, "test-id", work.Labels[networkingv1alpha1.MultiClusterServicePermanentIDLabel])
				}
			}
		})
	}
}

func TestBuildResourceBinding(t *testing.T) {
	tests := []struct {
		name             string
		svc              *corev1.Service
		mcs              *networkingv1alpha1.MultiClusterService
		providerClusters sets.Set[string]
		consumerClusters sets.Set[string]
	}{
		{
			name: "Build ResourceBinding with non-overlapping clusters",
			svc: &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-service",
					Namespace:       "default",
					UID:             "test-uid",
					ResourceVersion: "1234",
				},
			},
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
				},
			},
			providerClusters: sets.New("cluster1", "cluster2"),
			consumerClusters: sets.New("cluster3", "cluster4"),
		},
		{
			name: "Build ResourceBinding with empty consumer clusters",
			svc: &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-service",
					Namespace:       "default",
					UID:             "test-uid",
					ResourceVersion: "1234",
				},
			},
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
				},
			},
			providerClusters: sets.New("cluster1", "cluster2"),
			consumerClusters: sets.New[string](),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := newFakeController()
			rb, err := controller.buildResourceBinding(tt.svc, tt.mcs, tt.providerClusters, tt.consumerClusters)

			assert.NoError(t, err)
			assert.NotNil(t, rb)

			// ObjectMeta Check
			assert.Equal(t, names.GenerateBindingName(tt.svc.Kind, tt.svc.Name), rb.Name)
			assert.Equal(t, tt.svc.Namespace, rb.Namespace)

			// Annotations Check
			assert.Equal(t, tt.mcs.Name, rb.Annotations[networkingv1alpha1.MultiClusterServiceNameAnnotation])
			assert.Equal(t, tt.mcs.Namespace, rb.Annotations[networkingv1alpha1.MultiClusterServiceNamespaceAnnotation])

			// Labels Check
			assert.Equal(t, util.MultiClusterServiceKind, rb.Labels[workv1alpha2.BindingManagedByLabel])
			assert.Equal(t, "test-id", rb.Labels[networkingv1alpha1.MultiClusterServicePermanentIDLabel])

			// OwnerReferences Check
			assert.Len(t, rb.OwnerReferences, 1)
			assert.Equal(t, tt.svc.APIVersion, rb.OwnerReferences[0].APIVersion)
			assert.Equal(t, tt.svc.Kind, rb.OwnerReferences[0].Kind)
			assert.Equal(t, tt.svc.Name, rb.OwnerReferences[0].Name)
			assert.Equal(t, tt.svc.UID, rb.OwnerReferences[0].UID)

			// Finalizers Check
			assert.Contains(t, rb.Finalizers, util.BindingControllerFinalizer)

			// Spec Check
			expectedClusters := tt.providerClusters.Union(tt.consumerClusters).UnsortedList()
			actualClusters := rb.Spec.Placement.ClusterAffinity.ClusterNames

			// Sort both slices before comparison
			sort.Strings(expectedClusters)
			sort.Strings(actualClusters)

			assert.Equal(t, expectedClusters, actualClusters, "Cluster names should match regardless of order")

			// Resource reference Check
			assert.Equal(t, tt.svc.APIVersion, rb.Spec.Resource.APIVersion)
			assert.Equal(t, tt.svc.Kind, rb.Spec.Resource.Kind)
			assert.Equal(t, tt.svc.Namespace, rb.Spec.Resource.Namespace)
			assert.Equal(t, tt.svc.Name, rb.Spec.Resource.Name)
			assert.Equal(t, tt.svc.UID, rb.Spec.Resource.UID)
			assert.Equal(t, tt.svc.ResourceVersion, rb.Spec.Resource.ResourceVersion)
		})
	}
}

func TestClaimMultiClusterServiceForService(t *testing.T) {
	tests := []struct {
		name          string
		svc           *corev1.Service
		mcs           *networkingv1alpha1.MultiClusterService
		updateError   bool
		expectedError bool
	}{
		{
			name: "Claim service for MCS - basic case",
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
			},
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
				},
			},
		},
		{
			name: "Claim service for MCS - with existing labels and annotations",
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
					Labels: map[string]string{
						"existing-label": "value",
						policyv1alpha1.PropagationPolicyPermanentIDLabel: "should-be-removed",
					},
					Annotations: map[string]string{
						"existing-annotation":                          "value",
						policyv1alpha1.PropagationPolicyNameAnnotation: "should-be-removed",
					},
				},
			},
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
				},
			},
		},
		{
			name: "Claim service for MCS - update error",
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
			},
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcs",
					Namespace: "default",
					Labels: map[string]string{
						networkingv1alpha1.MultiClusterServicePermanentIDLabel: "test-id",
					},
				},
			},
			updateError:   true,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := newFakeController(tt.svc)
			if tt.updateError {
				controller.Client = newFakeClientWithUpdateError(tt.svc, true)
			}

			err := controller.claimMultiClusterServiceForService(context.Background(), tt.svc, tt.mcs)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			updatedSvc := &corev1.Service{}
			err = controller.Client.Get(context.Background(), types.NamespacedName{Namespace: tt.svc.Namespace, Name: tt.svc.Name}, updatedSvc)
			assert.NoError(t, err)

			// Added labels and annotations check
			assert.Equal(t, util.MultiClusterServiceKind, updatedSvc.Labels[util.ResourceTemplateClaimedByLabel])
			assert.Equal(t, "test-id", updatedSvc.Labels[networkingv1alpha1.MultiClusterServicePermanentIDLabel])
			assert.Equal(t, tt.mcs.Name, updatedSvc.Annotations[networkingv1alpha1.MultiClusterServiceNameAnnotation])
			assert.Equal(t, tt.mcs.Namespace, updatedSvc.Annotations[networkingv1alpha1.MultiClusterServiceNamespaceAnnotation])

			// Removed labels and annotations check
			assert.NotContains(t, updatedSvc.Labels, policyv1alpha1.PropagationPolicyPermanentIDLabel)
			assert.NotContains(t, updatedSvc.Labels, policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel)
			assert.NotContains(t, updatedSvc.Annotations, policyv1alpha1.PropagationPolicyNameAnnotation)
			assert.NotContains(t, updatedSvc.Annotations, policyv1alpha1.PropagationPolicyNamespaceAnnotation)
			assert.NotContains(t, updatedSvc.Annotations, policyv1alpha1.ClusterPropagationPolicyAnnotation)

			// Check existing labels and annotations are preserved
			if tt.svc.Labels != nil {
				assert.Contains(t, updatedSvc.Labels, "existing-label")
			}
			if tt.svc.Annotations != nil {
				assert.Contains(t, updatedSvc.Annotations, "existing-annotation")
			}
		})
	}
}

func TestServiceHasCrossClusterMultiClusterService(t *testing.T) {
	tests := []struct {
		name     string
		svc      *corev1.Service
		mcs      *networkingv1alpha1.MultiClusterService
		expected bool
	}{
		{
			name: "Service has cross-cluster MCS",
			svc:  &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test-svc", Namespace: "default"}},
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-svc", Namespace: "default"},
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Types: []networkingv1alpha1.ExposureType{networkingv1alpha1.ExposureType("CrossCluster")},
				},
			},
			expected: true,
		},
		{
			name: "Service has no cross-cluster MCS",
			svc:  &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test-svc", Namespace: "default"}},
			mcs: &networkingv1alpha1.MultiClusterService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-svc", Namespace: "default"},
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Types: []networkingv1alpha1.ExposureType{networkingv1alpha1.ExposureType("LocalCluster")},
				},
			},
			expected: false,
		},
		{
			name:     "Service has no MCS",
			svc:      &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test-svc", Namespace: "default"}},
			mcs:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []runtime.Object{tt.svc}
			if tt.mcs != nil {
				objs = append(objs, tt.mcs)
			}

			controller := newFakeController(objs...)

			result := controller.serviceHasCrossClusterMultiClusterService(tt.svc)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClusterMapFunc(t *testing.T) {
	tests := []struct {
		name             string
		object           client.Object
		mcsList          []*networkingv1alpha1.MultiClusterService
		expectedRequests []reconcile.Request
	}{
		{
			name:   "Cluster matches MCS provider",
			object: &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster1"}},
			mcsList: []*networkingv1alpha1.MultiClusterService{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "mcs1", Namespace: "default"},
					Spec: networkingv1alpha1.MultiClusterServiceSpec{
						Types:            []networkingv1alpha1.ExposureType{networkingv1alpha1.ExposureType("CrossCluster")},
						ProviderClusters: []networkingv1alpha1.ClusterSelector{{Name: "cluster1"}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "mcs2", Namespace: "default"},
					Spec: networkingv1alpha1.MultiClusterServiceSpec{
						Types:            []networkingv1alpha1.ExposureType{networkingv1alpha1.ExposureType("CrossCluster")},
						ProviderClusters: []networkingv1alpha1.ClusterSelector{{Name: "cluster2"}},
					},
				},
			},
			expectedRequests: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: "default", Name: "mcs1"}},
				{NamespacedName: types.NamespacedName{Namespace: "default", Name: "mcs2"}},
			},
		},
		{
			name:   "Cluster doesn't match any MCS",
			object: &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster3"}},
			mcsList: []*networkingv1alpha1.MultiClusterService{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "mcs1", Namespace: "default"},
					Spec: networkingv1alpha1.MultiClusterServiceSpec{
						Types:            []networkingv1alpha1.ExposureType{networkingv1alpha1.ExposureType("CrossCluster")},
						ProviderClusters: []networkingv1alpha1.ClusterSelector{{Name: "cluster1"}},
					},
				},
			},
			expectedRequests: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: "default", Name: "mcs1"}},
			},
		},
		{
			name:             "Empty MCS list",
			object:           &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster1"}},
			mcsList:          []*networkingv1alpha1.MultiClusterService{},
			expectedRequests: []reconcile.Request{},
		},
		{
			name:             "Non-Cluster object",
			object:           &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1"}},
			mcsList:          []*networkingv1alpha1.MultiClusterService{},
			expectedRequests: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []runtime.Object{tt.object}
			objs = append(objs, toRuntimeObjects(tt.mcsList)...)

			controller := newFakeController(objs...)
			mapFunc := controller.clusterMapFunc()

			requests := mapFunc(context.Background(), tt.object)

			assert.Equal(t, len(tt.expectedRequests), len(requests), "Number of requests does not match expected")
			assert.ElementsMatch(t, tt.expectedRequests, requests, "Requests do not match expected")

			if _, ok := tt.object.(*clusterv1alpha1.Cluster); ok {
				for _, request := range requests {
					found := false
					for _, mcs := range tt.mcsList {
						if mcs.Name == request.Name && mcs.Namespace == request.Namespace {
							found = true
							break
						}
					}
					assert.True(t, found, "Generated request does not correspond to any MCS in the list")
				}
			}
		})
	}
}

func TestNeedSyncMultiClusterService(t *testing.T) {
	tests := []struct {
		name         string
		mcs          *networkingv1alpha1.MultiClusterService
		clusterName  string
		expectedNeed bool
		expectedErr  bool
	}{
		{
			name: "MCS with CrossCluster type and matching provider cluster",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Types:            []networkingv1alpha1.ExposureType{networkingv1alpha1.ExposureType("CrossCluster")},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{{Name: "cluster1"}},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{{Name: "cluster2"}},
				},
			},
			clusterName:  "cluster1",
			expectedNeed: true,
			expectedErr:  false,
		},
		{
			name: "MCS with CrossCluster type and matching consumer cluster",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Types:            []networkingv1alpha1.ExposureType{networkingv1alpha1.ExposureType("CrossCluster")},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{{Name: "cluster1"}},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{{Name: "cluster2"}},
				},
			},
			clusterName:  "cluster2",
			expectedNeed: true,
			expectedErr:  false,
		},
		{
			name: "MCS without CrossCluster type",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Types: []networkingv1alpha1.ExposureType{networkingv1alpha1.ExposureType("LocalCluster")},
				},
			},
			clusterName:  "cluster1",
			expectedNeed: false,
			expectedErr:  false,
		},
		{
			name: "MCS with empty ProviderClusters and ConsumerClusters",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Types: []networkingv1alpha1.ExposureType{networkingv1alpha1.ExposureType("CrossCluster")},
				},
			},
			clusterName:  "cluster1",
			expectedNeed: true,
			expectedErr:  false,
		},
		{
			name: "Cluster doesn't match ProviderClusters or ConsumerClusters",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Types:            []networkingv1alpha1.ExposureType{networkingv1alpha1.ExposureType("CrossCluster")},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{{Name: "cluster1"}},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{{Name: "cluster2"}},
				},
			},
			clusterName:  "cluster3",
			expectedNeed: false,
			expectedErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusters := createClustersFromMCS(tt.mcs)
			objs := append([]runtime.Object{tt.mcs}, toRuntimeObjects(clusters)...)

			controller := newFakeController(objs...)
			need, err := controller.needSyncMultiClusterService(tt.mcs, tt.clusterName)

			assert.Equal(t, tt.expectedNeed, need, "Expected need %v, but got %v", tt.expectedNeed, need)
			if tt.expectedErr {
				assert.Error(t, err, "Expected an error, but got none")
			} else {
				assert.NoError(t, err, "Expected no error, but got %v", err)
			}
		})
	}
}

// Helper Functions

// Helper function to create fake Cluster objects based on the MCS spec
func createClustersFromMCS(mcs *networkingv1alpha1.MultiClusterService) []*clusterv1alpha1.Cluster {
	var clusters []*clusterv1alpha1.Cluster
	for _, pc := range mcs.Spec.ProviderClusters {
		clusters = append(clusters, &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: pc.Name},
		})
	}
	for _, cc := range mcs.Spec.ConsumerClusters {
		clusters = append(clusters, &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: cc.Name},
		})
	}
	return clusters
}

// Helper function to set up a scheme with all necessary types
func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = networkingv1alpha1.Install(s)
	_ = workv1alpha1.Install(s)
	_ = workv1alpha2.Install(s)
	_ = clusterv1alpha1.Install(s)
	_ = scheme.AddToScheme(s)
	return s
}

// Helper function to create a new MCSController with a fake client
func newFakeController(objs ...runtime.Object) *MCSController {
	s := setupScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	return &MCSController{
		Client:        fakeClient,
		EventRecorder: record.NewFakeRecorder(100),
	}
}

// Helper function to convert a slice of objects to a slice of runtime.Object
func toRuntimeObjects(objs interface{}) []runtime.Object {
	var result []runtime.Object
	switch v := objs.(type) {
	case []*workv1alpha1.Work:
		for _, obj := range v {
			result = append(result, obj)
		}
	case []*clusterv1alpha1.Cluster:
		for _, obj := range v {
			result = append(result, obj)
		}
	case []*networkingv1alpha1.MultiClusterService:
		for _, obj := range v {
			result = append(result, obj)
		}
	}
	return result
}

// Helper function to create a fake client that can simulate update errors
func newFakeClientWithUpdateError(svc *corev1.Service, shouldError bool) client.Client {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = networkingv1alpha1.Install(s)

	fakeClient := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(svc).Build()

	if shouldError {
		return &errorInjectingClient{
			Client:      fakeClient,
			shouldError: shouldError,
		}
	}

	return fakeClient
}

type errorInjectingClient struct {
	client.Client
	shouldError bool
}

func (c *errorInjectingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if c.shouldError {
		return fmt.Errorf("simulated update error")
	}
	return c.Client.Update(ctx, obj, opts...)
}
