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

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
)

func TestMockEstimatorClientGetUnschedulableReplicas(t *testing.T) {
	testCases := []struct {
		name          string
		request       *pb.UnschedulableReplicasRequest
		setupMock     func(*MockEstimatorClient)
		expectedResp  *pb.UnschedulableReplicasResponse
		expectedError error
	}{
		{
			name: "successful response",
			request: &pb.UnschedulableReplicasRequest{
				Cluster: "cluster-1",
				Resource: pb.ObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Namespace:  "default",
					Name:       "nginx",
				},
				UnschedulableThreshold: 5 * time.Minute,
			},
			setupMock: func(m *MockEstimatorClient) {
				expectedResp := &pb.UnschedulableReplicasResponse{
					UnschedulableReplicas: 3,
				}
				m.On("GetUnschedulableReplicas", context.Background(),
					&pb.UnschedulableReplicasRequest{
						Cluster: "cluster-1",
						Resource: pb.ObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Namespace:  "default",
							Name:       "nginx",
						},
						UnschedulableThreshold: 5 * time.Minute,
					}).Return(expectedResp, nil)
			},
			expectedResp: &pb.UnschedulableReplicasResponse{
				UnschedulableReplicas: 3,
			},
			expectedError: nil,
		},
		{
			name: "error response",
			request: &pb.UnschedulableReplicasRequest{
				Cluster: "invalid-cluster",
				Resource: pb.ObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Namespace:  "default",
					Name:       "nginx",
				},
			},
			setupMock: func(m *MockEstimatorClient) {
				m.On("GetUnschedulableReplicas", context.Background(),
					&pb.UnschedulableReplicasRequest{
						Cluster: "invalid-cluster",
						Resource: pb.ObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Namespace:  "default",
							Name:       "nginx",
						},
					}).Return(nil, errors.New("cluster not found"))
			},
			expectedResp:  nil,
			expectedError: errors.New("cluster not found"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &MockEstimatorClient{}
			tc.setupMock(client)

			resp, err := client.GetUnschedulableReplicas(context.Background(), tc.request)

			assert.Equal(t, tc.expectedResp, resp)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMockEstimatorClientMaxAvailableReplicas(t *testing.T) {
	testCases := []struct {
		name          string
		request       *pb.MaxAvailableReplicasRequest
		setupMock     func(*MockEstimatorClient)
		expectedResp  *pb.MaxAvailableReplicasResponse
		expectedError error
	}{
		{
			name: "successful response",
			request: &pb.MaxAvailableReplicasRequest{
				Cluster: "cluster-1",
				ReplicaRequirements: pb.ReplicaRequirements{
					NodeClaim: &pb.NodeClaim{
						NodeSelector: map[string]string{
							"zone": "us-east-1",
						},
						Tolerations: []corev1.Toleration{
							{
								Key:      "key1",
								Operator: corev1.TolerationOpEqual,
								Value:    "value1",
								Effect:   corev1.TaintEffectNoSchedule,
							},
						},
					},
					ResourceRequest: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
					Namespace:         "default",
					PriorityClassName: "high-priority",
				},
			},
			setupMock: func(m *MockEstimatorClient) {
				expectedResp := &pb.MaxAvailableReplicasResponse{
					MaxReplicas: 10,
				}
				m.On("MaxAvailableReplicas", context.Background(),
					mock.MatchedBy(func(req *pb.MaxAvailableReplicasRequest) bool {
						return req.Cluster == "cluster-1" &&
							req.ReplicaRequirements.Namespace == "default"
					})).Return(expectedResp, nil)
			},
			expectedResp: &pb.MaxAvailableReplicasResponse{
				MaxReplicas: 10,
			},
			expectedError: nil,
		},
		{
			name: "error response",
			request: &pb.MaxAvailableReplicasRequest{
				Cluster: "invalid-cluster",
				ReplicaRequirements: pb.ReplicaRequirements{
					Namespace: "default",
				},
			},
			setupMock: func(m *MockEstimatorClient) {
				m.On("MaxAvailableReplicas", context.Background(),
					mock.MatchedBy(func(req *pb.MaxAvailableReplicasRequest) bool {
						return req.Cluster == "invalid-cluster"
					})).Return(nil, errors.New("cluster not found"))
			},
			expectedResp:  nil,
			expectedError: errors.New("cluster not found"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &MockEstimatorClient{}
			tc.setupMock(client)

			resp, err := client.MaxAvailableReplicas(context.Background(), tc.request)

			assert.Equal(t, tc.expectedResp, resp)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
