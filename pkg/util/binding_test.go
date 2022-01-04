package util

import (
	"testing"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

const (
	ClusterMember1 = "member1"
	ClusterMember2 = "member2"
	ClusterMember3 = "member3"
)

func TestDivideReplicasByTargetCluster(t *testing.T) {
	type args struct {
		clusters []workv1alpha2.TargetCluster
		sum      int32
	}
	tests := []struct {
		name string
		args args
		want []workv1alpha2.TargetCluster
	}{
		{
			name: "empty clusters",
			args: args{
				clusters: []workv1alpha2.TargetCluster{},
				sum:      10,
			},
			want: []workv1alpha2.TargetCluster{},
		},
		{
			name: "1 cluster, 5 replicas, 10 sum",
			args: args{
				clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 5,
					},
				},
				sum: 10,
			},
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 10,
				},
			},
		},
		{
			name: "3 cluster, 1:1:1, 12 sum",
			args: args{
				clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 5,
					},
					{
						Name:     ClusterMember2,
						Replicas: 5,
					},
					{
						Name:     ClusterMember3,
						Replicas: 5,
					},
				},
				sum: 12,
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
		},
		{
			name: "3 cluster, 1:1:1, 10 sum",
			args: args{
				clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 5,
					},
					{
						Name:     ClusterMember2,
						Replicas: 5,
					},
					{
						Name:     ClusterMember3,
						Replicas: 5,
					},
				},
				sum: 10,
			},
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 4,
				},
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
				{
					Name:     ClusterMember3,
					Replicas: 3,
				},
			},
		},
		{
			name: "3 cluster, 1:2:3, 13 sum",
			args: args{
				clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 1,
					},
					{
						Name:     ClusterMember2,
						Replicas: 2,
					},
					{
						Name:     ClusterMember3,
						Replicas: 3,
					},
				},
				sum: 13,
			},
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
				{
					Name:     ClusterMember2,
					Replicas: 4,
				},
				{
					Name:     ClusterMember3,
					Replicas: 6,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DivideReplicasByTargetCluster(tt.args.clusters, tt.args.sum); !testhelper.IsScheduleResultEqual(got, tt.want) {
				t.Errorf("DivideReplicasByTargetCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}
