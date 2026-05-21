/*
Copyright 2026 The Karmada Authors.

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

package client

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/estimator/pb"
	estimatorservice "github.com/karmada-io/karmada/pkg/estimator/service"
)

// fakeEstimatorClient is a hand-written stub for estimatorservice.EstimatorClient.
// It avoids testify/mock's Arguments.Diff, which formats proto messages via %v and
// triggers a lazyInitOnce deadlock caused by the incomplete rawDescGZIP in estimator.pb.go.
type fakeEstimatorClient struct {
	capturedReq *pb.MaxAvailableReplicasRequest
	maxReplicas int32
	err         error
}

func (f *fakeEstimatorClient) MaxAvailableReplicas(_ context.Context, in *pb.MaxAvailableReplicasRequest, _ ...grpc.CallOption) (*pb.MaxAvailableReplicasResponse, error) {
	f.capturedReq = in
	if f.err != nil {
		return nil, f.err
	}
	return &pb.MaxAvailableReplicasResponse{MaxReplicas: f.maxReplicas}, nil
}

func (f *fakeEstimatorClient) MaxAvailableComponentSets(_ context.Context, _ *pb.MaxAvailableComponentSetsRequest, _ ...grpc.CallOption) (*pb.MaxAvailableComponentSetsResponse, error) {
	return nil, nil
}

func (f *fakeEstimatorClient) GetUnschedulableReplicas(_ context.Context, _ *pb.UnschedulableReplicasRequest, _ ...grpc.CallOption) (*pb.UnschedulableReplicasResponse, error) {
	return nil, nil
}

// compile-time check
var _ estimatorservice.EstimatorClient = (*fakeEstimatorClient)(nil)

func Test_toAssumedWorkload(t *testing.T) {
	tests := []struct {
		name           string
		input          AssumedWorkload
		wantNS         string
		wantLen        int
		wantComponents func(t *testing.T, comps []*pb.Component)
		wantErr        bool
	}{
		{
			name: "no components",
			input: AssumedWorkload{
				Namespace:  "default",
				Components: nil,
			},
			wantNS:  "default",
			wantLen: 0,
			wantComponents: func(t *testing.T, comps []*pb.Component) {
				assert.Empty(t, comps)
			},
		},
		{
			name: "component with nil ReplicaRequirements",
			input: AssumedWorkload{
				Namespace: "ns1",
				Components: []workv1alpha2.Component{
					{
						Name:                "worker",
						Replicas:            3,
						ReplicaRequirements: nil,
					},
				},
			},
			wantNS:  "ns1",
			wantLen: 1,
			wantComponents: func(t *testing.T, comps []*pb.Component) {
				assert.Equal(t, "worker", comps[0].Name)
				assert.EqualValues(t, 3, comps[0].Replicas)
				assert.Nil(t, comps[0].ReplicaRequirements)
			},
		},
		{
			name: "component with ReplicaRequirements",
			input: AssumedWorkload{
				Namespace: "ns2",
				Components: []workv1alpha2.Component{
					{
						Name:     "server",
						Replicas: 2,
						ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
							ResourceRequest: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
					},
				},
			},
			wantNS:  "ns2",
			wantLen: 1,
			wantComponents: func(t *testing.T, comps []*pb.Component) {
				assert.Equal(t, "server", comps[0].Name)
				assert.EqualValues(t, 2, comps[0].Replicas)
				require.NotNil(t, comps[0].ReplicaRequirements)
			},
		},
		{
			name: "multiple components",
			input: AssumedWorkload{
				Namespace: "ns3",
				Components: []workv1alpha2.Component{
					{Name: "a", Replicas: 1, ReplicaRequirements: nil},
					{Name: "b", Replicas: 2, ReplicaRequirements: nil},
				},
			},
			wantNS:  "ns3",
			wantLen: 2,
			wantComponents: func(t *testing.T, comps []*pb.Component) {
				assert.Equal(t, "a", comps[0].Name)
				assert.Equal(t, "b", comps[1].Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toAssumedWorkload(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantNS, got.Namespace)
			assert.Len(t, got.Components, tt.wantLen)
			if tt.wantComponents != nil {
				tt.wantComponents(t, got.Components)
			}
		})
	}
}

func Test_maxAvailableReplicas_assumedWorkloads(t *testing.T) {
	const clusterName = "cluster-a"
	const wantMaxReplicas = int32(10)

	tests := []struct {
		name             string
		assumedWorkloads []AssumedWorkload
		wantPBCount      int
	}{
		{
			name:             "no assumed workloads — AssumedWorkloads field empty in request",
			assumedWorkloads: nil,
			wantPBCount:      0,
		},
		{
			name: "single assumed workload forwarded in gRPC request",
			assumedWorkloads: []AssumedWorkload{
				{
					Namespace: "default",
					Components: []workv1alpha2.Component{
						{Name: "web", Replicas: 2, ReplicaRequirements: nil},
					},
				},
			},
			wantPBCount: 1,
		},
		{
			name: "multiple assumed workloads forwarded in gRPC request",
			assumedWorkloads: []AssumedWorkload{
				{Namespace: "ns1", Components: []workv1alpha2.Component{{Name: "a", Replicas: 1}}},
				{Namespace: "ns2", Components: []workv1alpha2.Component{{Name: "b", Replicas: 3}}},
			},
			wantPBCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeEstimatorClient{maxReplicas: wantMaxReplicas}

			c := NewSchedulerEstimatorCache()
			c.AddCluster(clusterName, nil, fake)

			se := NewSchedulerEstimator(c, 5*time.Second)
			got, err := se.maxAvailableReplicas(context.Background(), clusterName, nil, tt.assumedWorkloads)

			require.NoError(t, err)
			assert.Equal(t, wantMaxReplicas, got)

			require.NotNil(t, fake.capturedReq, "expected gRPC request to be captured")
			assert.Len(t, fake.capturedReq.AssumedWorkloads, tt.wantPBCount)

			// Verify namespace is preserved for each assumed workload.
			for i, aw := range tt.assumedWorkloads {
				assert.Equal(t, aw.Namespace, fake.capturedReq.AssumedWorkloads[i].Namespace)
			}
		})
	}
}
