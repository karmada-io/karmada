package strategy

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/test/helper"
	"testing"
)

func Test_divideReplicasByStaticWeight(t *testing.T) {
	type args struct {
		clusters   []*clusterv1alpha1.Cluster
		weightList []policyv1alpha1.StaticClusterWeight
		replicas   int32
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha2.TargetCluster
		wantErr bool
	}{
		{
			name: "replica 12, weight 3:2:1",
			args: args{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
				weightList: []policyv1alpha1.StaticClusterWeight{
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
				replicas: 12,
			},
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
			args: struct {
				clusters   []*clusterv1alpha1.Cluster
				weightList []policyv1alpha1.StaticClusterWeight
				replicas   int32
			}{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
				replicas: 12,
			},
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
			args: struct {
				clusters   []*clusterv1alpha1.Cluster
				weightList []policyv1alpha1.StaticClusterWeight
				replicas   int32
			}{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
				weightList: []policyv1alpha1.StaticClusterWeight{
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
				replicas: 14,
			},
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := divideReplicasByStaticWeight(tt.args.clusters, tt.args.weightList, tt.args.replicas)
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
