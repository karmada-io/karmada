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
			Cluster: NewCluster("member1", "P1", "R1", "Z1",
				map[string]string{
					"unit": "U1",
					"cell": "C1",
				}, map[string]string{
					"unit": "U1",
					"cell": "C1",
				}),
			Score: 20,
		},
		{
			Cluster: NewCluster("member2", "P1", "R1", "Z2",
				map[string]string{
					"unit": "U1",
					"cell": "C2",
				}, map[string]string{
					"unit": "U1",
					"cell": "C2",
				}),
			Score: 40,
		},
		{
			Cluster: NewCluster("member3", "P2", "R2", "Z3",
				map[string]string{
					"unit": "U2",
					"cell": "C3",
				}, map[string]string{
					"unit": "U2",
					"cell": "C3",
				}),
			Score: 30,
		},
		{
			Cluster: NewCluster("member4", "P2", "R2", "Z4",
				map[string]string{
					"unit": "U2",
					"cell": "C4",
				}, map[string]string{
					"unit": "U2",
					"cell": "C4",
				}),
			Score: 60,
		},
	}
}

type args struct {
	clustersScore framework.ClusterScoreList
	placement     *policyv1alpha1.Placement
	spec          *workv1alpha2.ResourceBindingSpec
}

type want struct {
	name     string
	clusters []string
	groups   []want
}

type test struct {
	name string
	arg  args
	want want
}

func (want *want) match(group *groupNode) error {
	if group.Name != want.name {
		return fmt.Errorf("name, want %s, got %s", want.name, group.Name)
	} else if len(want.clusters) != len(group.Clusters) {
		return fmt.Errorf("cluster size, want %d, got %d", len(want.clusters), len(group.Clusters))
	} else if len(want.groups) != len(group.Groups) {
		return fmt.Errorf("group size, want %d, got %d", len(want.groups), len(group.Groups))
	} else {
		for i, c := range want.clusters {
			if c != group.Clusters[i].Name {
				return fmt.Errorf("cluster, want %s, got %s", c, group.Clusters[i].Name)
			}
		}
		for i, w := range want.groups {
			err := w.match(group.Groups[i])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func Test_GroupClusters(t *testing.T) {

	tests := []test{
		{
			name: "test SpreadConstraints is nil",
			arg: args{
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
			arg: args{
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
				name:     "",
				clusters: []string{"member4", "member2", "member3", "member1"},
				groups: []want{
					{
						name:     "member4",
						clusters: []string{"member4"},
					},
					{
						name:     "member2",
						clusters: []string{"member2"},
					},
					{
						name:     "member3",
						clusters: []string{"member3"},
					},
					{
						name:     "member1",
						clusters: []string{"member1"},
					},
				},
			},
		},
		{
			name: "test SpreadConstraints is zone",
			arg: args{
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
				name:     "",
				clusters: []string{"member4", "member2", "member3", "member1"},
				groups: []want{
					{
						name:     "Z4",
						clusters: []string{"member4"},
					},
					{
						name:     "Z2",
						clusters: []string{"member2"},
					},
					{
						name:     "Z3",
						clusters: []string{"member3"},
					},
					{
						name:     "Z1",
						clusters: []string{"member1"},
					},
				},
			},
		},
		{
			name: "test SpreadConstraints is region",
			arg: args{
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
				name:     "",
				clusters: []string{"member4", "member2", "member3", "member1"},
				groups: []want{
					{
						name:     "R2",
						clusters: []string{"member4", "member3"},
					},
					{
						name:     "R1",
						clusters: []string{"member2", "member1"},
					},
				},
			},
		},
		{
			name: "test SpreadConstraints is provider",
			arg: args{
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
				name:     "",
				clusters: []string{"member4", "member2", "member3", "member1"},
				groups: []want{
					{
						name:     "P2",
						clusters: []string{"member4", "member3"},
					},
					{
						name:     "P1",
						clusters: []string{"member2", "member1"},
					},
				},
			},
		},
		{
			name: "test SpreadConstraints is provider/region/zone",
			arg: args{
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
				name:     "",
				clusters: []string{"member4", "member2", "member3", "member1"},
				groups: []want{
					{
						name:     "P2",
						clusters: []string{"member4", "member3"},
						groups: []want{
							{
								name:     "R2",
								clusters: []string{"member4", "member3"},
								groups: []want{
									{
										name:     "Z4",
										clusters: []string{"member4"},
									},
									{
										name:     "Z3",
										clusters: []string{"member3"},
									},
								},
							},
						},
					},
					{
						name:     "P1",
						clusters: []string{"member2", "member1"},
						groups: []want{
							{
								name:     "R1",
								clusters: []string{"member2", "member1"},
								groups: []want{
									{
										name:     "Z2",
										clusters: []string{"member2"},
									},
									{
										name:     "Z1",
										clusters: []string{"member1"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "test SpreadConstraints is label unit/cell",
			arg: args{
				clustersScore: generateClusterScore(),
				placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByLabel: "unit",
							MaxGroups:     1,
							MinGroups:     1,
						},
						{
							SpreadByLabel: "cell",
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
				},
				spec: &workv1alpha2.ResourceBindingSpec{},
			},
			want: want{
				name:     "",
				clusters: []string{"member4", "member2", "member3", "member1"},
				groups: []want{
					{
						name:     "U2",
						clusters: []string{"member4", "member3"},
						groups: []want{
							{
								name:     "C4",
								clusters: []string{"member4"},
							},
							{
								name:     "C3",
								clusters: []string{"member3"},
							},
						},
					},
					{
						name:     "U1",
						clusters: []string{"member2", "member1"},
						groups: []want{
							{
								name:     "C2",
								clusters: []string{"member2"},
							},
							{
								name:     "C1",
								clusters: []string{"member1"},
							},
						},
					},
				},
			},
		},
		{
			name: "test SpreadConstraints is annotation unit/cell",
			arg: args{
				clustersScore: generateClusterScore(),
				placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByLabel: "unit@annotation",
							MaxGroups:     1,
							MinGroups:     1,
						},
						{
							SpreadByLabel: "cell@annotation",
							MaxGroups:     1,
							MinGroups:     1,
						},
					},
				},
				spec: &workv1alpha2.ResourceBindingSpec{},
			},
			want: want{
				name:     "",
				clusters: []string{"member4", "member2", "member3", "member1"},
				groups: []want{
					{
						name:     "U2",
						clusters: []string{"member4", "member3"},
						groups: []want{
							{
								name:     "C4",
								clusters: []string{"member4"},
							},
							{
								name:     "C3",
								clusters: []string{"member3"},
							},
						},
					},
					{
						name:     "U1",
						clusters: []string{"member2", "member1"},
						groups: []want{
							{
								name:     "C2",
								clusters: []string{"member2"},
							},
							{
								name:     "C1",
								clusters: []string{"member1"},
							},
						},
					},
				},
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
			factory := SelectionRegistry[DefaultSelectionFactoryName]
			selection, err := factory.create(SelectionCtx{
				ClusterScores: tt.arg.clustersScore,
				Placement:     tt.arg.placement,
				Spec:          tt.arg.spec,
				ReplicasFunc:  calAvailableReplicasFunc,
			})
			if err != nil {
				t.Errorf("%s : %s", tt.name, err.Error())
			}
			if root, ok := selection.(*groupRoot); ok {
				err := tt.want.match(&root.groupNode)
				if err != nil {
					t.Errorf("%s : %s", tt.name, err.Error())
				}
			}
		})
	}
}
