package core

import (
	"reflect"
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func Test_rescheduleClusterChange(t *testing.T) {
	type args struct {
		candidateClusters       []*clusterv1alpha1.Cluster
		preClustersWithReplicas []workv1alpha2.TargetCluster
	}
	tests := []struct {
		name string
		args args
		want []workv1alpha2.TargetCluster
	}{
		{name: "clusters not change",
			args: args{candidateClusters: []*clusterv1alpha1.Cluster{{ObjectMeta: metav1.ObjectMeta{Name: "c1"}}, {ObjectMeta: metav1.ObjectMeta{Name: "c2"}}},
				preClustersWithReplicas: []workv1alpha2.TargetCluster{{Name: "c1", Replicas: 2}, {Name: "c2", Replicas: 3}},
			},
			want: []workv1alpha2.TargetCluster{{Name: "c1", Replicas: 2}, {Name: "c2", Replicas: 3}},
		},
		{name: "remove one cluster",
			args: args{candidateClusters: []*clusterv1alpha1.Cluster{{ObjectMeta: metav1.ObjectMeta{Name: "c1"}}, {ObjectMeta: metav1.ObjectMeta{Name: "c2"}}, {ObjectMeta: metav1.ObjectMeta{Name: "c3"}}},
				preClustersWithReplicas: []workv1alpha2.TargetCluster{{Name: "c1", Replicas: 3}, {Name: "c2", Replicas: 5}, {Name: "c3", Replicas: 3}, {Name: "c4", Replicas: 10}},
			},
			want: []workv1alpha2.TargetCluster{{Name: "c1", Replicas: 7}, {Name: "c2", Replicas: 8}, {Name: "c3", Replicas: 6}},
		},
		{name: "remove two clusters",
			args: args{candidateClusters: []*clusterv1alpha1.Cluster{{ObjectMeta: metav1.ObjectMeta{Name: "c2"}}, {ObjectMeta: metav1.ObjectMeta{Name: "c3"}}},
				preClustersWithReplicas: []workv1alpha2.TargetCluster{{Name: "c1", Replicas: 3}, {Name: "c2", Replicas: 5}, {Name: "c3", Replicas: 3}, {Name: "c4", Replicas: 10}},
			},
			want: []workv1alpha2.TargetCluster{{Name: "c2", Replicas: 12}, {Name: "c3", Replicas: 9}},
		},
		{name: "add two clusters",
			args: args{candidateClusters: []*clusterv1alpha1.Cluster{{ObjectMeta: metav1.ObjectMeta{Name: "c2"}}, {ObjectMeta: metav1.ObjectMeta{Name: "c3"}}, {ObjectMeta: metav1.ObjectMeta{Name: "c4"}}, {ObjectMeta: metav1.ObjectMeta{Name: "c5"}}},
				preClustersWithReplicas: []workv1alpha2.TargetCluster{{Name: "c2", Replicas: 5}, {Name: "c3", Replicas: 3}},
			},
			want: []workv1alpha2.TargetCluster{{Name: "c2", Replicas: 5}, {Name: "c3", Replicas: 3}},
		},

		{name: "add and remove clusters",
			args: args{candidateClusters: []*clusterv1alpha1.Cluster{{ObjectMeta: metav1.ObjectMeta{Name: "c2"}}, {ObjectMeta: metav1.ObjectMeta{Name: "c3"}}, {ObjectMeta: metav1.ObjectMeta{Name: "c5"}}},
				preClustersWithReplicas: []workv1alpha2.TargetCluster{{Name: "c1", Replicas: 3}, {Name: "c2", Replicas: 5}, {Name: "c3", Replicas: 3}, {Name: "c4", Replicas: 10}},
			},
			want: []workv1alpha2.TargetCluster{{Name: "c2", Replicas: 10}, {Name: "c3", Replicas: 7}, {Name: "c5", Replicas: 4}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rescheduleClusterChange(tt.args.candidateClusters, tt.args.preClustersWithReplicas); !scheduleResultEqual(got, tt.want) {
				t.Errorf("rescheduleClusterChange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func scheduleResultEqual(cand []workv1alpha2.TargetCluster, want []workv1alpha2.TargetCluster) bool {
	if len(cand) != len(want) {
		return false
	}

	sort.SliceStable(cand, func(i, j int) bool {
		return cand[i].Name < cand[j].Name
	})

	sort.SliceStable(want, func(i, j int) bool {
		return want[i].Name < want[j].Name
	})
	return reflect.DeepEqual(cand, want)
}
