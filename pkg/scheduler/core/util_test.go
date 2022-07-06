package core

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/test/helper"
)

func Test_findOutScheduledCluster(t *testing.T) {
	type args struct {
		tcs        []workv1alpha2.TargetCluster
		candidates []*clusterv1alpha1.Cluster
	}
	tests := []struct {
		name string
		args args
		want []workv1alpha2.TargetCluster
	}{
		{
			name: "tcs: member1,member2,member3, candidates: member1,member2",
			args: args{
				tcs: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 1,
					},
					{
						Name:     ClusterMember2,
						Replicas: 1,
					},
					{
						Name:     ClusterMember3,
						Replicas: 1,
					},
				},
				candidates: []*clusterv1alpha1.Cluster{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: ClusterMember1,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: ClusterMember2,
						},
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
					Replicas: 1,
				},
			},
		},
		{
			name: "tcs: member1,member2, candidates: member1,member2,member3",
			args: args{
				tcs: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 1,
					},
					{
						Name:     ClusterMember2,
						Replicas: 1,
					},
				},
				candidates: []*clusterv1alpha1.Cluster{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: ClusterMember1,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: ClusterMember2,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: ClusterMember3,
						},
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
					Replicas: 1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findOutScheduledCluster(tt.args.tcs, tt.args.candidates); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findOutScheduledCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
