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

package core

import (
	"reflect"
	"testing"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/core/spreadconstraint"
	"github.com/karmada-io/karmada/test/helper"
)

type testcase struct {
	name     string
	clusters []spreadconstraint.ClusterDetailInfo
	object   workv1alpha2.ResourceBindingSpec
	result   []workv1alpha2.TargetCluster
}

func Test_DistributionOfReplicas(t *testing.T) {
	tests := []testcase{
		{
			name: "replica 3, static weighted 1:1",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 3,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 3, static weighted 1:1:1, change replicas from 3 to 5, before change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 3,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
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
		},
		{
			name: "replica 3, static weighted 1:1:1, change replicas from 3 to 5, after change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 5, // change replicas from 3 to 5
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 7, static weighted 2:1:1:1, change replicas from 7 to 8, before change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 7,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 7, static weighted 2:1:1:1, change replicas from 7 to 8, after change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 8, // change replicas from 7 to 8
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 2,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 9, static weighted 2:1:1:1, change replicas from 9 to 8, before change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 9,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 2,
				},
				{
					Name:     ClusterMember4,
					Replicas: 2,
				},
			},
		},
		{
			name: "replica 9, static weighted 2:1:1:1, change replicas from 9 to 8, after change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 8,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 2,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 6, static weighted 1:1:1:1, change static weighted from 1:1:1:1 to 2:1:1:1, before change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 6,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 6, static weighted 1:1:1:1, change static weighted from 1:1:1:1 to 2:1:1:1, after change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 6,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 2},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
				{
					Name:     ClusterMember2,
					Replicas: 1,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 5, static weighted 1:1:1, add a new cluster and change static weight to 1:1:1:1, before change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 5,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 5, static weighted 1:1:1, add a new cluster and change static weight to 1:1:1:1, after change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 5,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 1,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 6, static weighted 1:1:1:1, remove a cluster and change static weight to 1:1:1, before change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				{Name: ClusterMember4, Cluster: helper.NewCluster(ClusterMember4)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 6,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember4}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 1,
				},
				{
					Name:     ClusterMember4,
					Replicas: 1,
				},
			},
		},
		{
			name: "replica 6, static weighted 1:1:1:1, remove a cluster and change static weight to 1:1:1, after change",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
			},
			object: workv1alpha2.ResourceBindingSpec{
				Replicas: 6,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
						ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
						ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
						WeightPreference: &policyv1alpha1.ClusterPreferences{
							StaticWeightList: []policyv1alpha1.StaticClusterWeight{
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember1}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember2}}, Weight: 1},
								{TargetCluster: policyv1alpha1.ClusterAffinity{ClusterNames: []string{ClusterMember3}}, Weight: 1},
							},
						},
					},
				},
			},
			result: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 2,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var g = &genericScheduler{}
			obj := tt.object

			// 2. schedule basing on previous schedule result
			got, err := g.assignReplicas(tt.clusters, &obj, &workv1alpha2.ResourceBindingStatus{})
			if err != nil {
				t.Errorf("AssignReplicas() error = %v", err)
				return
			}

			// 3. check if schedule result got match to expected
			if !reflect.DeepEqual(got, tt.result) {
				t.Errorf("AssignReplicas() got = %v, wants %v", got, tt.result)
			}
		})
	}
}
