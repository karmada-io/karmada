/*
Copyright 2025 The Karmada Authors.

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

package resourcehealth

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestResourceHealth_Name(t *testing.T) {
	plugin := &ResourceHealth{}
	if plugin.Name() != PluginName {
		t.Errorf("ResourceHealth.Name() = %v, want %v", plugin.Name(), PluginName)
	}
}

func TestResourceHealth_CanEvictRB(t *testing.T) {
	tests := []struct {
		name            string
		evictingCluster string
		scheduleResult  []workv1alpha2.TargetCluster
		observedStatus  []workv1alpha2.AggregatedStatusItem
		expected        bool
	}{
		{
			name:            "all clusters healthy",
			evictingCluster: "cluster1",
			scheduleResult: []workv1alpha2.TargetCluster{
				{Name: "cluster2"},
				{Name: "cluster3"},
			},
			observedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster2", Health: workv1alpha2.ResourceHealthy},
				{ClusterName: "cluster3", Health: workv1alpha2.ResourceHealthy},
			},
			expected: true,
		},
		{
			name:            "one cluster unhealthy",
			evictingCluster: "cluster1",
			scheduleResult: []workv1alpha2.TargetCluster{
				{Name: "cluster2"},
				{Name: "cluster3"},
			},
			observedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster2", Health: workv1alpha2.ResourceHealthy},
				{ClusterName: "cluster3", Health: workv1alpha2.ResourceUnhealthy},
			},
			expected: false,
		},
		{
			name:            "missing status for one cluster",
			evictingCluster: "cluster1",
			scheduleResult: []workv1alpha2.TargetCluster{
				{Name: "cluster2"},
				{Name: "cluster3"},
			},
			observedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster2", Health: workv1alpha2.ResourceHealthy},
				// cluster3 status is missing
			},
			expected: false,
		},
		{
			name:            "only evicting cluster in schedule result",
			evictingCluster: "cluster1",
			scheduleResult: []workv1alpha2.TargetCluster{
				{Name: "cluster1"},
			},
			observedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster1", Health: workv1alpha2.ResourceHealthy},
			},
			expected: true, // No other clusters to check
		},
		{
			name:            "empty schedule result",
			evictingCluster: "cluster1",
			scheduleResult:  []workv1alpha2.TargetCluster{},
			observedStatus:  []workv1alpha2.AggregatedStatusItem{},
			expected:        true, // No clusters to check
		},
		{
			name:            "unknown health status",
			evictingCluster: "cluster1",
			scheduleResult: []workv1alpha2.TargetCluster{
				{Name: "cluster2"},
			},
			observedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster2", Health: workv1alpha2.ResourceUnknown},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &ResourceHealth{}

			task := &workv1alpha2.GracefulEvictionTask{
				FromCluster: tt.evictingCluster,
			}

			binding := &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "test-namespace",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: tt.scheduleResult,
				},
				Status: workv1alpha2.ResourceBindingStatus{
					AggregatedStatus: tt.observedStatus,
				},
			}

			result := plugin.CanEvictRB(context.Background(), task, binding)
			if result != tt.expected {
				t.Errorf("ResourceHealth.CanEvictRB() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestResourceHealth_CanEvictCRB(t *testing.T) {
	tests := []struct {
		name            string
		evictingCluster string
		scheduleResult  []workv1alpha2.TargetCluster
		observedStatus  []workv1alpha2.AggregatedStatusItem
		expected        bool
	}{
		{
			name:            "all clusters healthy",
			evictingCluster: "cluster1",
			scheduleResult: []workv1alpha2.TargetCluster{
				{Name: "cluster2"},
				{Name: "cluster3"},
			},
			observedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster2", Health: workv1alpha2.ResourceHealthy},
				{ClusterName: "cluster3", Health: workv1alpha2.ResourceHealthy},
			},
			expected: true,
		},
		{
			name:            "one cluster unhealthy",
			evictingCluster: "cluster1",
			scheduleResult: []workv1alpha2.TargetCluster{
				{Name: "cluster2"},
				{Name: "cluster3"},
			},
			observedStatus: []workv1alpha2.AggregatedStatusItem{
				{ClusterName: "cluster2", Health: workv1alpha2.ResourceHealthy},
				{ClusterName: "cluster3", Health: workv1alpha2.ResourceUnhealthy},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &ResourceHealth{}

			task := &workv1alpha2.GracefulEvictionTask{
				FromCluster: tt.evictingCluster,
			}

			binding := &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: tt.scheduleResult,
				},
				Status: workv1alpha2.ResourceBindingStatus{
					AggregatedStatus: tt.observedStatus,
				},
			}

			result := plugin.CanEvictCRB(context.Background(), task, binding)
			if result != tt.expected {
				t.Errorf("ResourceHealth.CanEvictCRB() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNew(t *testing.T) {
	plugin, err := New()
	if err != nil {
		t.Errorf("New() error = %v", err)
		return
	}

	if plugin == nil {
		t.Error("New() returned nil plugin")
		return
	}

	if plugin.Name() != PluginName {
		t.Errorf("New() returned plugin with name %v, want %v", plugin.Name(), PluginName)
	}
}
