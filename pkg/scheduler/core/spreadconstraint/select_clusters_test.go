package spreadconstraint

import (
	"fmt"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
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
		want    []*clusterv1alpha1.Cluster
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
			want: []*clusterv1alpha1.Cluster{
				clusterInfos[0].Cluster,
				clusterInfos[1].Cluster,
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
			want: []*clusterv1alpha1.Cluster{
				clusterInfos[0].Cluster,
				clusterInfos[1].Cluster,
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
			want: []*clusterv1alpha1.Cluster{
				clusterInfos[0].Cluster,
				clusterInfos[1].Cluster,
				clusterInfos[2].Cluster,
				clusterInfos[3].Cluster,
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
			want: []*clusterv1alpha1.Cluster{
				clusterInfos[0].Cluster,
				clusterInfos[2].Cluster,
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
