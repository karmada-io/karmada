package v1alpha2

import (
	"reflect"
	"testing"

	"k8s.io/utils/pointer"
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
				Reason:      EvictionReasonTaintUntolerated,
				Message:     "graceful eviction",
				Producer:    EvictionProducerTaintManager,
			},
			ExpectSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m2", Replicas: 2}, {Name: "m3", Replicas: 3}},
				GracefulEvictionTasks: []GracefulEvictionTask{
					{
						FromCluster: "m1",
						Replicas:    pointer.Int32(1),
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
				Reason:      EvictionReasonTaintUntolerated,
				Message:     "graceful eviction",
				Producer:    EvictionProducerTaintManager,
			},
			ExpectSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m1", Replicas: 1}, {Name: "m3", Replicas: 3}},
				GracefulEvictionTasks: []GracefulEvictionTask{
					{
						FromCluster: "m2",
						Replicas:    pointer.Int32(2),
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
				Reason:      EvictionReasonTaintUntolerated,
				Message:     "graceful eviction",
				Producer:    EvictionProducerTaintManager,
			},
			ExpectSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m1", Replicas: 1}, {Name: "m2", Replicas: 2}},
				GracefulEvictionTasks: []GracefulEvictionTask{
					{
						FromCluster: "m3",
						Replicas:    pointer.Int32(3),
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
						Replicas:    pointer.Int32(3),
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
						Replicas:    pointer.Int32(1),
						Reason:      EvictionReasonTaintUntolerated,
						Message:     "graceful eviction v1",
						Producer:    EvictionProducerTaintManager,
					},
				},
			},
			EvictEvent: GracefulEvictionTask{
				FromCluster: "m1",
				Replicas:    pointer.Int32(1),
				Reason:      EvictionReasonTaintUntolerated,
				Message:     "graceful eviction v2",
				Producer:    EvictionProducerTaintManager,
			},
			ExpectSpec: ResourceBindingSpec{
				Clusters: []TargetCluster{{Name: "m2", Replicas: 2}},
				GracefulEvictionTasks: []GracefulEvictionTask{
					{
						FromCluster: "m1",
						Replicas:    pointer.Int32(1),
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
			tc.InputSpec.GracefulEvictCluster(tc.EvictEvent.FromCluster, NewTaskOptions(WithProducer(tc.EvictEvent.Producer), WithReason(tc.EvictEvent.Reason), WithMessage(tc.EvictEvent.Message)))

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
