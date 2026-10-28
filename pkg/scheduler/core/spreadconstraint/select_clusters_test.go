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
	"errors"
	"fmt"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

// NewClusterWithTopology will build a Cluster with topology.
func NewClusterWithTopology(name, provider, region, zone string) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: clusterv1alpha1.ClusterSpec{
			Provider: provider,
			Region:   region,
			Zones:    []string{zone},
		},
	}
}

func generateClusterInfo() []ClusterDetailInfo {
	return []ClusterDetailInfo{
		{
			Name:              "member4",
			Score:             60,
			AvailableReplicas: 50,
			Cluster:           NewClusterWithTopology("member4", "P2", "R2", "Z2"),
		},
		{
			Name:              "member2",
			Score:             40,
			AvailableReplicas: 60,
			Cluster:           NewClusterWithTopology("member2", "P1", "R1", "Z2"),
		},
		{
			Name:              "member3",
			Score:             30,
			AvailableReplicas: 80,
			Cluster:           NewClusterWithTopology("member3", "P2", "R1", "Z1"),
		},
		{
			Name:              "member1",
			Score:             20,
			AvailableReplicas: 40,
			Cluster:           NewClusterWithTopology("member1", "P1", "R1", "Z1"),
		},
	}
}

func TestSelectBestClusters(t *testing.T) {
	clusterInfos := generateClusterInfo()
	type args struct {
		placement         *policyv1alpha1.Placement
		groupClustersInfo *GroupClustersInfo
		needReplicas      int32
	}
	tests := []struct {
		name    string
		args    args
		want    []ClusterDetailInfo
		wantErr error
	}{
		{
			name: "select clusters by cluster score",
			args: args{
				placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     2,
							MinGroups:     1,
						},
					},
				},
				groupClustersInfo: &GroupClustersInfo{
					Clusters: clusterInfos,
				},
				needReplicas: 100,
			},
			want: []ClusterDetailInfo{
				clusterInfos[0],
				clusterInfos[1],
			},
		},
		{
			name: "select clusters by cluster score and ignore available resources when scheduling strategy is duplicated",
			args: args{
				placement: &policyv1alpha1.Placement{
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
				groupClustersInfo: &GroupClustersInfo{
					Clusters: clusterInfos,
				},
				needReplicas: 120,
			},
			want: []ClusterDetailInfo{
				clusterInfos[0],
				clusterInfos[1],
			},
		},
		{
			name: "select clusters by cluster score and ignore available resources when scheduling strategy is static weight",
			args: args{
				placement: &policyv1alpha1.Placement{
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
				groupClustersInfo: &GroupClustersInfo{
					Clusters: clusterInfos,
				},
				needReplicas: 120,
			},
			want: []ClusterDetailInfo{
				clusterInfos[0],
				clusterInfos[1],
				clusterInfos[2],
				clusterInfos[3],
			},
		},
		{
			name: "select clusters by cluster score and satisfy available resources",
			args: args{
				placement: &policyv1alpha1.Placement{
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
				groupClustersInfo: &GroupClustersInfo{
					Clusters: clusterInfos,
				},
				needReplicas: 120,
			},
			want: []ClusterDetailInfo{
				clusterInfos[0],
				clusterInfos[2],
			},
		},
		{
			name: "select clusters by cluster score and insufficient resources",
			args: args{
				placement: &policyv1alpha1.Placement{
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
				groupClustersInfo: &GroupClustersInfo{
					Clusters: clusterInfos,
				},
				needReplicas: 200,
			},
			want:    nil,
			wantErr: fmt.Errorf("no enough resource when selecting %d clusters", 2),
		},
		{
			name: "select clusters by cluster score and exceeded the number of available clusters.",
			args: args{
				placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     7,
							MinGroups:     7,
						},
					},
				},
				groupClustersInfo: &GroupClustersInfo{
					Clusters: clusterInfos,
				},
				needReplicas: 30,
			},
			want:    nil,
			wantErr: fmt.Errorf("the number of feasible clusters is less than spreadConstraint.MinGroups"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectBestClusters(tt.args.placement, tt.args.groupClustersInfo, tt.args.needReplicas)
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

func TestSelectClustersByAvailableResourceDoesNotMutateCandidates(t *testing.T) {
	tests := []struct {
		name         string
		needReplicas int32
	}{
		{
			name:         "replacement succeeds",
			needReplicas: 120,
		},
		{
			name:         "capacity is insufficient",
			needReplicas: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := generateClusterInfo()
			wantCandidates := append([]ClusterDetailInfo(nil), candidates...)

			selectClustersByAvailableResource(candidates, 2, tt.needReplicas)

			if !reflect.DeepEqual(candidates, wantCandidates) {
				t.Errorf("selectClustersByAvailableResource() mutated candidates = %v, want %v", candidates, wantCandidates)
			}
		})
	}
}

func TestSelectBestClustersReturnsInsufficientCapacityError(t *testing.T) {
	clusterInfos := generateClusterInfo()
	placement := &policyv1alpha1.Placement{
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
	}
	wantClusters := []*clusterv1alpha1.Cluster{
		clusterInfos[0].Cluster,
		clusterInfos[1].Cluster,
		clusterInfos[2].Cluster,
		clusterInfos[3].Cluster,
	}

	_, err := SelectBestClusters(placement, &GroupClustersInfo{Clusters: clusterInfos}, 200)
	err = fmt.Errorf("failed to select clusters: %w", err)
	var capacityErr *framework.InsufficientCapacityError
	if !errors.As(err, &capacityErr) {
		t.Fatalf("SelectBestClusters() error = %v, want *framework.InsufficientCapacityError", err)
	}

	if !reflect.DeepEqual(capacityErr.CandidateClusters, wantClusters) {
		t.Errorf("InsufficientCapacityError.CandidateClusters = %v, want %v", capacityErr.CandidateClusters, wantClusters)
	}
	if want := "no enough resource when selecting 2 clusters"; capacityErr.Error() != want {
		t.Errorf("InsufficientCapacityError.Error() = %q, want %q", capacityErr.Error(), want)
	}
}

func TestSelectBestClustersDoesNotReturnInsufficientCapacityErrorForNonCapacityFailure(t *testing.T) {
	clusterInfos := generateClusterInfo()
	tests := []struct {
		name      string
		placement *policyv1alpha1.Placement
	}{
		{
			name: "insufficient feasible clusters",
			placement: &policyv1alpha1.Placement{
				SpreadConstraints: []policyv1alpha1.SpreadConstraint{
					{
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
						MaxGroups:     7,
						MinGroups:     7,
					},
				},
			},
		},
		{
			name: "unsupported spread topology",
			placement: &policyv1alpha1.Placement{
				SpreadConstraints: []policyv1alpha1.SpreadConstraint{
					{
						SpreadByField: policyv1alpha1.SpreadByFieldProvider,
						MaxGroups:     1,
						MinGroups:     1,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SelectBestClusters(tt.placement, &GroupClustersInfo{Clusters: clusterInfos}, 200)
			if err == nil {
				t.Fatal("SelectBestClusters() error = nil, want non-capacity error")
			}

			var capacityErr *framework.InsufficientCapacityError
			if errors.As(err, &capacityErr) {
				t.Fatalf("SelectBestClusters() error = %v, do not want *framework.InsufficientCapacityError", err)
			}
		})
	}
}
