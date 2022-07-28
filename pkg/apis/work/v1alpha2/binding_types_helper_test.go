package v1alpha2

import (
	"reflect"
	"testing"
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
