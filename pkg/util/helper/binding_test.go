package helper

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestHasScheduledReplica(t *testing.T) {
	tests := []struct {
		name           string
		scheduleResult []workv1alpha2.TargetCluster
		want           bool
	}{
		{
			name: "all targetCluster have replicas",
			scheduleResult: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
				{
					Name:     "bar",
					Replicas: 2,
				},
			},
			want: true,
		},
		{
			name: "a targetCluster has replicas",
			scheduleResult: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
				{
					Name: "bar",
				},
			},
			want: true,
		},
		{
			name: "another targetCluster has replicas",
			scheduleResult: []workv1alpha2.TargetCluster{
				{
					Name: "foo",
				},
				{
					Name:     "bar",
					Replicas: 1,
				},
			},
			want: true,
		},
		{
			name: "not assigned replicas for a cluster",
			scheduleResult: []workv1alpha2.TargetCluster{
				{
					Name: "foo",
				},
				{
					Name: "bar",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasScheduledReplica(tt.scheduleResult); got != tt.want {
				t.Errorf("HasScheduledReplica() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObtainBindingSpecExistingClusters(t *testing.T) {
	tests := []struct {
		name        string
		bindingSpec workv1alpha2.ResourceBindingSpec
		want        sets.String
	}{
		{
			name: "unique cluster name without GracefulEvictionTasks field",
			bindingSpec: workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
					{
						Name:     "member2",
						Replicas: 3,
					},
				},
				RequiredBy: []workv1alpha2.BindingSnapshot{
					{
						Clusters: []workv1alpha2.TargetCluster{
							{
								Name:     "member3",
								Replicas: 2,
							},
						},
					},
				},
			},
			want: sets.NewString("member1", "member2", "member3"),
		},
		{
			name: "all spec fields do not contain duplicate cluster names",
			bindingSpec: workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
					{
						Name:     "member2",
						Replicas: 3,
					},
				},
				RequiredBy: []workv1alpha2.BindingSnapshot{
					{
						Clusters: []workv1alpha2.TargetCluster{
							{
								Name:     "member3",
								Replicas: 2,
							},
						},
					},
				},
				GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "member4",
					},
				},
			},
			want: sets.NewString("member1", "member2", "member3", "member4"),
		},
		{
			name: "duplicate cluster name",
			bindingSpec: workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
					{
						Name:     "member2",
						Replicas: 3,
					},
				},
				RequiredBy: []workv1alpha2.BindingSnapshot{
					{
						Clusters: []workv1alpha2.TargetCluster{
							{
								Name:     "member3",
								Replicas: 2,
							},
						},
					},
				},
				GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "member3",
					},
				},
			},
			want: sets.NewString("member1", "member2", "member3"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ObtainBindingSpecExistingClusters(tt.bindingSpec); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ObtainBindingSpecExistingClusters() = %v, want %v", got, tt.want)
			}
		})
	}
}
