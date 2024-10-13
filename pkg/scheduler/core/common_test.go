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

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

func TestSelectClusters(t *testing.T) {
	tests := []struct {
		name           string
		clustersScore  framework.ClusterScoreList
		placement      *policyv1alpha1.Placement
		spec           *workv1alpha2.ResourceBindingSpec
		expectedResult []*clusterv1alpha1.Cluster
		expectedError  bool
	}{
		{
			name: "select all clusters",
			clustersScore: framework.ClusterScoreList{
				{Cluster: &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster1"}}, Score: 10},
				{Cluster: &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster2"}}, Score: 20},
			},
			placement: &policyv1alpha1.Placement{},
			spec:      &workv1alpha2.ResourceBindingSpec{},
			expectedResult: []*clusterv1alpha1.Cluster{
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster2"}},
			},
			expectedError: false,
		},
		{
			name: "select top 1 cluster",
			clustersScore: framework.ClusterScoreList{
				{Cluster: &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster1"}}, Score: 10},
				{Cluster: &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster2"}}, Score: 20},
				{Cluster: &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster3"}}, Score: 15},
			},
			placement: &policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{"cluster1", "cluster2", "cluster3"},
				},
				SpreadConstraints: []policyv1alpha1.SpreadConstraint{
					{
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
						MaxGroups:     1,
					},
				},
			},
			spec: &workv1alpha2.ResourceBindingSpec{},
			expectedResult: []*clusterv1alpha1.Cluster{
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster2"}},
			},
			expectedError: false,
		},
		{
			name: "select clusters with affinity",
			clustersScore: framework.ClusterScoreList{
				{Cluster: &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster1"}}, Score: 10},
				{Cluster: &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster3"}}, Score: 15},
			},
			placement: &policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{"cluster1", "cluster3"},
				},
			},
			spec: &workv1alpha2.ResourceBindingSpec{},
			expectedResult: []*clusterv1alpha1.Cluster{
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster3"}},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SelectClusters(tt.clustersScore, tt.placement, tt.spec)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expectedResult), len(result))

				expectedNames := make([]string, len(tt.expectedResult))
				for i, cluster := range tt.expectedResult {
					expectedNames[i] = cluster.Name
				}

				actualNames := make([]string, len(result))
				for i, cluster := range result {
					actualNames[i] = cluster.Name
				}

				assert.ElementsMatch(t, expectedNames, actualNames)
			}
		})
	}
}

func TestAssignReplicas(t *testing.T) {
	tests := []struct {
		name           string
		clusters       []*clusterv1alpha1.Cluster
		spec           *workv1alpha2.ResourceBindingSpec
		status         *workv1alpha2.ResourceBindingStatus
		expectedResult []workv1alpha2.TargetCluster
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name: "Assign replicas to single cluster",
			clusters: []*clusterv1alpha1.Cluster{
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster1"}},
			},
			spec: &workv1alpha2.ResourceBindingSpec{
				Replicas: 3,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					},
				},
			},
			status:         &workv1alpha2.ResourceBindingStatus{},
			expectedResult: []workv1alpha2.TargetCluster{{Name: "cluster1", Replicas: 3}},
			expectedError:  false,
		},
		{
			name:           "No clusters available",
			clusters:       []*clusterv1alpha1.Cluster{},
			spec:           &workv1alpha2.ResourceBindingSpec{Replicas: 1},
			status:         &workv1alpha2.ResourceBindingStatus{},
			expectedResult: nil,
			expectedError:  true,
			expectedErrMsg: "no clusters available to schedule",
		},
		{
			name: "Non-workload scenario (zero replicas)",
			clusters: []*clusterv1alpha1.Cluster{
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster2"}},
			},
			spec: &workv1alpha2.ResourceBindingSpec{
				Replicas: 0,
			},
			status:         &workv1alpha2.ResourceBindingStatus{},
			expectedResult: []workv1alpha2.TargetCluster{{Name: "cluster1"}, {Name: "cluster2"}},
			expectedError:  false,
		},
		{
			name: "Unsupported replica scheduling strategy",
			clusters: []*clusterv1alpha1.Cluster{
				{Spec: clusterv1alpha1.ClusterSpec{ID: "cluster1"}},
			},
			spec: &workv1alpha2.ResourceBindingSpec{
				Replicas: 3,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     "UnsupportedType",
						ReplicaDivisionPreference: "UnsupportedPreference",
					},
				},
			},
			status:         &workv1alpha2.ResourceBindingStatus{},
			expectedResult: nil,
			expectedError:  true,
			expectedErrMsg: "unsupported replica scheduling strategy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AssignReplicas(tt.clusters, tt.spec, tt.status)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expectedResult), len(result))

				// Check if the total assigned replicas match the spec
				totalReplicas := int32(0)
				for i, cluster := range result {
					assert.Equal(t, tt.expectedResult[i].Name, cluster.Name)
					assert.Equal(t, tt.expectedResult[i].Replicas, cluster.Replicas)
					totalReplicas += cluster.Replicas
				}

				if tt.spec.Replicas > 0 {
					assert.Equal(t, tt.spec.Replicas, totalReplicas)
				}
			}
		})
	}
}
