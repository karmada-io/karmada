package scheduler

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func Test_needConsideredPlacementChanged(t *testing.T) {
	type args struct {
		placement                      policyv1alpha1.Placement
		appliedPlacement               *policyv1alpha1.Placement
		schedulerObservingAffinityName string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "nil appliedPlacement",
			args: args{
				placement: policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
				},
				appliedPlacement:               nil,
				schedulerObservingAffinityName: "",
			},
			want: true,
		},
		{
			name: "same placement and appliedPlacement",
			args: args{
				placement: policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
					ClusterTolerations: []corev1.Toleration{
						{Key: "foo", Operator: corev1.TolerationOpEqual, Value: "foo", Effect: corev1.TaintEffectNoSchedule},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{MinGroups: 1, MaxGroups: 1, SpreadByField: policyv1alpha1.SpreadByFieldCluster},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
					},
				},
				appliedPlacement: &policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
					ClusterTolerations: []corev1.Toleration{
						{Key: "foo", Operator: corev1.TolerationOpEqual, Value: "foo", Effect: corev1.TaintEffectNoSchedule},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{MinGroups: 1, MaxGroups: 1, SpreadByField: policyv1alpha1.SpreadByFieldCluster},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
					},
				},
				schedulerObservingAffinityName: "",
			},
			want: false,
		},
		{
			name: "different ClusterAffinity field in placement and appliedPlacement",
			args: args{
				placement: policyv1alpha1.Placement{
					ClusterAffinity: nil,
					ClusterTolerations: []corev1.Toleration{
						{Key: "foo", Operator: corev1.TolerationOpEqual, Value: "foo", Effect: corev1.TaintEffectNoSchedule},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{MinGroups: 1, MaxGroups: 1, SpreadByField: policyv1alpha1.SpreadByFieldCluster},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
					},
				},
				appliedPlacement: &policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m2"}},
					ClusterTolerations: []corev1.Toleration{
						{Key: "foo", Operator: corev1.TolerationOpEqual, Value: "foo", Effect: corev1.TaintEffectNoSchedule},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{MinGroups: 1, MaxGroups: 1, SpreadByField: policyv1alpha1.SpreadByFieldCluster},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
					},
				},
				schedulerObservingAffinityName: "",
			},
			want: true,
		},
		{
			name: "different ClusterTolerations field in placement and appliedPlacement",
			args: args{
				placement: policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
					ClusterTolerations: []corev1.Toleration{
						{Key: "foo", Operator: corev1.TolerationOpEqual, Value: "foo", Effect: corev1.TaintEffectNoSchedule},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{MinGroups: 1, MaxGroups: 1, SpreadByField: policyv1alpha1.SpreadByFieldCluster},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
					},
				},
				appliedPlacement: &policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
					ClusterTolerations: []corev1.Toleration{
						{Key: "foo", Operator: corev1.TolerationOpExists, Value: "foo", Effect: corev1.TaintEffectNoSchedule},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{MinGroups: 1, MaxGroups: 1, SpreadByField: policyv1alpha1.SpreadByFieldCluster},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
					},
				},
				schedulerObservingAffinityName: "",
			},
			want: true,
		},
		{
			name: "different SpreadConstraints field in placement and appliedPlacement",
			args: args{
				placement: policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
					ClusterTolerations: []corev1.Toleration{
						{Key: "foo", Operator: corev1.TolerationOpEqual, Value: "foo", Effect: corev1.TaintEffectNoSchedule},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{MinGroups: 1, MaxGroups: 1, SpreadByField: policyv1alpha1.SpreadByFieldCluster},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
					},
				},
				appliedPlacement: &policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
					ClusterTolerations: []corev1.Toleration{
						{Key: "foo", Operator: corev1.TolerationOpEqual, Value: "foo", Effect: corev1.TaintEffectNoSchedule},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{MinGroups: 1, MaxGroups: 2, SpreadByField: policyv1alpha1.SpreadByFieldCluster},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
					},
				},
				schedulerObservingAffinityName: "",
			},
			want: true,
		},
		{
			name: "different ReplicaScheduling field in placement and appliedPlacement",
			args: args{
				placement: policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
					ClusterTolerations: []corev1.Toleration{
						{Key: "foo", Operator: corev1.TolerationOpEqual, Value: "foo", Effect: corev1.TaintEffectNoSchedule},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{MinGroups: 1, MaxGroups: 1, SpreadByField: policyv1alpha1.SpreadByFieldCluster},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated,
					},
				},
				appliedPlacement: &policyv1alpha1.Placement{
					ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
					ClusterTolerations: []corev1.Toleration{
						{Key: "foo", Operator: corev1.TolerationOpEqual, Value: "foo", Effect: corev1.TaintEffectNoSchedule},
					},
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{MinGroups: 1, MaxGroups: 1, SpreadByField: policyv1alpha1.SpreadByFieldCluster},
					},
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
					},
				},
				schedulerObservingAffinityName: "",
			},
			want: true,
		},
		{
			name: "different ClusterAffinities field with empty schedulerObservingAffinityName",
			args: args{
				placement: policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
						},
					},
				},
				appliedPlacement:               &policyv1alpha1.Placement{},
				schedulerObservingAffinityName: "",
			},
			want: true,
		},
		{
			name: "update the target ClusterAffinityTerm",
			args: args{
				placement: policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1", "m2"}},
						},
					},
				},
				appliedPlacement: &policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
						},
					},
				},
				schedulerObservingAffinityName: "group1",
			},
			want: true,
		},
		{
			name: "delete the target ClusterAffinityTerm",
			args: args{
				placement: policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group2",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m2"}},
						},
					},
				},
				appliedPlacement: &policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
						},
						{
							AffinityName:    "group2",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m2"}},
						},
					},
				},
				schedulerObservingAffinityName: "group1",
			},
			want: true,
		},
		{
			name: "delete the whole ClusterAffinities",
			args: args{
				placement: policyv1alpha1.Placement{},
				appliedPlacement: &policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
						},
					},
				},
				schedulerObservingAffinityName: "group1",
			},
			want: true,
		},
		{
			name: "add a new ClusterAffinityTerm and don't update the target ClusterAffinityTerm ",
			args: args{
				placement: policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
						},
						{
							AffinityName:    "group2",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1", "m2"}},
						},
					},
				},
				appliedPlacement: &policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
						},
					},
				},
				schedulerObservingAffinityName: "group1",
			},
			want: false,
		},
		{
			name: "update a ClusterAffinityTerm and don't update the target ClusterAffinityTerm ",
			args: args{
				placement: policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
						},
						{
							AffinityName:    "group2",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1", "m2"}},
						},
					},
				},
				appliedPlacement: &policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
						},
						{
							AffinityName:    "group2",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m2"}},
						},
					},
				},
				schedulerObservingAffinityName: "group1",
			},
			want: false,
		},
		{
			name: "delete a ClusterAffinityTerm and don't update the target ClusterAffinityTerm ",
			args: args{
				placement: policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
						},
					},
				},
				appliedPlacement: &policyv1alpha1.Placement{
					ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
						{
							AffinityName:    "group1",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m1"}},
						},
						{
							AffinityName:    "group2",
							ClusterAffinity: policyv1alpha1.ClusterAffinity{ClusterNames: []string{"m2"}},
						},
					},
				},
				schedulerObservingAffinityName: "group1",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var appliedPlacementStr string
			if tt.args.appliedPlacement == nil {
				appliedPlacementStr = ""
			} else {
				placementBytes, err := json.Marshal(*tt.args.appliedPlacement)
				if err != nil {
					t.Errorf("jsom marshal failed: %v", err)
				}
				appliedPlacementStr = string(placementBytes)
			}
			if got := placementChanged(tt.args.placement, appliedPlacementStr, tt.args.schedulerObservingAffinityName); got != tt.want {
				t.Errorf("placementChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}
