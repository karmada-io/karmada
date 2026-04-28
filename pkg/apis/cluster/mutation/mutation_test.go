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

package mutation

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
)

func TestMutateCluster(t *testing.T) {
	type args struct {
		cluster *clusterapis.Cluster
	}
	tests := []struct {
		name string
		args args
		fun  func(args) error
	}{
		{
			name: "test mutate cluster Taints",
			args: args{
				cluster: &clusterapis.Cluster{
					Spec: clusterapis.ClusterSpec{
						Taints: []corev1.Taint{
							{
								Key:    "foo",
								Value:  "abc",
								Effect: corev1.TaintEffectNoSchedule,
							},
							{
								Key:    "bar",
								Effect: corev1.TaintEffectNoExecute,
							}}}},
			},
			fun: func(data args) error {
				for i := range data.cluster.Spec.Taints {
					if data.cluster.Spec.Taints[i].Effect == corev1.TaintEffectNoExecute && data.cluster.Spec.Taints[i].TimeAdded == nil {
						return fmt.Errorf("failed to mutate cluster, taints TimeAdded should not be nil")
					}
				}
				return nil
			},
		},
		{
			name: "test mutate cluster Zone",
			args: args{
				cluster: &clusterapis.Cluster{
					Spec: clusterapis.ClusterSpec{
						Zone: "zone1",
					},
				},
			},
			fun: func(data args) error {
				if data.cluster.Spec.Zone != "" && len(data.cluster.Spec.Zones) == 0 {
					return fmt.Errorf("failed to mutate cluster, zones should not be nil")
				}
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MutateCluster(tt.args.cluster)
			if err := tt.fun(tt.args); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestStandardizeClusterResourceModels(t *testing.T) {
	testCases := map[string]struct {
		models         []clusterapis.ResourceModel
		expectedModels []clusterapis.ResourceModel
	}{
		"sort models": {
			models: []clusterapis.ResourceModel{
				{
					Grade: 2,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(2, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(2, resource.DecimalSI),
						},
					},
				},
			},
			expectedModels: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(2, resource.DecimalSI),
						},
					},
				},
				{
					Grade: 2,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(2, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
			},
		},
		"start with 0": {
			models: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(1, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
			},
			expectedModels: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
			},
		},
		"end with MaxInt64": {
			models: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(2, resource.DecimalSI),
						},
					},
				},
			},
			expectedModels: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	for name, testCase := range testCases {
		StandardizeClusterResourceModels(testCase.models)
		if !reflect.DeepEqual(testCase.models, testCase.expectedModels) {
			t.Errorf("expected sorted resource models for %q, but it did not work", name)
			return
		}
	}
}

func TestSetDefaultClusterResourceModels(t *testing.T) {
	type args struct {
		cluster *clusterapis.Cluster
	}
	tests := []struct {
		name       string
		args       args
		wantModels []clusterapis.ResourceModel
	}{
		{
			name: "test set default Cluster",
			args: args{
				cluster: &clusterapis.Cluster{},
			},
			wantModels: []clusterapis.ResourceModel{
				{
					Grade: 0,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(1, resource.DecimalSI),
						},
						{
							Name: corev1.ResourceMemory,
							Min:  *resource.NewQuantity(0, resource.BinarySI),
							Max:  *resource.NewQuantity(4*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(1, resource.DecimalSI),
							Max:  *resource.NewQuantity(2, resource.DecimalSI),
						},
						{
							Name: corev1.ResourceMemory,
							Min:  *resource.NewQuantity(4*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(16*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 2,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(2, resource.DecimalSI),
							Max:  *resource.NewQuantity(4, resource.DecimalSI),
						},
						{
							Name: corev1.ResourceMemory,
							Min:  *resource.NewQuantity(16*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(32*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 3,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(4, resource.DecimalSI),
							Max:  *resource.NewQuantity(8, resource.DecimalSI),
						},
						{
							Name: corev1.ResourceMemory,
							Min:  *resource.NewQuantity(32*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(64*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 4,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(8, resource.DecimalSI),
							Max:  *resource.NewQuantity(16, resource.DecimalSI),
						},
						{
							Name: corev1.ResourceMemory,
							Min:  *resource.NewQuantity(64*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(128*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 5,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(16, resource.DecimalSI),
							Max:  *resource.NewQuantity(32, resource.DecimalSI),
						},
						{
							Name: corev1.ResourceMemory,
							Min:  *resource.NewQuantity(128*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(256*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 6,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(32, resource.DecimalSI),
							Max:  *resource.NewQuantity(64, resource.DecimalSI),
						},
						{
							Name: corev1.ResourceMemory,
							Min:  *resource.NewQuantity(256*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(512*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 7,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(64, resource.DecimalSI),
							Max:  *resource.NewQuantity(128, resource.DecimalSI),
						},
						{
							Name: corev1.ResourceMemory,
							Min:  *resource.NewQuantity(512*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(1024*GB, resource.BinarySI),
						},
					},
				},
				{
					Grade: 8,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewQuantity(128, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
						{
							Name: corev1.ResourceMemory,
							Min:  *resource.NewQuantity(1024*GB, resource.BinarySI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.BinarySI),
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			SetDefaultClusterResourceModels(tt.args.cluster)
		})
		if !reflect.DeepEqual(tt.args.cluster.Spec.ResourceModels, tt.wantModels) {
			t.Errorf("SetDefaultClusterResourceModels expected resourceModels %+v, bud get %+v", tt.wantModels, tt.args.cluster.Spec.ResourceModels)
			return
		}
	}
}
