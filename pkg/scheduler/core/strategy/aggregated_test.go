package strategy

//
//import (
//	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
//	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
//	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
//	"github.com/karmada-io/karmada/pkg/util"
//	"github.com/karmada-io/karmada/test/helper"
//	corev1 "k8s.io/api/core/v1"
//	"k8s.io/apimachinery/pkg/api/resource"
//	"testing"
//)
//
//func Test_divideReplicasByPreference(t *testing.T) {
//	type args struct {
//		clusterAvailableReplicas []workv1alpha2.TargetCluster
//		replicas                 int32
//		clustersMaxReplicas      int32
//		preference               policyv1alpha1.ReplicaDivisionPreference
//		scheduledClusterNames    sets.String
//	}
//	tests := []struct {
//		name    string
//		args    args
//		want    []workv1alpha2.TargetCluster
//		wantErr bool
//	}{
//		{
//			name: "replica 12, dynamic weight 18:12:6",
//			args: args{
//				clusterAvailableReplicas: TargetClustersList{
//					workv1alpha2.TargetCluster{Name: ClusterMember1, Replicas: 18},
//					workv1alpha2.TargetCluster{Name: ClusterMember2, Replicas: 12},
//					workv1alpha2.TargetCluster{Name: ClusterMember3, Replicas: 6},
//				},
//				replicas:              12,
//				clustersMaxReplicas:   36,
//				preference:            policyv1alpha1.ReplicaDivisionPreferenceWeighted,
//				scheduledClusterNames: sets.NewString(),
//			},
//			want: []workv1alpha2.TargetCluster{
//				{Name: ClusterMember1, Replicas: 6},
//				{Name: ClusterMember2, Replicas: 4},
//				{Name: ClusterMember3, Replicas: 2},
//			},
//			wantErr: false,
//		},
//		{
//			name: "replica 12, dynamic weight 20:12:6",
//			args: args{
//				clusterAvailableReplicas: TargetClustersList{
//					workv1alpha2.TargetCluster{Name: ClusterMember1, Replicas: 20},
//					workv1alpha2.TargetCluster{Name: ClusterMember2, Replicas: 12},
//					workv1alpha2.TargetCluster{Name: ClusterMember3, Replicas: 6},
//				},
//				replicas:              12,
//				clustersMaxReplicas:   38,
//				preference:            policyv1alpha1.ReplicaDivisionPreferenceWeighted,
//				scheduledClusterNames: sets.NewString(),
//			},
//			want: []workv1alpha2.TargetCluster{
//				{Name: ClusterMember1, Replicas: 7},
//				{Name: ClusterMember2, Replicas: 4},
//				{Name: ClusterMember3, Replicas: 1},
//			},
//			wantErr: false,
//		},
//		{
//			name: "replica 12, dynamic weight 6:12:6",
//			args: args{
//				clusterAvailableReplicas: TargetClustersList{
//					workv1alpha2.TargetCluster{Name: ClusterMember1, Replicas: 6},
//					workv1alpha2.TargetCluster{Name: ClusterMember2, Replicas: 12},
//					workv1alpha2.TargetCluster{Name: ClusterMember3, Replicas: 6},
//				},
//				replicas:              12,
//				clustersMaxReplicas:   24,
//				preference:            policyv1alpha1.ReplicaDivisionPreferenceWeighted,
//				scheduledClusterNames: sets.NewString(),
//			},
//			want: []workv1alpha2.TargetCluster{
//				{Name: ClusterMember1, Replicas: 3},
//				{Name: ClusterMember2, Replicas: 6},
//				{Name: ClusterMember3, Replicas: 3},
//			},
//			wantErr: false,
//		},
//		{
//			name: "replica 12, aggregated 12:6:6",
//			args: args{
//				clusterAvailableReplicas: TargetClustersList{
//					workv1alpha2.TargetCluster{Name: ClusterMember2, Replicas: 12},
//					workv1alpha2.TargetCluster{Name: ClusterMember1, Replicas: 6},
//					workv1alpha2.TargetCluster{Name: ClusterMember3, Replicas: 6},
//				},
//				replicas:              12,
//				clustersMaxReplicas:   24,
//				preference:            policyv1alpha1.ReplicaDivisionPreferenceAggregated,
//				scheduledClusterNames: sets.NewString(),
//			},
//			want: []workv1alpha2.TargetCluster{
//				{Name: ClusterMember2, Replicas: 12},
//			},
//			wantErr: false,
//		},
//		{
//			name: "replica 12, aggregated 6:6:6",
//			args: args{
//				clusterAvailableReplicas: TargetClustersList{
//					workv1alpha2.TargetCluster{Name: ClusterMember1, Replicas: 6},
//					workv1alpha2.TargetCluster{Name: ClusterMember2, Replicas: 6},
//					workv1alpha2.TargetCluster{Name: ClusterMember3, Replicas: 6},
//				},
//				replicas:              12,
//				clustersMaxReplicas:   18,
//				preference:            policyv1alpha1.ReplicaDivisionPreferenceAggregated,
//				scheduledClusterNames: sets.NewString(),
//			},
//			want: []workv1alpha2.TargetCluster{
//				{Name: ClusterMember1, Replicas: 6},
//				{Name: ClusterMember2, Replicas: 6},
//			},
//			wantErr: false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			got, err := divideReplicasByPreference(tt.args.clusterAvailableReplicas, tt.args.replicas, tt.args.preference, tt.args.scheduledClusterNames)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("divideReplicasByPreference() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !helper.IsScheduleResultEqual(got, tt.want) {
//				t.Errorf("divideReplicasByPreference() got = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
//
//func Test_divideReplicasByResource(t *testing.T) {
//	type args struct {
//		clusters   []*clusterv1alpha1.Cluster
//		spec       *workv1alpha2.ResourceBindingSpec
//		preference policyv1alpha1.ReplicaDivisionPreference
//	}
//	tests := []struct {
//		name    string
//		args    args
//		want    []workv1alpha2.TargetCluster
//		wantErr bool
//	}{
//		{
//			name: "replica 12, dynamic weight 6:8:10",
//			args: args{
//				clusters: []*clusterv1alpha1.Cluster{
//					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(6, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(8, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//				},
//				spec: &workv1alpha2.ResourceBindingSpec{
//					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
//						ResourceRequest: util.EmptyResource().ResourceList(),
//					},
//					Replicas: 12,
//				},
//				preference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
//			},
//			want: []workv1alpha2.TargetCluster{
//				{Name: ClusterMember1, Replicas: 3},
//				{Name: ClusterMember2, Replicas: 4},
//				{Name: ClusterMember3, Replicas: 5},
//			},
//			wantErr: false,
//		},
//		{
//			name: "replica 12, dynamic weight 8:8:10",
//			args: args{
//				clusters: []*clusterv1alpha1.Cluster{
//					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(8, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(8, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//				},
//				spec: &workv1alpha2.ResourceBindingSpec{
//					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
//						ResourceRequest: util.EmptyResource().ResourceList(),
//					},
//					Replicas: 12,
//				},
//				preference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
//			},
//			want: []workv1alpha2.TargetCluster{
//				{Name: ClusterMember1, Replicas: 4},
//				{Name: ClusterMember2, Replicas: 3},
//				{Name: ClusterMember3, Replicas: 5},
//			},
//			wantErr: false,
//		},
//		{
//			name: "replica 12, dynamic weight 3:3:3",
//			args: args{
//				clusters: []*clusterv1alpha1.Cluster{
//					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(3, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(3, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(3, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//				},
//				spec: &workv1alpha2.ResourceBindingSpec{
//					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
//						ResourceRequest: util.EmptyResource().ResourceList(),
//					},
//					Replicas: 12,
//				},
//				preference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
//			},
//			wantErr: true,
//		},
//		{
//			name: "replica 12, aggregated 6:8:10",
//			args: args{
//				clusters: []*clusterv1alpha1.Cluster{
//					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(6, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(8, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//				},
//				spec: &workv1alpha2.ResourceBindingSpec{
//					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
//						ResourceRequest: util.EmptyResource().ResourceList(),
//					},
//					Replicas: 12,
//				},
//				preference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
//			},
//			want: []workv1alpha2.TargetCluster{
//				{Name: ClusterMember2, Replicas: 5},
//				{Name: ClusterMember3, Replicas: 7},
//			},
//			wantErr: false,
//		},
//		{
//			name: "replica 12, aggregated 12:8:10",
//			args: args{
//				clusters: []*clusterv1alpha1.Cluster{
//					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(12, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(8, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//				},
//				spec: &workv1alpha2.ResourceBindingSpec{
//					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
//						ResourceRequest: util.EmptyResource().ResourceList(),
//					},
//					Replicas: 12,
//				},
//				preference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
//			},
//			want: []workv1alpha2.TargetCluster{
//				{Name: ClusterMember1, Replicas: 12},
//			},
//			wantErr: false,
//		},
//		{
//			name: "replica 12, aggregated 3:3:3",
//			args: args{
//				clusters: []*clusterv1alpha1.Cluster{
//					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(3, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(3, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
//						corev1.ResourcePods: *resource.NewQuantity(3, resource.DecimalSI),
//					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
//				},
//				spec: &workv1alpha2.ResourceBindingSpec{
//					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
//						ResourceRequest: util.EmptyResource().ResourceList(),
//					},
//					Replicas: 12,
//				},
//				preference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
//			},
//			wantErr: true,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			got, err := divideReplicasByResource(tt.args.clusters, tt.args.spec, tt.args.preference)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("divideReplicasByResource() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !helper.IsScheduleResultEqual(got, tt.want) {
//				t.Errorf("divideReplicasByResource() got = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
