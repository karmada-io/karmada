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

package status

import (
	"testing"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func Test_targetClustersEqual(t *testing.T) {
	tests := []struct {
		name string
		a    []workv1alpha2.TargetCluster
		b    []workv1alpha2.TargetCluster
		want bool
	}{
		{
			name: "both empty",
			a:    []workv1alpha2.TargetCluster{},
			b:    []workv1alpha2.TargetCluster{},
			want: true,
		},
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "same clusters same order",
			a: []workv1alpha2.TargetCluster{
				{Name: "cluster-a", Replicas: 1},
				{Name: "cluster-b", Replicas: 2},
			},
			b: []workv1alpha2.TargetCluster{
				{Name: "cluster-a", Replicas: 1},
				{Name: "cluster-b", Replicas: 2},
			},
			want: true,
		},
		{
			name: "same clusters different order",
			a: []workv1alpha2.TargetCluster{
				{Name: "cluster-a", Replicas: 1},
				{Name: "cluster-b", Replicas: 2},
			},
			b: []workv1alpha2.TargetCluster{
				{Name: "cluster-b", Replicas: 2},
				{Name: "cluster-a", Replicas: 1},
			},
			want: true,
		},
		{
			name: "different lengths",
			a: []workv1alpha2.TargetCluster{
				{Name: "cluster-a", Replicas: 1},
			},
			b: []workv1alpha2.TargetCluster{
				{Name: "cluster-a", Replicas: 1},
				{Name: "cluster-b", Replicas: 2},
			},
			want: false,
		},
		{
			name: "same names different replicas",
			a: []workv1alpha2.TargetCluster{
				{Name: "cluster-a", Replicas: 1},
			},
			b: []workv1alpha2.TargetCluster{
				{Name: "cluster-a", Replicas: 2},
			},
			want: false,
		},
		{
			name: "different cluster names",
			a: []workv1alpha2.TargetCluster{
				{Name: "cluster-a", Replicas: 1},
			},
			b: []workv1alpha2.TargetCluster{
				{Name: "cluster-b", Replicas: 1},
			},
			want: false,
		},
		{
			name: "one empty one not",
			a:    []workv1alpha2.TargetCluster{},
			b: []workv1alpha2.TargetCluster{
				{Name: "cluster-a", Replicas: 1},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := targetClustersEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("targetClustersEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
