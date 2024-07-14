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
			name: "select cluster from the region with enough clusters and higher score",
			args: args{
				spreadConstraintMap: map[policyv1alpha1.SpreadFieldValue]policyv1alpha1.SpreadConstraint{
					policyv1alpha1.SpreadByFieldRegion: {
						SpreadByField: policyv1alpha1.SpreadByFieldRegion,
						MaxGroups:     2,
						MinGroups:     2,
					},
					policyv1alpha1.SpreadByFieldCluster: {
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
						MaxGroups:     5,
						MinGroups:     4,
					},
				},
				groupClustersInfo: &GroupClustersInfo{
					Regions: map[string]RegionInfo{
						"R1": {
							Name:  "R1",
							Score: 90,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member1",
									Score:             90,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member1", "P1", "R1", "Z1"),
								},
							},
						},
						"R2": {
							Name:  "R2",
							Score: 70,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member2",
									Score:             70,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member2", "P1", "R2", "Z1"),
								},
								{
									Name:              "member3",
									Score:             60,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member3", "P1", "R2", "Z1"),
								},
								{
									Name:              "member4",
									Score:             30,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member4", "P1", "R2", "Z1"),
								},
							},
						},
						"R3": {
							Name:  "R3",
							Score: 80,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member5",
									Score:             80,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member5", "P1", "R3", "Z1"),
								},
								{
									Name:              "member6",
									Score:             50,
									AvailableReplicas: 60,
									Cluster:           NewClusterWithTopology("member6", "P1", "R3", "Z1"),
								},
							},
						},
						"R4": {
							Name:  "R4",
							Score: 50,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member7",
									Score:             50,
									AvailableReplicas: 50,
									Cluster:           NewClusterWithTopology("member7", "P1", "R4", "Z1"),
								},
								{
									Name:              "member8",
									Score:             40,
									AvailableReplicas: 50,
									Cluster:           NewClusterWithTopology("member8", "P1", "R4", "Z1"),
								},
								{
									Name:              "member9",
									Score:             10,
									AvailableReplicas: 50,
									Cluster:           NewClusterWithTopology("member9", "P1", "R4", "Z1"),
								},
								{
									Name:              "member10",
									Score:             0,
									AvailableReplicas: 50,
									Cluster:           NewClusterWithTopology("member10", "P1", "R4", "Z1"),
								},
							},
						},
					},
				},
			},
			want: []*clusterv1alpha1.Cluster{
				NewClusterWithTopology("member1", "P1", "R1", "Z1"),
				NewClusterWithTopology("member2", "P1", "R2", "Z1"),
				NewClusterWithTopology("member3", "P1", "R2", "Z1"),
				NewClusterWithTopology("member4", "P1", "R2", "Z1"),
			},
			wantErr: false,
		},
		{
			name: "select cluster from more than `minGroups` regions with enough clusters and higher score",
			args: args{
				spreadConstraintMap: map[policyv1alpha1.SpreadFieldValue]policyv1alpha1.SpreadConstraint{
					policyv1alpha1.SpreadByFieldRegion: {
						SpreadByField: policyv1alpha1.SpreadByFieldRegion,
						MaxGroups:     3,
						MinGroups:     1,
					},
					policyv1alpha1.SpreadByFieldCluster: {
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
						MaxGroups:     8,
						MinGroups:     6,
					},
				},
				groupClustersInfo: &GroupClustersInfo{
					Regions: map[string]RegionInfo{
						"R1": {
							Name:  "R1",
							Score: 70,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member1",
									Score:             70,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member1", "P1", "R1", "Z1"),
								},
							},
						},
						"R2": {
							Name:  "R2",
							Score: 40,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member2",
									Score:             40,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member2", "P1", "R2", "Z1"),
								},
								{
									Name:              "member3",
									Score:             40,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member3", "P1", "R2", "Z1"),
								},
								{
									Name:              "member4",
									Score:             40,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member4", "P1", "R2", "Z1"),
								},
							},
						},
						"R3": {
							Name:  "R3",
							Score: 60,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member5",
									Score:             60,
									AvailableReplicas: 40,
									Cluster:           NewClusterWithTopology("member5", "P1", "R3", "Z1"),
								},
								{
									Name:              "member6",
									Score:             50,
									AvailableReplicas: 60,
									Cluster:           NewClusterWithTopology("member6", "P1", "R3", "Z1"),
								},
							},
						},
						"R4": {
							Name:  "R4",
							Score: 70,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member7",
									Score:             70,
									AvailableReplicas: 100,
									Cluster:           NewClusterWithTopology("member7", "P1", "R4", "Z1"),
								},
							},
						},
						"R5": {
							Name:  "R5",
							Score: 50,
							Clusters: []ClusterDetailInfo{
								{
									Name:              "member8",
									Score:             50,
									AvailableReplicas: 50,
									Cluster:           NewClusterWithTopology("member8", "P1", "R5", "Z1"),
								},
							},
						},
					},
				},
			},
			want: []*clusterv1alpha1.Cluster{
				NewClusterWithTopology("member1", "P1", "R1", "Z1"),
				NewClusterWithTopology("member5", "P1", "R3", "Z1"),
				NewClusterWithTopology("member2", "P1", "R2", "Z1"),
				NewClusterWithTopology("member6", "P1", "R3", "Z1"),
				NewClusterWithTopology("member3", "P1", "R2", "Z1"),
				NewClusterWithTopology("member4", "P1", "R2", "Z1"),
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
									AvailableReplicas: 60,
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
			name: "no more regions to replace",
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
									AvailableReplicas: 20,
									Cluster:           NewClusterWithTopology("member2", "P1", "R1", "Z1"),
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
