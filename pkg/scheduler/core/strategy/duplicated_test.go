package strategy

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/helper"
	"testing"
)

func Test_duplicatedScheduler(t *testing.T) {
	type args struct {
		spec     *workv1alpha2.ResourceBindingSpec
		strategy *policyv1alpha1.ReplicaSchedulingStrategy
		clusters []*clusterv1alpha1.Cluster
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha2.TargetCluster
		wantErr bool
	}{
		{
			name: "replicas 6, duplicated less",
			args: args{
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 6,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 1},
						{Name: ClusterMember2, Replicas: 2},
						{Name: ClusterMember3, Replicas: 3},
					},
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
				},
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 6},
				{Name: ClusterMember2, Replicas: 6},
				{Name: ClusterMember3, Replicas: 6},
			},
			wantErr: false,
		},
		{
			name: "replicas 6, duplicated all zero",
			args: args{
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 6,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 0},
						{Name: ClusterMember2, Replicas: 0},
						{Name: ClusterMember3, Replicas: 0},
					},
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
				},
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 6},
				{Name: ClusterMember2, Replicas: 6},
				{Name: ClusterMember3, Replicas: 6},
			},
			wantErr: false,
		},
		{
			name: "replicas 6, duplicated more",
			args: args{
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 6,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 12},
						{Name: ClusterMember2, Replicas: 24},
						{Name: ClusterMember3, Replicas: 36},
					},
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
				},
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 6},
				{Name: ClusterMember2, Replicas: 6},
				{Name: ClusterMember3, Replicas: 6},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if duplicatedScheduler, ok := GetAssignReplicas(tt.args.strategy); ok {
				got, err := duplicatedScheduler.AssignReplica(tt.args.spec, tt.args.strategy, tt.args.clusters)
				if (err != nil) != tt.wantErr {
					t.Errorf("DuplicatedScheduler error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !helper.IsScheduleResultEqual(got, tt.want) {
					t.Errorf("DuplicatedScheduler = %v, want %v", got, tt.want)
				}
			} else {
				t.Errorf("DuplicatedScheduler not found")
			}
		})
	}
}
