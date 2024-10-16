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

package spreadconstraint

import (
	"fmt"
	"testing"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

func generateClusterScore() framework.ClusterScoreList {
	return framework.ClusterScoreList{
		{
			Cluster: NewClusterWithTopology("member1", "P1", "R1", "Z1"),
			Score:   20,
		},
		{
			Cluster: NewClusterWithTopology("member2", "P1", "R1", "Z2"),
			Score:   40,
		},
		{
			Cluster: NewClusterWithTopology("member3", "P2", "R1", "Z1"),
			Score:   30,
		},
		{
			Cluster: NewClusterWithTopology("member4", "P2", "R2", "Z2"),
			Score:   60,
		},
	}
}
func Test_GroupClustersWithScore(t *testing.T) {
	type args struct {
		clustersScore framework.ClusterScoreList
		placement     *policyv1alpha1.Placement
		spec          *workv1alpha2.ResourceBindingSpec
	}
	type want struct {
		clusters    []string
		zoneCnt     int
		regionCnt   int
		providerCnt int
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "test SpreadConstraints is nil",
			args: args{
				clustersScore: generateClusterScore(),
				placement:     &policyv1alpha1.Placement{},
				spec:          &workv1alpha2.ResourceBindingSpec{},
			},
			want: want{
				clusters: []string{"member4", "member2", "member3", "member1"},
			},
		},
		{
			name: "test SpreadConstraints is cluster",
			args: args{
				clustersScore: generateClusterScore(),
				placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
				},
				spec: &workv1alpha2.ResourceBindingSpec{},
			},
			want: want{
				clusters: []string{"member4", "member2", "member3", "member1"},
			},
		},
		{
			name: "test SpreadConstraints is zone",
			args: args{
				clustersScore: generateClusterScore(),
				placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldZone,
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
				},
				spec: &workv1alpha2.ResourceBindingSpec{},
			},
			want: want{
				clusters: []string{"member4", "member2", "member3", "member1"},
				zoneCnt:  2,
			},
		},
		{
			name: "test SpreadConstraints is region",
			args: args{
				clustersScore: generateClusterScore(),
				placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldRegion,
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
				},
				spec: &workv1alpha2.ResourceBindingSpec{},
			},
			want: want{
				clusters:  []string{"member4", "member2", "member3", "member1"},
				regionCnt: 2,
			},
		},
		{
			name: "test SpreadConstraints is provider",
			args: args{
				clustersScore: generateClusterScore(),
				placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldProvider,
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
				},
				spec: &workv1alpha2.ResourceBindingSpec{},
			},
			want: want{
				clusters:    []string{"member4", "member2", "member3", "member1"},
				providerCnt: 2,
			},
		},
		{
			name: "test SpreadConstraints is provider/region/zone",
			args: args{
				clustersScore: generateClusterScore(),
				placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldProvider,
							MaxGroups:     1,
							MinGroups:     1,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldRegion,
							MaxGroups:     1,
							MinGroups:     1,
						},
						{
							SpreadByField: policyv1alpha1.SpreadByFieldZone,
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
				},
				spec: &workv1alpha2.ResourceBindingSpec{},
			},
			want: want{
				clusters:    []string{"member4", "member2", "member3", "member1"},
				providerCnt: 2,
				regionCnt:   2,
				zoneCnt:     2,
			},
		},
	}

	calAvailableReplicasFunc := func(clusters []*clusterv1alpha1.Cluster, _ *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster {
		availableTargetClusters := make([]workv1alpha2.TargetCluster, len(clusters))

		for i := range availableTargetClusters {
			availableTargetClusters[i].Name = clusters[i].Name
			availableTargetClusters[i].Replicas = 100
		}

		return availableTargetClusters
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groupInfo := GroupClustersWithScore(tt.args.clustersScore, tt.args.placement, tt.args.spec, calAvailableReplicasFunc)
			for i, cluster := range groupInfo.Clusters {
				if cluster.Name != tt.want.clusters[i] {
					t.Errorf("test %s : the clusters aren't sorted", tt.name)
				}
			}

			if tt.want.zoneCnt != len(groupInfo.Zones) {
				t.Errorf("test %s : zoneCnt = %v, want %v", tt.name, len(groupInfo.Zones), tt.want.zoneCnt)
			}
			if tt.want.regionCnt != len(groupInfo.Regions) {
				t.Errorf("test %s : regionCnt = %v, want %v", tt.name, len(groupInfo.Regions), tt.want.regionCnt)
			}
			if tt.want.providerCnt != len(groupInfo.Providers) {
				t.Errorf("test %s : providerCnt = %v, want %v", tt.name, len(groupInfo.Providers), tt.want.providerCnt)
			}
		})
	}
}

