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
	"strconv"
	"strings"
	"testing"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/test/helper"
)

type testcase struct {
	name      string
	clusters  []*clusterv1alpha1.Cluster
	object    *workv1alpha2.ResourceBindingSpec
	placement *policyv1alpha1.Placement
	// changeCondForNextSchedule following cases will schedule twice, this function aims to change the schedule condition for second schedule
	changeCondForNextSchedule func(tt *testcase)
	// wants key is first time schedule possible result, value is second time schedule possible result
	wants   map[string][]string
	wantErr bool
}

var clusterToIndex = map[string]int{
	ClusterMember1: 0,
	ClusterMember2: 1,
	ClusterMember3: 2,
	ClusterMember4: 3,
}

// isScheduleResultEqual change []workv1alpha2.TargetCluster to the string format just like 1:1:1, and compare it to expect string
func isScheduleResultEqual(tcs []workv1alpha2.TargetCluster, expect string) bool {
	res := make([]string, len(tcs))
	for _, cluster := range tcs {
		idx := clusterToIndex[cluster.Name]
		res[idx] = strconv.Itoa(int(cluster.Replicas))
	}
	actual := strings.Join(res, ":")
	return actual == expect
}

// These are acceptance test cases given by QA for requirement: dividing replicas by static weight evenly
// https://github.com/karmada-io/karmada/issues/4220
func Test_EvenDistributionOfReplicas(t *testing.T) {
	tests := []*testcase{
		{
			// Test Case No.1 of even distribution of replicas
			// 1. create deployment (replicas=3), weight=1:1
			// 2. check two member cluster replicas, should be 2:1 or 1:2
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
			changeCondForNextSchedule: nil,
			wants: map[string][]string{
				"1:2": {},
				"2:1": {},
			},
			wantErr: false,
		},
		{
			// Test Case No.2 of even distribution of replicas
			// 1. create deployment (replicas=3), weight=1:1:1
			// 2. check three member cluster replicas, should be 1:1:1
			// 3. update replicas from 3 to 5
			// 4. check three member cluster replicas, should be 2:2:1 or 2:1:2 or 1:2:2
			name: "replica 3, static weighted 1:1:1, change replicas from 3 to 5",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
				helper.NewCluster(ClusterMember3),
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
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
						},
					},
				},
			},
			changeCondForNextSchedule: func(tt *testcase) {
				tt.object.Replicas = 5
			},
			wants: map[string][]string{
				"1:1:1": {"2:2:1", "2:1:2", "1:2:2"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var g = &genericScheduler{}
			var firstScheduleResult, secondScheduleResult string

			// 1. schedule for the first time, and check whether first schedule result within tt.wants
			got, err := g.assignReplicas(tt.clusters, tt.placement, tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("AssignReplicas() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for firstScheduleResult = range tt.wants {
				if isScheduleResultEqual(got, firstScheduleResult) {
					break
				}
			}
			if !isScheduleResultEqual(got, firstScheduleResult) {
				t.Errorf("AssignReplicas() got = %v, wants %v", got, tt.wants)
				return
			}

			// 2. change the schedule condition
			if tt.changeCondForNextSchedule == nil {
				return
			}
			tt.changeCondForNextSchedule(tt)

			// 3. schedule for the second time, and check whether second schedule result within tt.wants
			got, err = g.assignReplicas(tt.clusters, tt.placement, tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("AssignReplicas() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, secondScheduleResult = range tt.wants[firstScheduleResult] {
				if isScheduleResultEqual(got, secondScheduleResult) {
					return
				}
			}
			t.Errorf("AssignReplicas() got = %v, wants %v", got, tt.wants)
		})
	}
}
