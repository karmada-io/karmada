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

func generateRbSpec() []*workv1alpha2.ResourceBindingSpec {
	rbspecDuplicated := &workv1alpha2.ResourceBindingSpec{
		Replicas: 20,
		Placement: &policyv1alpha1.Placement{
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
			},
		},
	}
	rbspecAggregated := &workv1alpha2.ResourceBindingSpec{
		Replicas: 20,
		Placement: &policyv1alpha1.Placement{
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
		},
	}
	rbspecDynamicWeight := &workv1alpha2.ResourceBindingSpec{
		Replicas: 20,
		Placement: &policyv1alpha1.Placement{
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				WeightPreference: &policyv1alpha1.ClusterPreferences{
					DynamicWeight: "AvailableReplicas",
				},
			},
		},
	}
	rbspecStaticWeight := &workv1alpha2.ResourceBindingSpec{
		Replicas: 20,
		Placement: &policyv1alpha1.Placement{
			ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
				ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
				ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				WeightPreference: &policyv1alpha1.ClusterPreferences{
					StaticWeightList: []policyv1alpha1.StaticClusterWeight{
						{
							TargetCluster: policyv1alpha1.ClusterAffinity{},
							Weight:        20,
						},
					},
				},
			},
		},
	}
	return []*workv1alpha2.ResourceBindingSpec{rbspecDuplicated, rbspecAggregated, rbspecDynamicWeight, rbspecStaticWeight}
}

func generateClusterScores() [][]ClusterDetailInfo {
	// score: [20, 40, 60, 80, 100]
	// available: [20, 40, 60, 80, 100]
	info1 := []ClusterDetailInfo{
		{
			Name:              "member1",
			Score:             20,
			AvailableReplicas: 20,
		},
		{
			Name:              "member2",
			Score:             40,
			AvailableReplicas: 40,
		},
		{
			Name:              "member3",
			Score:             60,
			AvailableReplicas: 60,
		},
		{
			Name:              "member4",
			Score:             80,
			AvailableReplicas: 80,
		},
		{
			Name:              "member5",
			Score:             100,
			AvailableReplicas: 100,
		},
	}

	// score: [100, 10, 10, 10, 10]
	// available: [20, 20, 20, 20, 20]
	info2 := []ClusterDetailInfo{
		{
			Name:              "member1",
			Score:             100,
			AvailableReplicas: 20,
		},
		{
			Name:              "member2",
			Score:             10,
			AvailableReplicas: 20,
		},
		{
			Name:              "member3",
			Score:             10,
			AvailableReplicas: 20,
		},
		{
			Name:              "member4",
			Score:             10,
			AvailableReplicas: 20,
		},
		{
			Name:              "member5",
			Score:             10,
			AvailableReplicas: 20,
		},
	}

	// score: [60, 60, 60, 60, 60]
	// available: [20, 20, 20, 20, 20]
	info3 := []ClusterDetailInfo{
		{
			Name:              "member1",
			Score:             60,
			AvailableReplicas: 20,
		},
		{
			Name:              "member2",
			Score:             60,
			AvailableReplicas: 20,
		},
		{
			Name:              "member3",
			Score:             60,
			AvailableReplicas: 20,
		},
		{
			Name:              "member4",
			Score:             60,
			AvailableReplicas: 20,
		},
		{
			Name:              "member5",
			Score:             60,
			AvailableReplicas: 20,
		},
	}

	// score: [120, 100, 80, 60, 40]
	// available: [20, 40, 60, 80, 100]
	info4 := []ClusterDetailInfo{
		{
			Name:              "member1",
			Score:             120,
			AvailableReplicas: 20,
		},
		{
			Name:              "member2",
			Score:             100,
			AvailableReplicas: 40,
		},
		{
			Name:              "member3",
			Score:             80,
			AvailableReplicas: 60,
		},
		{
			Name:              "member4",
			Score:             60,
			AvailableReplicas: 80,
		},
		{
			Name:              "member5",
			Score:             40,
			AvailableReplicas: 100,
		},
	}

	return [][]ClusterDetailInfo{info1, info2, info3, info4}
}

func Test_CalcGroupScore(t *testing.T) {
	rbSpecs := generateRbSpec()
	scores := generateClusterScores()

	type args struct {
		clusters1 []ClusterDetailInfo
		clusters2 []ClusterDetailInfo
		rbSpec    *workv1alpha2.ResourceBindingSpec
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "avoid extreme cases",
			args: args{
				clusters1: scores[1],
				clusters2: scores[2],
				rbSpec:    rbSpecs[3],
			},
			want: false,
		},
		{
			name: "the index of high score should be closed to the index of high AvailableReplicas.",
			args: args{
				clusters1: scores[0],
				clusters2: scores[3],
				rbSpec:    rbSpecs[2],
			},
			want: true,
		},
	}

	groupClustersInfo := &GroupClustersInfo{
		Providers: make(map[string]ProviderInfo),
		Regions:   make(map[string]RegionInfo),
		Zones:     make(map[string]ZoneInfo),

		averageScore: 20,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score1 := groupClustersInfo.calcGroupScore(tt.args.clusters1, tt.args.rbSpec)
			score2 := groupClustersInfo.calcGroupScore(tt.args.clusters2, tt.args.rbSpec)
			t.Logf("test %s : score1 = %v, score2 = %v, score1 > score 2 res: %v", tt.name, score1, score2, score1 > score2)
			if tt.want != (score1 > score2) {
				t.Errorf("test %s : score1 = %v, score2 = %v, score1 > score 2 want %v, but not", tt.name, score1, score2, tt.want)
			}
		})
	}
}
