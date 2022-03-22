package util

import (
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
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

func TestIsBindingReplicasChanged(t *testing.T) {
	type args struct {
		bindingSpec *workv1alpha2.ResourceBindingSpec
		strategy    *policyv1alpha1.ReplicaSchedulingStrategy
	}
	var tests = []struct {
		name string
		args args
		want bool
	}{
		{
			name: "first example",
			args: args{
				bindingSpec: &workv1alpha2.ResourceBindingSpec{},
				strategy:    nil,
			},
			want: false,
		},
		{
			name: "second example",
			args: args{
				bindingSpec: &workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "second",
							Replicas: 3,
						},
					},
					Replicas: 3,
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: "Duplicated"},
			},
			want: false,
		},
		{
			name: "third example",
			args: args{
				bindingSpec: &workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "third",
							Replicas: 3,
						},
					},
					Replicas: 4,
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: "Duplicated"},
			},
			want: true,
		},
		{
			name: "fourth example",
			args: args{
				bindingSpec: &workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "fourth",
							Replicas: 3,
						},
					},
					Replicas: 4,
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: "Divided"},
			},
			want: true,
		},
		{
			name: "fifth example",
			args: args{
				bindingSpec: &workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "fifth",
							Replicas: 4,
						},
					},
					Replicas: 4,
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: "Divided"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBindingReplicasChanged(tt.args.bindingSpec, tt.args.strategy); got != tt.want {
				t.Errorf("IsBindingReplicasChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}
