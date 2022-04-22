package strategy

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/helper"
	"testing"
)

func Test_divideReplicasByStaticWeight(t *testing.T) {
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
			name: "replicas 12, static weight 3:2:1",
			args: args{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 12,
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
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
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 6},
				{Name: ClusterMember2, Replicas: 4},
				{Name: ClusterMember3, Replicas: 2},
			},
			wantErr: false,
		},
		{
			name: "replicas 12, default weight average",
			args: struct {
				spec     *workv1alpha2.ResourceBindingSpec
				strategy *policyv1alpha1.ReplicaSchedulingStrategy
				clusters []*clusterv1alpha1.Cluster
			}{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 12,
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 4},
				{Name: ClusterMember2, Replicas: 4},
				{Name: ClusterMember3, Replicas: 4},
			},
			wantErr: false,
		},
		{
			name: "replicas 14, static weight 3:2:1",
			args: struct {
				spec     *workv1alpha2.ResourceBindingSpec
				strategy *policyv1alpha1.ReplicaSchedulingStrategy
				clusters []*clusterv1alpha1.Cluster
			}{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 14,
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
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
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 8},
				{Name: ClusterMember2, Replicas: 4},
				{Name: ClusterMember3, Replicas: 2},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if staticWeightScheduler, ok := GetAssignReplicas(tt.args.strategy); ok {
				got, err := staticWeightScheduler.AssignReplica(tt.args.spec, tt.args.strategy, tt.args.clusters)
				if (err != nil) != tt.wantErr {
					t.Errorf("StaticWeightScheduler error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !helper.IsScheduleResultEqual(got, tt.want) {
					t.Errorf("StaticWeightScheduler = %v, want %v", got, tt.want)
				}
			} else {
				t.Errorf("StaticWeightScheduler not found")
			}
		})
	}
}
