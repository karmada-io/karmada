package core

import (
	"reflect"
	"testing"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/test/helper"
)

func Test_attachZeroReplicasCluster(t *testing.T) {
	type args struct {
		clusters       []*clusterv1alpha1.Cluster
		targetClusters []workv1alpha2.TargetCluster
	}
	tests := []struct {
		name string
		args args
		want []workv1alpha2.TargetCluster
	}{
		{
			name: "clusters: member1,member2,member3, targetClusters:member1,member2",
			args: args{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
				targetClusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 1,
					},
					{
						Name:     ClusterMember2,
						Replicas: 2,
					},
				},
			},
			want: []workv1alpha2.TargetCluster{
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
					Replicas: 0,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := attachZeroReplicasCluster(tt.args.clusters, tt.args.targetClusters); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attachZeroReplicasCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}
