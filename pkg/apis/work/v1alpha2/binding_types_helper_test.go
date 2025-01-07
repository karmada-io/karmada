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

package v1alpha2

import (
	"reflect"
	"testing"

	"k8s.io/utils/ptr"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func TestResourceBindingSpec_TargetContains(t *testing.T) {
	tests := []struct {
		Name        string
		Spec        ResourceBindingSpec
		ClusterName string
		Expect      bool
	}{
		{
			Name:        "cluster present in target",
			Spec:        ResourceBindingSpec{Clusters: []TargetCluster{{Name: "m1"}, {Name: "m2"}}},
			ClusterName: "m1",
			Expect:      true,
		},
		{
			Name:        "cluster not present in target",
			Spec:        ResourceBindingSpec{Clusters: []TargetCluster{{Name: "m1"}, {Name: "m2"}}},
			ClusterName: "m3",
			Expect:      false,
		},
		{
			Name:        "cluster is empty",
			Spec:        ResourceBindingSpec{Clusters: []TargetCluster{{Name: "m1"}, {Name: "m2"}}},
			ClusterName: "",
			Expect:      false,
		},
		{
			Name:        "target list is empty",
			Spec:        ResourceBindingSpec{Clusters: []TargetCluster{}},
			ClusterName: "m1",
			Expect:      false,
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Spec.TargetContains(tc.ClusterName) != tc.Expect {
				t.Fatalf("expect: %v, but got: %v", tc.Expect, tc.Spec.TargetContains(tc.ClusterName))
			}
		})
	}
}

func TestResourceBindingSpec_AssignedReplicasForCluster(t *testing.T) {
	tests := []struct {
		Name           string
		Spec           ResourceBindingSpec
		ClusterName    string
		ExpectReplicas int32
	}{
		{
			Name:           "returns valid replicas in case cluster present",
			Spec:           ResourceBindingSpec{Clusters: []TargetCluster{{Name: "m1", Replicas: 1}, {Name: "m2", Replicas: 2}}},
			ClusterName:    "m1",
			ExpectReplicas: 1,
		},
		{
			Name:           "returns 0 in case cluster not present",
			Spec:           ResourceBindingSpec{Clusters: []TargetCluster{{Name: "m1", Replicas: 1}, {Name: "m2", Replicas: 2}}},
			ClusterName:    "non-exist",
			ExpectReplicas: 0,
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.Name, func(t *testing.T) {
			got := tc.Spec.AssignedReplicasForCluster(tc.ClusterName)
			if tc.ExpectReplicas != got {
				t.Fatalf("expect: %d, but got: %d", tc.ExpectReplicas, got)
			}
		})
	}
}

func TestResourceBindingSpec_RemoveCluster(t *testing.T) {
	tests := []struct {
		Name        string
		InputSpec   ResourceBindingSpec
		ClusterName string
		ExpectSpec  ResourceBindingSpec
	}{
		{
			Name:        "cluster not exist should do nothing",
			InputSpec:   ResourceBindingSpec{Clusters: []TargetCluster{{Name: "m1"}, {Name: "m2"}, {Name: "m3"}}},
			ClusterName: "no-exist",
			ExpectSpec:  ResourceBindingSpec{Clusters: []TargetCluster{{Name: "m1"}, {Name: "m2"}, {Name: "m3"}}},
		},
		{
			Name:        "remove cluster from head",
			InputSpec:   ResourceBindingSpec{Clusters: []TargetCluster{{Name: "m1"}, {Name: "m2"}, {Name: "m3"}}},
			ClusterName: "m1",
			ExpectSpec:  ResourceBindingSpec{Clusters: []TargetCluster{{Name: "m2"}, {Name: "m3"}}},
		},
		{
			Name:        "remove cluster from middle",
			InputSpec:   ResourceBindingSpec{Clusters: []TargetCluster{{Name: "m1"}, {Name: "m2"}, {Name: "m3"}}},
			ClusterName: "m2",
			ExpectSpec:  ResourceBindingSpec{Clusters: []TargetCluster{{Name: "m1"}, {Name: "m3"}}},
		},
		{
			Name:        "remove cluster from tail",
			InputSpec:   ResourceBindingSpec{Clusters: []TargetCluster{{Name: "m1"}, {Name: "m2"}, {Name: "m3"}}},
			ClusterName: "m3",
			ExpectSpec:  ResourceBindingSpec{Clusters: []TargetCluster{{Name: "m1"}, {Name: "m2"}}},
		},
		{
			Name:        "remove cluster from empty list",
			InputSpec:   ResourceBindingSpec{Clusters: []TargetCluster{}},
			ClusterName: "na",
			ExpectSpec:  ResourceBindingSpec{Clusters: []TargetCluster{}},
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.Name, func(t *testing.T) {
			tc.InputSpec.RemoveCluster(tc.ClusterName)
			if !reflect.DeepEqual(tc.InputSpec.Clusters, tc.ExpectSpec.Clusters) {
				t.Fatalf("expect: %v, but got: %v", tc.ExpectSpec.Clusters, tc.InputSpec.Clusters)
			}
		})
	}
}