type RbSpecMap map[string]*workv1alpha2.ResourceBindingSpec

//var duplicated = "duplicated"
//var aggregated = "aggregated"
//var dynamicWeight = "dynamicWeight"
//var staticWeight = "staticWeight"
//
//func generateRbSpec() RbSpecMap {
//	rbspecDuplicated := &workv1alpha2.ResourceBindingSpec{
//		Replicas: 20,
//		Placement: &policyv1alpha1.Placement{
//			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
//				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
//			},
//		},
//	}
//	rbspecAggregated := &workv1alpha2.ResourceBindingSpec{
//		Replicas: 20,
//		Placement: &policyv1alpha1.Placement{
//			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
//				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
//				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
//			},
//		},
//	}
//	rbspecDynamicWeight := &workv1alpha2.ResourceBindingSpec{
//		Replicas: 20,
//		Placement: &policyv1alpha1.Placement{
//			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
//				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
//				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
//				WeightPreference: &policyv1alpha1.ClusterPreferences{
//					DynamicWeight: "AvailableReplicas",
//				},
//			},
//		},
//	}
//	rbspecStaticWeight := &workv1alpha2.ResourceBindingSpec{
//		Replicas: 20,
//		Placement: &policyv1alpha1.Placement{
//			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
//				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
//				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
//				WeightPreference: &policyv1alpha1.ClusterPreferences{
//					StaticWeightList: []policyv1alpha1.StaticClusterWeight{
//						{
//							TargetCluster: policyv1alpha1.ClusterAffinity{},
//							Weight:        20,
//						},
//					},
//				},
//			},
//		},
//	}
//	return RbSpecMap{
//		"duplicated":    rbspecDuplicated,
//		"aggregated":    rbspecAggregated,
//		"dynamicWeight": rbspecDynamicWeight,
//		"staticWeight":  rbspecStaticWeight,
//	}
//}

func generateClusterScores(n int, scores []int64, replicas []int64) []ClusterDetailInfo {
	info := make([]ClusterDetailInfo, n)
	for i := 0; i < n; i++ {
		info[i] = ClusterDetailInfo{
			Name:              fmt.Sprintf("member%d", i+1),
			Score:             scores[i],
			AvailableReplicas: replicas[i],
		}
	}
	return info
}

type GroupScoreArgs struct {
	clusters1 []ClusterDetailInfo
	clusters2 []ClusterDetailInfo
	want      bool
	// if clusters1 is better than clusters2, want is true
}

