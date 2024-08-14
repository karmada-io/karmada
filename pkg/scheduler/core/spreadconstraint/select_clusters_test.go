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
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

// newCluster will build a Cluster with topology.
func newCluster(name, provider, region, zone string,
	labels map[string]string,
	annotations map[string]string) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: clusterv1alpha1.ClusterSpec{
			Provider: provider,
			Region:   region,
			Zones:    []string{zone},
		},
	}
}

type clusterScore struct {
	framework.ClusterScore
	replicas int32
}

func newClusterScore(cluster *clusterv1alpha1.Cluster, score int64, replicas int32) clusterScore {
	result := clusterScore{
		ClusterScore: framework.ClusterScore{
			Cluster: cluster,
			Score:   score,
		},
		replicas: replicas,
	}
	return result
}

func newClusterScoreList() []clusterScore {
	return []clusterScore{
		newClusterScore(newCluster("member4", "P2", "R2", "Z4", nil, nil), 60, 50),
		newClusterScore(newCluster("member2", "P1", "R1", "Z2", nil, nil), 40, 60),
		newClusterScore(newCluster("member3", "P2", "R2", "Z3", nil, nil), 30, 80),
		newClusterScore(newCluster("member1", "P1", "R1", "Z1", nil, nil), 20, 40),
	}
}

func TestSelectBestClusters(t *testing.T) {
	var clusterScores framework.ClusterScoreList
	replicas := make(map[string]int32)
	for _, score := range newClusterScoreList() {
		sc := score.ClusterScore
		clusterScores = append(clusterScores, sc)
		replicas[sc.Cluster.Name] = score.replicas
	}
	replicasFunc := func(clusters []*clusterv1alpha1.Cluster, _ *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster {
		var result []workv1alpha2.TargetCluster
		for _, c := range clusters {
			result = append(result, workv1alpha2.TargetCluster{
				Name:     c.Name,
				Replicas: replicas[c.Name],
			})
		}
		return result
	}
	tests := []struct {
		name    string
		ctx     SelectionCtx
		want    []*clusterv1alpha1.Cluster
		wantErr error
	}{
		{
			name: "select clusters by cluster score",
			ctx: SelectionCtx{
				ClusterScores: clusterScores,
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     2,
							MinGroups:     1,
						},
					},
				},
				Spec: &workv1alpha2.ResourceBindingSpec{
					Replicas: 100,
				},
				ReplicasFunc: replicasFunc,
			},
			want: []*clusterv1alpha1.Cluster{
				clusterScores[0].Cluster,
				clusterScores[1].Cluster,
			},
		},
		{
			name: "select clusters by cluster score and ignore available resources when scheduling strategy is duplicated",
			ctx: SelectionCtx{
				ClusterScores: clusterScores,
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     2,
							MinGroups:     1,
						},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
					},
				},
				Spec: &workv1alpha2.ResourceBindingSpec{
					Replicas: 120,
				},
				ReplicasFunc: replicasFunc,
			},
			want: []*clusterv1alpha1.Cluster{
				clusterScores[0].Cluster,
				clusterScores[1].Cluster,
			},
		},
		{
			name: "select clusters by cluster score and ignore available resources when scheduling strategy is static weight",
			ctx: SelectionCtx{
				ClusterScores: clusterScores,
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     2,
							MinGroups:     1,
						},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{
									TargetCluster: policyv1alpha1.ClusterAffinity{
										ClusterNames: []string{"member1"},
									},
									Weight: 2,
								},
							},
						},
					},
				},
				Spec: &workv1alpha2.ResourceBindingSpec{
					Replicas: 120,
				},
				ReplicasFunc: replicasFunc,
			},
			want: []*clusterv1alpha1.Cluster{
				clusterScores[0].Cluster,
				clusterScores[1].Cluster,
				clusterScores[2].Cluster,
				clusterScores[3].Cluster,
			},
		},
		{
			name: "select clusters by cluster score and satisfy available resources",
			ctx: SelectionCtx{
				ClusterScores: clusterScores,
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     2,
							MinGroups:     1,
						},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
					},
				},
				Spec: &workv1alpha2.ResourceBindingSpec{
					Replicas: 120,
				},
				ReplicasFunc: replicasFunc,
			},
			want: []*clusterv1alpha1.Cluster{
				clusterScores[0].Cluster,
				clusterScores[2].Cluster,
			},
		},
		{
			name: "select clusters by cluster score and insufficient resources",
			ctx: SelectionCtx{
				ClusterScores: clusterScores,
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     2,
							MinGroups:     1,
						},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
					},
				},
				Spec: &workv1alpha2.ResourceBindingSpec{
					Replicas: 200,
				},
				ReplicasFunc: replicasFunc,
			},
			want:    nil,
			wantErr: fmt.Errorf("no enough resource when selecting %d clusters", 2),
		},
		{
			name: "select clusters by cluster score and exceeded the number of available clusters.",
			ctx: SelectionCtx{
				ClusterScores: clusterScores,
				Placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     7,
							MinGroups:     7,
						},
					},
				},
				Spec: &workv1alpha2.ResourceBindingSpec{
					Replicas: 30,
				},
				ReplicasFunc: replicasFunc,
			},
			want:    nil,
			wantErr: fmt.Errorf("the number of feasible clusters is less than spreadConstraint.MinGroups"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectBestClusters(tt.ctx, "")
			if err != nil && err.Error() != tt.wantErr.Error() {
				t.Errorf("SelectBestClusters() error = %v, wantErr = %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SelectBestClusters() = %v, want %v", got, tt.want)
			}
		})
	}
}
