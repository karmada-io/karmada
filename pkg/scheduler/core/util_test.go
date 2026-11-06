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

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/core/spreadconstraint"
	"github.com/karmada-io/karmada/test/helper"
)

func Test_getDefaultWeightPreference(t *testing.T) {
	tests := []struct {
		name     string
		clusters []spreadconstraint.ClusterDetailInfo
		want     *policyv1alpha1.ClusterPreferences
	}{
		{
			name:     "empty cluster list",
			clusters: []spreadconstraint.ClusterDetailInfo{},
			want: &policyv1alpha1.ClusterPreferences{
				StaticWeightList: []policyv1alpha1.StaticClusterWeight{},
			},
		},
		{
			name: "single cluster",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
			},
			want: &policyv1alpha1.ClusterPreferences{
				StaticWeightList: []policyv1alpha1.StaticClusterWeight{
					{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{ClusterMember1},
						},
						Weight: 1,
					},
				},
			},
		},
		{
			name: "multiple clusters",
			clusters: []spreadconstraint.ClusterDetailInfo{
				{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
				{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
				{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
			},
			want: &policyv1alpha1.ClusterPreferences{
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
							ClusterNames: []string{ClusterMember3},
						},
						Weight: 1,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getDefaultWeightPreference(tt.clusters); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDefaultWeightPreference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_removeZeroReplicasCluster(t *testing.T) {
	tests := []struct {
		name          string
		assignResults []workv1alpha2.TargetCluster
		want          []workv1alpha2.TargetCluster
	}{
		{
			name: "all zero replicas",
			assignResults: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 0},
				{Name: ClusterMember2, Replicas: 0},
			},
			want: []workv1alpha2.TargetCluster{},
		},
		{
			name: "no zero replicas",
			assignResults: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 3},
				{Name: ClusterMember2, Replicas: 5},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 3},
				{Name: ClusterMember2, Replicas: 5},
			},
		},
		{
			name: "mixed zero and non-zero replicas",
			assignResults: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 3},
				{Name: ClusterMember2, Replicas: 0},
				{Name: ClusterMember3, Replicas: 5},
			},
			want: []workv1alpha2.TargetCluster{
				{Name: ClusterMember1, Replicas: 3},
				{Name: ClusterMember3, Replicas: 5},
			},
		},
		{
			name:          "empty input",
			assignResults: []workv1alpha2.TargetCluster{},
			want:          []workv1alpha2.TargetCluster{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeZeroReplicasCluster(tt.assignResults)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("removeZeroReplicasCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_attachZeroReplicasCluster(t *testing.T) {
	type args struct {
		clusters       []spreadconstraint.ClusterDetailInfo
		targetClusters []workv1alpha2.TargetCluster
	}
	tests := []struct {
		name string
		args args
		want []workv1alpha2.TargetCluster
	}{
		{
			name: "clusters: member1,member2,member3, targetClusters:member1,member2",
			args: args{
				clusters: []spreadconstraint.ClusterDetailInfo{
					{Name: ClusterMember1, Cluster: helper.NewCluster(ClusterMember1)},
					{Name: ClusterMember2, Cluster: helper.NewCluster(ClusterMember2)},
					{Name: ClusterMember3, Cluster: helper.NewCluster(ClusterMember3)},
				},
				targetClusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 1,
					},
					{
						Name:     ClusterMember2,
						Replicas: 2,
					},
				},
			},
			want: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 1,
				},
				{
					Name:     ClusterMember2,
					Replicas: 2,
				},
				{
					Name:     ClusterMember3,
					Replicas: 0,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := attachZeroReplicasCluster(tt.args.clusters, tt.args.targetClusters); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attachZeroReplicasCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}
