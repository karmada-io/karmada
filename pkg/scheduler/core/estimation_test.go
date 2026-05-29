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

package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	schedulercache "github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/test/helper"
)

// mockReplicaEstimator is a mock implementation of ReplicaEstimator for testing
type mockReplicaEstimator struct {
	maxAvailableComponentSetsResponse []estimatorclient.ComponentSetEstimationResponse
	maxAvailableComponentSetsError    error
	lastComponentSetRequest           estimatorclient.ComponentSetEstimationRequest
}

func (m *mockReplicaEstimator) MaxAvailableReplicas(_ context.Context, _ estimatorclient.ReplicaEstimationRequest) ([]workv1alpha2.TargetCluster, error) {
	return nil, nil
}

func (m *mockReplicaEstimator) MaxAvailableComponentSets(_ context.Context, req estimatorclient.ComponentSetEstimationRequest) ([]estimatorclient.ComponentSetEstimationResponse, error) {
	m.lastComponentSetRequest = req
	return m.maxAvailableComponentSetsResponse, m.maxAvailableComponentSetsError
}

func Test_isMultiTemplateSchedulingApplicable(t *testing.T) {
	tests := []struct {
		name string
		spec *workv1alpha2.ResourceBindingSpec
		want bool
	}{
		{
			name: "nil spec should not be applicable",
			spec: nil,
			want: false,
		},
		{
			name: "spec with multiple components but without placement should not be applicable",
			spec: &workv1alpha2.ResourceBindingSpec{
				Components: []workv1alpha2.Component{
					{Name: "component1"},
					{Name: "component2"},
				},
			},
			want: false,
		},
		{
			name: "spec with nil placement should not be applicable",
			spec: &workv1alpha2.ResourceBindingSpec{
				Components: []workv1alpha2.Component{
					{Name: "component1"},
					{Name: "component2"},
				},
				Placement: nil,
			},
			want: false,
		},
		{
			name: "spec with empty spread constraints should not be applicable",
			spec: &workv1alpha2.ResourceBindingSpec{
				Components: []workv1alpha2.Component{
					{Name: "component1"},
					{Name: "component2"},
				},
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{},
				},
			},
			want: false,
		},
		{
			name: "spec with non-cluster spread constraint should not be applicable",
			spec: &workv1alpha2.ResourceBindingSpec{
				Components: []workv1alpha2.Component{
					{Name: "component1"},
					{Name: "component2"},
				},
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldRegion,
							MinGroups:     1,
							MaxGroups:     1,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "spec with cluster spread constraint but wrong min/max groups should not be applicable",
			spec: &workv1alpha2.ResourceBindingSpec{
				Components: []workv1alpha2.Component{
					{Name: "component1"},
					{Name: "component2"},
				},
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MinGroups:     2,
							MaxGroups:     2,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "spec with single component and valid spread constraint should be applicable",
			spec: &workv1alpha2.ResourceBindingSpec{
				Components: []workv1alpha2.Component{
					{Name: "component1"},
				},
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MinGroups:     1,
							MaxGroups:     1,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "spec with valid cluster spread constraint should be applicable",
			spec: &workv1alpha2.ResourceBindingSpec{
				Components: []workv1alpha2.Component{
					{Name: "component1"},
					{Name: "component2"},
				},
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MinGroups:     1,
							MaxGroups:     1,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "spec with multiple spread constraints, one valid should be applicable",
			spec: &workv1alpha2.ResourceBindingSpec{
				Components: []workv1alpha2.Component{
					{Name: "component1"},
					{Name: "component2"},
				},
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldRegion,
							MinGroups:     1,
							MaxGroups:     1,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MinGroups:     1,
							MaxGroups:     1,
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMultiTemplateSchedulingApplicable(tt.spec)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_calculateMultiTemplateAvailableSets(t *testing.T) {
	ctx := context.Background()
	estimatorName := "test-estimator"
	clusters := []*clusterv1alpha1.Cluster{
		helper.NewCluster("cluster1"),
		helper.NewCluster("cluster2"),
		helper.NewCluster("cluster3"),
	}

	spec := &workv1alpha2.ResourceBindingSpec{
		Resource: workv1alpha2.ObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  "default",
			Name:       "test-deployment",
		},
		Components: []workv1alpha2.Component{
			{Name: "component1"},
			{Name: "component2"},
		},
		Placement: &policyv1alpha1.Placement{
			SpreadConstraints: []policyv1alpha1.SpreadConstraint{
				{
					SpreadByField: policyv1alpha1.SpreadByFieldCluster,
					MinGroups:     1,
					MaxGroups:     1,
				},
			},
		},
	}

	tests := []struct {
		name           string
		mockResponse   []estimatorclient.ComponentSetEstimationResponse
		mockError      error
		expectedResult []workv1alpha2.TargetCluster
		expectedError  bool
	}{
		{
			name: "all clusters in response — returns converted results",
			mockResponse: []estimatorclient.ComponentSetEstimationResponse{
				{Name: "cluster1", Sets: 50},
				{Name: "cluster2", Sets: 150},
				{Name: "cluster3", Sets: 250},
			},
			expectedResult: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 50},
				{Name: "cluster2", Replicas: 150},
				{Name: "cluster3", Replicas: 250},
			},
		},
		{
			name: "response in different order — result follows clusters slice order",
			mockResponse: []estimatorclient.ComponentSetEstimationResponse{
				{Name: "cluster3", Sets: 250},
				{Name: "cluster1", Sets: 50},
				{Name: "cluster2", Sets: 150},
			},
			expectedResult: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 50},
				{Name: "cluster2", Replicas: 150},
				{Name: "cluster3", Replicas: 250},
			},
		},
		{
			name: "unauthentic replica — cluster skipped in result",
			mockResponse: []estimatorclient.ComponentSetEstimationResponse{
				{Name: "cluster1", Sets: estimatorclient.UnauthenticReplica},
				{Name: "cluster2", Sets: 150},
				{Name: "cluster3", Sets: 250},
			},
			expectedResult: []workv1alpha2.TargetCluster{
				{Name: "cluster2", Replicas: 150},
				{Name: "cluster3", Replicas: 250},
			},
		},
		{
			name: "cluster missing from response — cluster absent from result",
			mockResponse: []estimatorclient.ComponentSetEstimationResponse{
				{Name: "cluster1", Sets: 50},
				// cluster2 missing from response
				{Name: "cluster3", Sets: 250},
			},
			expectedResult: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 50},
				{Name: "cluster3", Replicas: 250},
			},
		},
		{
			name:           "estimator error — returns nil result",
			mockResponse:   nil,
			mockError:      errors.New("estimator error"),
			expectedResult: nil,
			expectedError:  true,
		},
		{
			name:           "empty estimator response — returns empty result",
			mockResponse:   []estimatorclient.ComponentSetEstimationResponse{},
			expectedResult: []workv1alpha2.TargetCluster{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEstimator := &mockReplicaEstimator{
				maxAvailableComponentSetsResponse: tt.mockResponse,
				maxAvailableComponentSetsError:    tt.mockError,
			}

			result, err := calculateMultiTemplateAvailableSets(ctx, multiTemplateEstimationContext{
				estimator:        mockEstimator,
				estimatorName:    estimatorName,
				clusters:         clusters,
				spec:             spec,
				assumedWorkloads: nil,
			})

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func Test_buildAssumedWorkloadsByCluster(t *testing.T) {
	t.Run("nil cache returns empty map", func(t *testing.T) {
		clusters := []*clusterv1alpha1.Cluster{
			helper.NewCluster("cluster1"),
		}
		got := buildAssumedWorkloadsByCluster(clusters, nil)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("returns assumptions only for requested clusters", func(t *testing.T) {
		cache := schedulercache.NewCache(nil, nil, 0).AssigningResourceBindings()
		cache.Assume("default/rb1", "cluster1", schedulercache.AssumedWorkload{
			Namespace: "default",
			Components: []workv1alpha2.Component{
				{Name: "jobmanager", Replicas: 1},
			},
		})
		cache.Assume("default/rb2", "cluster2", schedulercache.AssumedWorkload{
			Namespace: "default",
			Components: []workv1alpha2.Component{
				{Name: "taskmanager", Replicas: 2},
			},
		})

		clusters := []*clusterv1alpha1.Cluster{
			helper.NewCluster("cluster1"),
			helper.NewCluster("cluster3"), // no assumption
		}
		got := buildAssumedWorkloadsByCluster(clusters, cache)

		assert.Len(t, got, 1)
		assert.Contains(t, got, "cluster1")
		assert.NotContains(t, got, "cluster2")
		assert.NotContains(t, got, "cluster3")

		cluster1Assumed := got["cluster1"]
		assert.Len(t, cluster1Assumed, 1)
		assert.Equal(t, "default", cluster1Assumed[0].Namespace)
		assert.Len(t, cluster1Assumed[0].Components, 1)
		assert.Equal(t, "jobmanager", cluster1Assumed[0].Components[0].Name)
		assert.Equal(t, int32(1), cluster1Assumed[0].Components[0].Replicas)
	})

	t.Run("empty clusters returns empty map", func(t *testing.T) {
		cache := schedulercache.NewCache(nil, nil, 0).AssigningResourceBindings()
		cache.Assume("default/rb1", "cluster1", schedulercache.AssumedWorkload{
			Namespace: "default",
			Components: []workv1alpha2.Component{
				{Name: "jobmanager", Replicas: 1},
			},
		})

		got := buildAssumedWorkloadsByCluster(nil, cache)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}
