package spreadconstraint

import (
	"reflect"
	"testing"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

func TestIsSpreadConstraintExisted(t *testing.T) {
	type args struct {
		spreadConstraints []policyv1alpha1.SpreadConstraint
		field             policyv1alpha1.SpreadFieldValue
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "the specific field is existed in the spread constraints",
			args: args{
				spreadConstraints: []policyv1alpha1.SpreadConstraint{
					{
						SpreadByField: policyv1alpha1.SpreadByFieldCluster,
					},
					{
						SpreadByField: policyv1alpha1.SpreadByFieldZone,
					},
				},
				field: policyv1alpha1.SpreadByFieldCluster,
			},
			want: true,
		},
		{
			name: "the specific field is not existed in the spread constraints",
			args: args{
				spreadConstraints: []policyv1alpha1.SpreadConstraint{
					{
						SpreadByField: policyv1alpha1.SpreadByFieldRegion,
					},
					{
						SpreadByField: policyv1alpha1.SpreadByFieldZone,
					},
				},
				field: policyv1alpha1.SpreadByFieldCluster,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSpreadConstraintExisted(tt.args.spreadConstraints, tt.args.field); got != tt.want {
				t.Errorf("IsSpreadConstraintExisted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sortClusters(t *testing.T) {
	tests := []struct {
		name  string
		infos []ClusterDetailInfo
		want  []ClusterDetailInfo
	}{
		{
			name: "different scores",
			infos: []ClusterDetailInfo{
				{
					Name:  "b",
					Score: 2,
				},
				{
					Name:  "a",
					Score: 1,
				},
			},
			want: []ClusterDetailInfo{
				{
					Name:  "b",
					Score: 2,
				},
				{
					Name:  "a",
					Score: 1,
				},
			},
		},
		{
			name: "same score",
			infos: []ClusterDetailInfo{
				{
					Name:              "b",
					Score:             1,
					AvailableReplicas: 5,
				},
				{
					Name:              "a",
					Score:             1,
					AvailableReplicas: 10,
				},
			},
			want: []ClusterDetailInfo{
				{
					Name:              "a",
					Score:             1,
					AvailableReplicas: 10,
				},
				{
					Name:              "b",
					Score:             1,
					AvailableReplicas: 5,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortClusters(tt.infos)
			if !reflect.DeepEqual(tt.infos, tt.want) {
				t.Errorf("sortClusters() = %v, want %v", tt.infos, tt.want)
			}
		})
	}
}
