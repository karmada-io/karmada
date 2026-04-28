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

package federatedresourcequota

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// setupTest initializes a test environment with the given runtime objects
// It returns a fake client and a SyncController for use in tests
func setupTest(t *testing.T, objs ...runtime.Object) (client.Client, *SyncController) {
	scheme := runtime.NewScheme()
	assert.NoError(t, policyv1alpha1.Install(scheme))
	assert.NoError(t, workv1alpha1.Install(scheme))
	assert.NoError(t, clusterv1alpha1.Install(scheme))
	assert.NoError(t, corev1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	controller := &SyncController{
		Client:        fakeClient,
		EventRecorder: record.NewFakeRecorder(100),
	}
	return fakeClient, controller
}

// TestCleanUpWorks tests the cleanUpWorks function of the SyncController
func TestCleanUpWorks(t *testing.T) {
	tests := []struct {
		name          string
		existingWorks []runtime.Object
		namespace     string
		quotaName     string
		expectedError bool
	}{
		{
			name: "Successfully delete works",
			existingWorks: []runtime.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work-1",
						Namespace: "default",
						Labels: map[string]string{
							util.FederatedResourceQuotaNamespaceLabel: "default",
							util.FederatedResourceQuotaNameLabel:      "test-quota",
						},
					},
				},
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work-2",
						Namespace: "default",
						Labels: map[string]string{
							util.FederatedResourceQuotaNamespaceLabel: "default",
							util.FederatedResourceQuotaNameLabel:      "test-quota",
						},
					},
				},
			},
			namespace:     "default",
			quotaName:     "test-quota",
			expectedError: false,
		},
		{
			name:          "No works to delete",
			existingWorks: []runtime.Object{},
			namespace:     "default",
			quotaName:     "test-quota",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, controller := setupTest(t, tt.existingWorks...)

			err := controller.cleanUpWorks(context.Background(), tt.namespace, tt.quotaName)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify that works are deleted
			workList := &workv1alpha1.WorkList{}
			err = fakeClient.List(context.Background(), workList, client.MatchingLabels{
				util.FederatedResourceQuotaNamespaceLabel: tt.namespace,
				util.FederatedResourceQuotaNameLabel:      tt.quotaName,
			})
			assert.NoError(t, err)
			assert.Empty(t, workList.Items)
		})
	}
}

// It verifies that works are correctly created for the given FederatedResourceQuota and clusters
func TestBuildWorks(t *testing.T) {
	tests := []struct {
		name          string
		quota         *policyv1alpha1.FederatedResourceQuota
		clusters      []clusterv1alpha1.Cluster
		expectedError bool
		expectedWorks int
	}{
		{
			name: "Successfully build works for each cluster declared in StaticAssignments",
			quota: &policyv1alpha1.FederatedResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-quota",
					Namespace: "default",
				},
				Spec: policyv1alpha1.FederatedResourceQuotaSpec{
					StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
						{
							ClusterName: "cluster1",
							Hard: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("1"),
							},
						},
						{
							ClusterName: "cluster2",
							Hard: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("2"),
							},
						},
					},
				},
			},
			clusters: []clusterv1alpha1.Cluster{
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster2"}},
			},
			expectedError: false,
			expectedWorks: 2,
		},
		{
			name: "No clusters declared in StaticAssignments",
			quota: &policyv1alpha1.FederatedResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-quota",
					Namespace: "default",
				},
				Spec: policyv1alpha1.FederatedResourceQuotaSpec{},
			},
			clusters: []clusterv1alpha1.Cluster{
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster2"}},
			},
			expectedError: false,
			expectedWorks: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, controller := setupTest(t)

			err := controller.buildWorks(context.Background(), tt.quota, tt.clusters)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify the number of created works
			workList := &workv1alpha1.WorkList{}
			err = fakeClient.List(context.Background(), workList, client.MatchingLabels{
				util.FederatedResourceQuotaNamespaceLabel: tt.quota.Namespace,
				util.FederatedResourceQuotaNameLabel:      tt.quota.Name,
			})
			assert.NoError(t, err)
			assert.Len(t, workList.Items, tt.expectedWorks)
		})
	}
}

