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
	componentSetRequests              []estimatorclient.ComponentSetEstimationRequest
	maxAvailableComponentSetsFunc     func(estimatorclient.ComponentSetEstimationRequest) ([]estimatorclient.ComponentSetEstimationResponse, error)
}

func (m *mockReplicaEstimator) MaxAvailableReplicas(_ context.Context, _ estimatorclient.ReplicaEstimationRequest) ([]workv1alpha2.TargetCluster, error) {
	return nil, nil
}

func (m *mockReplicaEstimator) MaxAvailableComponentSets(_ context.Context, req estimatorclient.ComponentSetEstimationRequest) ([]estimatorclient.ComponentSetEstimationResponse, error) {
	m.componentSetRequests = append(m.componentSetRequests, req)
	if m.maxAvailableComponentSetsFunc != nil {
		return m.maxAvailableComponentSetsFunc(req)
	}
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

func Test_componentReplicaScaleDirection(t *testing.T) {
	tests := []struct {
		name     string
		desired  []workv1alpha2.Component
		accepted []workv1alpha2.TargetComponent
		want     componentScaleDirection
	}{
		{
			name:     "equal snapshots ignore order",
			desired:  []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			accepted: []workv1alpha2.TargetComponent{{Name: "taskmanager", Replicas: 4}, {Name: "jobmanager", Replicas: 1}},
			want:     componentScaleEqual,
		},
		{
			name:     "pure scale up",
			desired:  []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6}},
			accepted: []workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			want:     componentScaleUp,
		},
		{
			name:     "pure scale down",
			desired:  []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 2}},
			accepted: []workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			want:     componentScaleDown,
		},
		{
			name:     "mixed scale",
			desired:  []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6}},
			accepted: []workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 2}, {Name: "taskmanager", Replicas: 4}},
			want:     componentScaleMixed,
		},
		{
			name:    "missing accepted snapshot",
			desired: []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}},
			want:    componentScaleUnknown,
		},
		{
			name:     "component names differ",
			desired:  []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			accepted: []workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "historyserver", Replicas: 4}},
			want:     componentScaleUnknown,
		},
		{
			name:     "accepted names are duplicated",
			desired:  []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			accepted: []workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "jobmanager", Replicas: 4}},
			want:     componentScaleUnknown,
		},
		{
			name:     "desired names are duplicated",
			desired:  []workv1alpha2.Component{{Name: "worker", Replicas: 2}, {Name: "worker", Replicas: 3}},
			accepted: []workv1alpha2.TargetComponent{{Name: "worker", Replicas: 2}, {Name: "server", Replicas: 1}},
			want:     componentScaleUnknown,
		},
		{
			name:     "desired name is empty",
			desired:  []workv1alpha2.Component{{Name: "", Replicas: 1}},
			accepted: []workv1alpha2.TargetComponent{{Name: "", Replicas: 1}},
			want:     componentScaleUnknown,
		},
		{
			name: "empty snapshots",
			want: componentScaleUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, componentReplicaScaleDirection(tt.desired, tt.accepted))
		})
	}
}

