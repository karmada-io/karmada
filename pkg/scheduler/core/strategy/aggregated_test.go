package strategy

import (
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/helper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"testing"
)

func Test_aggregatedScheduler(t *testing.T) {
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
			name: "replicas 12, max replicas 24, aggregated 12:6:6",
			args: args{
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 12,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 12},
						{Name: ClusterMember2, Replicas: 6},
						{Name: ClusterMember3, Replicas: 6},
					},
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
				},
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 12},
			},
			wantErr: false,
		},
		{
			name: "replica 12, max replicas 18, aggregated 6:6:6",
			args: args{
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 12,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember2, Replicas: 6},
						{Name: ClusterMember1, Replicas: 6},
						{Name: ClusterMember3, Replicas: 6},
					},
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
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
			},
			wantErr: false,
		},
		{
			name: "replica 12, max replicas 18, aggregated 6:8:10",
			args: args{
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 12,
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
				},
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(6, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(8, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember2, Replicas: 5},
				{Name: ClusterMember3, Replicas: 7},
			},
			wantErr: false,
		},
		{
			name: "replica 12, aggregated 12:8:10",
			args: args{
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 12,
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
				},
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(12, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(8, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 12},
			},
			wantErr: false,
		},
		{
			name: "replica 12, aggregated 3:3:3",
			args: args{
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 12,
				},
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
				},
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(3, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(3, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(3, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if aggregatedScheduler, ok := GetAssignReplicas(tt.args.strategy); ok {
				got, err := aggregatedScheduler.AssignReplica(tt.args.spec, tt.args.strategy, tt.args.clusters)
				if (err != nil) != tt.wantErr {
					t.Errorf("aggregatedScheduler error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !helper.IsScheduleResultEqual(got, tt.want) {
					t.Errorf("aggregatedScheduler = %v, want %v", got, tt.want)
				}
			} else {
				t.Errorf("aggregatedScheduler not found")
			}
		})
	}
}