func generateArgs() []GroupScoreArgs {
	argsList := []GroupScoreArgs{
		// simple 1
		{
			clusters1: generateClusterScores(3, []int64{0, 100, 0}, []int64{42, 7, 99}),
			clusters2: generateClusterScores(3, []int64{100, 0, 100}, []int64{15, 60, 23}),
			want:      false,
		},
		// simple 2
		{
			clusters1: generateClusterScores(5, []int64{100, 0, 100, 0, 100}, []int64{10, 20, 30, 40, 50}),
			clusters2: generateClusterScores(5, []int64{0, 0, 0, 0, 0}, []int64{100, 200, 300, 400, 500}),
			want:      false,
		},
		// simple 3
		{
			clusters1: generateClusterScores(4, []int64{0, 100, 0, 100}, []int64{13, 26, 39, 52}),
			clusters2: generateClusterScores(4, []int64{100, 0, 100, 0}, []int64{52, 39, 26, 13}),
		},
		// simple 4
		{
			clusters1: generateClusterScores(6, []int64{100, 0, 0, 100, 0, 0}, []int64{1000, 2000, 3000, 4000, 5000, 6000}),
			clusters2: generateClusterScores(6, []int64{0, 100, 100, 0, 100, 100}, []int64{6000, 5000, 4000, 3000, 2000, 1000}),
		},
		// simple 5
		{
			clusters1: generateClusterScores(2, []int64{0, 100}, []int64{123456, 654321}),
			clusters2: generateClusterScores(2, []int64{100, 0}, []int64{654321, 123456}),
		},
		// simple 6
		{
			clusters1: generateClusterScores(7, []int64{0, 0, 0, 0, 0, 0, 0}, []int64{1, 2, 3, 4, 5, 6, 7}),
			clusters2: generateClusterScores(7, []int64{100, 100, 100, 100, 100, 100, 100}, []int64{7, 6, 5, 4, 3, 2, 1}),
		},
		// simple 7
		{
			clusters1: generateClusterScores(5, []int64{0, 100, 0, 100, 0}, []int64{0, 0, 0, 0, 0}),
			clusters2: generateClusterScores(5, []int64{100, 0, 100, 0, 100}, []int64{99999, 88888, 77777, 66666, 55555}),
		},
		// simple 8
		{
			clusters1: generateClusterScores(9, []int64{0, 100, 0, 100, 0, 100, 0, 100, 0}, []int64{1, 3, 5, 7, 9, 11, 13, 15, 17}),
			clusters2: generateClusterScores(9, []int64{100, 0, 100, 0, 100, 0, 100, 0, 100}, []int64{2, 4, 6, 8, 10, 12, 14, 16, 18}),
		},
		// simple 9
		{
			clusters1: generateClusterScores(1, []int64{0}, []int64{2147483647}),
			clusters2: generateClusterScores(1, []int64{100}, []int64{2147483647}),
		},
		// simple 10
		{
			clusters1: generateClusterScores(4, []int64{0, 0, 100, 100}, []int64{17, 29, 31, 37}),
			clusters2: generateClusterScores(4, []int64{100, 100, 0, 0}, []int64{37, 31, 29, 17}),
		},
		// simple 11
		{
			clusters1: generateClusterScores(3, []int64{100, 100, 100}, []int64{1000000, 2000000, 3000000}),
			clusters2: generateClusterScores(3, []int64{0, 0, 0}, []int64{3000000, 2000000, 1000000}),
		},
		// simple 12
		{
			clusters1: generateClusterScores(6, []int64{0, 0, 100, 100, 0, 0}, []int64{17, 19, 23, 29, 31, 37}),
			clusters2: generateClusterScores(6, []int64{100, 100, 0, 0, 100, 100}, []int64{37, 31, 29, 23, 19, 17}),
		},
		// simple 13
		{
			clusters1: generateClusterScores(5, []int64{0, 100, 0, 100, 0}, []int64{2, 4, 6, 8, 10}),
			clusters2: generateClusterScores(5, []int64{100, 0, 100, 0, 100}, []int64{1, 3, 5, 7, 9}),
		},
		// simple 14
		{
			clusters1: generateClusterScores(8, []int64{0, 0, 0, 0, 100, 100, 100, 100}, []int64{11, 22, 33, 44, 55, 66, 77, 88}),
			clusters2: generateClusterScores(8, []int64{100, 100, 100, 100, 0, 0, 0, 0}, []int64{88, 77, 66, 55, 44, 33, 22, 11}),
		},
		// simple 15
		{
			clusters1: generateClusterScores(2, []int64{100, 0}, []int64{123, 321}),
			clusters2: generateClusterScores(2, []int64{0, 100}, []int64{321, 123}),
		},
		// simple 16
		{
			clusters1: generateClusterScores(7, []int64{0, 100, 0, 100, 0, 100, 0}, []int64{14, 28, 42, 56, 70, 84, 98}),
			clusters2: generateClusterScores(7, []int64{100, 0, 100, 0, 100, 0, 100}, []int64{98, 84, 70, 56, 42, 28, 14}),
		},
		// simple 17
		{
			clusters1: generateClusterScores(5, []int64{0, 0, 0, 0, 0}, []int64{999, 888, 777, 666, 555}),
			clusters2: generateClusterScores(5, []int64{100, 100, 100, 100, 100}, []int64{555, 666, 777, 888, 999}),
		},
		// simple 18
		{
			clusters1: generateClusterScores(9, []int64{100, 0, 100, 0, 100, 0, 100, 0, 100}, []int64{12, 24, 36, 48, 60, 72, 84, 96, 108}),
			clusters2: generateClusterScores(9, []int64{0, 100, 0, 100, 0, 100, 0, 100, 0}, []int64{108, 96, 84, 72, 60, 48, 36, 24, 12}),
		},
		// simple 19
		{
			clusters1: generateClusterScores(1, []int64{0}, []int64{0}),
			clusters2: generateClusterScores(1, []int64{100}, []int64{999999999}),
		},
		// simple 20
		{
			clusters1: generateClusterScores(4, []int64{0, 100, 0, 100}, []int64{50, 100, 150, 200}),
			clusters2: generateClusterScores(4, []int64{100, 0, 100, 0}, []int64{200, 150, 100, 50}),
		},
		// simple 21
		{
			clusters1: generateClusterScores(3, []int64{0, 100, 0}, []int64{333, 666, 999}),
			clusters2: generateClusterScores(3, []int64{100, 0, 100}, []int64{999, 666, 333}),
		},
		// simple 22
		{
			clusters1: generateClusterScores(6, []int64{0, 100, 0, 100, 0, 100}, []int64{1, 4, 9, 16, 25, 36}),
			clusters2: generateClusterScores(6, []int64{100, 0, 100, 0, 100, 0}, []int64{36, 25, 16, 9, 4, 1}),
		},
		// simple 23
		{
			clusters1: generateClusterScores(2, []int64{100, 0}, []int64{111111, 222222}),
			clusters2: generateClusterScores(2, []int64{0, 100}, []int64{222222, 111111}),
		},
		// simple 24
		{
			clusters1: generateClusterScores(5, []int64{100, 100, 0, 0, 100}, []int64{7, 14, 21, 28, 35}),
			clusters2: generateClusterScores(5, []int64{0, 0, 100, 100, 0}, []int64{35, 28, 21, 14, 7}),
		},
		// simple 25
		{
			clusters1: generateClusterScores(8, []int64{0, 0, 100, 100, 0, 0, 100, 100}, []int64{100, 200, 300, 400, 500, 600, 700, 800}),
			clusters2: generateClusterScores(8, []int64{100, 100, 0, 0, 100, 100, 0, 0}, []int64{800, 700, 600, 500, 400, 300, 200, 100}),
		},
		// simple 26
		{
			clusters1: generateClusterScores(7, []int64{0, 100, 0, 100, 0, 100, 0}, []int64{13, 26, 39, 52, 65, 78, 91}),
			clusters2: generateClusterScores(7, []int64{100, 0, 100, 0, 100, 0, 100}, []int64{91, 78, 65, 52, 39, 26, 13}),
		},
		// simple 27
		{
			clusters1: generateClusterScores(5, []int64{0, 0, 100, 100, 0}, []int64{17, 19, 23, 29, 31}),
			clusters2: generateClusterScores(5, []int64{100, 100, 0, 0, 100}, []int64{31, 29, 23, 19, 17}),
		},
		// simple 28
		{
			clusters1: generateClusterScores(2, []int64{0, 100}, []int64{1000000000, 2000000000}),
			clusters2: generateClusterScores(2, []int64{100, 0}, []int64{2000000000, 1000000000}),
		},
		// simple 29
		{
			clusters1: generateClusterScores(6, []int64{100, 0, 100, 0, 100, 0}, []int64{5, 10, 15, 20, 25, 30}),
			clusters2: generateClusterScores(6, []int64{0, 100, 0, 100, 0, 100}, []int64{30, 25, 20, 15, 10, 5}),
		},
		// simple 30
		{
			clusters1: generateClusterScores(9, []int64{0, 100, 0, 100, 0, 100, 0, 100, 0}, []int64{1, 8, 27, 64, 125, 216, 343, 512, 729}),
			clusters2: generateClusterScores(9, []int64{100, 0, 100, 0, 100, 0, 100, 0, 100}, []int64{729, 512, 343, 216, 125, 64, 27, 8, 1}),
		},
		// simple 31
		{
			clusters1: generateClusterScores(4, []int64{100, 100, 0, 0}, []int64{100, 200, 300, 400}),
			clusters2: generateClusterScores(4, []int64{0, 0, 100, 100}, []int64{400, 300, 200, 100}),
		},
		// simple 32
		{
			clusters1: generateClusterScores(3, []int64{0, 100, 0}, []int64{17, 34, 51}),
			clusters2: generateClusterScores(3, []int64{100, 0, 100}, []int64{51, 34, 17}),
		},
		// simple 33
		{
			clusters1: generateClusterScores(7, []int64{100, 0, 100, 0, 100, 0, 100}, []int64{13, 26, 39, 52, 65, 78, 91}),
			clusters2: generateClusterScores(7, []int64{0, 100, 0, 100, 0, 100, 0}, []int64{91, 78, 65, 52, 39, 26, 13}),
		},
		// simple 34
		{
			clusters1: generateClusterScores(5, []int64{0, 100, 0, 100, 0}, []int64{1000, 2000, 3000, 4000, 5000}),
			clusters2: generateClusterScores(5, []int64{100, 0, 100, 0, 100}, []int64{5000, 4000, 3000, 2000, 1000}),
		},
		// simple 35
		{
			clusters1: generateClusterScores(8, []int64{100, 0, 100, 0, 100, 0, 100, 0}, []int64{2, 4, 6, 8, 10, 12, 14, 16}),
			clusters2: generateClusterScores(8, []int64{0, 100, 0, 100, 0, 100, 0, 100}, []int64{16, 14, 12, 10, 8, 6, 4, 2}),
		},
		// simple 36
		{
			clusters1: generateClusterScores(2, []int64{0, 0}, []int64{777777777, 888888888}),
			clusters2: generateClusterScores(2, []int64{100, 100}, []int64{888888888, 777777777}),
		},
		// simple 37
		{
			clusters1: generateClusterScores(6, []int64{0, 100, 0, 100, 0, 100}, []int64{15, 30, 45, 60, 75, 90}),
			clusters2: generateClusterScores(6, []int64{100, 0, 100, 0, 100, 0}, []int64{90, 75, 60, 45, 30, 15}),
		},
		// simple 38
		{
			clusters1: generateClusterScores(9, []int64{0, 0, 100, 100, 0, 0, 100, 100, 0}, []int64{19, 23, 29, 31, 37, 41, 43, 47, 53}),
			clusters2: generateClusterScores(9, []int64{100, 100, 0, 0, 100, 100, 0, 0, 100}, []int64{53, 47, 43, 41, 37, 31, 29, 23, 19}),
		},
		// simple 39
		{
			clusters1: generateClusterScores(4, []int64{100, 0, 100, 0}, []int64{500, 1000, 1500, 2000}),
			clusters2: generateClusterScores(4, []int64{0, 100, 0, 100}, []int64{2000, 1500, 1000, 500}),
		},
		// simple 40
		{
			clusters1: generateClusterScores(5, []int64{0, 0, 0, 0, 0}, []int64{1, 1, 1, 1, 1}),
			clusters2: generateClusterScores(5, []int64{100, 100, 100, 100, 100}, []int64{1, 1, 1, 1, 1}),
		},
		// simple 41
		{
			clusters1: generateClusterScores(3, []int64{0, 100, 0}, []int64{9999, 8888, 7777}),
			clusters2: generateClusterScores(3, []int64{100, 0, 100}, []int64{7777, 8888, 9999}),
		},
		// simple 42
		{
			clusters1: generateClusterScores(6, []int64{100, 0, 100, 0, 100, 0}, []int64{1, 8, 27, 64, 125, 216}),
			clusters2: generateClusterScores(6, []int64{0, 100, 0, 100, 0, 100}, []int64{216, 125, 64, 27, 8, 1}),
		},
		// simple 43
		{
			clusters1: generateClusterScores(2, []int64{0, 100}, []int64{555555555, 666666666}),
			clusters2: generateClusterScores(2, []int64{100, 0}, []int64{666666666, 555555555}),
		},
		// simple 44
		{
			clusters1: generateClusterScores(5, []int64{100, 100, 0, 0, 100}, []int64{9, 18, 27, 36, 45}),
			clusters2: generateClusterScores(5, []int64{0, 0, 100, 100, 0}, []int64{45, 36, 27, 18, 9}),
		},
		// simple 45
		{
			clusters1: generateClusterScores(8, []int64{0, 0, 100, 100, 0, 0, 100, 100}, []int64{11, 22, 33, 44, 55, 66, 77, 88}),
			clusters2: generateClusterScores(8, []int64{100, 100, 0, 0, 100, 100, 0, 0}, []int64{88, 77, 66, 55, 44, 33, 22, 11}),
		},
		// simple 46
		{
			clusters1: generateClusterScores(7, []int64{0, 100, 0, 100, 0, 100, 0}, []int64{21, 42, 63, 84, 105, 126, 147}),
			clusters2: generateClusterScores(7, []int64{100, 0, 100, 0, 100, 0, 100}, []int64{147, 126, 105, 84, 63, 42, 21}),
		},
		// simple 47
		{
			clusters1: generateClusterScores(5, []int64{0, 0, 100, 100, 0}, []int64{2, 3, 5, 7, 11}),
			clusters2: generateClusterScores(5, []int64{100, 100, 0, 0, 100}, []int64{11, 7, 5, 3, 2}),
		},
		// simple 48
		{
			clusters1: generateClusterScores(2, []int64{100, 0}, []int64{999999999, 888888888}),
			clusters2: generateClusterScores(2, []int64{0, 100}, []int64{888888888, 999999999}),
		},
		// simple 49
		{
			clusters1: generateClusterScores(6, []int64{0, 100, 0, 100, 0, 100}, []int64{2, 4, 6, 8, 10, 12}),
			clusters2: generateClusterScores(6, []int64{100, 0, 100, 0, 100, 0}, []int64{12, 10, 8, 6, 4, 2}),
		},
		// simple 50
		{
			clusters1: generateClusterScores(9, []int64{100, 0, 100, 0, 100, 0, 100, 0, 100}, []int64{5, 10, 15, 20, 25, 30, 35, 40, 45}),
			clusters2: generateClusterScores(9, []int64{0, 100, 0, 100, 0, 100, 0, 100, 0}, []int64{45, 40, 35, 30, 25, 20, 15, 10, 5}),
		},
	}
	return argsList
}

func Test_CalcGroupScore(t *testing.T) {
	tests := generateArgs()
	groupClustersInfo := &GroupClustersInfo{
		Providers: make(map[string]ProviderInfo),
		Regions:   make(map[string]RegionInfo),
		Zones:     make(map[string]ZoneInfo),
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			score1 := groupClustersInfo.calcGroupScore(tt.clusters1)
			score2 := groupClustersInfo.calcGroupScore(tt.clusters2)
			t.Logf("score1 = %v, score2 = %v, score1 > score 2 res: %v", score1, score2, score1 > score2)
			if tt.want != (score1 > score2) {
				t.Errorf("score1 = %v, score2 = %v, score1 > score 2 want %v, but not", score1, score2, tt.want)
			}
		})
	}
}