func TestResourceBindingSpec_GracefulEvictCluster(t *testing.T) {
	tests := []struct {
		Name       string
		InputSpec  ResourceBindingSpec
		EvictEvent GracefulEvictionTask
		ExpectSpec ResourceBindingSpec
	}{
		{
			Name: "cluster not exist should do nothing",
			InputSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m1"}, {Name: "m2"}, {Name: "m3"}},
			},
			EvictEvent: GracefulEvictionTask{FromCluster: "non-exist"},
			ExpectSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m1"}, {Name: "m2"}, {Name: "m3"}},
			},
		},
		{
			Name: "evict cluster from head",
			InputSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m1", Replicas: 1}, {Name: "m2", Replicas: 2}, {Name: "m3", Replicas: 3}},
			},
			EvictEvent: GracefulEvictionTask{
				FromCluster: "m1",
				PurgeMode:   policyv1alpha1.Immediately,
				Reason:      EvictionReasonTaintUntolerated,
				Message:     "graceful eviction",
				Producer:    EvictionProducerTaintManager,
			},
			ExpectSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m2", Replicas: 2}, {Name: "m3", Replicas: 3}},
				GracefulEvictionTasks: []GracefulEvictionTask{
					{
						FromCluster: "m1",
						PurgeMode:   policyv1alpha1.Immediately,
						Replicas:    ptr.To[int32](1),
						Reason:      EvictionReasonTaintUntolerated,
						Message:     "graceful eviction",
						Producer:    EvictionProducerTaintManager,
					},
				},
			},
		},
		{
			Name: "remove cluster from middle",
			InputSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m1", Replicas: 1}, {Name: "m2", Replicas: 2}, {Name: "m3", Replicas: 3}},
			},
			EvictEvent: GracefulEvictionTask{
				FromCluster: "m2",
				PurgeMode:   policyv1alpha1.Never,
				Reason:      EvictionReasonTaintUntolerated,
				Message:     "graceful eviction",
				Producer:    EvictionProducerTaintManager,
			},
			ExpectSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m1", Replicas: 1}, {Name: "m3", Replicas: 3}},
				GracefulEvictionTasks: []GracefulEvictionTask{
					{
						FromCluster: "m2",
						PurgeMode:   policyv1alpha1.Never,
						Replicas:    ptr.To[int32](2),
						Reason:      EvictionReasonTaintUntolerated,
						Message:     "graceful eviction",
						Producer:    EvictionProducerTaintManager,
					},
				},
			},
		},
		{
			Name: "remove cluster from tail",
			InputSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m1", Replicas: 1}, {Name: "m2", Replicas: 2}, {Name: "m3", Replicas: 3}},
			},
			EvictEvent: GracefulEvictionTask{
				FromCluster: "m3",
				PurgeMode:   policyv1alpha1.Graciously,
				Reason:      EvictionReasonTaintUntolerated,
				Message:     "graceful eviction",
				Producer:    EvictionProducerTaintManager,
			},
			ExpectSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m1", Replicas: 1}, {Name: "m2", Replicas: 2}},
				GracefulEvictionTasks: []GracefulEvictionTask{
					{
						FromCluster: "m3",
						PurgeMode:   policyv1alpha1.Graciously,
						Replicas:    ptr.To[int32](3),
						Reason:      EvictionReasonTaintUntolerated,
						Message:     "graceful eviction",
						Producer:    EvictionProducerTaintManager,
					},
				},
			},
		},
		{
			Name: "eviction task should be appended to non-empty tasks",
			InputSpec: ResourceBindingSpec{
				Clusters:              []TargetCluster{{Name: "m1", Replicas: 1}, {Name: "m2", Replicas: 2}, {Name: "m3", Replicas: 3}},
				GracefulEvictionTasks: []GracefulEvictionTask{{FromCluster: "original-cluster"}},
			},
			EvictEvent: GracefulEvictionTask{
				FromCluster: "m3",
				PurgeMode:   policyv1alpha1.Graciously,
				Reason:      EvictionReasonTaintUntolerated,
				Message:     "graceful eviction",
				Producer:    EvictionProducerTaintManager,
			},
			ExpectSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m1", Replicas: 1}, {Name: "m2", Replicas: 2}},
				GracefulEvictionTasks: []GracefulEvictionTask{
					{
						FromCluster: "original-cluster",
					},
					{
						FromCluster: "m3",
						PurgeMode:   policyv1alpha1.Graciously,
						Replicas:    ptr.To[int32](3),
						Reason:      EvictionReasonTaintUntolerated,
						Message:     "graceful eviction",
						Producer:    EvictionProducerTaintManager,
					},
				},
			},
		},
		{
			Name:       "remove cluster from empty list",
			InputSpec:  ResourceBindingSpec{Clusters: []TargetCluster{}},
			ExpectSpec: ResourceBindingSpec{Clusters: []TargetCluster{}},
		},
		{
			Name: "same eviction task should not be appended multiple times",
			InputSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m1", Replicas: 1}, {Name: "m2", Replicas: 2}},
				GracefulEvictionTasks: []GracefulEvictionTask{
					{
						FromCluster: "m1",
						Replicas:    ptr.To[int32](1),
						Reason:      EvictionReasonTaintUntolerated,
						Message:     "graceful eviction v1",
						Producer:    EvictionProducerTaintManager,
					},
				},
			},
			EvictEvent: GracefulEvictionTask{
				FromCluster: "m1",
				PurgeMode:   policyv1alpha1.Graciously,
				Replicas:    ptr.To[int32](1),
				Reason:      EvictionReasonTaintUntolerated,
				Message:     "graceful eviction v2",
				Producer:    EvictionProducerTaintManager,
			},
			ExpectSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m2", Replicas: 2}},
				GracefulEvictionTasks: []GracefulEvictionTask{
					{
						FromCluster: "m1",
						Replicas:    ptr.To[int32](1),
						Reason:      EvictionReasonTaintUntolerated,
						Message:     "graceful eviction v1",
						Producer:    EvictionProducerTaintManager,
					},
				},
			},
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.Name, func(t *testing.T) {
			tc.InputSpec.GracefulEvictCluster(tc.EvictEvent.FromCluster, NewTaskOptions(
				WithPurgeMode(tc.EvictEvent.PurgeMode),
				WithProducer(tc.EvictEvent.Producer),
				WithReason(tc.EvictEvent.Reason),
				WithMessage(tc.EvictEvent.Message)))

			if !reflect.DeepEqual(tc.InputSpec.Clusters, tc.ExpectSpec.Clusters) {
				t.Fatalf("expect clusters: %v, but got: %v", tc.ExpectSpec.Clusters, tc.InputSpec.Clusters)
			}
			if !reflect.DeepEqual(tc.InputSpec.GracefulEvictionTasks, tc.ExpectSpec.GracefulEvictionTasks) {
				t.Fatalf("expect tasks: %v, but got: %v", tc.ExpectSpec.GracefulEvictionTasks, tc.InputSpec.GracefulEvictionTasks)
			}
		})
	}
}

