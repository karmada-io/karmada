/*
Copyright 2023 The Karmada Authors.

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

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/test/helper"
)

// Test Case of even distribution of replicas, case 1
// 1. create deployment (replicas=3), weight=1:1
// 2. check two member cluster replicas, should be 2:1 or 1:2
func Test_genericScheduler_AssignReplicas(t *testing.T) {
	tests := []struct {
		name      string
		clusters  []*clusterv1alpha1.Cluster
		placement *policyv1alpha1.Placement
		object    *workv1alpha2.ResourceBindingSpec
		wants     [][]workv1alpha2.TargetCluster
		wantErr   bool
	}{
		{
			name: "replica 3, static weighted 1:1",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
			},
			object: &workv1alpha2.ResourceBindingSpec{
				Replicas: 3,
			},
			placement: &policyv1alpha1.Placement{
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
						StaticWeightList: []policyv1alpha1.StaticClusterWeight{
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
						},
					},
				},
			},
			wants: [][]workv1alpha2.TargetCluster{
				{
					{Name: ClusterMember1, Replicas: 1},
					{Name: ClusterMember2, Replicas: 2},
				},
				{
					{Name: ClusterMember1, Replicas: 2},
					{Name: ClusterMember2, Replicas: 1},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &genericScheduler{}
			got, err := g.assignReplicas(tt.clusters, tt.placement, tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("AssignReplicas() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for _, want := range tt.wants {
				if helper.IsScheduleResultEqual(got, want) {
					return
				}
			}
			t.Errorf("AssignReplicas() got = %v, wants %v", got, tt.wants)
		})
	}
}
