/*
Copyright 2022 The Karmada Authors.

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/helper"
)

var (
	dynamicWeightStrategy = &policyv1alpha1.ReplicaSchedulingStrategy{
		ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
		ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
		WeightPreference: &policyv1alpha1.ClusterPreferences{
			DynamicWeight: policyv1alpha1.DynamicWeightByAvailableReplicas,
		},
	}
	aggregatedStrategy = &policyv1alpha1.ReplicaSchedulingStrategy{
		ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
		ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
	}
)

func Test_assignByStaticWeightStrategy(t *testing.T) {
	tests := []struct {
		name             string
		clusters         []*clusterv1alpha1.Cluster
		weightPreference *policyv1alpha1.ClusterPreferences
		replicas         int32
		want             []workv1alpha2.TargetCluster
		wantErr          bool
	}{
		{
			name: "replica 12, weight 3:2:1",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
				helper.NewCluster(ClusterMember3),
			},
			weightPreference: &policyv1alpha1.ClusterPreferences{
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
			replicas: 12,
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 6,
				},
				{
					Name:     ClusterMember2,
					Replicas: 4,
				},
				{
					Name:     ClusterMember3,
					Replicas: 2,
				},
			},
			wantErr: false,
		},
		{
			name: "replica 12, default weight",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
				helper.NewCluster(ClusterMember3),
			},
			replicas: 12,
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
			wantErr: false,
		},
		{
			name: "replica 14, weight 3:2:1",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
				helper.NewCluster(ClusterMember3),
			},
			weightPreference: &policyv1alpha1.ClusterPreferences{
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
			replicas: 14,
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 8,
				},
				{
					Name:     ClusterMember2,
					Replicas: 4,
				},
				{
					Name:     ClusterMember3,
					Replicas: 2,
				},
			},
			wantErr: false,
		},
		{
			name: "insufficient replica assignment should get 0 replica",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
			},
			weightPreference: &policyv1alpha1.ClusterPreferences{
				StaticWeightList: []policyv1alpha1.StaticClusterWeight{
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember1},
						},
						Weight: 1,
					},
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember2},
						},
						Weight: 1,
					},
				},
			},
			replicas: 0,
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 0,
				},
				{
					Name:     ClusterMember2,
					Replicas: 0,
				},
			},
			wantErr: false,
		},
		{
			name: "selected cluster without weight should be ignored",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
			},
			weightPreference: &policyv1alpha1.ClusterPreferences{
				StaticWeightList: []policyv1alpha1.StaticClusterWeight{
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember1},
						},
						Weight: 1,
					},
				},
			},
			replicas: 2,
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
			},
			wantErr: false,
		},
		{
			name: "cluster with multiple weights",
			clusters: []*clusterv1alpha1.Cluster{
				helper.NewCluster(ClusterMember1),
				helper.NewCluster(ClusterMember2),
			},
			weightPreference: &policyv1alpha1.ClusterPreferences{
				StaticWeightList: []policyv1alpha1.StaticClusterWeight{
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember1},
						},
						Weight: 1,
					},
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember2},
						},
						Weight: 1,
					},
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember1},
						},
						Weight: 2,
					},
				},
			},
			replicas: 3,
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 1,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assignByStaticWeightStrategy(&assignState{
				candidates: tt.clusters,
				strategy: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					WeightPreference:          tt.weightPreference,
				},
				spec: &workv1alpha2.ResourceBindingSpec{Replicas: tt.replicas},
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("divideReplicasByStaticWeight() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !helper.IsScheduleResultEqual(got, tt.want) {
				t.Errorf("divideReplicasByStaticWeight() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_dynamicScale(t *testing.T) {
	tests := []struct {
		name       string
		candidates []*clusterv1alpha1.Cluster
		object     *workv1alpha2.ResourceBindingSpec
		want       []workv1alpha2.TargetCluster
		wantErr    bool
	}{
		{
			name: "replica 12 -> 6, dynamic weighted 1:1:1",
			candidates: []*clusterv1alpha1.Cluster{
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
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 6,
				Clusters: []workv1alpha2.TargetCluster{
					{Name: ClusterMember1, Replicas: 2},
					{Name: ClusterMember2, Replicas: 4},
					{Name: ClusterMember3, Replicas: 6},
				},
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: dynamicWeightStrategy,
				},
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
			candidates: []*clusterv1alpha1.Cluster{
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
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 24,
				Clusters: []workv1alpha2.TargetCluster{
					{Name: ClusterMember1, Replicas: 2},
					{Name: ClusterMember2, Replicas: 4},
					{Name: ClusterMember3, Replicas: 6},
				},
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: dynamicWeightStrategy,
				},
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
			candidates: []*clusterv1alpha1.Cluster{
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
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 24,
				Clusters: []workv1alpha2.TargetCluster{
					{Name: ClusterMember1, Replicas: 2},
					{Name: ClusterMember2, Replicas: 4},
					{Name: ClusterMember3, Replicas: 6},
				},
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: dynamicWeightStrategy,
				},
			},
			wantErr: true,
		},
		{
			name: "replica 12 -> 6, aggregated 1:1:1",
			candidates: []*clusterv1alpha1.Cluster{
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
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 6,
				Clusters: []workv1alpha2.TargetCluster{
					{Name: ClusterMember1, Replicas: 4},
					{Name: ClusterMember2, Replicas: 8},
				},
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: aggregatedStrategy,
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember2, Replicas: 6},
			},
			wantErr: false,
		},
		{
			name: "replica 12 -> 8, aggregated 100:100",
			candidates: []*clusterv1alpha1.Cluster{
				helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
					corev1.ResourcePods: *resource.NewQuantity(100, resource.DecimalSI),
				}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
					corev1.ResourcePods: *resource.NewQuantity(100, resource.DecimalSI),
				}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
			},
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 8,
				Clusters: []workv1alpha2.TargetCluster{
					{Name: ClusterMember1, Replicas: 4},
					{Name: ClusterMember2, Replicas: 8},
				},
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: aggregatedStrategy,
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember2, Replicas: 8},
			},
			wantErr: false,
		},
		{
			name: "replica 12 -> 24, aggregated 4:6:8",
			candidates: []*clusterv1alpha1.Cluster{
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
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 24,
				Clusters: []workv1alpha2.TargetCluster{
					{Name: ClusterMember1, Replicas: 4},
					{Name: ClusterMember2, Replicas: 8},
				},
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: aggregatedStrategy,
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 6},
				{Name: ClusterMember2, Replicas: 12},
				{Name: ClusterMember3, Replicas: 6},
			},
			wantErr: false,
		},
		{
			name: "replica 12 -> 24, aggregated 6:6:20",
			candidates: []*clusterv1alpha1.Cluster{
				helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
					corev1.ResourcePods: *resource.NewQuantity(6, resource.DecimalSI),
				}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
					corev1.ResourcePods: *resource.NewQuantity(6, resource.DecimalSI),
				}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
					corev1.ResourcePods: *resource.NewQuantity(20, resource.DecimalSI),
				}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
			},
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 24,
				Clusters: []workv1alpha2.TargetCluster{
					{Name: ClusterMember1, Replicas: 4},
					{Name: ClusterMember2, Replicas: 8},
				},
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: aggregatedStrategy,
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 10},
				{Name: ClusterMember2, Replicas: 14},
			},
			wantErr: false,
		},
		{
			name: "replica 12 -> 24, aggregated 1:1:1",
			candidates: []*clusterv1alpha1.Cluster{
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
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 24,
				Clusters: []workv1alpha2.TargetCluster{
					{Name: ClusterMember1, Replicas: 4},
					{Name: ClusterMember2, Replicas: 8},
				},
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: aggregatedStrategy,
				},
			},
			wantErr: true,
		},
		{
			name: "replica 12 -> 24, aggregated 4:8:12, with cluster2 disappeared and cluster4 appeared",
			candidates: []*clusterv1alpha1.Cluster{
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
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 24,
				Clusters: []workv1alpha2.TargetCluster{
					{Name: ClusterMember1, Replicas: 4},
					{Name: ClusterMember2, Replicas: 8},
				},
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: aggregatedStrategy,
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 7},
				{Name: ClusterMember3, Replicas: 6},
				{Name: ClusterMember4, Replicas: 11},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := newAssignState(tt.candidates, tt.object, &workv1alpha2.ResourceBindingStatus{})
			got, err := assignByDynamicStrategy(state)
			if (err != nil) != tt.wantErr {
				t.Errorf("assignByDynamicStrategy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !helper.IsScheduleResultEqual(got, tt.want) {
				t.Errorf("assignByDynamicStrategy() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_dynamicScaleUp(t *testing.T) {
	tests := []struct {
		name       string
		candidates []*clusterv1alpha1.Cluster
		object     *workv1alpha2.ResourceBindingSpec
		// wants specifies multi possible desired result, any one got is expected
		wants   [][]workv1alpha2.TargetCluster
		wantErr bool
	}{
		{
			name: "replica 12, dynamic weight 6:8:10",
			candidates: []*clusterv1alpha1.Cluster{
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
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 12,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: dynamicWeightStrategy,
				},
			},
			wants: [][]workv1alpha2.TargetCluster{
				{
					{Name: ClusterMember1, Replicas: 3},
					{Name: ClusterMember2, Replicas: 4},
					{Name: ClusterMember3, Replicas: 5},
				},
			},
			wantErr: false,
		},
		{
			name: "replica 12, dynamic weight 8:8:10",
			candidates: []*clusterv1alpha1.Cluster{
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
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 12,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: dynamicWeightStrategy,
				},
			},
			wants: [][]workv1alpha2.TargetCluster{
				{
					{Name: ClusterMember1, Replicas: 4},
					{Name: ClusterMember2, Replicas: 3},
					{Name: ClusterMember3, Replicas: 5},
				},
				{
					{Name: ClusterMember1, Replicas: 3},
					{Name: ClusterMember2, Replicas: 4},
					{Name: ClusterMember3, Replicas: 5},
				},
			},
			wantErr: false,
		},
		{
			name: "replica 12, dynamic weight 3:3:3",
			candidates: []*clusterv1alpha1.Cluster{
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
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 12,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: dynamicWeightStrategy,
				},
			},
			wantErr: true,
		},
		{
			name: "replica 12, aggregated 6:8:10",
			candidates: []*clusterv1alpha1.Cluster{
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
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 12,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: aggregatedStrategy,
				},
			},
			wants: [][]workv1alpha2.TargetCluster{
				{
					{Name: ClusterMember2, Replicas: 5},
					{Name: ClusterMember3, Replicas: 7},
				},
			},
			wantErr: false,
		},
		{
			name: "replica 12, aggregated 12:8:10",
			candidates: []*clusterv1alpha1.Cluster{
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
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 12,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: aggregatedStrategy,
				},
			},
			wants: [][]workv1alpha2.TargetCluster{
				{
					{Name: ClusterMember1, Replicas: 12},
				},
			},
			wantErr: false,
		},
		{
			name: "replica 12, aggregated 3:3:3",
			candidates: []*clusterv1alpha1.Cluster{
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
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Replicas: 12,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: aggregatedStrategy,
				},
			},
			wantErr: true,
		},
		{
			name: "replica 12, dynamic weight 3:3, with cluster3 disappeared and cluster2 appeared",
			candidates: []*clusterv1alpha1.Cluster{
				helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
					corev1.ResourcePods: *resource.NewQuantity(3, resource.DecimalSI),
				}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
					corev1.ResourcePods: *resource.NewQuantity(3, resource.DecimalSI),
				}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
			},
			object: &workv1alpha2.ResourceBindingSpec{
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: util.EmptyResource().ResourceList(),
				},
				Clusters: []workv1alpha2.TargetCluster{
					{Name: ClusterMember1, Replicas: 6},
					{Name: ClusterMember3, Replicas: 6},
				},
				Replicas: 12,
				Placement: &policyv1alpha1.Placement{
					ReplicaScheduling: dynamicWeightStrategy,
				},
			},
			wants: [][]workv1alpha2.TargetCluster{
				{
					{Name: ClusterMember1, Replicas: 9},
					{Name: ClusterMember2, Replicas: 3},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := newAssignState(tt.candidates, tt.object, &workv1alpha2.ResourceBindingStatus{})
			state.buildScheduledClusters()
			got, err := dynamicScaleUp(state)
			if (err != nil) != tt.wantErr {
				t.Errorf("dynamicScaleUp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for _, want := range tt.wants {
				if helper.IsScheduleResultEqual(got, want) {
					return
				}
			}
			t.Errorf("dynamicScaleUp() got = %v, wants %v", got, tt.wants)
		})
	}
}

func Test_assignByDuplicatedStrategy(t *testing.T) {
	tests := []struct {
		name    string
		state   *assignState
		want    []workv1alpha2.TargetCluster
		wantErr bool
	}{
		{
			name: "test with multiple cluster",
			state: &assignState{
				candidates: []*clusterv1alpha1.Cluster{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster2"},
					},
				},
				spec: &workv1alpha2.ResourceBindingSpec{Replicas: 2},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 2},
				{Name: "cluster2", Replicas: 2},
			},
			wantErr: false,
		},
		{
			name: "the target cluster is null",
			state: &assignState{
				candidates: []*clusterv1alpha1.Cluster{},
				spec:       &workv1alpha2.ResourceBindingSpec{Replicas: 2},
			},
			want:    []workv1alpha2.TargetCluster{},
			wantErr: false,
		},
		{
			name: "replicas is null",
			state: &assignState{
				candidates: []*clusterv1alpha1.Cluster{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster2"},
					},
				},
				spec: &workv1alpha2.ResourceBindingSpec{},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 0},
				{Name: "cluster2", Replicas: 0},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assignByDuplicatedStrategy(tt.state)
			if (err != nil) != tt.wantErr {
				t.Errorf("assignByDuplicatedStrategy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("assignByDuplicatedStrategy() = %v, want %v", got, tt.want)
			}
		})
	}
}
