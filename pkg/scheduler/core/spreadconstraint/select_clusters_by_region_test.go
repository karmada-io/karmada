package spreadconstraint

import (
	"reflect"
	"testing"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func Test_selectBestClustersByRegion(t *testing.T) {
	type args struct {
		spreadConstraintMap map[policyv1alpha1.SpreadFieldValue]policyv1alpha1.SpreadConstraint
		groupClustersInfo   *GroupClustersInfo
	}
	tests := []struct {
		name    string
		args    args
		want    []*clusterv1alpha1.Cluster
		wantErr bool
	}{
		{
			name: "select cluster from the region with the higher score",
			args: args{
				spreadConstraintMap: map[policyv1alpha1.SpreadFieldValue]policyv1alpha1.SpreadConstraint{
					policyv1alpha1.SpreadByFieldRegion: {
						SpreadByField: policyv1alpha1.SpreadByFieldRegion,
						MaxGroups:     1,
						MinGroups:     1,
					},
					policyv1alpha1.SpreadByFieldCluster: {
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
						MaxGroups:     2,
						MinGroups:     2,
					},
				},
				groupClustersInfo: &GroupClustersInfo{
					Regions: map[string]RegionInfo{
						"R1": {
							Name:  "R1",
							Score: 100,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member1",
									Score:             100,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member1", "P1", "R1", "Z1"),
								},
								{
									Name:              "member2",
									Score:             80,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member2", "P1", "R1", "Z1"),
								},
							},
						},
						"R2": {
							Name:  "R2",
							Score: 80,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member3",
									Score:             80,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member3", "P1", "R2", "Z1"),
								},
								{
									Name:              "member4",
									Score:             80,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member4", "P1", "R2", "Z1"),
								},
								{
									Name:              "member5",
									Score:             60,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member5", "P1", "R2", "Z1"),
								},
							},
						},
					},
				},
			},
			want: []*clusterv1alpha1.Cluster{
				NewClusterWithTopology("member1", "P1", "R1", "Z1"),
				NewClusterWithTopology("member2", "P1", "R1", "Z1"),
			},
			wantErr: false,
		},
		{
			name: "select cluster from the region with enough clusters",
			args: args{
				spreadConstraintMap: map[policyv1alpha1.SpreadFieldValue]policyv1alpha1.SpreadConstraint{
					policyv1alpha1.SpreadByFieldRegion: {
						SpreadByField: policyv1alpha1.SpreadByFieldRegion,
						MaxGroups:     1,
						MinGroups:     1,
					},
					policyv1alpha1.SpreadByFieldCluster: {
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
						MaxGroups:     3,
						MinGroups:     3,
					},
				},
				groupClustersInfo: &GroupClustersInfo{
					Regions: map[string]RegionInfo{
						"R1": {
							Name:  "R1",
							Score: 100,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member1",
									Score:             100,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member1", "P1", "R1", "Z1"),
								},
								{
									Name:              "member2",
									Score:             80,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member2", "P1", "R1", "Z1"),
								},
							},
						},
						"R2": {
							Name:  "R2",
							Score: 80,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member3",
									Score:             80,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member3", "P1", "R2", "Z1"),
								},
								{
									Name:              "member4",
									Score:             80,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member4", "P1", "R2", "Z1"),
								},
								{
									Name:              "member5",
									Score:             60,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member5", "P1", "R2", "Z1"),
								},
							},
						},
					},
				},
			},
			want: []*clusterv1alpha1.Cluster{
				NewClusterWithTopology("member3", "P1", "R2", "Z1"),
				NewClusterWithTopology("member4", "P1", "R2", "Z1"),
				NewClusterWithTopology("member5", "P1", "R2", "Z1"),
			},
			wantErr: false,
		},
		{
			name: "no enough region",
			args: args{
				spreadConstraintMap: map[policyv1alpha1.SpreadFieldValue]policyv1alpha1.SpreadConstraint{
					policyv1alpha1.SpreadByFieldRegion: {
						SpreadByField: policyv1alpha1.SpreadByFieldRegion,
						MaxGroups:     3,
						MinGroups:     3,
					},
					policyv1alpha1.SpreadByFieldCluster: {
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
						MaxGroups:     3,
						MinGroups:     3,
					},
				},
				groupClustersInfo: &GroupClustersInfo{
					Regions: map[string]RegionInfo{
						"R1": {
							Name:  "R1",
							Score: 100,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member1",
									Score:             100,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member1", "P1", "R1", "Z1"),
								},
								{
									Name:              "member2",
									Score:             80,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member2", "P1", "R1", "Z1"),
								},
							},
						},
						"R2": {
							Name:  "R2",
							Score: 80,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member3",
									Score:             80,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member3", "P1", "R2", "Z1"),
								},
								{
									Name:              "member4",
									Score:             80,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member4", "P1", "R2", "Z1"),
								},
								{
									Name:              "member5",
									Score:             60,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member5", "P1", "R2", "Z1"),
								},
							},
						},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "no enough clusters",
			args: args{
				spreadConstraintMap: map[policyv1alpha1.SpreadFieldValue]policyv1alpha1.SpreadConstraint{
					policyv1alpha1.SpreadByFieldRegion: {
						SpreadByField: policyv1alpha1.SpreadByFieldRegion,
						MaxGroups:     1,
						MinGroups:     1,
					},
					policyv1alpha1.SpreadByFieldCluster: {
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
						MaxGroups:     4,
						MinGroups:     4,
					},
				},
				groupClustersInfo: &GroupClustersInfo{
					Regions: map[string]RegionInfo{
						"R1": {
							Name:  "R1",
							Score: 100,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member1",
									Score:             100,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member1", "P1", "R1", "Z1"),
								},
								{
									Name:              "member2",
									Score:             80,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member2", "P1", "R1", "Z1"),
								},
							},
						},
						"R2": {
							Name:  "R2",
							Score: 80,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member3",
									Score:             80,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member3", "P1", "R2", "Z1"),
								},
								{
									Name:              "member4",
									Score:             80,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member4", "P1", "R2", "Z1"),
								},
								{
									Name:              "member5",
									Score:             60,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member5", "P1", "R2", "Z1"),
								},
							},
						},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectBestClustersByRegion(tt.args.spreadConstraintMap, tt.args.groupClustersInfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("selectBestClustersByRegion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("selectBestClustersByRegion() = %v, want %v", got, tt.want)
			}
		})
	}
}