func TestResourceBindingSpec_ClusterInGracefulEvictionTasks(t *testing.T) {
	gracefulEvictionTasks := []GracefulEvictionTask{
		{
			FromCluster: "member1",
			Producer:    EvictionProducerTaintManager,
			Reason:      EvictionReasonTaintUntolerated,
		},
		{
			FromCluster: "member2",
			Producer:    EvictionProducerTaintManager,
			Reason:      EvictionReasonTaintUntolerated,
		},
	}

	tests := []struct {
		name          string
		InputSpec     ResourceBindingSpec
		targetCluster string
		expect        bool
	}{
		{
			name: "targetCluster is in the process of eviction",
			InputSpec: ResourceBindingSpec{
				GracefulEvictionTasks: gracefulEvictionTasks,
			},
			targetCluster: "member1",
			expect:        true,
		},
		{
			name: "targetCluster is not in the process of eviction",
			InputSpec: ResourceBindingSpec{
				GracefulEvictionTasks: gracefulEvictionTasks,
			},
			targetCluster: "member3",
			expect:        false,
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			result := tc.InputSpec.ClusterInGracefulEvictionTasks(tc.targetCluster)
			if result != tc.expect {
				t.Errorf("expected: %v, but got: %v", tc.expect, result)
			}
		})
	}
}

func TestResourceBindingSpec_SchedulingSuspended(t *testing.T) {
	tests := []struct {
		name      string
		rbSpec    *ResourceBindingSpec
		Suspended bool
	}{
		{
			name:      "nil ResourceBindingSpec results in not suspended",
			rbSpec:    nil,
			Suspended: false,
		},
		{
			name: "nil Suspension results in not suspended",
			rbSpec: &ResourceBindingSpec{
				Suspension: nil,
			},
			Suspended: false,
		},
		{
			name: "nil Scheduling results in not suspended",
			rbSpec: &ResourceBindingSpec{
				Suspension: &Suspension{
					Scheduling: nil,
				},
			},
			Suspended: false,
		},
		{
			name: "false Scheduling results in not suspended",
			rbSpec: &ResourceBindingSpec{
				Suspension: &Suspension{
					Scheduling: ptr.To(false),
				},
			},
			Suspended: false,
		},
		{
			name: "true Scheduling results in suspended",
			rbSpec: &ResourceBindingSpec{
				Suspension: &Suspension{
					Scheduling: ptr.To(true),
				},
			},
			Suspended: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suspended := tc.rbSpec.SchedulingSuspended()
			if suspended != tc.Suspended {
				t.Fatalf("SchedulingSuspended(): expected: %t, but got: %t", tc.Suspended, suspended)
			}
		})
	}
}
