package helper

import (
	"testing"

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
