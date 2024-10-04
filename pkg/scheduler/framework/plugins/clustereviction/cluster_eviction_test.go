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

package clustereviction

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

func TestClusterEviction_Filter(t *testing.T) {
	tests := []struct {
		name         string
		bindingSpec  *workv1alpha2.ResourceBindingSpec
		cluster      *clusterv1alpha1.Cluster
		expectedCode framework.Code
		expectError  bool
	}{
		{
			name: "cluster is in graceful eviction tasks",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "cluster1",
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
			},
			expectedCode: framework.Unschedulable,
			expectError:  true,
		},
		{
			name: "cluster is not in graceful eviction tasks",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "cluster2",
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
			},
			expectedCode: framework.Success,
			expectError:  false,
		},
		{
			name:        "no graceful eviction tasks",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
			},
			expectedCode: framework.Success,
			expectError:  false,
		},
	}

	p := &ClusterEviction{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Filter(context.Background(), tt.bindingSpec, nil, tt.cluster)
			assert.Equal(t, tt.expectedCode, result.Code())
			assert.Equal(t, tt.expectError, result.AsError() != nil)
		})
	}
}

func TestNew(t *testing.T) {
	plugin, err := New()
	assert.NoError(t, err)
	assert.NotNil(t, plugin)
	_, ok := plugin.(*ClusterEviction)
	assert.True(t, ok)
}

func TestClusterEviction_Name(t *testing.T) {
	p := &ClusterEviction{}
	assert.Equal(t, Name, p.Name())
}
