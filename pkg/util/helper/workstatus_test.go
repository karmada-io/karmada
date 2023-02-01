package helper

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestWorksFullyApplied(t *testing.T) {
	type args struct {
		aggregatedStatuses []workv1alpha2.AggregatedStatusItem
		targetClusters     sets.Set[string]
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "no cluster",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
				},
				targetClusters: nil,
			},
			want: false,
		},
		{
			name: "no aggregatedStatuses",
			args: args{
				aggregatedStatuses: nil,
				targetClusters:     sets.New("member1"),
			},
			want: false,
		},
		{
			name: "cluster size is not equal to aggregatedStatuses",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
				},
				targetClusters: sets.New("member1", "member2"),
			},
			want: false,
		},
		{
			name: "aggregatedStatuses is equal to clusterNames and all applied",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
					{
						ClusterName: "member2",
						Applied:     true,
					},
				},
				targetClusters: sets.New("member1", "member2"),
			},
			want: true,
		},
		{
			name: "aggregatedStatuses is equal to clusterNames but not all applied",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
					{
						ClusterName: "member2",
						Applied:     false,
					},
				},
				targetClusters: sets.New("member1", "member2"),
			},
			want: false,
		},
		{
			name: "target clusters not match expected status",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
				},
				targetClusters: sets.New("member2"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := worksFullyApplied(tt.args.aggregatedStatuses, tt.args.targetClusters); got != tt.want {
				t.Errorf("worksFullyApplied() = %v, want %v", got, tt.want)
			}
		})
	}
}
