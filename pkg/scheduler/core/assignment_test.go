package core

import (
	"testing"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/test/helper"
)

func Test_assignByStaticWeightStrategy(t *testing.T) {
	tests := []struct {
		name             string
		clusters         []*clusterv1alpha1.Cluster
		weightPreference *policyv1alpha1.ClusterPreferences
		replicas         int32
		want             []workv1alpha2.TargetCluster
		wantErr          bool
	}{
		{
			name: "replica 12, weight 3:2:1",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
				helper.NewCluster(ClusterMember3),
			},
			weightPreference: &policyv1alpha1.ClusterPreferences{
				StaticWeightList: []policyv1alpha1.StaticClusterWeight{
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember1},
						},
						Weight: 3,
					},
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember2},
						},
						Weight: 2,
					},
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember3},
						},
						Weight: 1,
					},
				},
			},
			replicas: 12,
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 6,
				},
				{
					Name:     ClusterMember2,
					Replicas: 4,
				},
				{
					Name:     ClusterMember3,
					Replicas: 2,
				},
			},
			wantErr: false,
		},
		{
			name: "replica 12, default weight",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
				helper.NewCluster(ClusterMember3),
			},
			replicas: 12,
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 4,
				},
				{
					Name:     ClusterMember2,
					Replicas: 4,
				},
				{
					Name:     ClusterMember3,
					Replicas: 4,
				},
			},
			wantErr: false,
		},
		{
			name: "replica 14, weight 3:2:1",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
				helper.NewCluster(ClusterMember3),
			},
			weightPreference: &policyv1alpha1.ClusterPreferences{
				StaticWeightList: []policyv1alpha1.StaticClusterWeight{
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember1},
						},
						Weight: 3,
					},
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember2},
						},
						Weight: 2,
					},
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember3},
						},
						Weight: 1,
					},
				},
			},
			replicas: 14,
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 8,
				},
				{
					Name:     ClusterMember2,
					Replicas: 4,
				},
				{
					Name:     ClusterMember3,
					Replicas: 2,
				},
			},
			wantErr: false,
		},
		{
			name: "insufficient replica assignment should get 0 replica",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
			},
			weightPreference: &policyv1alpha1.ClusterPreferences{
				StaticWeightList: []policyv1alpha1.StaticClusterWeight{
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember1},
						},
						Weight: 1,
					},
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember2},
						},
						Weight: 1,
					},
				},
			},
			replicas: 0,
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 0,
				},
				{
					Name:     ClusterMember2,
					Replicas: 0,
				},
			},
			wantErr: false,
		},
		{
			name: "selected cluster without weight should be ignored",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
			},
			weightPreference: &policyv1alpha1.ClusterPreferences{
				StaticWeightList: []policyv1alpha1.StaticClusterWeight{
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember1},
						},
						Weight: 1,
					},
				},
			},
			replicas: 2,
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
			},
			wantErr: false,
		},
		{
			name: "cluster with multiple weights",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
			},
			weightPreference: &policyv1alpha1.ClusterPreferences{
				StaticWeightList: []policyv1alpha1.StaticClusterWeight{
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember1},
						},
						Weight: 1,
					},
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember2},
						},
						Weight: 1,
					},
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember1},
						},
						Weight: 2,
					},
				},
			},
			replicas: 3,
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 1,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assignByStaticWeightStrategy(&assignState{
				candidates: tt.clusters,
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					WeightPreference:          tt.weightPreference,
				},
				object: &workv1alpha2.ResourceBindingSpec{Replicas: tt.replicas},
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("divideReplicasByStaticWeight() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !helper.IsScheduleResultEqual(got, tt.want) {
				t.Errorf("divideReplicasByStaticWeight() got = %v, want %v", got, tt.want)
			}
		})
	}
}