// TestExtractClusterHardResourceList tests the extractClusterHardResourceList function
func TestExtractClusterHardResourceList(t *testing.T) {
	tests := []struct {
		name           string
		spec           policyv1alpha1.FederatedResourceQuotaSpec
		clusterName    string
		expectedResult corev1.ResourceList
	}{
		{
			name: "Cluster found in static assignments",
			spec: policyv1alpha1.FederatedResourceQuotaSpec{
				StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
					{
						ClusterName: "cluster1",
						Hard: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			clusterName: "cluster1",
			expectedResult: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		},
		{
			name: "Cluster not found in static assignments",
			spec: policyv1alpha1.FederatedResourceQuotaSpec{
				StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
					{
						ClusterName: "cluster1",
						Hard: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
			},
			clusterName:    "cluster2",
			expectedResult: nil,
		},
		{
			name: "Empty static assignments",
			spec: policyv1alpha1.FederatedResourceQuotaSpec{
				StaticAssignments: []policyv1alpha1.StaticClusterAssignment{},
			},
			clusterName:    "cluster1",
			expectedResult: nil,
		},
		{
			name: "Multiple static assignments",
			spec: policyv1alpha1.FederatedResourceQuotaSpec{
				StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
					{
						ClusterName: "cluster1",
						Hard: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
					{
						ClusterName: "cluster2",
						Hard: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"),
						},
					},
				},
			},
			clusterName: "cluster2",
			expectedResult: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("2"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractClusterHardResourceList(tt.spec, tt.clusterName)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestCleanUpOrphanWorks(t *testing.T) {
	tests := []struct {
		name           string
		quota          policyv1alpha1.FederatedResourceQuota
		totalWorks     []runtime.Object
		remainingWorks []workv1alpha1.Work
	}{
		{
			name: "Clean up orphan works",
			quota: policyv1alpha1.FederatedResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-quota",
					Namespace: "default",
				},
				Spec: policyv1alpha1.FederatedResourceQuotaSpec{
					StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
						{
							ClusterName: "cluster1",
							Hard: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("1"),
							},
						},
						{
							ClusterName: "cluster2",
							Hard: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("2"),
							},
						},
					},
				},
				Status: policyv1alpha1.FederatedResourceQuotaStatus{
					AggregatedStatus: []policyv1alpha1.ClusterQuotaStatus{
						{
							ClusterName: "cluster1",
						},
						{
							ClusterName: "cluster2",
						},
						{
							ClusterName: "cluster3",
						},
					},
				},
			},
			totalWorks: []runtime.Object{
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      names.GenerateWorkName("ResourceQuota", "test-quota", "default"),
						Namespace: names.GenerateExecutionSpaceName("cluster1"),
						Labels: map[string]string{
							util.FederatedResourceQuotaNamespaceLabel: "default",
							util.FederatedResourceQuotaNameLabel:      "test-quota",
						},
					},
				},
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      names.GenerateWorkName("ResourceQuota", "test-quota", "default"),
						Namespace: names.GenerateExecutionSpaceName("cluster2"),
						Labels: map[string]string{
							util.FederatedResourceQuotaNamespaceLabel: "default",
							util.FederatedResourceQuotaNameLabel:      "test-quota",
						},
					},
				},
				&workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      names.GenerateWorkName("ResourceQuota", "test-quota", "default"),
						Namespace: names.GenerateExecutionSpaceName("cluster3"),
						Labels: map[string]string{
							util.FederatedResourceQuotaNamespaceLabel: "default",
							util.FederatedResourceQuotaNameLabel:      "test-quota",
						},
					},
				},
			},
			remainingWorks: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      names.GenerateWorkName("ResourceQuota", "test-quota", "default"),
						Namespace: names.GenerateExecutionSpaceName("cluster1"),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      names.GenerateWorkName("ResourceQuota", "test-quota", "default"),
						Namespace: names.GenerateExecutionSpaceName("cluster2"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, controller := setupTest(t, tt.totalWorks...)
			err := controller.cleanUpOrphanWorks(context.Background(), &tt.quota)
			if err != nil {
				t.Errorf("Failed to cleanup the orphan works: %s", err.Error())
			}

			// Verify the number of created works
			workList := &workv1alpha1.WorkList{}
			err = fakeClient.List(context.Background(), workList, client.MatchingLabels{
				util.FederatedResourceQuotaNamespaceLabel: tt.quota.Namespace,
				util.FederatedResourceQuotaNameLabel:      tt.quota.Name,
			})
			if err != nil {
				t.Errorf("Failed to list works: %s", err.Error())
			}

			var actualRemainingWorks []workv1alpha1.Work
			for _, item := range workList.Items {
				actualRemainingWorks = append(actualRemainingWorks, workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      item.GetName(),
						Namespace: item.GetNamespace(),
					},
				})
			}
			assert.ElementsMatch(t, tt.remainingWorks, actualRemainingWorks)
		})
	}
}
