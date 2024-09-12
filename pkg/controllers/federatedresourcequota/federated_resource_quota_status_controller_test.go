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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

// TestAggregatedStatusFormWorks tests the aggregatedStatusFormWorks function
func TestAggregatedStatusFormWorks(t *testing.T) {
	tests := []struct {
		name          string
		works         []workv1alpha1.Work
		expected      []policyv1alpha1.ClusterQuotaStatus
		expectedError bool
	}{
		{
			name: "Single work, applied successfully",
			works: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "karmada-es-member-cluster-1",
					},
					Status: workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{
							{
								Type:   workv1alpha1.WorkApplied,
								Status: metav1.ConditionTrue,
							},
						},
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Status: &runtime.RawExtension{Raw: mustMarshal(corev1.ResourceQuotaStatus{
									Used: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("1"),
									},
								})},
							},
						},
					},
				},
			},
			expected: []policyv1alpha1.ClusterQuotaStatus{
				{
					ClusterName: "member-cluster-1",
					ResourceQuotaStatus: corev1.ResourceQuotaStatus{
						Used: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "Work not applied",
			works: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "karmada-es-member-cluster-1",
					},
					Status: workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{
							{
								Type:   workv1alpha1.WorkApplied,
								Status: metav1.ConditionFalse,
							},
						},
					},
				},
			},
			expected:      nil,
			expectedError: false,
		},
		{
			name: "Multiple works from different clusters",
			works: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "karmada-es-member-cluster-1",
					},
					Status: workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{
							{
								Type:   workv1alpha1.WorkApplied,
								Status: metav1.ConditionTrue,
							},
						},
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Status: &runtime.RawExtension{Raw: mustMarshal(corev1.ResourceQuotaStatus{
									Used: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("1"),
									},
								})},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "karmada-es-member-cluster-2",
					},
					Status: workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{
							{
								Type:   workv1alpha1.WorkApplied,
								Status: metav1.ConditionTrue,
							},
						},
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Status: &runtime.RawExtension{Raw: mustMarshal(corev1.ResourceQuotaStatus{
									Used: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
								})},
							},
						},
					},
				},
			},
			expected: []policyv1alpha1.ClusterQuotaStatus{
				{
					ClusterName: "member-cluster-1",
					ResourceQuotaStatus: corev1.ResourceQuotaStatus{
						Used: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
				{
					ClusterName: "member-cluster-2",
					ResourceQuotaStatus: corev1.ResourceQuotaStatus{
						Used: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "Work with empty ManifestStatuses",
			works: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "karmada-es-member-cluster-1",
					},
					Status: workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{
							{
								Type:   workv1alpha1.WorkApplied,
								Status: metav1.ConditionTrue,
							},
						},
						ManifestStatuses: []workv1alpha1.ManifestStatus{},
					},
				},
			},
			expected:      nil,
			expectedError: false,
		},
		{
			name: "Work with invalid JSON in ManifestStatuses",
			works: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "karmada-es-member-cluster-1",
					},
					Status: workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{
							{
								Type:   workv1alpha1.WorkApplied,
								Status: metav1.ConditionTrue,
							},
						},
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Status: &runtime.RawExtension{Raw: []byte(`invalid json`)},
							},
						},
					},
				},
			},
			expected:      nil,
			expectedError: true,
		},
		{
			name: "Work with invalid namespace",
			works: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "invalid-namespace",
					},
					Status: workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{
							{
								Type:   workv1alpha1.WorkApplied,
								Status: metav1.ConditionTrue,
							},
						},
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Status: &runtime.RawExtension{Raw: mustMarshal(corev1.ResourceQuotaStatus{
									Used: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("1"),
									},
								})},
							},
						},
					},
				},
			},
			expected:      nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := aggregatedStatusFormWorks(tt.works)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expected == nil {
					assert.Nil(t, result)
				} else {
					assert.Equal(t, len(tt.expected), len(result))
					for i, expected := range tt.expected {
						assert.Equal(t, expected.ClusterName, result[i].ClusterName)
						assert.Equal(t, expected.ResourceQuotaStatus.Used, result[i].ResourceQuotaStatus.Used)
					}
				}
			}
		})
	}
}

// TestCalculateUsed tests the calculateUsed function
func TestCalculateUsed(t *testing.T) {
	tests := []struct {
		name               string
		aggregatedStatuses []policyv1alpha1.ClusterQuotaStatus
		expectedUsed       corev1.ResourceList
	}{
		{
			name: "Single cluster",
			aggregatedStatuses: []policyv1alpha1.ClusterQuotaStatus{
				{
					ClusterName: "cluster1",
					ResourceQuotaStatus: corev1.ResourceQuotaStatus{
						Used: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			expectedUsed: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		},
		{
			name: "Multiple clusters",
			aggregatedStatuses: []policyv1alpha1.ClusterQuotaStatus{
				{
					ClusterName: "cluster1",
					ResourceQuotaStatus: corev1.ResourceQuotaStatus{
						Used: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
				{
					ClusterName: "cluster2",
					ResourceQuotaStatus: corev1.ResourceQuotaStatus{
						Used: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
			expectedUsed: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("3"),
				corev1.ResourceMemory: resource.MustParse("3Gi"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateUsed(tt.aggregatedStatuses)
			assert.Equal(t, tt.expectedUsed.Cpu().Value(), result.Cpu().Value())
			assert.Equal(t, tt.expectedUsed.Memory().Value(), result.Memory().Value())
		})
	}
}

// Helper function to marshal ResourceQuotaStatus to JSON
func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
