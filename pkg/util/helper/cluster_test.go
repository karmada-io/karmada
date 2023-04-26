package helper

import (
	"testing"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestIsAPIEnabled(t *testing.T) {
	clusterEnablements := []clusterv1alpha1.APIEnablement{
		{
			GroupVersion: "foo.example.io/v1beta1",
			Resources: []clusterv1alpha1.APIResource{
				{
					Kind: "FooA",
				},
				{
					Kind: "FooB",
				},
			},
		},
		{
			GroupVersion: "bar.example.io/v1beta1",
			Resources: []clusterv1alpha1.APIResource{
				{
					Kind: "BarA",
				},
				{
					Kind: "BarB",
				},
			},
		},
	}

	tests := []struct {
		name               string
		enablements        []clusterv1alpha1.APIEnablement
		targetGroupVersion string
		targetKind         string
		expect             bool
	}{
		{
			name:               "group version not enabled",
			enablements:        clusterEnablements,
			targetGroupVersion: "notexist",
			targetKind:         "Dummy",
			expect:             false,
		},
		{
			name:               "kind not enabled",
			enablements:        clusterEnablements,
			targetGroupVersion: "foo.example.io/v1beta1",
			targetKind:         "Dummy",
			expect:             false,
		},
		{
			name:               "enabled resource",
			enablements:        clusterEnablements,
			targetGroupVersion: "bar.example.io/v1beta1",
			targetKind:         "BarA",
			expect:             true,
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			result := IsAPIEnabled(tc.enablements, tc.targetGroupVersion, tc.targetKind)
			if result != tc.expect {
				t.Errorf("expected: %v, but got: %v", tc.expect, result)
			}
		})
	}
}

func TestClusterInGracefulEvictionTasks(t *testing.T) {
	gracefulEvictionTasks := []workv1alpha2.GracefulEvictionTask{
		{
			FromCluster: ClusterMember1,
			Producer:    workv1alpha2.EvictionProducerTaintManager,
			Reason:      workv1alpha2.EvictionReasonTaintUntolerated,
		},
		{
			FromCluster: ClusterMember2,
			Producer:    workv1alpha2.EvictionProducerTaintManager,
			Reason:      workv1alpha2.EvictionReasonTaintUntolerated,
		},
	}

	tests := []struct {
		name                  string
		gracefulEvictionTasks []workv1alpha2.GracefulEvictionTask
		targetCluster         string
		expect                bool
	}{
		{
			name:                  "targetCluster is in the process of eviction",
			gracefulEvictionTasks: gracefulEvictionTasks,
			targetCluster:         ClusterMember1,
			expect:                true,
		},
		{
			name:                  "targetCluster is not in the process of eviction",
			gracefulEvictionTasks: gracefulEvictionTasks,
			targetCluster:         ClusterMember3,
			expect:                false,
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			result := ClusterInGracefulEvictionTasks(tc.gracefulEvictionTasks, tc.targetCluster)
			if result != tc.expect {
				t.Errorf("expected: %v, but got: %v", tc.expect, result)
			}
		})
	}
}
