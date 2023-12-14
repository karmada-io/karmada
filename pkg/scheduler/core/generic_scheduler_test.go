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
	name                      string
	clusters                  []*clusterv1alpha1.Cluster
	object                    workv1alpha2.ResourceBindingSpec
	placement                 *policyv1alpha1.Placement
	previousResultToNewResult map[string][]string
	wantErr                   bool
}

var clusterToIndex = map[string]int{ClusterMember1: 0, ClusterMember2: 1, ClusterMember3: 2, ClusterMember4: 3}
var indexToCluster = []string{ClusterMember1, ClusterMember2, ClusterMember3, ClusterMember4}

func targetClusterToString(tcs []workv1alpha2.TargetCluster) string {
	res := make([]string, len(tcs))
	for _, cluster := range tcs {
		idx := clusterToIndex[cluster.Name]
		res[idx] = strconv.Itoa(int(cluster.Replicas))
	}
	return strings.Join(res, ":")
}

func stringToTargetCluster(str string) []workv1alpha2.TargetCluster {
	arr := strings.Split(str, ":")
	tcs := make([]workv1alpha2.TargetCluster, len(arr))
	for i, replicas := range arr {
		num, _ := strconv.Atoi(replicas)
		tcs[i].Replicas = int32(num) // #nosec G109
		tcs[i].Name = indexToCluster[i]
	}
	return tcs
}

// These are acceptance test cases given by QA for requirement: dividing replicas by static weight evenly
// https://github.com/karmada-io/karmada/issues/4220
func Test_EvenDistributionOfReplicas(t *testing.T) {
	tests := []testcase{
		// Test Case No.1 of even distribution of replicas
		// 1. create deployment (replicas=3), weight=1:1
		// 2. check two member cluster replicas, should be 2:1 or 1:2
		{
			name: "replica 3, static weighted 1:1",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
			},
			object: workv1alpha2.ResourceBindingSpec{
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
			previousResultToNewResult: map[string][]string{
				"": {"1:2", "2:1"},
			},
			wantErr: false,
		},
		// Test Case No.2 of even distribution of replicas
		// 1. create deployment (replicas=3), weight=1:1:1
		// 2. check three member cluster replicas, should be 1:1:1
		// 3. update replicas from 3 to 5
		// 4. check three member cluster replicas, should be 2:2:1 or 2:1:2 or 1:2:2
		{
			name: "replica 3, static weighted 1:1:1, change replicas from 3 to 5, before change",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
				helper.NewCluster(ClusterMember3),
			},
			object: workv1alpha2.ResourceBindingSpec{
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
			previousResultToNewResult: map[string][]string{
				"": {"1:1:1"},
			},
			wantErr: false,
		},
		{
			name: "replica 3, static weighted 1:1:1, change replicas from 3 to 5, after change",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
				helper.NewCluster(ClusterMember3),
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 5, // change replicas from 3 to 5
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
			previousResultToNewResult: map[string][]string{
				"1:1:1": {"2:2:1", "2:1:2", "1:2:2"},
			},
			wantErr: false,
		},
		// Test Case No.3 of even distribution of replicas
		// 1. create deployment (replicas=7), weight=2:1:1:1
		// 2. check four member cluster replicas, can be 3:2:1:1、3:1:2:1、3:1:1:2
		// 3. update replicas from 7 to 8
		// 4. check four member cluster replicas, the added replica should lie in first cluster, i.e:
		//    * 3:2:1:1 --> 4:2:1:1
		//    * 3:1:2:1 --> 4:1:2:1
		//    * 3:1:1:2 --> 4:1:1:2
		{
			name: "replica 7, static weighted 2:1:1:1, change replicas from 7 to 8, before change",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
				helper.NewCluster(ClusterMember3),
				helper.NewCluster(ClusterMember4),
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 7,
			},
			placement: &policyv1alpha1.Placement{
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
						StaticWeightList: []policyv1alpha1.StaticClusterWeight{
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
						},
					},
				},
			},
			previousResultToNewResult: map[string][]string{
				"": {"3:2:1:1", "3:1:2:1", "3:1:1:2"},
			},
			wantErr: false,
		},
		{
			name: "replica 7, static weighted 2:1:1:1, change replicas from 7 to 8, after change",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
				helper.NewCluster(ClusterMember3),
				helper.NewCluster(ClusterMember4),
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 8, // change replicas from 7 to 8
			},
			placement: &policyv1alpha1.Placement{
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
						StaticWeightList: []policyv1alpha1.StaticClusterWeight{
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
						},
					},
				},
			},
			previousResultToNewResult: map[string][]string{
				"3:2:1:1": {"4:2:1:1"},
				"3:1:2:1": {"4:1:2:1"},
				"3:1:1:2": {"4:1:1:2"},
			},
			wantErr: false,
		},
		// Test Case No.4 of even distribution of replicas
		// 1. create deployment (replicas=9), weight=2:1:1:1
		// 2. check four member cluster replicas, can be 4:2:2:1、4:1:2:2、4:2:1:2
		// 3. update replicas from 9 to 8
		// 4. check four member cluster replicas, the reduced replica should be scaled down from cluster with 2 replicas previously, i.e:
		//    * 4:2:2:1 --> 4:2:1:1 or 4:1:2:1
		//    * 4:1:2:2 --> 4:1:1:2 or 4:1:2:1
		//    * 4:2:1:2 --> 4:1:1:2 or 4:2:1:1
		{
			name: "replica 9, static weighted 2:1:1:1, change replicas from 9 to 8, before change",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
				helper.NewCluster(ClusterMember3),
				helper.NewCluster(ClusterMember4),
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 9,
			},
			placement: &policyv1alpha1.Placement{
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
						StaticWeightList: []policyv1alpha1.StaticClusterWeight{
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
						},
					},
				},
			},
			previousResultToNewResult: map[string][]string{
				"": {"4:2:2:1", "4:1:2:2", "4:2:1:2"},
			},
			wantErr: false,
		},
		{
			name: "replica 9, static weighted 2:1:1:1, change replicas from 9 to 8, after change",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
				helper.NewCluster(ClusterMember3),
				helper.NewCluster(ClusterMember4),
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 8,
			},
			placement: &policyv1alpha1.Placement{
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
						StaticWeightList: []policyv1alpha1.StaticClusterWeight{
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
							{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
						},
					},
				},
			},
			previousResultToNewResult: map[string][]string{
				"4:2:2:1": {"4:2:1:1", "4:1:2:1"},
				"4:1:2:2": {"4:1:1:2", "4:1:2:1"},
				"4:2:1:2": {"4:1:1:2", "4:2:1:1"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var g = &genericScheduler{}
			for previous, expect := range tt.previousResultToNewResult {
				obj := tt.object

				// 1. get previous schedule result, if previous == "" means no previous schedule result
				if previous != "" {
					obj.Clusters = stringToTargetCluster(previous)
				}

				// 2. schedule basing on previous schedule result
				got, err := g.assignReplicas(tt.clusters, tt.placement, &obj)
				if (err != nil) != tt.wantErr {
					t.Errorf("AssignReplicas() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				// 3. check if schedule result got match to expected
				gotResult := targetClusterToString(got)
				for _, expectResult := range expect {
					if gotResult == expectResult {
						return
					}
				}
				t.Errorf("AssignReplicas() got = %v, wants %s --> %v", got, gotResult, expect)
			}
		})
	}
}
