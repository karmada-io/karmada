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

package spreadconstraint

import (
	"reflect"
	"testing"

	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func TestIsSpreadConstraintExisted(t *testing.T) {
	type args struct {
		spreadConstraints []policyv1alpha1.SpreadConstraint
		field             policyv1alpha1.SpreadFieldValue
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "the specific field is existed in the spread constraints",
			args: args{
				spreadConstraints: []policyv1alpha1.SpreadConstraint{
					{
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
					},
					{
						SpreadByField: policyv1alpha1.SpreadByFieldZone,
					},
				},
				field: policyv1alpha1.SpreadByFieldCluster,
			},
			want: true,
		},
		{
			name: "the specific field is not existed in the spread constraints",
			args: args{
				spreadConstraints: []policyv1alpha1.SpreadConstraint{
					{
						SpreadByField: policyv1alpha1.SpreadByFieldRegion,
					},
					{
						SpreadByField: policyv1alpha1.SpreadByFieldZone,
					},
				},
				field: policyv1alpha1.SpreadByFieldCluster,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSpreadConstraintExisted(tt.args.spreadConstraints, tt.args.field); got != tt.want {
				t.Errorf("IsSpreadConstraintExisted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sortClusters(t *testing.T) {
	tests := []struct {
		name            string
		infos           []ClusterDetailInfo
		want            []ClusterDetailInfo
		compareFunction func(*ClusterDetailInfo, *ClusterDetailInfo) *bool
	}{
		{
			name: "different scores",
			infos: []ClusterDetailInfo{
				{
					Name:  "b",
					Score: 2,
				},
				{
					Name:  "a",
					Score: 1,
				},
			},
			want: []ClusterDetailInfo{
				{
					Name:  "b",
					Score: 2,
				},
				{
					Name:  "a",
					Score: 1,
				},
			},
		},
		{
			name: "same score, default to sort by name",
			infos: []ClusterDetailInfo{
				{
					Name:              "b",
					Score:             1,
					AvailableReplicas: 5,
				},
				{
					Name:              "a",
					Score:             1,
					AvailableReplicas: 10,
				},
			},
			want: []ClusterDetailInfo{
				{
					Name:              "a",
					Score:             1,
					AvailableReplicas: 10,
				},
				{
					Name:              "b",
					Score:             1,
					AvailableReplicas: 5,
				},
			},
		},
		{
			name: "same score, sort by availableReplicas",
			infos: []ClusterDetailInfo{
				{
					Name:              "a",
					Score:             1,
					AvailableReplicas: 5,
				},
				{
					Name:              "b",
					Score:             1,
					AvailableReplicas: 10,
				},
			},
			want: []ClusterDetailInfo{
				{
					Name:              "b",
					Score:             1,
					AvailableReplicas: 10,
				},
				{
					Name:              "a",
					Score:             1,
					AvailableReplicas: 5,
				},
			},
			compareFunction: func(i *ClusterDetailInfo, j *ClusterDetailInfo) *bool {
				if i.AvailableReplicas != j.AvailableReplicas {
					return ptr.To(i.AvailableReplicas > j.AvailableReplicas)
				}
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.compareFunction != nil {
				sortClusters(tt.infos, tt.compareFunction)
			} else {
				sortClusters(tt.infos)
			}
			if !reflect.DeepEqual(tt.infos, tt.want) {
				t.Errorf("sortClusters() = %v, want %v", tt.infos, tt.want)
			}
		})
	}
}
