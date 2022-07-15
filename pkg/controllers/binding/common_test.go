package binding

import (
	"reflect"
	"testing"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func Test_mergeTargetClusters(t *testing.T) {
	tests := []struct {
		name                      string
		targetClusters            []workv1alpha2.TargetCluster
		requiredByBindingSnapshot []workv1alpha2.BindingSnapshot
		want                      []workv1alpha2.TargetCluster
	}{
		{
			name: "the same cluster",
			targetClusters: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
			},
			requiredByBindingSnapshot: []workv1alpha2.BindingSnapshot{
				{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "foo",
							Replicas: 1,
						},
					},
				},
			},
			want: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
			},
		},
		{
			name: "different clusters",
			targetClusters: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
			},
			requiredByBindingSnapshot: []workv1alpha2.BindingSnapshot{
				{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "bar",
							Replicas: 1,
						},
					},
				},
			},
			want: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
				{
					Name:     "bar",
					Replicas: 1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeTargetClusters(tt.targetClusters, tt.requiredByBindingSnapshot); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeTargetClusters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_transScheduleResultToMap(t *testing.T) {
	tests := []struct {
		name           string
		scheduleResult []workv1alpha2.TargetCluster
		want           map[string]int64
	}{
		{
			name: "one cluster",
			scheduleResult: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
			},
			want: map[string]int64{
				"foo": 1,
			},
		},
		{
			name: "different clusters",
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
			want: map[string]int64{
				"foo": 1,
				"bar": 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transScheduleResultToMap(tt.scheduleResult); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("transScheduleResultToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
