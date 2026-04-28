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

package clusteraffinity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

func TestClusterAffinity_Filter(t *testing.T) {
	tests := []struct {
		name          string
		bindingSpec   *workv1alpha2.ResourceBindingSpec
		bindingStatus *workv1alpha2.ResourceBindingStatus
		cluster       *clusterv1alpha1.Cluster
		expectedCode  framework.Code
		expectError   bool
	}{
		{
			name: "matching affinity",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{"cluster1"},
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
			name: "non-matching affinity",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{"cluster2"},
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
			name: "matching affinity from ClusterAffinities",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName: "affinity1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{
								ClusterNames: []string{"cluster1"},
							},
						},
					},
				},
			},
			bindingStatus: &workv1alpha2.ResourceBindingStatus{
				SchedulerObservedAffinityName: "affinity1",
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
			name: "no affinity specified",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Placement: &policyv1alpha1.Placement{},
			},
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
			},
			expectedCode: framework.Success,
			expectError:  false,
		},
	}

	p := &ClusterAffinity{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Filter(context.Background(), tt.bindingSpec, tt.bindingStatus, tt.cluster)
			assert.Equal(t, tt.expectedCode, result.Code())
			assert.Equal(t, tt.expectError, result.AsError() != nil)
		})
	}
}

func TestClusterAffinity_Score(t *testing.T) {
	p := &ClusterAffinity{}
	spec := &workv1alpha2.ResourceBindingSpec{}
	cluster := &clusterv1alpha1.Cluster{}

	score, result := p.Score(context.Background(), spec, cluster)

	assert.Equal(t, framework.MinClusterScore, score)
	assert.Equal(t, framework.Success, result.Code())
}

func TestClusterAffinity_ScoreExtensions(t *testing.T) {
	p := &ClusterAffinity{}
	assert.Equal(t, p, p.ScoreExtensions())
}

func TestClusterAffinity_NormalizeScore(t *testing.T) {
	p := &ClusterAffinity{}
	result := p.NormalizeScore(context.Background(), nil)
	assert.Equal(t, framework.Success, result.Code())
}

func TestNew(t *testing.T) {
	plugin, err := New()

	assert.NoError(t, err)
	assert.NotNil(t, plugin)
	_, ok := plugin.(*ClusterAffinity)
	assert.True(t, ok)
}

func TestClusterAffinity_Name(t *testing.T) {
	p := &ClusterAffinity{}
	assert.Equal(t, Name, p.Name())
}
