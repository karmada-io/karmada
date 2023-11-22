/*
Copyright 2023 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scheduler

import (
	"encoding/json"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
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

func Test_getAffinityIndex(t *testing.T) {
	type args struct {
		affinities   []policyv1alpha1.ClusterAffinityTerm
		observedName string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "empty observedName",
			args: args{
				affinities: []policyv1alpha1.ClusterAffinityTerm{
					{AffinityName: "group1"},
					{AffinityName: "group2"},
					{AffinityName: "group3"},
				},
				observedName: "",
			},
			want: 0,
		},
		{
			name: "observedName can not find in affinities",
			args: args{
				affinities: []policyv1alpha1.ClusterAffinityTerm{
					{AffinityName: "group1"},
					{AffinityName: "group2"},
					{AffinityName: "group3"},
				},
				observedName: "group0",
			},
			want: 0,
		},
		{
			name: "observedName can find in affinities",
			args: args{
				affinities: []policyv1alpha1.ClusterAffinityTerm{
					{AffinityName: "group1"},
					{AffinityName: "group2"},
					{AffinityName: "group3"},
				},
				observedName: "group3",
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getAffinityIndex(tt.args.affinities, tt.args.observedName); got != tt.want {
				t.Errorf("getAffinityIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getConditionByError(t *testing.T) {
	tests := []struct {
		name              string
		err               error
		expectedCondition metav1.Condition
		ignoreErr         bool
	}{
		{
			name:              "no error",
			err:               nil,
			expectedCondition: metav1.Condition{Type: workv1alpha2.Scheduled, Reason: workv1alpha2.BindingReasonSuccess, Status: metav1.ConditionTrue},
			ignoreErr:         true,
		},
		{
			name:              "failed to schedule",
			err:               utilerrors.NewAggregate([]error{errors.New("")}),
			expectedCondition: metav1.Condition{Type: workv1alpha2.Scheduled, Reason: workv1alpha2.BindingReasonSchedulerError, Status: metav1.ConditionFalse},
			ignoreErr:         false,
		},
		{
			name:              "no cluster fit",
			err:               &framework.FitError{},
			expectedCondition: metav1.Condition{Type: workv1alpha2.Scheduled, Reason: workv1alpha2.BindingReasonNoClusterFit, Status: metav1.ConditionFalse},
			ignoreErr:         true,
		},
		{
			name:              "aggregated fit error",
			err:               utilerrors.NewAggregate([]error{&framework.FitError{}, errors.New("")}),
			expectedCondition: metav1.Condition{Type: workv1alpha2.Scheduled, Reason: workv1alpha2.BindingReasonNoClusterFit, Status: metav1.ConditionFalse},
			ignoreErr:         false,
		},
		{
			name:              "unschedulable error",
			err:               &framework.UnschedulableError{},
			expectedCondition: metav1.Condition{Type: workv1alpha2.Scheduled, Reason: workv1alpha2.BindingReasonUnschedulable, Status: metav1.ConditionFalse},
			ignoreErr:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			condition, ignoreErr := getConditionByError(tt.err)
			if condition.Type != tt.expectedCondition.Type ||
				condition.Reason != tt.expectedCondition.Reason ||
				condition.Status != tt.expectedCondition.Status {
				t.Errorf("expected condition: (%s, %s, %s), but got (%s, %s, %s)",
					tt.expectedCondition.Type, tt.expectedCondition.Reason, tt.expectedCondition.Status, condition.Type, condition.Reason, condition.Status)
			}

			if ignoreErr != tt.ignoreErr {
				t.Errorf("expect to ignore error: %v. but got: %v", tt.ignoreErr, ignoreErr)
			}
		})
	}
}
