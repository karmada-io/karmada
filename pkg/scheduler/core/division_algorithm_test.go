/*
Copyright The Karmada Authors.

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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/test/helper"
)

const (
	ClusterMember1 = "member1"
	ClusterMember2 = "member2"
	ClusterMember3 = "member3"
)

func Test_divideReplicasByStaticWeight(t *testing.T) {
	type args struct {
		clusters   []*clusterv1alpha1.Cluster
		weightList []policyv1alpha1.StaticClusterWeight
		replicas   int32
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha2.TargetCluster
		wantErr bool
	}{
		{
			name: "replica 12, weight 3:2:1",
			args: args{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
				weightList: []policyv1alpha1.StaticClusterWeight{
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
				replicas: 12,
			},
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
			args: struct {
				clusters   []*clusterv1alpha1.Cluster
				weightList []policyv1alpha1.StaticClusterWeight
				replicas   int32
			}{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
				replicas: 12,
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
			wantErr: false,
		},
		{
			name: "replica 14, weight 3:2:1",
			args: struct {
				clusters   []*clusterv1alpha1.Cluster
				weightList []policyv1alpha1.StaticClusterWeight
				replicas   int32
			}{
				clusters: []*clusterv1alpha1.Cluster{
					helper.NewCluster(ClusterMember1),
					helper.NewCluster(ClusterMember2),
					helper.NewCluster(ClusterMember3),
				},
				weightList: []policyv1alpha1.StaticClusterWeight{
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
				replicas: 14,
			},
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := divideReplicasByStaticWeight(tt.args.clusters, tt.args.weightList, tt.args.replicas)
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

func Test_divideReplicasByPreference(t *testing.T) {
	type args struct {
		clusterAvailableReplicas []workv1alpha2.TargetCluster
		replicas                 int32
		clustersMaxReplicas      int32
		preference               policyv1alpha1.ReplicaDivisionPreference
		preUsedClustersName      []string
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
				replicas:            12,
				clustersMaxReplicas: 36,
				preference:          policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				preUsedClustersName: nil,
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
				replicas:            12,
				clustersMaxReplicas: 38,
				preference:          policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				preUsedClustersName: nil,
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
				replicas:            12,
				clustersMaxReplicas: 24,
				preference:          policyv1alpha1.ReplicaDivisionPreferenceWeighted,
				preUsedClustersName: nil,
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
				replicas:            12,
				clustersMaxReplicas: 24,
				preference:          policyv1alpha1.ReplicaDivisionPreferenceAggregated,
				preUsedClustersName: nil,
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 0},
				{Name: ClusterMember2, Replicas: 12},
				{Name: ClusterMember3, Replicas: 0},
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
				replicas:            12,
				clustersMaxReplicas: 18,
				preference:          policyv1alpha1.ReplicaDivisionPreferenceAggregated,
				preUsedClustersName: nil,
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 6},
				{Name: ClusterMember2, Replicas: 6},
				{Name: ClusterMember3, Replicas: 0},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := divideReplicasByPreference(tt.args.clusterAvailableReplicas, tt.args.replicas, tt.args.preference, tt.args.preUsedClustersName...)
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
		clusters            []*clusterv1alpha1.Cluster
		spec                *workv1alpha2.ResourceBindingSpec
		preference          policyv1alpha1.ReplicaDivisionPreference
		preUsedClustersName []string
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
				{Name: ClusterMember1, Replicas: 0},
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
				{Name: ClusterMember2, Replicas: 0},
				{Name: ClusterMember3, Replicas: 0},
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
			got, err := divideReplicasByResource(tt.args.clusters, tt.args.spec, tt.args.preference, tt.args.preUsedClustersName...)
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

func Test_scaleScheduleByReplicaDivisionPreference(t *testing.T) {
	type args struct {
		spec                *workv1alpha2.ResourceBindingSpec
		preference          policyv1alpha1.ReplicaDivisionPreference
		preSelectedClusters []*clusterv1alpha1.Cluster
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha2.TargetCluster
		wantErr bool
	}{
		{
			name: "replica 12 -> 6, dynamic weighted 2:4:6",
			args: args{
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
				preSelectedClusters: []*clusterv1alpha1.Cluster{
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
				preSelectedClusters: []*clusterv1alpha1.Cluster{
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
				preSelectedClusters: []*clusterv1alpha1.Cluster{
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
			},
			wantErr: true,
		},
		{
			name: "replica 12 -> 6, aggregated 2:4:6",
			args: args{
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 6,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 4},
						{Name: ClusterMember2, Replicas: 8},
						{Name: ClusterMember3, Replicas: 0},
					},
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
				preSelectedClusters: []*clusterv1alpha1.Cluster{
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
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 2},
				{Name: ClusterMember2, Replicas: 4},
				{Name: ClusterMember3, Replicas: 0},
			},
			wantErr: false,
		},
		{
			name: "replica 12 -> 24, aggregated 4:6:8",
			args: args{
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 24,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 4},
						{Name: ClusterMember2, Replicas: 8},
						{Name: ClusterMember3, Replicas: 0},
					},
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
				preSelectedClusters: []*clusterv1alpha1.Cluster{
					helper.NewClusterWithResource(ClusterMember1, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(4, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember2, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(6, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
					helper.NewClusterWithResource(ClusterMember3, corev1.ResourceList{
						corev1.ResourcePods: *resource.NewQuantity(14, resource.DecimalSI),
					}, util.EmptyResource().ResourceList(), util.EmptyResource().ResourceList()),
				},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 6},
				{Name: ClusterMember2, Replicas: 11},
				{Name: ClusterMember3, Replicas: 7},
			},
			wantErr: false,
		},
		{
			name: "replica 12 -> 24, dynamic weighted 1:1:1",
			args: args{
				spec: &workv1alpha2.ResourceBindingSpec{
					ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
						ResourceRequest: util.EmptyResource().ResourceList(),
					},
					Replicas: 24,
					Clusters: []workv1alpha2.TargetCluster{
						{Name: ClusterMember1, Replicas: 4},
						{Name: ClusterMember2, Replicas: 8},
						{Name: ClusterMember3, Replicas: 0},
					},
				},
				preference: policyv1alpha1.ReplicaDivisionPreferenceAggregated,
				preSelectedClusters: []*clusterv1alpha1.Cluster{
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
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scaleScheduleByReplicaDivisionPreference(tt.args.spec, tt.args.preference, tt.args.preSelectedClusters)
			if (err != nil) != tt.wantErr {
				t.Errorf("scaleScheduleByReplicaDivisionPreference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !helper.IsScheduleResultEqual(got, tt.want) {
				t.Errorf("scaleScheduleByReplicaDivisionPreference() got = %v, want %v", got, tt.want)
			}
		})
	}
}
