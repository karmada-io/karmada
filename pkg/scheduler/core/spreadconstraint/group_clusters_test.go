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
	id        int
	clusters1 []ClusterDetailInfo
	clusters2 []ClusterDetailInfo
	want      bool
	// if clusters1 is better than clusters2, want is true
}

func generateArgs() []GroupScoreArgs {

	argsList := []GroupScoreArgs{
		{
			id:        1,
			clusters1: generateClusterScores(5, []int64{100, 100, 0, 0, 0}, []int64{78, 729, 840, 722, 708}),
			clusters2: generateClusterScores(5, []int64{100, 0, 0, 0, 0}, []int64{226, 405, 385, 486, 282}),
			want:      true,
		},
		{
			id:        2,
			clusters1: generateClusterScores(3, []int64{100, 100, 100}, []int64{630141, 421020, 507178}),
			clusters2: generateClusterScores(3, []int64{0, 0, 0}, []int64{448783, 963163, 693666}),
			want:      true,
		},
		{
			id:        3,
			clusters1: generateClusterScores(4, []int64{0, 0, 0, 0}, []int64{21, 80, 44, 16}),
			clusters2: generateClusterScores(4, []int64{100, 100, 100, 100}, []int64{29, 93, 45, 46}),
			want:      false,
		},
		{
			id:        4,
			clusters1: generateClusterScores(6, []int64{100, 100, 0, 0, 0, 0}, []int64{453, 867, 140, 770, 213, 56}),
			clusters2: generateClusterScores(6, []int64{100, 100, 100, 0, 0, 0}, []int64{657, 670, 341, 44, 892, 182}),
			want:      false,
		},
		{
			id:        5,
			clusters1: generateClusterScores(2, []int64{100, 0}, []int64{740319, 6935856}),
			clusters2: generateClusterScores(2, []int64{100, 0}, []int64{134411, 1764495}),
			want:      true,
		},
		{
			id:        6,
			clusters1: generateClusterScores(7, []int64{100, 100, 100, 0, 0, 0, 0}, []int64{797417, 7837026, 1964746, 9664532, 3172027, 3602837, 4939293}),
			clusters2: generateClusterScores(7, []int64{100, 100, 0, 0, 0, 0, 0}, []int64{8334825, 4935171, 9218829, 3038557, 8879763, 9799731, 4838061}),
			want:      false,
		},
		{
			id:        7,
			clusters1: generateClusterScores(12, []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, []int64{591, 892, 735, 348, 732, 339, 452, 276, 342, 791, 667, 271}),
			clusters2: generateClusterScores(12, []int64{100, 100, 100, 0, 0, 0, 0, 0, 0, 0, 0, 0}, []int64{644, 933, 256, 679, 497, 88, 596, 295, 980, 966, 567, 244}),
			want:      false,
		},
		{
			id:        8,
			clusters1: generateClusterScores(4, []int64{100, 100, 100, 100}, []int64{2, 91, 160, 369, 388}),
			clusters2: generateClusterScores(4, []int64{100, 100, 0, 0}, []int64{874, 705, 443, 365, 645}),
			want:      false,
		},
		{
			id:        9,
			clusters1: generateClusterScores(3, []int64{0, 0, 0}, []int64{96, 793, 207}),
			clusters2: generateClusterScores(3, []int64{100, 100, 100}, []int64{890, 292, 719}),
			want:      false,
		},
		{
			id:        10,
			clusters1: generateClusterScores(5, []int64{100, 100, 100, 100, 100}, []int64{568, 48, 396, 403, 219}),
			clusters2: generateClusterScores(5, []int64{100, 100, 0, 0, 0}, []int64{794, 466, 401, 56, 579}),
			want:      true,
		},
		{
			id:        11,
			clusters1: generateClusterScores(5, []int64{100, 100, 0, 0, 0}, []int64{812, 673, 290, 418, 546}),
			clusters2: generateClusterScores(5, []int64{100, 0, 0, 0, 0}, []int64{759, 834, 621, 237, 490}),
			want:      true,
		},
		{
			id:        12,
			clusters1: generateClusterScores(3, []int64{0, 0, 0}, []int64{142, 658, 913}),
			clusters2: generateClusterScores(3, []int64{100, 100, 100}, []int64{84, 297, 466}),
			want:      false,
		},
		{
			id:        13,
			clusters1: generateClusterScores(6, []int64{100, 100, 100, 0, 0, 0}, []int64{376, 589, 710, 843, 257, 634}),
			clusters2: generateClusterScores(6, []int64{100, 100, 0, 0, 0, 0}, []int64{455, 502, 391, 678, 849, 120}),
			want:      true,
		},
		{
			id:        14,
			clusters1: generateClusterScores(4, []int64{100, 100, 0, 0}, []int64{950, 860, 740, 630}),
			clusters2: generateClusterScores(4, []int64{100, 100, 100, 100}, []int64{150, 250, 350, 450}),
			want:      true,
		},
		{
			id:        15,
			clusters1: generateClusterScores(2, []int64{100, 0}, []int64{520, 810}),
			clusters2: generateClusterScores(2, []int64{100, 0}, []int64{630, 700}),
			want:      false,
		},
		{
			id:        16,
			clusters1: generateClusterScores(7, []int64{100, 100, 100, 100, 100, 100, 100}, []int64{712, 645, 578, 511, 444, 377, 310}),
			clusters2: generateClusterScores(7, []int64{100, 100, 100, 100, 0, 0, 0}, []int64{689, 622, 555, 488, 421, 354, 287}),
			want:      true,
		},
		{
			id:        17,
			clusters1: generateClusterScores(5, []int64{100, 100, 0, 0, 0}, []int64{372, 461, 550, 639, 728}),
			clusters2: generateClusterScores(5, []int64{0, 0, 0, 0, 0}, []int64{817, 906, 995, 184, 273}),
			want:      true,
		},
		{
			id:        18,
			clusters1: generateClusterScores(6, []int64{0, 0, 0, 0, 0, 0}, []int64{110, 220, 330, 440, 550, 660}),
			clusters2: generateClusterScores(6, []int64{0, 0, 0, 0, 0, 0}, []int64{165, 275, 385, 495, 605, 715}),
			want:      false,
		},
		{
			id:        19,
			clusters1: generateClusterScores(4, []int64{100, 100, 0, 0}, []int64{1020, 2040, 3060, 4080}),
			clusters2: generateClusterScores(4, []int64{100, 0, 0, 0}, []int64{1530, 2550, 3570, 4590}),
			want:      true,
		},
		{
			id:        20,
			clusters1: generateClusterScores(5, []int64{100, 100, 100, 100, 100}, []int64{15, 30, 45, 60, 75}),
			clusters2: generateClusterScores(5, []int64{100, 100, 100, 0, 0}, []int64{20, 35, 50, 65, 80}),
			want:      true,
		},
		{
			id:        21,
			clusters1: generateClusterScores(5, []int64{100, 100, 0, 0, 0}, []int64{314, 159, 265, 358, 979}),
			clusters2: generateClusterScores(5, []int64{100, 0, 0, 0, 0}, []int64{323, 846, 264, 338, 327}),
			want:      true,
		},
		{
			id:        22,
			clusters1: generateClusterScores(3, []int64{100, 100, 100}, []int64{271, 828, 182}),
			clusters2: generateClusterScores(3, []int64{0, 0, 0}, []int64{459, 230, 781}),
			want:      true,
		},
		{
			id:        23,
			clusters1: generateClusterScores(4, []int64{0, 0, 0, 0}, []int64{112, 358, 132, 134}),
			clusters2: generateClusterScores(4, []int64{100, 100, 100, 100}, []int64{558, 907, 176, 164}),
			want:      false,
		},
		{
			id:        24,
			clusters1: generateClusterScores(6, []int64{100, 100, 0, 0, 0, 0}, []int64{625, 349, 360, 288, 419, 716}),
			clusters2: generateClusterScores(6, []int64{100, 100, 100, 0, 0, 0}, []int64{939, 937, 510, 582, 97, 494}),
			want:      false,
		},
		{
			id:        25,
			clusters1: generateClusterScores(2, []int64{100, 0}, []int64{4592, 307}),
			clusters2: generateClusterScores(2, []int64{100, 0}, []int64{8164, 62}),
			want:      false,
		},
		{
			id:        26,
			clusters1: generateClusterScores(7, []int64{100, 100, 100, 100, 100, 100, 100}, []int64{862, 803, 482, 534, 211, 706, 798}),
			clusters2: generateClusterScores(7, []int64{100, 100, 100, 100, 100, 0, 0}, []int64{214, 808, 651, 328, 230, 780, 729}),
			want:      true,
		},
		{
			id:        27,
			clusters1: generateClusterScores(5, []int64{100, 100, 100, 0, 0}, []int64{716, 939, 937, 510, 582}),
			clusters2: generateClusterScores(5, []int64{100, 100, 100, 100, 0}, []int64{97, 494, 459, 230, 781}),
			want:      true,
		},
		{
			id:        28,
			clusters1: generateClusterScores(4, []int64{0, 0, 0, 0}, []int64{640, 628, 620, 899}),
			clusters2: generateClusterScores(4, []int64{0, 0, 0, 0}, []int64{862, 803, 482, 534}),
			want:      true,
		},
		{
			id:        29,
			clusters1: generateClusterScores(3, []int64{100, 0, 0}, []int64{211, 706, 798}),
			clusters2: generateClusterScores(3, []int64{100, 100, 0}, []int64{214, 808, 651}),
			want:      false,
		},
		{
			id:        30,
			clusters1: generateClusterScores(5, []int64{100, 100, 100, 0, 0}, []int64{328, 230, 780, 729, 716}),
			clusters2: generateClusterScores(5, []int64{100, 100, 100, 0, 0}, []int64{939, 937, 510, 582, 97}),
			want:      false,
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
			score1 := groupClustersInfo.calcGroupScore(tt.clusters1, 10)
			score2 := groupClustersInfo.calcGroupScore(tt.clusters2, 10)
			t.Logf("test ID: %v, score1 = %v, score2 = %v, score1 >= score 2 res: %v", tt.id, score1, score2, score1 > score2)
			if tt.want != (score1 >= score2) {
				t.Errorf("test ID: %v, score1 = %v, score2 = %v, score1 >= score 2 want %v, but res is %v", tt.id, score1, score2, tt.want, score1 > score2)
			}
		})
	}
}
