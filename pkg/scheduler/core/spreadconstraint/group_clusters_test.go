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

var duplicated = "duplicated"
var aggregated = "aggregated"
var dynamicWeight = "dynamicWeight"
var staticWeight = "staticWeight"

type RbSpecMap map[string]*workv1alpha2.ResourceBindingSpec

func generateRbSpec(replica int32) RbSpecMap {
	rbspecDuplicated := &workv1alpha2.ResourceBindingSpec{
		Replicas: replica,
		Placement: &policyv1alpha1.Placement{
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
			},
		},
	}
	rbspecAggregated := &workv1alpha2.ResourceBindingSpec{
		Replicas: replica,
		Placement: &policyv1alpha1.Placement{
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
		},
	}
	rbspecDynamicWeight := &workv1alpha2.ResourceBindingSpec{
		Replicas: replica,
		Placement: &policyv1alpha1.Placement{
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				WeightPreference: &policyv1alpha1.ClusterPreferences{
					DynamicWeight: policyv1alpha1.DynamicWeightByAvailableReplicas,
				},
			},
		},
	}
	rbspecStaticWeight := &workv1alpha2.ResourceBindingSpec{
		Replicas: replica,
		Placement: &policyv1alpha1.Placement{
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				WeightPreference: &policyv1alpha1.ClusterPreferences{
					StaticWeightList: []policyv1alpha1.StaticClusterWeight{},
				},
			},
		},
	}
	return RbSpecMap{
		"duplicated":    rbspecDuplicated,
		"aggregated":    rbspecAggregated,
		"dynamicWeight": rbspecDynamicWeight,
		"staticWeight":  rbspecStaticWeight,
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
	id          int
	clusters1   []ClusterDetailInfo
	clusters2   []ClusterDetailInfo
	rbSpec      *workv1alpha2.ResourceBindingSpec
	minGroups   int
	group1Wins  bool
	description string
}

func generateArgs() []GroupScoreArgs {
	argsList := []GroupScoreArgs{
		{
			id:          1,
			clusters1:   generateClusterScores(5, []int64{100, 100, 0, 0, 0}, []int64{30, 30, 30, 30, 30}),
			clusters2:   generateClusterScores(5, []int64{100, 100, 100, 100, 100}, []int64{30, 30, 30, 10, 10}),
			rbSpec:      generateRbSpec(20)[duplicated],
			group1Wins:  true,
			description: "clusters1 is better than clusters2, because Because clusters1 meets the replica requirements for a larger number of clusters.",
		},
		{
			id:          2,
			clusters1:   generateClusterScores(5, []int64{100, 100, 0, 0, 0}, []int64{30, 30, 30, 10, 10}),
			clusters2:   generateClusterScores(5, []int64{100, 100, 100, 100, 100}, []int64{30, 30, 30, 10, 10}),
			rbSpec:      generateRbSpec(20)[duplicated],
			group1Wins:  false,
			description: "clusters2 is better than clusters1, because clusters1 and clusters2 meet the replica requirements for the same number of clusters, but clusters2 has a higher score.",
		},
		{
			id:          3,
			clusters1:   generateClusterScores(5, []int64{100, 100, 0, 0, 0}, []int64{30, 30, 30, 10, 10}),
			clusters2:   generateClusterScores(5, []int64{100, 100, 100, 100, 100}, []int64{10, 10, 10, 5, 5}),
			rbSpec:      generateRbSpec(100)[aggregated],
			minGroups:   2,
			group1Wins:  true,
			description: "clusters1 is better than clusters2, because clusters1 meets the replica requirements, but clusters2 does not meet.",
		},
		{
			id:          4,
			clusters1:   generateClusterScores(5, []int64{100, 100, 0, 0, 0}, []int64{10, 10, 10, 10, 5}),
			clusters2:   generateClusterScores(5, []int64{100, 100, 100, 100, 100}, []int64{10, 10, 10, 5, 5}),
			rbSpec:      generateRbSpec(100)[dynamicWeight],
			minGroups:   2,
			group1Wins:  true,
			description: "clusters1 is better than clusters2, because clusters1's available replica is larger than clusters2.",
		},
		{
			id:          5,
			clusters1:   generateClusterScores(5, []int64{100, 100, 0, 0, 0}, []int64{10, 10, 10, 5, 5}),
			clusters2:   generateClusterScores(5, []int64{100, 100, 100, 100, 100}, []int64{10, 10, 10, 5, 5}),
			rbSpec:      generateRbSpec(100)[staticWeight],
			minGroups:   2,
			group1Wins:  false,
			description: "clusters2 is better than clusters1, because clusters2's score is higher than clusters1.",
		},
		{
			id:          6,
			clusters1:   generateClusterScores(5, []int64{0, 0, 0, 0, 0}, []int64{100, 100, 100, 100, 100}),
			clusters2:   generateClusterScores(5, []int64{100, 100, 100, 100, 100}, []int64{50, 50, 50, 50, 50}),
			rbSpec:      generateRbSpec(100)[aggregated],
			minGroups:   2,
			group1Wins:  false,
			description: "clusters2 is better than clusters1, because clusters2's score is higher than clusters1, although clusters2's available replica is less than clusters1.",
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
			score1 := groupClustersInfo.calcGroupScore(tt.clusters1, tt.rbSpec, tt.minGroups)
			score2 := groupClustersInfo.calcGroupScore(tt.clusters2, tt.rbSpec, tt.minGroups)
			t.Logf("test ID: %v, score1 = %v, score2 = %v, score1 >= score 2 res: %v, the description => %v", tt.id, score1, score2, score1 > score2, tt.description)
			if tt.group1Wins != (score1 >= score2) {
				t.Errorf("test ID: %v, score1 = %v, score2 = %v, score1 >= score 2 want %v, but res is %v", tt.id, score1, score2, tt.group1Wins, score1 > score2)
			}
		})
	}
}
