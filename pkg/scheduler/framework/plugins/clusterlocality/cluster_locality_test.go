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

package clusterlocality

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

func TestClusterLocality_Score(t *testing.T) {
	tests := []struct {
		name          string
		bindingSpec   *workv1alpha2.ResourceBindingSpec
		cluster       *clusterv1alpha1.Cluster
		expectedScore int64
	}{
		{
			name: "no clusters in spec",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
			},
			expectedScore: framework.MinClusterScore,
		},
		{
			name: "cluster in spec",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{
					{Name: "cluster1"},
					{Name: "cluster2"},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
			},
			expectedScore: framework.MaxClusterScore,
		},
		{
			name: "cluster not in spec",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{
					{Name: "cluster2"},
					{Name: "cluster3"},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
			},
			expectedScore: framework.MinClusterScore,
		},
	}

	p := &ClusterLocality{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, result := p.Score(context.Background(), tt.bindingSpec, tt.cluster)
			assert.Equal(t, tt.expectedScore, score)
			assert.Equal(t, framework.Success, result.Code())
		})
	}
}

func TestNew(t *testing.T) {
	plugin, err := New()
	assert.NoError(t, err)
	assert.NotNil(t, plugin)
	_, ok := plugin.(*ClusterLocality)
	assert.True(t, ok)
}

func TestClusterLocality_Name(t *testing.T) {
	p := &ClusterLocality{}
	assert.Equal(t, Name, p.Name())
}

func TestClusterLocality_ScoreExtensions(t *testing.T) {
	p := &ClusterLocality{}
	assert.Nil(t, p.ScoreExtensions())
}
