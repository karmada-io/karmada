package util

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

const (
	ClusterMember1 = "member1"
	ClusterMember2 = "member2"
)

func TestGetBindingClusterNames(t *testing.T) {
	tests := []struct {
		name     string
		binding  *workv1alpha2.ResourceBinding
		expected []string
	}{
		{
			name: "nil",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-name",
					Namespace: "demo-ns",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{},
				},
				Status: workv1alpha2.ResourceBindingStatus{},
			},
			expected: nil,
		},
		{
			name: "not nil",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-name",
					Namespace: "demo-ns",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name: ClusterMember1,
						},
						{
							Name: ClusterMember2,
						},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{},
			},
			expected: []string{ClusterMember1, ClusterMember2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetBindingClusterNames(&tt.binding.Spec)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("GetBindingClusterNames() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsBindingReplicasChanged(t *testing.T) {
	tests := []struct {
		name        string
		bindingSpec *workv1alpha2.ResourceBindingSpec
		strategy    *policyv1alpha1.ReplicaSchedulingStrategy
		expected    bool
	}{
		{
			name:        "nil strategy",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{},
			strategy:    nil,
			expected:    false,
		},
		{
			name:        "empty strategy",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{},
			strategy:    &policyv1alpha1.ReplicaSchedulingStrategy{},
			expected:    false,
		},
		{
			name: "Duplicated strategy and replicas not changed",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Replicas: 5,
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 5,
					},
					{
						Name:     ClusterMember2,
						Replicas: 5,
					},
				}},
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated},
			expected: false,
		},
		{
			name: "Duplicated strategy and replicas changed",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Replicas: 5,
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 3,
					},
					{
						Name:     ClusterMember2,
						Replicas: 5,
					},
				}},
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated},
			expected: true,
		},
		{
			name: "Divided strategy and replicas not changed",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Replicas: 5,
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 2,
					},
					{
						Name:     ClusterMember2,
						Replicas: 3,
					},
				},
			},
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided},
			expected: false,
		},
		{
			name: "Divided strategy and replicas changed",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Replicas: 5,
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 3,
					},
					{
						Name:     ClusterMember2,
						Replicas: 3,
					},
				},
			},
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.bindingSpec == nil {
				t.FailNow()
			}
			got := IsBindingReplicasChanged(tt.bindingSpec, tt.strategy)
			if got != tt.expected {
				t.Errorf("IsBindingReplicasChanged() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetSumOfReplicas(t *testing.T) {
	tests := []struct {
		name     string
		clusters []workv1alpha2.TargetCluster
		expected int32
	}{
		{
			name:     "empty",
			clusters: []workv1alpha2.TargetCluster{},
			expected: 0,
		},
		{
			name: "not empty",
			clusters: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetSumOfReplicas(tt.clusters)
			if got != tt.expected {
				t.Errorf("GetSumOfReplicas() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConvertToClusterNames(t *testing.T) {
	tests := []struct {
		name     string
		clusters []workv1alpha2.TargetCluster
		expected sets.String
	}{
		{
			name:     "empty",
			clusters: []workv1alpha2.TargetCluster{},
			expected: sets.String{},
		},
		{
			name: "not empty",
			clusters: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
			expected: sets.NewString(ClusterMember1, ClusterMember2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertToClusterNames(tt.clusters)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ConvertToClusterNames() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMergeTargetClusters(t *testing.T) {
	tests := []struct {
		name     string
		old      []workv1alpha2.TargetCluster
		new      []workv1alpha2.TargetCluster
		expected []workv1alpha2.TargetCluster
	}{
		{
			name:     "empty",
			old:      []workv1alpha2.TargetCluster{},
			new:      []workv1alpha2.TargetCluster{},
			expected: []workv1alpha2.TargetCluster{},
		},
		{
			name: "old clusters are empty",
			old:  []workv1alpha2.TargetCluster{},
			new: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
			expected: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
		},
		{
			name: "new clusters are empty",
			old: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
			},
			new: []workv1alpha2.TargetCluster{},
			expected: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
			},
		},
		{
			name: "no cluster with the same name",
			old: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
			},
			new: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
			expected: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
		},
		{
			name: "some clusters have the same name in the old and new clusters",
			old: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
			},
			new: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 4,
				},
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
			expected: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 6,
				},
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeTargetClusters(tt.old, tt.new)
			if !testhelper.IsScheduleResultEqual(got, tt.expected) {
				t.Errorf("MergeTargetClusters() = %v, want %v", got, tt.expected)
			}
		})
	}
}
