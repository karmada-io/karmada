package core

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	utilhelper "github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/test/helper"
)

const (
	ClusterMember1 = "member1"
	ClusterMember2 = "member2"
	ClusterMember3 = "member3"
	ClusterMember4 = "member4"
)

func Test_dispenser_takeByWeight(t *testing.T) {
	tests := []struct {
		name        string
		numReplicas int32
		result      []workv1alpha2.TargetCluster
		weightList  utilhelper.ClusterWeightInfoList
		desired     []workv1alpha2.TargetCluster
		done        bool
	}{
		{
			name:        "Scale up 6 replicas",
			numReplicas: 6,
			result: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 1},
				{Name: "B", Replicas: 2},
				{Name: "C", Replicas: 3},
			},
			weightList: []utilhelper.ClusterWeightInfo{
				{ClusterName: "A", Weight: 1},
				{ClusterName: "B", Weight: 2},
				{ClusterName: "C", Weight: 3},
			},
			desired: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 2},
				{Name: "B", Replicas: 4},
				{Name: "C", Replicas: 6},
			},
			done: true,
		},
		{
			name:        "Scale up 3 replicas",
			numReplicas: 3,
			result: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 1},
				{Name: "B", Replicas: 2},
				{Name: "C", Replicas: 3},
			},
			weightList: []utilhelper.ClusterWeightInfo{
				{ClusterName: "A", Weight: 1},
				{ClusterName: "B", Weight: 2},
				{ClusterName: "C", Weight: 3},
			},
			desired: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 1},
				{Name: "B", Replicas: 3},
				{Name: "C", Replicas: 5},
			},
			done: true,
		},
		{
			name:        "Scale up 2 replicas",
			numReplicas: 2,
			result: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 1},
				{Name: "B", Replicas: 2},
				{Name: "C", Replicas: 3},
			},
			weightList: []utilhelper.ClusterWeightInfo{
				{ClusterName: "A", Weight: 1},
				{ClusterName: "B", Weight: 2},
				{ClusterName: "C", Weight: 3},
			},
			desired: []workv1alpha2.TargetCluster{
				{Name: "A", Replicas: 1},
				{Name: "B", Replicas: 2},
				{Name: "C", Replicas: 5},
			},
			done: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newDispenser(tt.numReplicas, tt.result)
			a.takeByWeight(tt.weightList)
			if a.done() != tt.done {
				t.Errorf("expected after takeByWeight: %v, but got: %v", tt.done, a.done())
			}
			if !helper.IsScheduleResultEqual(a.result, tt.desired) {
				t.Errorf("expected result after takeByWeight: %v, but got: %v", tt.desired, a.result)
			}
		})
	}
}

func Test_divideRemainingReplicas(t *testing.T) {
	type args struct {
		remainingReplicas   int
		desiredReplicaInfos map[string]int64
		clusterNames        []string
	}
	tests := []struct {
		name string
		args args
		want map[string]int64
	}{
		{
			name: "remainingReplicas 13",
			args: args{
				remainingReplicas: 13,
				desiredReplicaInfos: map[string]int64{
					ClusterMember1: 2,
					ClusterMember2: 3,
					ClusterMember3: 4,
				},
				clusterNames: []string{
					ClusterMember1, ClusterMember2, ClusterMember3,
				},
			},
			want: map[string]int64{
				ClusterMember1: 7,
				ClusterMember2: 7,
				ClusterMember3: 8,
			},
		},
		{
			name: "remainingReplicas 17",
			args: args{
				remainingReplicas: 17,
				desiredReplicaInfos: map[string]int64{
					ClusterMember1: 4,
					ClusterMember2: 3,
					ClusterMember3: 2,
				},
				clusterNames: []string{
					ClusterMember1, ClusterMember2, ClusterMember3,
				},
			},
			want: map[string]int64{
				ClusterMember1: 10,
				ClusterMember2: 9,
				ClusterMember3: 7,
			},
		},
		{
			name: "remainingReplicas 2",
			args: args{
				remainingReplicas: 2,
				desiredReplicaInfos: map[string]int64{
					ClusterMember1: 1,
					ClusterMember2: 1,
					ClusterMember3: 1,
				},
				clusterNames: []string{
					ClusterMember1, ClusterMember2, ClusterMember3,
				},
			},
			want: map[string]int64{
				ClusterMember1: 2,
				ClusterMember2: 2,
				ClusterMember3: 1,
			},
		},
		{
			name: "remainingReplicas 0",
			args: args{
				remainingReplicas: 0,
				desiredReplicaInfos: map[string]int64{
					ClusterMember1: 3,
					ClusterMember2: 3,
					ClusterMember3: 3,
				},
				clusterNames: []string{
					ClusterMember1, ClusterMember2, ClusterMember3,
				},
			},
			want: map[string]int64{
				ClusterMember1: 3,
				ClusterMember2: 3,
				ClusterMember3: 3,
			},
		},
	}
	IsTwoMapEqual := func(a, b map[string]int64) bool {
		return a[ClusterMember1] == b[ClusterMember1] && a[ClusterMember2] == b[ClusterMember2] && a[ClusterMember3] == b[ClusterMember3]
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			divideRemainingReplicas(tt.args.remainingReplicas, tt.args.desiredReplicaInfos, tt.args.clusterNames)
			if !IsTwoMapEqual(tt.args.desiredReplicaInfos, tt.want) {
				t.Errorf("divideRemainingReplicas() got = %v, want %v", tt.args.desiredReplicaInfos, tt.want)
			}
		})
	}
}

