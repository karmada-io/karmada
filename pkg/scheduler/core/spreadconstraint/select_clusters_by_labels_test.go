package spreadconstraint

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func Test_selectBestClustersByClusterLabels(t *testing.T) {
	clusterInfos := generateClusterInfoByLabels()
	type args struct {
		spreadConstraint  []policyv1alpha1.SpreadConstraint
		groupClustersInfo *GroupClustersInfo
		needReplicas      int32
	}
	tests := []struct {
		name    string
		args    args
		want    []*clusterv1alpha1.Cluster
		wantErr bool
	}{
		{
			name: "There are three clusters with foo label at the same time. but select only two clusters",
			args: args{
				spreadConstraint: []policyv1alpha1.SpreadConstraint{
					{
						SpreadByLabel: "foo",
						MaxGroups:     2,
						MinGroups:     2,
					},
				},
				groupClustersInfo: &GroupClustersInfo{
					Clusters: clusterInfos,
				},
				needReplicas: 30,
			},
			want: []*clusterv1alpha1.Cluster{
				clusterInfos[0].Cluster,
				clusterInfos[1].Cluster,
			},
			wantErr: false,
		},
		{
			name: "There is a cluster with two labels at the same time.",
			args: args{
				spreadConstraint: []policyv1alpha1.SpreadConstraint{
					{
						SpreadByLabel: "foo",
						MaxGroups:     2,
						MinGroups:     2,
					},
					{
						SpreadByLabel: "bar",
						MaxGroups:     1,
						MinGroups:     1,
					},
				},
				groupClustersInfo: &GroupClustersInfo{
					Clusters: clusterInfos,
				},
				needReplicas: 30,
			},
			want: []*clusterv1alpha1.Cluster{
				clusterInfos[1].Cluster,
			},
		},
		{
			name: "There are no clusters with three labels at the same time",
			args: args{
				spreadConstraint: []policyv1alpha1.SpreadConstraint{
					{
						SpreadByLabel: "foo",
						MaxGroups:     2,
						MinGroups:     2,
					},
					{
						SpreadByLabel: "bar",
						MaxGroups:     1,
						MinGroups:     1,
					},
					{
						SpreadByLabel: "baz",
						MaxGroups:     1,
						MinGroups:     1,
					},
				},
				groupClustersInfo: &GroupClustersInfo{
					Clusters: clusterInfos,
				},
				needReplicas: 30,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectBestClustersByClusterLabels(tt.args.spreadConstraint, tt.args.groupClustersInfo, tt.args.needReplicas)
			if (err != nil) != tt.wantErr {
				t.Errorf("selectBestClustersByClusterLabels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("selectBestClustersByClusterLabels() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func generateClusterInfoByLabels() []ClusterDetailInfo {
	return []ClusterDetailInfo{
		{
			Name:              "member4",
			Score:             60,
			AvailableReplicas: 50,
			Cluster:           newClusterWithLabels("member4", "P2", "R2", "Z2", map[string]string{"foo": "foo"}),
		},
		{
			Name:              "member2",
			Score:             40,
			AvailableReplicas: 60,
			Cluster:           newClusterWithLabels("member2", "P1", "R1", "Z2", map[string]string{"foo": "foo", "bar": "bar"}),
		},
		{
			Name:              "member3",
			Score:             30,
			AvailableReplicas: 80,
			Cluster:           newClusterWithLabels("member3", "P2", "R1", "Z1", map[string]string{"bar": "bar"}),
		},
		{
			Name:              "member1",
			Score:             20,
			AvailableReplicas: 40,
			Cluster:           newClusterWithLabels("member1", "P1", "R1", "Z1", map[string]string{"foo": "foo", "baz": "baz"}),
		},
	}
}

func newClusterWithLabels(name, provider, region, zone string, labels map[string]string) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec: clusterv1alpha1.ClusterSpec{
			Provider: provider,
			Region:   region,
			Zone:     zone,
		},
	}
}
