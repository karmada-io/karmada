/*
Copyright 2021 The Karmada Authors.

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

package v1alpha1

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestString(t *testing.T) {
	clusterName := "cluster1"
	cluster1 := &Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: clusterName},
	}

	tests := []struct {
		name    string
		fmtFunc func() string
		want    string
	}{
		{
			name: "%s pointer test",
			fmtFunc: func() string {
				return fmt.Sprintf("%s", cluster1) // nolint
			},
			want: clusterName,
		},
		{
			name: "%v pointer test",
			fmtFunc: func() string {
				return fmt.Sprintf("%v", cluster1)
			},
			want: clusterName,
		},
		{
			name: "%v pointer array test",
			fmtFunc: func() string {
				return fmt.Sprintf("%v", []*Cluster{cluster1})
			},
			want: fmt.Sprintf("[%s]", clusterName),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fmtFunc(); got != tt.want {
				t.Errorf("%s String() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestAPIEnablement(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *Cluster
		gvk      schema.GroupVersionKind
		expected APIEnablementStatus
	}{
		{
			name: "API enabled - exact match found",
			cluster: &Cluster{
				Status: ClusterStatus{
					APIEnablements: []APIEnablement{
						{
							GroupVersion: "apps/v1",
							Resources: []APIResource{
								{Name: "deployments", Kind: "Deployment"},
								{Name: "replicasets", Kind: "ReplicaSet"},
							},
						},
					},
					Conditions: []metav1.Condition{
						{
							Type:   ClusterConditionCompleteAPIEnablements,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			gvk:      schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			expected: APIEnabled,
		},
		{
			name: "API disabled - not found in complete list",
			cluster: &Cluster{
				Status: ClusterStatus{
					APIEnablements: []APIEnablement{
						{
							GroupVersion: "apps/v1",
							Resources: []APIResource{
								{Name: "deployments", Kind: "Deployment"},
							},
						},
					},
					Conditions: []metav1.Condition{
						{
							Type:   ClusterConditionCompleteAPIEnablements,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			gvk:      schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
			expected: APIDisabled,
		},
		{
			name: "API unknown - not found in partial list",
			cluster: &Cluster{
				Status: ClusterStatus{
					APIEnablements: []APIEnablement{
						{
							GroupVersion: "apps/v1",
							Resources: []APIResource{
								{Name: "deployments", Kind: "Deployment"},
							},
						},
					},
					Conditions: []metav1.Condition{
						{
							Type:   ClusterConditionCompleteAPIEnablements,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			gvk:      schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
			expected: APIUnknown,
		},
		{
			name: "API unknown - no CompleteAPIEnablements condition",
			cluster: &Cluster{
				Status: ClusterStatus{
					APIEnablements: []APIEnablement{
						{
							GroupVersion: "apps/v1",
							Resources: []APIResource{
								{Name: "deployments", Kind: "Deployment"},
							},
						},
					},
					Conditions: []metav1.Condition{},
				},
			},
			gvk:      schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
			expected: APIUnknown,
		},
		{
			name: "API enabled - found in core group",
			cluster: &Cluster{
				Status: ClusterStatus{
					APIEnablements: []APIEnablement{
						{
							GroupVersion: "v1",
							Resources: []APIResource{
								{Name: "pods", Kind: "Pod"},
								{Name: "services", Kind: "Service"},
							},
						},
					},
					Conditions: []metav1.Condition{
						{
							Type:   ClusterConditionCompleteAPIEnablements,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			gvk:      schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			expected: APIEnabled,
		},
		{
			name: "API disabled - wrong kind in same group version",
			cluster: &Cluster{
				Status: ClusterStatus{
					APIEnablements: []APIEnablement{
						{
							GroupVersion: "apps/v1",
							Resources: []APIResource{
								{Name: "deployments", Kind: "Deployment"},
							},
						},
					},
					Conditions: []metav1.Condition{
						{
							Type:   ClusterConditionCompleteAPIEnablements,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			gvk:      schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
			expected: APIDisabled,
		},
		{
			name: "API disabled - empty APIEnablements with complete condition",
			cluster: &Cluster{
				Status: ClusterStatus{
					APIEnablements: []APIEnablement{},
					Conditions: []metav1.Condition{
						{
							Type:   ClusterConditionCompleteAPIEnablements,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			gvk:      schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			expected: APIDisabled,
		},
		{
			name: "API enabled - custom resource found",
			cluster: &Cluster{
				Status: ClusterStatus{
					APIEnablements: []APIEnablement{
						{
							GroupVersion: "example.com/v1alpha1",
							Resources: []APIResource{
								{Name: "customresources", Kind: "CustomResource"},
							},
						},
					},
					Conditions: []metav1.Condition{
						{
							Type:   ClusterConditionCompleteAPIEnablements,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			gvk:      schema.GroupVersionKind{Group: "example.com", Version: "v1alpha1", Kind: "CustomResource"},
			expected: APIEnabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cluster.APIEnablement(tt.gvk)
			if result != tt.expected {
				t.Errorf("APIEnablement() = %v, want %v", result, tt.expected)
			}
		})
	}
}
