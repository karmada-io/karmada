/*
Copyright 2022 The Karmada Authors.

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
	"fmt"
	"maps"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/estimator/pb"
	estimatorservice "github.com/karmada-io/karmada/pkg/estimator/service"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/scheduler/core/spreadconstraint"
	"github.com/karmada-io/karmada/test/helper"
)

func Test_attachZeroReplicasCluster(t *testing.T) {
	type args struct {
		clusters       []spreadconstraint.ClusterDetailInfo
		targetClusters []workv1alpha2.TargetCluster
	}
	tests := []struct {
		name string
		args args
		want []workv1alpha2.TargetCluster
	}{
		{
			name: "clusters: member1,member2,member3, targetClusters:member1,member2",
			args: args{
				clusters: []spreadconstraint.ClusterDetailInfo{
					{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
					{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
					{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				},
				targetClusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 1,
					},
					{
						Name:     ClusterMember2,
						Replicas: 2,
					},
				},
			},
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 1,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 0,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := attachZeroReplicasCluster(tt.args.clusters, tt.args.targetClusters); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attachZeroReplicasCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}

func snapshotReplicaEstimators(t *testing.T) {
	t.Helper()
	estimators := estimatorclient.GetReplicaEstimators()
	snapshot := make(map[string]estimatorclient.ReplicaEstimator, len(estimators))
	maps.Copy(snapshot, estimators)
	t.Cleanup(func() {
		for name := range estimators {
			delete(estimators, name)
		}
		maps.Copy(estimators, snapshot)
	})
}

func Test_calAvailableReplicas_partialEstimatorFailure(t *testing.T) {
	snapshotReplicaEstimators(t)
	estimators := estimatorclient.GetReplicaEstimators()

	clusters := []*clusterv1alpha1.Cluster{helper.NewCluster("cluster1"), helper.NewCluster("cluster2")}
	spec := &workv1alpha2.ResourceBindingSpec{Replicas: 5}
	unavailable := errors.New("cluster1 unavailable")

	tests := []struct {
		name        string
		general     []workv1alpha2.TargetCluster
		accurate    []workv1alpha2.TargetCluster
		accurateErr error
		want        []workv1alpha2.TargetCluster
	}{
		{
			name: "one accurate cluster fails, the other keeps its accurate estimate",
			general: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 10},
			},
			accurate: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: estimatorclient.UnauthenticReplica},
				{Name: "cluster2", Replicas: 3},
			},
			accurateErr: unavailable,
			want: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 3},
			},
		},
		{
			name: "accurate estimate above the general estimate keeps the minimum",
			general: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 10},
			},
			accurate: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 20},
				{Name: "cluster2", Replicas: 20},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 10},
			},
		},
		{
			name: "zero accurate estimate is authoritative",
			general: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 10},
			},
			accurate: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 0},
				{Name: "cluster2", Replicas: 0},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 0},
				{Name: "cluster2", Replicas: 0},
			},
		},
		{
			name: "zero estimate alongside a failure is authoritative",
			general: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 10},
			},
			accurate: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: estimatorclient.UnauthenticReplica},
				{Name: "cluster2", Replicas: 0},
			},
			accurateErr: unavailable,
			want: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 0},
			},
		},
		{
			name: "zero-filled result with an error is not trusted",
			general: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 10},
			},
			accurate: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 0},
				{Name: "cluster2", Replicas: 0},
			},
			accurateErr: unavailable,
			want: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 10},
			},
		},
		{
			name: "cluster absent from every estimator falls back to requested replicas",
			general: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
			},
			accurate: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: estimatorclient.UnauthenticReplica},
			},
			accurateErr: unavailable,
			want: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 5},
			},
		},
		{
			name: "all accurate clusters fail, general results are retained",
			general: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 10},
			},
			accurate: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: estimatorclient.UnauthenticReplica},
				{Name: "cluster2", Replicas: estimatorclient.UnauthenticReplica},
			},
			accurateErr: unavailable,
			want: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 10},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimators["general-estimator"] = &mockReplicaEstimator{maxAvailableReplicasResponse: tt.general}
			estimators["scheduler-estimator"] = &mockReplicaEstimator{
				maxAvailableReplicasResponse: tt.accurate,
				maxAvailableReplicasError:    tt.accurateErr,
			}

			got := calAvailableReplicas(clusters, spec, nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_mergeReplicaResults(t *testing.T) {
	available := []workv1alpha2.TargetCluster{
		{Name: "cluster1", Replicas: 10},
		{Name: "cluster2", Replicas: 10},
		{Name: "cluster3", Replicas: 100},
	}
	res := []workv1alpha2.TargetCluster{
		{Name: "cluster2", Replicas: estimatorclient.UnauthenticReplica},
		{Name: "cluster1", Replicas: 3},
		{Name: "cluster1", Replicas: 5},
		{Name: "cluster4", Replicas: 5},
	}
	want := []workv1alpha2.TargetCluster{
		{Name: "cluster1", Replicas: 3},
		{Name: "cluster2", Replicas: 10},
		{Name: "cluster3", Replicas: 100},
	}

	assert.Equal(t, want, mergeReplicaResults(available, res))
}

func Test_calAvailableReplicas_componentSetPartialFailure(t *testing.T) {
	snapshotReplicaEstimators(t)
	original := features.FeatureGate.Enabled(features.MultiplePodTemplatesScheduling)
	t.Cleanup(func() {
		if err := features.FeatureGate.Set(fmt.Sprintf("%s=%t", features.MultiplePodTemplatesScheduling, original)); err != nil {
			t.Errorf("restore %s: %v", features.MultiplePodTemplatesScheduling, err)
		}
	})
	if err := features.FeatureGate.Set(fmt.Sprintf("%s=true", features.MultiplePodTemplatesScheduling)); err != nil {
		t.Fatalf("enable %s: %v", features.MultiplePodTemplatesScheduling, err)
	}

	estimators := estimatorclient.GetReplicaEstimators()
	clusters := []*clusterv1alpha1.Cluster{helper.NewCluster("cluster1"), helper.NewCluster("cluster2")}
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
	unavailable := errors.New("cluster1 unavailable")

	tests := []struct {
		name        string
		accurate    []estimatorclient.ComponentSetEstimationResponse
		accurateErr error
		want        []workv1alpha2.TargetCluster
	}{
		{
			name: "partial failure keeps the successful cluster estimate",
			accurate: []estimatorclient.ComponentSetEstimationResponse{
				{Name: "cluster1", Sets: estimatorclient.UnauthenticReplica},
				{Name: "cluster2", Sets: 3},
			},
			accurateErr: unavailable,
			want: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 3},
			},
		},
		{
			name: "zero estimate alongside a failure is authoritative",
			accurate: []estimatorclient.ComponentSetEstimationResponse{
				{Name: "cluster1", Sets: estimatorclient.UnauthenticReplica},
				{Name: "cluster2", Sets: 0},
			},
			accurateErr: unavailable,
			want: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 0},
			},
		},
		{
			name: "zero-only result with an error falls back to the general estimator",
			accurate: []estimatorclient.ComponentSetEstimationResponse{
				{Name: "cluster1", Sets: 0},
				{Name: "cluster2", Sets: 0},
			},
			accurateErr: unavailable,
			want: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 10},
				{Name: "cluster2", Replicas: 10},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimators["general-estimator"] = &mockReplicaEstimator{
				maxAvailableComponentSetsResponse: []estimatorclient.ComponentSetEstimationResponse{
					{Name: "cluster1", Sets: 10},
					{Name: "cluster2", Sets: 10},
				},
			}
			estimators["scheduler-estimator"] = &mockReplicaEstimator{
				maxAvailableComponentSetsResponse: tt.accurate,
				maxAvailableComponentSetsError:    tt.accurateErr,
			}

			got := calAvailableReplicas(clusters, spec, nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

type fakeReplicaEstimatorClient struct {
	maxReplicas int32
}

func (f *fakeReplicaEstimatorClient) MaxAvailableReplicas(context.Context, *pb.MaxAvailableReplicasRequest, ...grpc.CallOption) (*pb.MaxAvailableReplicasResponse, error) {
	return &pb.MaxAvailableReplicasResponse{MaxReplicas: f.maxReplicas}, nil
}

func (f *fakeReplicaEstimatorClient) MaxAvailableComponentSets(context.Context, *pb.MaxAvailableComponentSetsRequest, ...grpc.CallOption) (*pb.MaxAvailableComponentSetsResponse, error) {
	return &pb.MaxAvailableComponentSetsResponse{}, nil
}

func (f *fakeReplicaEstimatorClient) GetUnschedulableReplicas(context.Context, *pb.UnschedulableReplicasRequest, ...grpc.CallOption) (*pb.UnschedulableReplicasResponse, error) {
	return &pb.UnschedulableReplicasResponse{}, nil
}

var _ estimatorservice.EstimatorClient = (*fakeReplicaEstimatorClient)(nil)

func Test_calAvailableReplicas_realSchedulerEstimatorPartialFailure(t *testing.T) {
	snapshotReplicaEstimators(t)
	estimators := estimatorclient.GetReplicaEstimators()
	estimators["general-estimator"] = &mockReplicaEstimator{
		maxAvailableReplicasResponse: []workv1alpha2.TargetCluster{
			{Name: "cluster1", Replicas: 10},
			{Name: "cluster2", Replicas: 10},
		},
	}

	cache := estimatorclient.NewSchedulerEstimatorCache()
	cache.AddCluster("cluster2", nil, &fakeReplicaEstimatorClient{maxReplicas: 3})
	estimators["scheduler-estimator"] = estimatorclient.NewSchedulerEstimator(cache, time.Second)

	clusters := []*clusterv1alpha1.Cluster{helper.NewCluster("cluster1"), helper.NewCluster("cluster2")}
	spec := &workv1alpha2.ResourceBindingSpec{Replicas: 5}

	got := calAvailableReplicas(clusters, spec, nil)
	assert.Equal(t, []workv1alpha2.TargetCluster{
		{Name: "cluster1", Replicas: 10},
		{Name: "cluster2", Replicas: 3},
	}, got)
}
