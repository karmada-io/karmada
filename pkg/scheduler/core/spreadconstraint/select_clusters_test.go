package spreadconstraint

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// NewClusterWithTopology will build a Cluster with topology.
func NewClusterWithTopology(name, provider, region, zone string) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: clusterv1alpha1.ClusterSpec{
			Provider: provider,
			Region:   region,
			Zone:     zone,
		},
	}
}

func generateClusterInfo() []ClusterDetailInfo {
	return []ClusterDetailInfo{
		{
			Name:              "member4",
			Score:             60,
			AvailableReplicas: 101,
			Cluster:           NewClusterWithTopology("member4", "P2", "R2", "Z2"),
		},
		{
			Name:              "member2",
			Score:             40,
			AvailableReplicas: 101,
			Cluster:           NewClusterWithTopology("member2", "P1", "R1", "Z2"),
		},
		{
			Name:              "member3",
			Score:             30,
			AvailableReplicas: 101,
			Cluster:           NewClusterWithTopology("member3", "P2", "R1", "Z1"),
		},
		{
			Name:              "member1",
			Score:             20,
			AvailableReplicas: 101,
			Cluster:           NewClusterWithTopology("member1", "P1", "R1", "Z1"),
		},
	}
}

func TestSelectBestClusters(t *testing.T) {
	clustetInfos := generateClusterInfo()
	type args struct {
		placement         *policyv1alpha1.Placement
		groupClustersInfo *GroupClustersInfo
	}
	tests := []struct {
		name    string
		args    args
		want    []*clusterv1alpha1.Cluster
		wantErr bool
	}{
		{
			name: "select clusters by cluster score",
			args: args{
				placement: &policyv1alpha1.Placement{
					SpreadConstraints: []policyv1alpha1.SpreadConstraint{
						{
							SpreadByField: policyv1alpha1.SpreadByFieldCluster,
							MaxGroups:     2,
							MinGroups:     1,
						},
					},
				},
				groupClustersInfo: &GroupClustersInfo{
					Clusters: clustetInfos,
				},
			},
			want: []*clusterv1alpha1.Cluster{
				clustetInfos[0].Cluster,
				clustetInfos[1].Cluster,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectBestClusters(tt.args.placement, tt.args.groupClustersInfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("SelectBestClusters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SelectBestClusters() = %v, want %v", got, tt.want)
			}
		})
	}
}
