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

func TestCluster_IsAPIEnabledAndTrusted(t *testing.T) {
	tests := []struct {
		name            string
		cluster         *Cluster
		gvk             schema.GroupVersionKind
		expectedEnabled bool
		expectedTrusted bool
	}{
		{
			name: "API enabled - should be enabled and trusted",
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
			gvk:             schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			expectedEnabled: true,
			expectedTrusted: true,
		},
		{
			name: "API not enabled but APIEnablements complete - should be disabled but trusted",
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
			gvk:             schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
			expectedEnabled: false,
			expectedTrusted: true,
		},
		{
			name: "API not enabled and APIEnablements incomplete - should be disabled and untrusted",
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
			gvk:             schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
			expectedEnabled: false,
			expectedTrusted: false,
		},
		{
			name: "API enabled in different group version - should match exactly",
			cluster: &Cluster{
				Status: ClusterStatus{
					APIEnablements: []APIEnablement{
						{
							GroupVersion: "apps/v1beta1",
							Resources: []APIResource{
								{Name: "deployments", Kind: "Deployment"},
							},
						},
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
			gvk:             schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			expectedEnabled: true,
			expectedTrusted: true,
		},
		{
			name: "API kind not found in matching group version",
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
			gvk:             schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
			expectedEnabled: false,
			expectedTrusted: true,
		},
		{
			name: "No CompleteAPIEnablements condition - should return false and false",
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
							Type:   ClusterConditionReady,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			gvk:             schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
			expectedEnabled: false,
			expectedTrusted: false,
		},
		{
			name: "Empty APIEnablements with complete condition true",
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
			gvk:             schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			expectedEnabled: false,
			expectedTrusted: true,
		},
		{
			name: "Core API group test",
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
			gvk:             schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			expectedEnabled: true,
			expectedTrusted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled, trusted := tt.cluster.IsAPIEnabledAndTrusted(tt.gvk)
			if enabled != tt.expectedEnabled {
				t.Errorf("IsAPIEnabledAndTrusted() enabled = %v, want %v", enabled, tt.expectedEnabled)
			}
			if trusted != tt.expectedTrusted {
				t.Errorf("IsAPIEnabledAndTrusted() trusted = %v, want %v", trusted, tt.expectedTrusted)
			}
		})
	}
}