func Test_divideReplicasByPreference(t *testing.T) {
	type args struct {
		clusterAvailableReplicas []workv1alpha2.TargetCluster
		replicas                 int32
		clustersMaxReplicas      int32
		preference               policyv1alpha1.ReplicaDivisionPreference
		scheduledClusterNames    sets.String
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha2.TargetCluster
		wantErr bool
	}{
		{
			name: "replica 12, dynamic weight 18:12:6",
			args: args{
				clusterAvailableReplicas: TargetClustersList{
					workv1alpha2.TargetCluster{Name: ClusterMember1, Replicas: 18},
					workv1alpha2.TargetCluster{Name: ClusterMember2, Replicas: 12},
					workv1alpha2.TargetCluster{Name: ClusterMember3, Replicas: 6},
				},
				replicas:              12,
				clustersMaxReplicas:   36,
				preference:            policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				scheduledClusterNames: sets.NewString(),
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 6},
				{Name: ClusterMember2, Replicas: 4},
				{Name: ClusterMember3, Replicas: 2},
			},
			wantErr: false,
		},
		{
			name: "replica 12, dynamic weight 20:12:6",
			args: args{
				clusterAvailableReplicas: TargetClustersList{
					workv1alpha2.TargetCluster{Name: ClusterMember1, Replicas: 20},
					workv1alpha2.TargetCluster{Name: ClusterMember2, Replicas: 12},
					workv1alpha2.TargetCluster{Name: ClusterMember3, Replicas: 6},
				},
				replicas:              12,
				clustersMaxReplicas:   38,
				preference:            policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				scheduledClusterNames: sets.NewString(),
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 7},
				{Name: ClusterMember2, Replicas: 4},
				{Name: ClusterMember3, Replicas: 1},
			},
			wantErr: false,
		},
		{
			name: "replica 12, dynamic weight 6:12:6",
			args: args{
				clusterAvailableReplicas: TargetClustersList{
					workv1alpha2.TargetCluster{Name: ClusterMember1, Replicas: 6},
					workv1alpha2.TargetCluster{Name: ClusterMember2, Replicas: 12},
					workv1alpha2.TargetCluster{Name: ClusterMember3, Replicas: 6},
				},
				replicas:              12,
				clustersMaxReplicas:   24,
				preference:            policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				scheduledClusterNames: sets.NewString(),
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 3},
				{Name: ClusterMember2, Replicas: 6},
				{Name: ClusterMember3, Replicas: 3},
			},
			wantErr: false,
		},
		{
			name: "replica 12, aggregated 12:6:6",
			args: args{
				clusterAvailableReplicas: TargetClustersList{
					workv1alpha2.TargetCluster{Name: ClusterMember2, Replicas: 12},
					workv1alpha2.TargetCluster{Name: ClusterMember1, Replicas: 6},
					workv1alpha2.TargetCluster{Name: ClusterMember3, Replicas: 6},
				},
				replicas:              12,
				clustersMaxReplicas:   24,
				preference:            policyv1alpha1.ReplicaDivisionPreferenceAggregated,
				scheduledClusterNames: sets.NewString(),
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember2, Replicas: 12},
			},
			wantErr: false,
		},
		{
			name: "replica 12, aggregated 6:6:6",
			args: args{
				clusterAvailableReplicas: TargetClustersList{
					workv1alpha2.TargetCluster{Name: ClusterMember1, Replicas: 6},
					workv1alpha2.TargetCluster{Name: ClusterMember2, Replicas: 6},
					workv1alpha2.TargetCluster{Name: ClusterMember3, Replicas: 6},
				},
				replicas:              12,
				clustersMaxReplicas:   18,
				preference:            policyv1alpha1.ReplicaDivisionPreferenceAggregated,
				scheduledClusterNames: sets.NewString(),
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 6},
				{Name: ClusterMember2, Replicas: 6},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := divideReplicasByPreference(tt.args.clusterAvailableReplicas, tt.args.replicas, tt.args.preference, tt.args.scheduledClusterNames)
			if (err != nil) != tt.wantErr {
				t.Errorf("divideReplicasByPreference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !helper.IsScheduleResultEqual(got, tt.want) {
				t.Errorf("divideReplicasByPreference() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_divideReplicasByResource(t *testing.T) {
	type args struct {
		clusters   []*clusterv1alpha1.Cluster
		spec       *workv1alpha2.ResourceBindingSpec
		preference policyv1alpha1.ReplicaDivisionPreference
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha2.TargetCluster
		wantErr bool
	}{
		{
			name: "replica 12, dynamic weight 6:8:10",
			args: args{
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
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 12,
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 3},
				{Name: ClusterMember2, Replicas: 4},
				{Name: ClusterMember3, Replicas: 5},
			},
			wantErr: false,
		},
		{
			name: "replica 12, dynamic weight 8:8:10",
			args: args{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(8, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(8, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				},
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 12,
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 4},
				{Name: ClusterMember2, Replicas: 3},
				{Name: ClusterMember3, Replicas: 5},
			},
			wantErr: false,
		},
		{
			name: "replica 12, dynamic weight 3:3:3",
			args: args{
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
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 12,
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			},
			wantErr: true,
		},
		{
			name: "replica 12, aggregated 6:8:10",
			args: args{
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
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 12,
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
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
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 12,
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 12},
			},
			wantErr: false,
		},
		{
			name: "replica 12, aggregated 3:3:3",
			args: args{
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
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 12,
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := divideReplicasByResource(tt.args.clusters, tt.args.spec, tt.args.preference)
			if (err != nil) != tt.wantErr {
				t.Errorf("divideReplicasByResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !helper.IsScheduleResultEqual(got, tt.want) {
				t.Errorf("divideReplicasByResource() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_scaleScheduling(t *testing.T) {
	type args struct {
		clusters   []*clusterv1alpha1.Cluster
		spec       *workv1alpha2.ResourceBindingSpec
		preference policyv1alpha1.ReplicaDivisionPreference
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha2.TargetCluster
		wantErr bool
	}{
		{
			name: "replica 12 -> 6, dynamic weighted 1:1:1",
			args: args{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(1, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(1, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(1, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				},
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 6,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 2},
						{Name: ClusterMember2, Replicas: 4},
						{Name: ClusterMember3, Replicas: 6},
					},
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 1},
				{Name: ClusterMember2, Replicas: 2},
				{Name: ClusterMember3, Replicas: 3},
			},
			wantErr: false,
		},
		{
			name: "replica 12 -> 24, dynamic weighted 10:10:10",
			args: args{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				},
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 24,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 2},
						{Name: ClusterMember2, Replicas: 4},
						{Name: ClusterMember3, Replicas: 6},
					},
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 6},
				{Name: ClusterMember2, Replicas: 8},
				{Name: ClusterMember3, Replicas: 10},
			},
			wantErr: false,
		},
		{
			name: "replica 12 -> 24, dynamic weighted 1:1:1",
			args: args{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(1, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(1, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(1, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				},
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 24,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 2},
						{Name: ClusterMember2, Replicas: 4},
						{Name: ClusterMember3, Replicas: 6},
					},
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
			},
			wantErr: true,
		},
		{
			name: "replica 12 -> 6, aggregated 1:1:1",
			args: args{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(1, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(1, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(1, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				},
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 6,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 4},
						{Name: ClusterMember2, Replicas: 8},
					},
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 2},
				{Name: ClusterMember2, Replicas: 4},
			},
			wantErr: false,
		},
		{
			name: "replica 12 -> 24, aggregated 4:6:8",
			args: args{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(4, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(6, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(8, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				},
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 24,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 4},
						{Name: ClusterMember2, Replicas: 8},
					},
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 6},
				{Name: ClusterMember2, Replicas: 13},
				{Name: ClusterMember3, Replicas: 5},
			},
			wantErr: false,
		},
		{
			name: "replica 12 -> 24, dynamic weighted 1:1:1",
			args: args{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(1, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(1, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(1, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				},
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 24,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 4},
						{Name: ClusterMember2, Replicas: 8},
					},
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
			wantErr: true,
		},
		{
			name: "replica 12 -> 24, aggregated 4:8:12, with cluster2 disappeared and cluster4 appeared",
			args: args{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(4, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(8, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember4, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(12, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				},
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 24,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 4},
						{Name: ClusterMember2, Replicas: 8},
					},
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 8},
				{Name: ClusterMember3, Replicas: 6},
				{Name: ClusterMember4, Replicas: 10},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := divideReplicasByResource(tt.args.clusters, tt.args.spec, tt.args.preference)
			if (err != nil) != tt.wantErr {
				t.Errorf("scaleScheduleByReplicaDivisionPreference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !helper.IsScheduleResultEqual(got, tt.want) {
				t.Errorf("scaleScheduling() got = %v, want %v", got, tt.want)
			}
		})
	}
}