func Test_calculateMultiTemplateAvailableSetsForScale(t *testing.T) {
	clusters := []*clusterv1alpha1.Cluster{helper.NewCluster("cluster1")}
	newSpec := func(desired []workv1alpha2.Component, accepted []workv1alpha2.TargetComponent) *workv1alpha2.ResourceBindingSpec {
		return &workv1alpha2.ResourceBindingSpec{
			Resource:   workv1alpha2.ObjectReference{Namespace: "default", Name: "flink"},
			Components: desired,
			Clusters:   []workv1alpha2.TargetCluster{{Name: "cluster1", Components: accepted}},
		}
	}

	tests := []struct {
		name           string
		spec           *workv1alpha2.ResourceBindingSpec
		wantComponents []workv1alpha2.Component
		wantResult     []workv1alpha2.TargetCluster
		wantError      string
		wantCalls      int
		estimatorError error
	}{
		{
			name: "scale up estimates only the positive delta",
			spec: newSpec(
				[]workv1alpha2.Component{
					{Name: "jobmanager", Replicas: 1},
					{Name: "taskmanager", Replicas: 6, ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{PriorityClassName: "high-priority"}},
				},
				[]workv1alpha2.TargetComponent{{Name: "taskmanager", Replicas: 4}, {Name: "jobmanager", Replicas: 1}},
			),
			wantComponents: []workv1alpha2.Component{{
				Name:                "taskmanager",
				Replicas:            2,
				ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{PriorityClassName: "high-priority"},
			}},
			wantResult: []workv1alpha2.TargetCluster{{Name: "cluster1", Replicas: 1}},
			wantCalls:  1,
		},
		{
			name: "pure scale down skips estimation",
			spec: newSpec(
				[]workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 2}},
				[]workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			),
			wantResult: []workv1alpha2.TargetCluster{{Name: "cluster1", Replicas: minimumAvailableComponentSets}},
		},
		{
			name: "all components scale up by their positive delta",
			spec: newSpec(
				[]workv1alpha2.Component{{Name: "jobmanager", Replicas: 2}, {Name: "taskmanager", Replicas: 6}},
				[]workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			),
			wantComponents: []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 2}},
			wantResult:     []workv1alpha2.TargetCluster{{Name: "cluster1", Replicas: 1}},
			wantCalls:      1,
		},
		{
			name: "scale up returns estimator error",
			spec: newSpec(
				[]workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6}},
				[]workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			),
			wantComponents: []workv1alpha2.Component{{Name: "taskmanager", Replicas: 2}},
			wantError:      "estimator failed",
			wantCalls:      1,
			estimatorError: errors.New("estimator failed"),
		},
		{
			name: "existing target without accepted snapshot is unknown",
			spec: newSpec(
				[]workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6}},
				nil,
			),
			wantError: "requires a comparable accepted component snapshot",
		},
		{
			name: "equal state is not a scale operation",
			spec: newSpec(
				[]workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
				[]workv1alpha2.TargetComponent{{Name: "taskmanager", Replicas: 4}, {Name: "jobmanager", Replicas: 1}},
			),
			wantError: "requires a replica change",
		},
		{
			name: "mixed scale is unsupported",
			spec: newSpec(
				[]workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6}},
				[]workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 2}, {Name: "taskmanager", Replicas: 4}},
			),
			wantError: "mixed component scaling is not supported",
		},
		{
			name: "duplicate desired names are rejected",
			spec: newSpec(
				[]workv1alpha2.Component{{Name: "worker", Replicas: 2}, {Name: "worker", Replicas: 3}},
				[]workv1alpha2.TargetComponent{{Name: "worker", Replicas: 2}, {Name: "server", Replicas: 1}},
			),
			wantError: "unique, non-empty names",
		},
		{
			name: "empty desired name is rejected",
			spec: newSpec(
				[]workv1alpha2.Component{{Name: "", Replicas: 2}},
				[]workv1alpha2.TargetComponent{{Name: "", Replicas: 1}},
			),
			wantError: "unique, non-empty names",
		},
		{
			name: "partial accepted snapshot is unknown",
			spec: newSpec(
				[]workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6}},
				[]workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}},
			),
			wantError: "requires a comparable accepted component snapshot",
		},
		{
			name:      "empty desired snapshot is rejected",
			spec:      newSpec(nil, nil),
			wantError: "unique, non-empty names",
		},
		{
			name: "candidate without accepted result is rejected",
			spec: &workv1alpha2.ResourceBindingSpec{
				Resource:   workv1alpha2.ObjectReference{Namespace: "default", Name: "flink"},
				Components: []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6}},
				Clusters: []workv1alpha2.TargetCluster{{Name: "cluster2", Components: []workv1alpha2.TargetComponent{
					{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
				}}},
			},
			wantError: "requires exactly one accepted target cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimator := &mockReplicaEstimator{}
			estimator.maxAvailableComponentSetsFunc = func(req estimatorclient.ComponentSetEstimationRequest) ([]estimatorclient.ComponentSetEstimationResponse, error) {
				assert.Equal(t, clusters, req.Clusters)
				assert.Equal(t, tt.wantComponents, req.Components)
				if tt.estimatorError != nil {
					return nil, tt.estimatorError
				}
				return []estimatorclient.ComponentSetEstimationResponse{{Name: "cluster1", Sets: 1}}, nil
			}

			got, err := calculateMultiTemplateAvailableSetsForScale(context.Background(), multiTemplateEstimationContext{
				estimator: estimator,
				clusters:  clusters,
				spec:      tt.spec,
			})
			if tt.wantError != "" {
				if assert.Error(t, err) {
					assert.Contains(t, err.Error(), tt.wantError)
				}
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResult, got)
			}
			assert.Len(t, estimator.componentSetRequests, tt.wantCalls)
		})
	}
}

func Test_calculateMultiTemplateAvailableSetsForScaleRejectsBeforeCallingEstimator(t *testing.T) {
	clusters := []*clusterv1alpha1.Cluster{helper.NewCluster("cluster1")}
	estimator := &mockReplicaEstimator{}
	estimator.maxAvailableComponentSetsFunc = func(estimatorclient.ComponentSetEstimationRequest) ([]estimatorclient.ComponentSetEstimationResponse, error) {
		t.Fatal("estimator must not be called before every transition is classified")
		return nil, nil
	}
	spec := &workv1alpha2.ResourceBindingSpec{
		Resource: workv1alpha2.ObjectReference{Namespace: "default", Name: "flink"},
		Components: []workv1alpha2.Component{
			{Name: "jobmanager", Replicas: 1},
			{Name: "taskmanager", Replicas: 6},
		},
		Clusters: []workv1alpha2.TargetCluster{{
			Name: "cluster1",
			Components: []workv1alpha2.TargetComponent{
				{Name: "jobmanager", Replicas: 2},
				{Name: "taskmanager", Replicas: 4},
			},
		}},
	}

	got, err := calculateMultiTemplateAvailableSetsForScale(context.Background(), multiTemplateEstimationContext{
		estimator: estimator,
		clusters:  clusters,
		spec:      spec,
	})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "mixed component scaling is not supported")
	}
	assert.Nil(t, got)
	assert.Empty(t, estimator.componentSetRequests)
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
